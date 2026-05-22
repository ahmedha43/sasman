package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	targetIP := "192.168.10.243"
	if len(os.Args) > 1 && strings.TrimSpace(os.Args[1]) != "" {
		targetIP = strings.TrimSpace(os.Args[1])
	}

	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS10,
			CipherSuites: []uint16{
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				return nil
			},
			VerifyConnection: func(cs tls.ConnectionState) error {
				return nil
			},
		},
	}

	baseURL := detectTargetURL(targetIP, transport)
	fmt.Printf("--- Testing Device Flow for %s ---\n\n", targetIP)
	fmt.Printf("[Base] %s\n", baseURL.String())

	finalResp, finalURL, err := followDeviceFlow(baseURL, jar, transport, 8)
	if err != nil {
		fmt.Printf("[FAIL] %v\n", err)
		return
	}
	defer finalResp.Body.Close()

	fmt.Printf("[Final URL] %s\n", finalURL.String())
	fmt.Printf("[Status] %s\n", finalResp.Status)

	body, _ := io.ReadAll(finalResp.Body)
	fmt.Printf("[Body Length] %d bytes\n", len(body))
	if len(body) > 0 {
		snippetLen := 240
		if len(body) < snippetLen {
			snippetLen = len(body)
		}
		fmt.Printf("\n--- HTML Snippet ---\n%s\n--- END ---\n", string(body[:snippetLen]))
	}
}

func detectTargetURL(ip string, transport *http.Transport) *url.URL {
	httpsURL, _ := url.Parse("https://" + ip + "/")
	if targetResponds(httpsURL.String(), transport) {
		return httpsURL
	}

	httpURL, _ := url.Parse("http://" + ip + "/")
	return httpURL
}

func targetResponds(rawURL string, transport *http.Transport) bool {
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
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

func followDeviceFlow(startURL *url.URL, jar http.CookieJar, transport *http.Transport, maxDepth int) (*http.Response, *url.URL, error) {
	current := *startURL

	for depth := 0; depth <= maxDepth; depth++ {
		client := &http.Client{
			Timeout:   20 * time.Second,
			Jar:       jar,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		fmt.Printf("[Request] GET %s\n", current.String())
		resp, err := client.Get(current.String())
		if err != nil {
			return nil, &current, err
		}

		if !isRedirect(resp.StatusCode) {
			return resp, &current, nil
		}

		location := resp.Header.Get("Location")
		fmt.Printf("[Redirect] %s -> %s\n", resp.Status, location)
		_ = resp.Body.Close()

		nextURL, err := url.Parse(location)
		if err != nil {
			return nil, &current, err
		}
		current = *current.ResolveReference(nextURL)
	}

	return nil, &current, fmt.Errorf("too many redirects")
}

func isRedirect(code int) bool {
	return code >= 300 && code <= 399
}
