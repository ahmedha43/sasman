package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"runtime/debug"

	"mikrotik-manager/pkg/core"
	"mikrotik-manager/pkg/lan"
	"mikrotik-manager/pkg/radius"
	"mikrotik-manager/pkg/routing"
	"mikrotik-manager/pkg/shared"
	"mikrotik-manager/pkg/streaming"
	"mikrotik-manager/pkg/wan"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
)

// Global state for Ngrok tunnels
var ngrokWebURL string
var ngrokTCPURL string

var (
	deviceProxyCookieMu   sync.Mutex
	deviceProxyCookieJars = map[string]*cookiejar.Jar{}
)

func main() {
	// Set Memory Limit to 150MB to prevent Out-Of-Memory on low-end devices
	debug.SetMemoryLimit(150 * 1024 * 1024)
	// Make the Garbage Collector more aggressive (default is 100)
	debug.SetGCPercent(50)

	// Initialize Shared State & Config
	shared.LoadData()
	shared.LoadConfig()

	// Initialize Radius Database
	radiusDBPath := os.Getenv("RADIUS_DB_PATH")
	if radiusDBPath == "" {
		radiusDBPath = "data/radius.db"
	}
	radius.InitDB()
	radius.EnsureDefaultAdmin()

	// Start Native Go RADIUS Server
	radius.StartRadiusServer()

	// Start expiration sweeper (disconnects expired online users)
	radius.StartExpirationSweeper()

	// Start daily pruning sweeper (removes old radacct/radpostauth records)
	radius.StartPruningSweeper()

	// Start log rotation to prevent disk bloat
	go radius.WatchAndRotateLogs()

	// Start Ngrok if NGROK_AUTHTOKEN is provided
	go startNgrokTunnels()

	// Start real-time session reconciler (sync MikroTik active users with RADIUS DB)
	radius.StartReconciliationWorker(60 * time.Second)

	// Start Telegram backup scheduler
	radius.StartTelegramBackupScheduler()

	// Start Telegram Interactive Bot Worker
	radius.StartTelegramBot()

	// Start WhatsApp Expiration Reminder Worker
	radius.StartReminderWorker()

	// Set up memory limit to ~150MB to prevent the app from consuming too much RAM over time
	// Adjust as necessary depending on your deployment environment
	// runtime/debug is imported, we need to add it to imports
	
	app := fiber.New(fiber.Config{
		AppName:           "SASMAN MikroTik Manager v2.0 [UNIFIED]",
		ReduceMemoryUsage: true,
	})

	app.Use(cors.New())
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	app.Use(func(c *fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/radius/js/") ||
			strings.HasPrefix(path, "/radius/css/") ||
			strings.HasPrefix(path, "/radius/fonts/") ||
			strings.HasPrefix(path, "/radius/vendor/") ||
			strings.HasPrefix(path, "/admin/js/") ||
			strings.HasPrefix(path, "/admin/css/") ||
			strings.HasPrefix(path, "/admin/fonts/") ||
			path == "/radius/unnamed.png" {
			c.Set("Cache-Control", "public, max-age=86400")
		}
		return c.Next()
	})

	// Redirect Middleware for unauthenticated access to UI
	app.Use(func(c *fiber.Ctx) error {
		path := c.Path()

		// 1. Whitelist: Always allow login page and its assets
		if path == "/radius/login.html" ||
			strings.HasPrefix(path, "/radius/api/auth") ||
			strings.HasPrefix(path, "/radius/js") ||
			strings.HasPrefix(path, "/radius/css") ||
			strings.HasPrefix(path, "/radius/fonts") ||
			strings.HasPrefix(path, "/radius/vendor") ||
			path == "/radius/unnamed.png" ||
			path == "/radius/favicon.ico" ||
			strings.HasPrefix(path, "/js/login.js") { // If any
			return c.Next()
		}

		// 2. Protection: Intercept root, /admin, and /radius (but skip /radius/api)
		if path == "/" || path == "/admin" || strings.HasPrefix(path, "/admin/") ||
			(strings.HasPrefix(path, "/radius") && !strings.HasPrefix(path, "/radius/api")) {

			// Check for RADIUS session cookie
			token := c.Cookies("sasman_admin_session")
			if token == "" {
				return c.Redirect("/radius/login.html")
			}
		}
		return c.Next()
	})

	// Default route → redirect to admin panel
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/admin")
	})

	// ==================== ADMIN PANEL ====================
	// Serve Static Files (UI) for Admin Panel
	app.Static("/admin", "./public")
	app.Static("/css", "./public/css")
	app.Static("/js", "./public/js")
	app.Get("/admin/*", func(c *fiber.Ctx) error {
		return c.SendFile("./public/index.html")
	})

	// Shared insecure transport for all proxies
	proxyTransport := &http.Transport{
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

	// Admin API Group
	// Protected Main API (Requires RADIUS Admin Account)
	api := app.Group("/api", radius.RequireAdmin)

	// MikroTik WebFig Proxy (Requires RADIUS Admin Account)
	app.All("/mikrotik/*", radius.RequireAdmin, func(c *fiber.Ctx) error {
		routerAddress := shared.RouterConfigState.Address
		if routerAddress == "" {
			return c.Status(400).SendString("Router address not configured. Please login first.")
		}

		parts := strings.Split(routerAddress, ":")
		routerIP := parts[0]

		targetPath := strings.TrimPrefix(c.Path(), "/mikrotik")
		if targetPath == "" {
			targetPath = "/"
		}
		// Force HTTP for local devices to avoid TLS certificate issues
		targetURL, _ := url.Parse("http://" + routerIP)

		return adaptor.HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Strip headers that trigger HTTPS upgrades
			r.Header.Del("Upgrade-Insecure-Requests")
			r.Header.Del("Accept-Encoding")     // Force uncompressed to avoid mangling
			r.Header.Set("Connection", "close") // Prevent keep-alive issues with some legacy devices

			r.Host = targetURL.Host
			r.URL.Host = targetURL.Host
			r.URL.Scheme = "http"
			r.URL.Path = targetPath

			proxy := httputil.NewSingleHostReverseProxy(targetURL)
			proxy.Transport = proxyTransport
			proxy.ModifyResponse = func(res *http.Response) error {
				// Rewrite Redirects
				if loc := res.Header.Get("Location"); loc != "" {
					u, _ := url.Parse(loc)
					if strings.Contains(loc, routerIP) {
						// Always keep it on HTTP and strip any :443 or https hints
						newLoc := "/mikrotik" + u.Path
						if u.RawQuery != "" {
							newLoc += "?" + u.RawQuery
						}
						res.Header.Set("Location", newLoc)
					}
				}
				res.Header.Del("X-Frame-Options")
				res.Header.Del("Content-Security-Policy")
				return nil
			}
			proxy.ServeHTTP(w, r)
		}))(c)
	})

	// Core & Authentication
	api.Post("/login", loginHandler)
	api.Post("/logout", logoutHandler)
	api.Get("/auth/status", authStatusHandler)
	api.Get("/license/status", core.GetLicenseStatus)
	api.Post("/license/activate", core.ActivateLicense)

	// Ngrok Tunnel Configuration
	api.Post("/ngrok/token", saveNgrokTokenHandler)
	api.Get("/ngrok/token", getNgrokTokenHandler)
	api.Get("/cloudflared/url", getCloudflareTunnelURL)

	// WAN Section
	api.Get("/interfaces", wan.GetInterfaces)
	api.Get("/wan/pppoe", wan.GetPPPoE)
	api.Post("/wan/pppoe", wan.AddPPPoE)
	api.Post("/wan/macvlan", wan.AddMacvlan)
	api.Delete("/wan/pppoe/:name", wan.DeletePPPoE)
	api.Get("/wan/dhcp-client", wan.GetDhcpClients)
	api.Post("/wan/dhcp-client", wan.AddDhcpClient)
	api.Delete("/wan/dhcp-client/:id", wan.DeleteDhcpClient)
	api.Post("/wan/pcc/rebalance", wan.ApplyPcc)
	api.Post("/wan/setup-batch", wan.SetupMultiWan)
	api.Delete("/purge", wan.PurgeSASMAN)
	api.Get("/status/wan", wan.GetWanStatus)

	// LAN Section
	api.Post("/lan/bridge", lan.SetupBridge)
	api.Post("/lan/purge", lan.PurgeLAN)

	// Routing Section
	api.Get("/routing/list", routingListHandler)
	api.Post("/routing/apply", routing.ApplyRouting)
	api.Post("/routing/toggle", routing.ToggleRouting)
	api.Post("/routing/remove", routing.RemoveRouting)
	api.Get("/status/routing", routing.GetRoutingStatus)
	api.Delete("/routing/purge", routing.PurgeRouting)
	api.Post("/routing/block", routing.ApplyBlock)

	// ==================== RADIUS BILLING ====================
	// RADIUS API Group (registered FIRST, before static/wildcard routes)
	radiusAPI := app.Group("/radius/api")

	if os.Getenv("DEBUG_RADIUS_API") == "1" {
		radiusAPI.Use(func(c *fiber.Ctx) error {
			log.Printf("[radius-api] %s %s", c.Method(), c.Path())
			return c.Next()
		})
	}

	// Public auth & license status (no auth)
	radiusAPI.Post("/auth/login", radius.LoginHandler)
	radiusAPI.Post("/auth/logout", radius.LogoutHandler)
	radiusAPI.Get("/license/status", radius.LicenseStatusHandler)
	radiusAPI.Post("/vouchers/redeem", radius.RedeemVoucher)

	// User Portal (Public login for subscribers)
	radiusAPI.Post("/portal/login", radius.PortalLoginHandler)
	radiusAPI.Get("/portal/status", radius.PortalStatusHandler)
	radiusAPI.Post("/portal/password", radius.PortalChangePasswordHandler)
	radiusAPI.Get("/portal/streams", streaming.GetActiveStreams)
	radiusAPI.Get("/portal/stream-proxy", streaming.ProxyExternalStream)

	// Authenticated account management (no license required)
	radiusAccount := radiusAPI.Group("/auth", radius.RequireAdmin)
	radiusAccount.Get("/me", radius.MeHandler)
	radiusAccount.Put("/profile", radius.UpdateProfileHandler)
	radiusAccount.Post("/password", radius.ChangePasswordHandler)
	radiusAccount.Post("/register", radius.RegisterAdminHandler)
	radiusAccount.Get("/admins", radius.ListAdminsHandler)
	radiusAccount.Delete("/admins/:id", radius.DeleteAdminHandler)
	radiusAccount.Post("/recharge", radius.RechargeAdminHandler)
	radiusAccount.Post("/withdraw", radius.WithdrawAdminHandler)
	radiusAccount.Get("/admins/transactions", radius.ListAdminTransactionsHandler)
	radiusAccount.Get("/backup", radius.BackupDatabaseHandler)
	radiusAccount.Post("/restore", radius.RestoreDatabaseHandler)
	radiusAccount.Get("/backup/telegram", radius.GetTelegramBackupConfig)
	radiusAccount.Post("/backup/telegram", radius.SaveTelegramBackupConfig)
	radiusAccount.Post("/backup/telegram/test", radius.TestTelegramBackup)

	// License activation requires admin auth only
	radiusAPI.Post("/license/activate", radius.RequireAdmin, radius.LicenseActivateHandler)

	// Licensed area (auth + license gate)
	radiusSecure := radiusAPI.Group("", radius.RequireAdmin, radius.RequireLicense)

	// Live Streaming (Admin Management)
	radiusSecure.Get("/streams", streaming.GetStreamsAdmin)
	radiusSecure.Post("/streams", streaming.CreateStreamAdmin)
	radiusSecure.Put("/streams/:id", streaming.UpdateStreamAdmin)
	radiusSecure.Delete("/streams/:id", streaming.DeleteStreamAdmin)
	radiusSecure.Get("/streams/server/status", streaming.GetMediaMTXStatus)
	radiusSecure.Post("/streams/server/control", streaming.ControlMediaMTX)

	// Profiles
	radiusSecure.Get("/profiles", radius.GetProfiles)
	radiusSecure.Post("/profiles", radius.CreateProfile)
	radiusSecure.Delete("/profiles/:name", radius.DeleteProfile)

	// Users
	radiusSecure.Get("/users", radius.GetUsers)
	radiusSecure.Post("/users", radius.CreateUser)
	radiusSecure.Post("/users/:user/renew", radius.RenewUser)
	radiusSecure.Delete("/users/:user", radius.DeleteUser)
	radiusSecure.Post("/users/:user/disconnect", radius.DisconnectUser)
	radiusSecure.Post("/users/:user/toggle-status", radius.ToggleUserStatus)
	radiusSecure.Post("/import/sas4", radius.ImportFromSAS4)
	radiusSecure.Post("/system/reset", radius.ResetDatabase)

	// RADIUS Logs
	radiusSecure.Get("/logs", radius.GetRadiusLogs)
	radiusSecure.Delete("/logs", radius.ClearRadiusLogs)

	// NAS
	radiusSecure.Get("/nas", radius.GetNAS)
	radiusSecure.Post("/nas", radius.CreateNAS)
	radiusSecure.Put("/nas/:id", radius.UpdateNAS)
	radiusSecure.Delete("/nas/:ip", radius.DeleteNAS)

	// Vouchers (Management)
	radiusSecure.Get("/vouchers", radius.GetVouchers)
	radiusSecure.Post("/vouchers/generate", radius.GenerateVouchers)
	radiusSecure.Delete("/vouchers/:id", radius.DeleteVoucher)
	radiusSecure.Delete("/vouchers/batch/:batch_id", radius.DeleteVoucherBatch)
	radiusSecure.Delete("/vouchers/all/clear", radius.ClearAllVouchers)

	// User Details & Transactions
	radiusSecure.Get("/users/:user/details", radius.GetUserDetails)
	radiusSecure.Get("/users/:user/transactions", radius.GetUserTransactions)
	radiusSecure.Post("/users/:user/transactions", radius.AddTransaction)
	radiusSecure.Get("/users/:user/balance", radius.GetUserBalance)

	// Database pruning
	radiusSecure.Post("/prune", radius.ManualPruneHandler)

	// WhatsApp
	radiusSecure.Get("/whatsapp/config", radius.GetWhatsappConfig)
	radiusSecure.Post("/whatsapp/config", radius.SaveWhatsappConfig)
	radiusSecure.Post("/whatsapp/test", radius.TestWhatsappNotification)
	radiusSecure.Get("/whatsapp/qr", radius.GetWhatsappQR)
	radiusSecure.Post("/whatsapp/logout", radius.LogoutWhatsapp)
	radiusSecure.Get("/whatsapp/templates", radius.GetMessageTemplates)
	radiusSecure.Post("/whatsapp/templates", radius.SaveMessageTemplate)

	// Global Blind Accept Bypass (SAS 4 style)
	radiusSecure.Get("/bypass", radius.GetBypassStatus)
	radiusSecure.Post("/bypass", radius.SetBypassStatus)

	// Dynamic Device Proxy (NanoStation, Home Routers)
	// Usage: /proxy/192.168.10.231/ -> proxies to the device web UI.
	app.All("/proxy/:target/*", radius.RequireAdmin, func(c *fiber.Ctx) error {
		targetIP := c.Params("target")
		if targetIP == "" {
			return c.Status(400).SendString("Target IP required")
		}

		// Set sticky cookie for orphaned absolute requests (like AirOS /12345/assets)
		c.Cookie(&fiber.Cookie{
			Name:     "last_proxy_target",
			Value:    targetIP,
			Path:     "/",
			HTTPOnly: true,
		})

		// Ensure trailing slash for directory-like access to maintain relative paths
		if !strings.HasSuffix(c.Path(), "/") && c.Params("*") == "" {
			return c.Redirect(c.Path() + "/")
		}

		cleanIP := targetIP
		if strings.Contains(targetIP, ":") {
			cleanIP, _, _ = net.SplitHostPort(targetIP)
		}

		if net.ParseIP(cleanIP) == nil {
			return c.Status(400).SendString("Invalid Target IP: " + cleanIP)
		}

		targetPath := strings.TrimPrefix(c.Path(), "/proxy/"+targetIP)
		if targetPath == "" {
			targetPath = "/"
		}
		targetURL := detectProxyTargetURL(cleanIP, proxyTransport)

		fmt.Printf("[dev-proxy] Proxying request to: %s%s\n", targetURL.String(), targetPath)

		return adaptor.HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serveDeviceProxy(w, r, cleanIP, targetURL, targetPath, proxyTransport)
		}))(c)
	})

	// Serve Static Assets (JS/CSS only) — scoped paths, won't intercept /radius/api/*
	app.Static("/radius/js", "./public_radius/js")
	app.Static("/radius/css", "./public_radius/css")
	app.Static("/radius/fonts", "./public_radius/fonts")
	app.Static("/radius/vendor", "./public_radius/vendor")
	app.Static("/radius/unnamed.png", "./public_radius/unnamed.png")

	// Explicit HTML pages
	app.Get("/radius", func(c *fiber.Ctx) error {
		return c.SendFile("./public_radius/index.html")
	})
	app.Get("/radius/", func(c *fiber.Ctx) error {
		return c.SendFile("./public_radius/index.html")
	})
	app.Get("/radius/login.html", func(c *fiber.Ctx) error {
		return c.SendFile("./public_radius/login.html")
	})
	app.Get("/portal", func(c *fiber.Ctx) error {
		return c.SendFile("./public_radius/portal.html")
	})
	app.Get("/portal/", func(c *fiber.Ctx) error {
		return c.SendFile("./public_radius/portal.html")
	})
	app.Get("/radius/portal.html", func(c *fiber.Ctx) error {
		return c.SendFile("./public_radius/portal.html")
	})

	// Wildcard route for RADIUS UI (SPA routing) — MUST be last
	// Skip /radius/api/* so API handlers can respond
	app.Get("/radius/*", func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/radius/api/") {
			return c.Next()
		}
		return c.SendFile("./public_radius/index.html")
	})

	// Catch-all route for any unhandled requests (Fallback to last proxy or MikroTik)
	app.All("/*", func(c *fiber.Ctx) error {
		path := c.Path()
		// If it's a known static radius path, don't proxy
		if strings.HasPrefix(path, "/radius") || strings.HasPrefix(path, "/proxy") || strings.HasPrefix(path, "/admin") {
			return c.Next()
		}

		// Try to use last proxy target cookie if the path looks like a device request
		lastTarget := c.Cookies("last_proxy_target")
		isDeviceRequest := lastTarget != "" && (strings.HasSuffix(path, ".cgi") ||
			strings.HasSuffix(path, ".php") ||
			strings.HasSuffix(path, ".js") ||
			strings.HasSuffix(path, ".css") ||
			strings.HasPrefix(path, "/images/") ||
			strings.HasPrefix(path, "/js/") ||
			strings.HasPrefix(path, "/css/") ||
			strings.HasPrefix(path, "/lib/") ||
			(len(path) > 5 && path[1] >= '0' && path[1] <= '9')) // Likely AirOS versioned path /123456/

		targetIP := shared.RouterConfigState.Address
		if isDeviceRequest {
			// Redirect orphaned request to the correct proxy path to fix 404s
			return c.Redirect("/proxy/" + lastTarget + path)
		}

		if targetIP == "" {
			return c.Next()
		}

		routerIP := strings.Split(targetIP, ":")[0]
		targetURL, err := url.Parse("http://" + routerIP)
		if err != nil {
			return c.Status(500).SendString("Invalid router address")
		}

		return adaptor.HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Del("Accept-Encoding")
			r.Host = targetURL.Host
			r.URL.Host = targetURL.Host
			r.URL.Scheme = targetURL.Scheme

			proxy := httputil.NewSingleHostReverseProxy(targetURL)
			proxy.Transport = proxyTransport
			proxy.ModifyResponse = func(res *http.Response) error {
				if loc := res.Header.Get("Location"); loc != "" {
					if strings.Contains(loc, routerIP) {
						u, _ := url.Parse(loc)
						newLoc := u.Path
						if u.RawQuery != "" {
							newLoc += "?" + u.RawQuery
						}
						res.Header.Set("Location", newLoc)
					}
				}
				res.Header.Del("X-Frame-Options")
				res.Header.Del("Content-Security-Policy")
				return nil
			}
			proxy.ServeHTTP(w, r)
		}))(c)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	fmt.Printf("Starting SASMAN Unified Server on :%s...\n", port)
	fmt.Printf("  Admin Panel: http://localhost:%s/admin\n", port)
	fmt.Printf("  RADIUS Panel: http://localhost:%s/radius\n", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Critical error: Failed to start server: %v", err)
	}
}

func detectProxyTargetURL(cleanIP string, transport *http.Transport) *url.URL {
	if proxyTargetResponds("https://"+cleanIP+"/", transport) {
		targetURL, _ := url.Parse("https://" + cleanIP)
		return targetURL
	}

	targetURL, _ := url.Parse("http://" + cleanIP)
	return targetURL
}

func proxyTargetResponds(rawURL string, transport *http.Transport) bool {
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

func serveDeviceProxy(w http.ResponseWriter, r *http.Request, cleanIP string, targetURL *url.URL, targetPath string, transport *http.Transport) {
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

	resp, err := deviceProxyDoRequest(r, sourceBody, cleanIP, &backendURL, transport, 0)
	if err != nil {
		http.Error(w, "Device proxy failed: "+err.Error(), http.StatusBadGateway)
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
			w.Header().Set(k, v) // Use Set instead of Add to avoid duplicate headers
		}
	}

	if loc := resp.Header.Get("Location"); loc != "" {
		if newLoc := rewriteDeviceProxyLocation(loc, cleanIP); newLoc != "" {
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
			body = []byte(rewriteDeviceProxyHTML(string(body), cleanIP))
		} else if isCSS || isJS {
			body = []byte(rewriteDeviceProxyAssets(string(body), cleanIP))
		}
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

func rewriteDeviceProxyAssets(body string, cleanIP string) string {
	prefix := "/proxy/" + cleanIP
	// Rewrite absolute paths in CSS and JS (limited to common asset/API patterns to avoid breaking code)
	body = strings.ReplaceAll(body, `url(/`, `url(`+prefix+`/`)
	body = strings.ReplaceAll(body, `url("/`, `url("`+prefix+`/`)
	body = strings.ReplaceAll(body, `url('/`, `url('`+prefix+`/`)
	return body
}

func deviceProxyDoRequest(source *http.Request, sourceBody []byte, cleanIP string, backendURL *url.URL, transport *http.Transport, depth int) (*http.Response, error) {
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
	copyProxyRequestHeaders(req.Header, source.Header)
	req.Header.Del("Upgrade-Insecure-Requests")
	req.Header.Del("Accept-Encoding") // Force uncompressed
	req.Header.Del("Cookie")
	req.Header.Set("Connection", "close")
	req.Host = backendURL.Host
	for _, cookie := range deviceProxyCookies(cleanIP, backendURL) {
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
	deviceProxyStoreCookies(cleanIP, backendURL, resp.Cookies())

	if isDeviceCookieRedirect(resp) {
		loc := resp.Header.Get("Location")
		_ = resp.Body.Close()
		nextURL := resolveDeviceLocation(backendURL, loc)
		return deviceProxyDoRequest(source, sourceBody, cleanIP, nextURL, transport, depth+1)
	}

	return resp, nil
}

func copyProxyRequestHeaders(dst http.Header, src http.Header) {
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

func isDeviceCookieRedirect(resp *http.Response) bool {
	if resp.StatusCode < 300 || resp.StatusCode > 399 {
		return false
	}
	loc := strings.ToLower(resp.Header.Get("Location"))
	return strings.Contains(loc, "cookiechecker") || strings.Contains(loc, "login.cgi") || loc == "/" || strings.HasSuffix(loc, "/")
}

func resolveDeviceLocation(base *url.URL, loc string) *url.URL {
	u, err := url.Parse(loc)
	if err != nil {
		next := *base
		return &next
	}
	return base.ResolveReference(u)
}

func deviceProxyJar(cleanIP string) *cookiejar.Jar {
	deviceProxyCookieMu.Lock()
	defer deviceProxyCookieMu.Unlock()

	if jar, ok := deviceProxyCookieJars[cleanIP]; ok {
		return jar
	}
	jar, _ := cookiejar.New(nil)
	deviceProxyCookieJars[cleanIP] = jar
	return jar
}

func deviceProxyCookies(cleanIP string, backendURL *url.URL) []*http.Cookie {
	return deviceProxyJar(cleanIP).Cookies(backendURL)
}

func deviceProxyStoreCookies(cleanIP string, backendURL *url.URL, cookies []*http.Cookie) {
	if len(cookies) == 0 {
		return
	}
	deviceProxyJar(cleanIP).SetCookies(backendURL, cookies)
}

func rewriteDeviceProxyLocation(loc string, cleanIP string) string {
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

func rewriteDeviceProxyHTML(body string, cleanIP string) string {
	prefix := "/proxy/" + cleanIP
	// Using a more robust replacement that avoids double slashes and handles more tags
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

// Global Auth Handlers (Managed here for simplicity in initial refactor)

func loginHandler(c *fiber.Ctx) error {
	type Request struct {
		Address string `json:"address"`
		User    string `json:"user"`
		Pass    string `json:"pass"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Test connection
	shared.RouterConfigState.Address = req.Address
	shared.RouterConfigState.Username = req.User
	shared.RouterConfigState.Password = req.Pass

	client, err := core.Connect()
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "فشل الاتصال بالراوتر: " + err.Error()})
	}
	if serial, serialErr := core.GetRouterSerial(client); serialErr == nil {
		shared.RouterConfigState.Serial = serial
	}
	client.Close()

	shared.SaveConfig()
	core.ResetSharedClient()
	return c.JSON(fiber.Map{"message": "تم الاتصال بنجاح"})
}

func logoutHandler(c *fiber.Ctx) error {
	shared.RemoveConfig()
	return c.JSON(fiber.Map{"message": "Logged out and session cleared"})
}

func authStatusHandler(c *fiber.Ctx) error {
	if shared.RouterConfigState.Address == "" {
		return c.Status(401).JSON(fiber.Map{"authenticated": false})
	}
	return c.JSON(fiber.Map{
		"authenticated": true,
		"address":       shared.RouterConfigState.Address,
		"user":          shared.RouterConfigState.Username,
	})
}

func routingListHandler(c *fiber.Ctx) error {
	keys := make([]string, 0)
	for k := range shared.RoutingDataState.Apps {
		keys = append(keys, k)
	}
	for k := range shared.RoutingDataState.Ips {
		keys = append(keys, k)
	}
	return c.JSON(keys)
}

// getCloudflareTunnelURL reads the cloudflared log file and extracts the tunnel URL
func getCloudflareTunnelURL(c *fiber.Ctx) error {
	// Check if cloudflared is enabled (default to true if not explicitly disabled)
	enabled := os.Getenv("CLOUDFLARE_TUNNEL_ENABLED")
	cfURL := ""

	// Try to read the cloudflared log file
	if enabled != "false" {
		if file, err := os.Open("/app/data/cloudflared.log"); err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			urlRegex := regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)
			for scanner.Scan() {
				line := scanner.Text()
				if matches := urlRegex.FindStringSubmatch(line); len(matches) > 0 {
					found := matches[0]
					if !strings.Contains(found, "api.trycloudflare.com") {
						cfURL = found
					}
				}
			}
		}
	}

	return c.JSON(fiber.Map{
		"enabled": (cfURL != "") || (ngrokWebURL != ""),
		"url":     cfURL,
		"ngrok": fiber.Map{
			"web": ngrokWebURL,
			"tcp": ngrokTCPURL,
		},
	})
}

func getNgrokTokenHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"token": shared.RouterConfigState.NgrokToken,
	})
}

func saveNgrokTokenHandler(c *fiber.Ctx) error {
	type Request struct {
		Token string `json:"token"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	shared.RouterConfigState.NgrokToken = req.Token
	shared.SaveConfig()

	// Restart tunnels in background
	go startNgrokTunnels()

	return c.JSON(fiber.Map{"message": "تم حفظ التوكن وستبدأ الخدمة قريباً"})
}

// Global cancellation for Ngrok
var ngrokCancel context.CancelFunc

func startNgrokTunnels() {
	// Cancel existing sessions if running
	if ngrokCancel != nil {
		ngrokCancel()
		time.Sleep(1 * time.Second)
	}

	token := os.Getenv("NGROK_AUTHTOKEN")
	if token == "" {
		token = shared.RouterConfigState.NgrokToken
	}

	if token == "" {
		return
	}

	var ctx context.Context
	ctx, ngrokCancel = context.WithCancel(context.Background())

	go func() {
		// 1. Establish a single Ngrok session
		sess, err := ngrok.Connect(ctx,
			ngrok.WithAuthtoken(token),
			ngrok.WithMetadata("SASMAN Manager"),
		)
		if err != nil {
			log.Printf("[Ngrok] Connection failed: %v", err)
			return
		}
		log.Printf("[Ngrok] Session established successfully")

		// 2. Start HTTP Tunnel for Dashboard
		go func() {
			tun, err := sess.Listen(ctx, config.HTTPEndpoint())
			if err != nil {
				log.Printf("[Ngrok] HTTP Tunnel error: %v", err)
				return
			}
			ngrokWebURL = tun.URL()
			log.Printf("[Ngrok] Dashboard Tunnel: %s", ngrokWebURL)

			for {
				conn, err := tun.Accept()
				if err != nil {
					log.Printf("[Ngrok] Web Accept error: %v", err)
					break
				}
				go handleWebProxy(conn)
			}
		}()

		// 3. Start TCP Tunnel for WinBox (8291)
		go func() {
			tun, err := sess.Listen(ctx, config.TCPEndpoint())
			if err != nil {
				log.Printf("[Ngrok] TCP Tunnel error: %v", err)
				return
			}
			ngrokTCPURL = strings.Replace(tun.URL(), "tcp://", "", 1)
			log.Printf("[Ngrok] WinBox Tunnel: %s", ngrokTCPURL)

			for {
				conn, err := tun.Accept()
				if err != nil {
					log.Printf("[Ngrok] TCP Accept error: %v", err)
					break
				}
				go handleWinboxProxy(conn)
			}
		}()

		// Keep session alive or handle its closure
		<-ctx.Done()
		sess.Close()
	}()
}

func handleWebProxy(src net.Conn) {
	defer src.Close()
	dest, err := net.DialTimeout("tcp", "127.0.0.1:8080", 5*time.Second)
	if err != nil {
		log.Printf("[Ngrok] Web Proxy dial error: %v", err)
		return
	}
	defer dest.Close()

	done := make(chan struct{}, 2)
	go func() { io.Copy(dest, src); done <- struct{}{} }()
	go func() { io.Copy(src, dest); done <- struct{}{} }()
	<-done
}

func handleWinboxProxy(src net.Conn) {
	defer src.Close()

	routerAddress := shared.RouterConfigState.Address
	if routerAddress == "" {
		routerAddress = "192.168.88.1"
	}

	// Extract IP only, use default WinBox port
	parts := strings.Split(routerAddress, ":")
	routerIP := parts[0]

	// Connect to local router WinBox
	dest, err := net.DialTimeout("tcp", routerIP+":8291", 5*time.Second)
	if err != nil {
		log.Printf("[Ngrok] Proxy: Failed to connect to %s:8291: %v", routerIP, err)
		return
	}
	defer dest.Close()

	// Bi-directional data transfer
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(dest, src)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(src, dest)
		done <- struct{}{}
	}()

	<-done
}
