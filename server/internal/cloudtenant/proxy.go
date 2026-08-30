package cloudtenant

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

var (
	cloudDeviceProxyCookieJars = make(map[string]*cookiejar.Jar)
	cloudDeviceProxyCookieMu   sync.Mutex

	cloudProxyTransport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
)

func (h *APIHandler) handleDeviceProxy(c *fiber.Ctx) error {
	targetIP := c.Params("target")
	if targetIP == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Target IP required")
	}

	c.Cookie(&fiber.Cookie{
		Name:     "last_proxy_target",
		Value:    targetIP,
		Path:     "/",
		HTTPOnly: true,
	})

	if !strings.HasSuffix(c.Path(), "/") && c.Params("*") == "" {
		return c.Redirect(c.Path() + "/")
	}

	cleanIP := targetIP
	if strings.Contains(targetIP, ":") {
		cleanIP, _, _ = net.SplitHostPort(targetIP)
	}

	if net.ParseIP(cleanIP) == nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid Target IP: " + cleanIP)
	}

	targetPath := strings.TrimPrefix(c.Path(), "/proxy/"+targetIP)
	if targetPath == "" {
		targetPath = "/"
	}
	targetURL := detectCloudProxyTargetURL(cleanIP, cloudProxyTransport)

	return adaptor.HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveCloudDeviceProxy(w, r, cleanIP, targetURL, targetPath, cloudProxyTransport)
	}))(c)
}

func detectCloudProxyTargetURL(cleanIP string, transport *http.Transport) *url.URL {
	if cloudProxyTargetResponds("https://"+cleanIP+"/", transport) {
		targetURL, _ := url.Parse("https://" + cleanIP)
		return targetURL
	}

	targetURL, _ := url.Parse("http://" + cleanIP)
	return targetURL
}

func cloudProxyTargetResponds(rawURL string, transport *http.Transport) bool {
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest(http.MethodHead, rawURL, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return true
}

func serveCloudDeviceProxy(w http.ResponseWriter, r *http.Request, cleanIP string, targetURL *url.URL, targetPath string, transport *http.Transport) {
	backendURL := *targetURL
	backendURL.Path = targetPath
	backendURL.RawQuery = r.URL.RawQuery

	var sourceBody []byte
	if r.Body != nil {
		var err error
		sourceBody, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Device proxy request read failed: "+err.Error(), http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()
	}

	resp, err := cloudDeviceProxyDoRequest(r, sourceBody, cleanIP, &backendURL, transport, 0)
	if err != nil {
		renderFriendlyProxyError(w, cleanIP, err)
		return
	}
	defer resp.Body.Close()

	for k, values := range resp.Header {
		if strings.EqualFold(k, "Set-Cookie") ||
			strings.EqualFold(k, "Content-Length") ||
			strings.EqualFold(k, "X-Frame-Options") ||
			strings.EqualFold(k, "Content-Security-Policy") {
			continue
		}
		for _, v := range values {
			w.Header().Set(k, v)
		}
	}

	if loc := resp.Header.Get("Location"); loc != "" {
		if newLoc := rewriteCloudDeviceProxyLocation(loc, cleanIP); newLoc != "" {
			w.Header().Set("Location", newLoc)
		}
	}

	statusCode := resp.StatusCode
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Device proxy read failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	isHTML := strings.Contains(contentType, "text/html")
	isCSS := strings.Contains(contentType, "text/css")
	isJS := strings.Contains(contentType, "javascript")
	isEncoded := resp.Header.Get("Content-Encoding") != ""

	if !isEncoded {
		if isHTML {
			body = []byte(rewriteCloudDeviceProxyHTML(string(body), cleanIP))
		} else if isCSS || isJS {
			body = []byte(rewriteCloudDeviceProxyAssets(string(body), cleanIP))
		}
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

func rewriteCloudDeviceProxyAssets(body string, cleanIP string) string {
	prefix := "/proxy/" + cleanIP
	body = strings.ReplaceAll(body, `url(/`, `url(`+prefix+`/`)
	body = strings.ReplaceAll(body, `url("/`, `url("`+prefix+`/`)
	body = strings.ReplaceAll(body, `url('/`, `url('`+prefix+`/`)
	return body
}

func cloudDeviceProxyDoRequest(source *http.Request, sourceBody []byte, cleanIP string, backendURL *url.URL, transport *http.Transport, depth int) (*http.Response, error) {
	if depth > 8 {
		return nil, fmt.Errorf("too many device redirects")
	}

	var body io.Reader
	if len(sourceBody) > 0 {
		body = bytes.NewReader(sourceBody)
	}

	req, err := http.NewRequest(source.Method, backendURL.String(), body)
	if err != nil {
		return nil, err
	}
	copyCloudProxyRequestHeaders(req.Header, source.Header)
	req.Header.Del("Upgrade-Insecure-Requests")
	req.Header.Del("Accept-Encoding")
	req.Header.Del("Cookie")
	req.Header.Set("Connection", "close")
	req.Host = backendURL.Host
	for _, cookie := range cloudDeviceProxyCookies(cleanIP, backendURL) {
		req.AddCookie(cookie)
	}

	client := &http.Client{
		Timeout:   20 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	cloudDeviceProxyStoreCookies(cleanIP, backendURL, resp.Cookies())

	if isCloudDeviceCookieRedirect(resp) {
		loc := resp.Header.Get("Location")
		_ = resp.Body.Close()
		nextURL := resolveCloudDeviceLocation(backendURL, loc)
		return cloudDeviceProxyDoRequest(source, sourceBody, cleanIP, nextURL, transport, depth+1)
	}

	return resp, nil
}

func copyCloudProxyRequestHeaders(dst http.Header, src http.Header) {
	for k, values := range src {
		if strings.EqualFold(k, "Host") ||
			strings.EqualFold(k, "Connection") ||
			strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range values {
			dst.Add(k, v)
		}
	}
}

func isCloudDeviceCookieRedirect(resp *http.Response) bool {
	if resp.StatusCode < 300 || resp.StatusCode > 399 {
		return false
	}
	loc := strings.ToLower(resp.Header.Get("Location"))
	return strings.Contains(loc, "cookiechecker") || strings.Contains(loc, "login.cgi") || loc == "/" || strings.HasSuffix(loc, "/")
}

func resolveCloudDeviceLocation(base *url.URL, loc string) *url.URL {
	u, err := url.Parse(loc)
	if err != nil {
		next := *base
		return &next
	}
	return base.ResolveReference(u)
}

func cloudDeviceProxyJar(cleanIP string) *cookiejar.Jar {
	cloudDeviceProxyCookieMu.Lock()
	defer cloudDeviceProxyCookieMu.Unlock()

	if jar, ok := cloudDeviceProxyCookieJars[cleanIP]; ok {
		return jar
	}
	jar, _ := cookiejar.New(nil)
	cloudDeviceProxyCookieJars[cleanIP] = jar
	return jar
}

func cloudDeviceProxyCookies(cleanIP string, backendURL *url.URL) []*http.Cookie {
	return cloudDeviceProxyJar(cleanIP).Cookies(backendURL)
}

func cloudDeviceProxyStoreCookies(cleanIP string, backendURL *url.URL, cookies []*http.Cookie) {
	if len(cookies) == 0 {
		return
	}
	cloudDeviceProxyJar(cleanIP).SetCookies(backendURL, cookies)
}

func rewriteCloudDeviceProxyLocation(loc string, cleanIP string) string {
	u, err := url.Parse(loc)
	if err != nil {
		return ""
	}

	if u.IsAbs() && u.Hostname() != cleanIP {
		return ""
	}

	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	newLoc := "/proxy/" + cleanIP + path
	if u.RawQuery != "" {
		newLoc += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		newLoc += "#" + u.EscapedFragment()
	}
	return newLoc
}

func rewriteCloudDeviceProxyHTML(body string, cleanIP string) string {
	prefix := "/proxy/" + cleanIP
	replacer := strings.NewReplacer(
		`href="/`, `href="`+prefix+`/`,
		`src="/`, `src="`+prefix+`/`,
		`action="/`, `action="`+prefix+`/`,
		`formaction="/`, `formaction="`+prefix+`/`,
		`href='/`, `href='`+prefix+`/`,
		`src='/`, `src='`+prefix+`/`,
		`action='/`, `action='`+prefix+`/`,
		`formaction='/`, `formaction='`+prefix+`/`,
		`url(/`, `url(`+prefix+`/`,
		`url("/`, `url("`+prefix+`/`,
		`url('/`, `url('`+prefix+`/`,
		`location="/`, `location="`+prefix+`/`,
		`location='/`, `location='`+prefix+`/`,
		`location.href="/`, `location.href="`+prefix+`/`,
		`location.href='/`, `location.href='`+prefix+`/`,
	)
	return replacer.Replace(body)
}

func renderFriendlyProxyError(w http.ResponseWriter, cleanIP string, err error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	html := fmt.Sprintf(`<!DOCTYPE html>
<html dir="rtl" lang="ar">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>تعذر فتح واجهة الجهاز</title>
<link href="https://fonts.googleapis.com/css2?family=Tajawal:wght@400;600;700;800&display=swap" rel="stylesheet">
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
<style>
  body {
    font-family: 'Tajawal', sans-serif;
    background: #0f172a;
    color: #f8fafc;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    margin: 0;
    padding: 20px;
    box-sizing: border-box;
  }
  .card {
    background: rgba(30, 41, 59, 0.85);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    padding: 35px 30px;
    max-width: 540px;
    width: 100%%;
    text-align: center;
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
  }
  .icon-wrapper {
    width: 72px;
    height: 72px;
    background: rgba(239, 68, 68, 0.15);
    border: 2px solid rgba(239, 68, 68, 0.4);
    border-radius: 50%%;
    display: flex;
    align-items: center;
    justify-content: center;
    margin: 0 auto 20px;
    color: #ef4444;
    font-size: 32px;
  }
  h2 {
    font-size: 20px;
    font-weight: 700;
    margin: 0 0 10px;
    color: #ffffff;
  }
  .ip-tag {
    display: inline-block;
    background: #1e293b;
    border: 1px solid #334155;
    padding: 4px 14px;
    border-radius: 20px;
    font-family: monospace;
    font-size: 15px;
    color: #38bdf8;
    margin-bottom: 20px;
    font-weight: bold;
    direction: ltr;
  }
  .reasons {
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 12px;
    padding: 16px;
    text-align: right;
    font-size: 13.5px;
    line-height: 1.8;
    color: #94a3b8;
    margin-bottom: 25px;
  }
  .reasons li {
    margin-bottom: 6px;
  }
  .btn-group {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    justify-content: center;
  }
  .btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 10px 18px;
    border-radius: 8px;
    font-family: inherit;
    font-size: 13.5px;
    font-weight: 600;
    cursor: pointer;
    text-decoration: none;
    transition: all 0.2s ease;
    border: none;
  }
  .btn-primary {
    background: #3b82f6;
    color: #ffffff;
  }
  .btn-primary:hover {
    background: #2563eb;
    transform: translateY(-2px);
  }
  .btn-secondary {
    background: #334155;
    color: #f1f5f9;
  }
  .btn-secondary:hover {
    background: #475569;
  }
  .err-details {
    font-size: 11px;
    color: #64748b;
    font-family: monospace;
    margin-top: 20px;
    direction: ltr;
  }
</style>
</head>
<body>
<div class="card">
  <div class="icon-wrapper">
    <i class="fa-solid fa-satellite-dish"></i>
  </div>
  <h2>تعذر الوصول لواجهة الجهاز</h2>
  <div class="ip-tag">%s</div>
  <div class="reasons">
    <div style="font-weight: 700; color: #cbd5e1; margin-bottom: 8px;"><i class="fa-solid fa-circle-info" style="color: #38bdf8;"></i> الأسباب المحتملة:</div>
    <ul style="margin: 0; padding-right: 20px;">
      <li>الجهاز غير متصل بالشبكة حالياً أو منطفئ (Offline).</li>
      <li>منفذ إدارة الويب (HTTP / Port 80) مغلق على جهاز المشترك.</li>
      <li>الجهاز يعمل على منفذ إدارة بديل (مثل 443 / 8080 / 81).</li>
      <li>جدار الحماية في الراوتر يمنع طلبات الوصول المباشرة.</li>
    </ul>
  </div>
  <div class="btn-group">
    <button class="btn btn-primary" onclick="window.location.reload()"><i class="fa-solid fa-rotate-right"></i> إعادة المحاولة</button>
    <a class="btn btn-secondary" href="/proxy/%s:8080/"><i class="fa-solid fa-network-wired"></i> تجربة منفذ 8080</a>
    <a class="btn btn-secondary" href="/proxy/%s:81/"><i class="fa-solid fa-network-wired"></i> تجربة منفذ 81</a>
  </div>
  <div class="err-details">%s</div>
</div>
</body>
</html>`, cleanIP, cleanIP, cleanIP, err.Error())
	_, _ = w.Write([]byte(html))
}
