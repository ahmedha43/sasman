package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"mikrotik-manager/pkg/broadcast"
	"mikrotik-manager/agent/pkg/core"
	"mikrotik-manager/agent/pkg/devices"
	_ "mikrotik-manager/agent/pkg/devices/drivers/mikrotik"
	_ "mikrotik-manager/agent/pkg/devices/drivers/ubiquiti"
	_ "mikrotik-manager/agent/pkg/devices/drivers/cambium"
	_ "mikrotik-manager/agent/pkg/devices/drivers/mimosa"
	"mikrotik-manager/agent/pkg/firebase"
	"mikrotik-manager/agent/pkg/lan"
	"mikrotik-manager/agent/pkg/radius"
	"mikrotik-manager/pkg/relay"
	"mikrotik-manager/agent/pkg/routing"
	"mikrotik-manager/pkg/shared"
	"mikrotik-manager/agent/pkg/streaming"
	"mikrotik-manager/pkg/tunnel"
	"mikrotik-manager/agent/pkg/wan"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"

	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
)

// Global state for Ngrok tunnels
var ngrokWebURL string
var ngrokTCPURL string

// Global control for SASMAN agent tunnel
var activeTunnelClient *tunnel.ResilientAgentClient
var sasmanTunnelMu sync.Mutex
var activeSplashMgr *broadcast.SplashManager

var (
	deviceProxyCookieMu   sync.Mutex
	deviceProxyCookieJars = map[string]*cookiejar.Jar{}
)

// triggerRealtimeSyncConfig pushes the latest router & admin credentials to central server in real-time
func triggerRealtimeSyncConfig() {
	sasmanTunnelMu.Lock()
	tc := activeTunnelClient
	sasmanTunnelMu.Unlock()
	if tc != nil {
		tc.TriggerSync()
	}
}

// startSasmanTunnel stops any existing tunnel and starts a new one with the given settings.
// It is safe to call from any goroutine.
func startSasmanTunnel(port string) {
	sasmanTunnelMu.Lock()
	if activeTunnelClient != nil {
		activeTunnelClient.Stop()
		activeTunnelClient = nil
	}

	mode := strings.TrimSpace(shared.RouterConfigState.TunnelMode)
	subdomain := strings.TrimSpace(shared.RouterConfigState.TunnelSubdomain)
	if subdomain == "" {
		subdomain = strings.TrimSpace(os.Getenv("SASMAN_SUBDOMAIN"))
	}
	token := strings.TrimSpace(shared.RouterConfigState.TunnelToken)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("SASMAN_TUNNEL_TOKEN"))
	}

	if os.Getenv("CLOUD_MODE") == "true" || os.Getenv("SASMAN_CLOUD_MODE") == "true" {
		mode = "agent"
		shared.RouterConfigState.TunnelMode = "agent"
		if subdomain == "" {
			subdomain = strings.TrimSpace(os.Getenv("SASMAN_SUBDOMAIN"))
		}
		if token == "" {
			token = strings.TrimSpace(os.Getenv("SASMAN_TUNNEL_TOKEN"))
		}
	}

	if mode != "agent" || subdomain == "" {
		sasmanTunnelMu.Unlock()
		log.Printf("[Tunnel] Agent mode disabled or subdomain not set. Tunnel stopped.")
		return
	}

	gatewayURL := strings.TrimSpace(shared.RouterConfigState.TunnelGatewayURL)
	if gatewayURL == "" {
		gatewayURL = strings.TrimSpace(os.Getenv("SASMAN_TUNNEL_GATEWAY_URL"))
	}
	if gatewayURL == "" && os.Getenv("SASMAN_CENTRAL_URL") != "" {
		gatewayURL = strings.TrimSpace(os.Getenv("SASMAN_CENTRAL_URL"))
	}
	if gatewayURL == "" {
		gatewayURL = "wss://sas-man.net/api/tunnel/ws"
	}

	client := tunnel.NewResilientAgentClient(tunnel.AgentClientConfig{
		Subdomain:       subdomain,
		Token:           token,
		Version:         "5.1.0",
		Arch:            "linux_" + runtime.GOARCH,
		DataDir:         os.Getenv("SASMAN_DATA_DIR"),
		GatewayURL:      gatewayURL,
		LocalPort:       port,
		EnableRelay:     true,
		RelayListenAddr: "0.0.0.0:18443",
		OnSyncConfig:    sendSyncConfig,
		OnBackupRequest: handleBackupRequest,
		OnLocalHTTP:     handleLocalHTTPRequest,
		OnMikroTikSync: func(services []relay.ServiceDefinition) {
			rClient, err := core.Connect()
			if err == nil && rClient != nil {
				defer rClient.Close()
				_ = relay.SyncMikroTikRelayRules(rClient, services, 18443)
			}
		},
		OnBroadcast: func(bc broadcast.BroadcastMessage) {
			broadcast.StoreActiveBroadcast(bc)
			if (bc.DisplayType == "splash" || bc.TargetType == "users" || bc.TargetType == "both" || bc.TargetType == "broadband" || bc.TargetType == "all") && activeSplashMgr != nil {
				go func() {
					_ = activeSplashMgr.ApplySplashCampaign(bc)
				}()
			}
		},
		OnLicenseLease: func(payload []byte) {
			var lease struct {
				Status        string `json:"status"`
				ExpiresAt     string `json:"expires_at"`
				DaysRemaining int    `json:"days_remaining"`
				IsExpired     bool   `json:"is_expired"`
				Valid         bool   `json:"valid"`
			}
			if err := json.Unmarshal(payload, &lease); err == nil {
				shared.RouterConfigState.CloudLicenseStatus = lease.Status
				shared.RouterConfigState.CloudLicenseExpiresAt = lease.ExpiresAt
				shared.RouterConfigState.CloudLicenseDaysLeft = lease.DaysRemaining
				shared.RouterConfigState.CloudLicenseValid = lease.Valid
				shared.SaveConfig()

				// Cache on disk
				_ = os.WriteFile(filepath.Join(shared.GetDataDir(), "cloud_license.json"), payload, 0644)
				log.Printf("[License Engine] 🛡️ Received Cloud License Lease: status=%s, expires=%s, valid=%v, days_left=%d", lease.Status, lease.ExpiresAt, lease.Valid, lease.DaysRemaining)
			}
		},
		OnTCPConnect: func(connID string) (net.Conn, error) {
			routerAddress := strings.TrimSpace(shared.RouterConfigState.Address)
			if routerAddress == "" {
				routerAddress = strings.TrimSpace(os.Getenv("ROUTER_ADDRESS"))
			}
			if routerAddress == "" {
				routerAddress = strings.TrimSpace(os.Getenv("ROUTER_IP"))
			}
			if routerAddress == "" {
				routerAddress = "192.168.88.1"
			}
			host := routerAddress
			if strings.Contains(routerAddress, ":") {
				h, _, err := net.SplitHostPort(routerAddress)
				if err == nil {
					host = h
				}
			}
			target := net.JoinHostPort(host, "8291")
			d := net.Dialer{Timeout: 5 * time.Second, KeepAlive: 15 * time.Second}
			conn, err := d.Dial("tcp", target)
			if err != nil {
				// Try fallback to container default gateway or 127.0.0.1 or standard gateway
				log.Printf("[Winbox Tunnel] Dial to %s failed (%v), trying fallback gateway...", target, err)
				for _, fallbackHost := range []string{"172.17.0.1", "127.0.0.1", "192.168.88.1", "172.16.0.1"} {
					if fallbackHost == host {
						continue
					}
					fbTarget := net.JoinHostPort(fallbackHost, "8291")
					fbConn, fbErr := d.Dial("tcp", fbTarget)
					if fbErr == nil && fbConn != nil {
						log.Printf("[Winbox Tunnel] Connected to fallback MikroTik at %s", fbTarget)
						return fbConn, nil
					}
				}
				return nil, err
			}
			return conn, nil
		},
	})
	activeTunnelClient = client
	sasmanTunnelMu.Unlock()

	client.Start()
	log.Printf("[Tunnel] Resilient agent tunnel started for subdomain=%s", subdomain)
}

func resolveStaticDir(dirs ...string) string {
	for _, dir := range dirs {
		candidates := []string{
			dir,
			filepath.Join("agent", dir),
			filepath.Join(".", dir),
		}
		for _, c := range candidates {
			if fi, err := os.Stat(c); err == nil && fi.IsDir() {
				return c
			}
		}
	}
	if len(dirs) > 0 {
		return dirs[0]
	}
	return ""
}

func main() {
	// Set Memory Limit to 150MB to prevent Out-Of-Memory on low-end devices
	debug.SetMemoryLimit(150 * 1024 * 1024)

	dashboardDir := resolveStaticDir("web_dashboard", "public")
	radiusDir := resolveStaticDir("web_radius", "public_radius")

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	if shared.RouterConfigState.TunnelMode == "agent" || os.Getenv("SASMAN_TUNNEL_MODE") == "agent" {
		log.Println("SASMAN tunnel agent mode enabled")
	}
	// Make the Garbage Collector more aggressive (default is 100)
	debug.SetGCPercent(50)

	// Initialize Shared State & Config
	shared.LoadData()
	shared.LoadConfig()
	shared.OnConfigSaved = triggerRealtimeSyncConfig
	firebase.StartBackgroundSync(func() firebase.RemoteAccess {
		return firebase.RemoteAccess{
			NgrokWebURL: ngrokWebURL,
			NgrokTCPURL: ngrokTCPURL,
		}
	})

	// Initialize Radius Database
	radiusDBPath := os.Getenv("RADIUS_DB_PATH")
	if radiusDBPath == "" {
		radiusDBPath = "data/radius.db"
	}
	radius.InitDB()
	radius.EnsureDefaultAdmin()
	radius.OnAdminPasswordChanged = triggerRealtimeSyncConfig

	// Initialize Network Devices Subsystem (Switches, PtP Links, Sectors)
	if _, err := devices.Init(radius.DB); err != nil {
		log.Printf("[devices] Warning: Failed to init network devices subsystem: %v", err)
	}

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

	// Start scheduled internet shutdown monitor
	radius.StartShutdownMonitor()

	// Initialize Smart Broadcast & Ad-Engine Local Store
	broadcast.InitStore(os.Getenv("SASMAN_DATA_DIR"))

	// Initialize PPPoE Broadband Splash Interceptor Manager
	activeSplashMgr = broadcast.InitSplashManager(core.Connect, func(bLog broadcast.BroadcastLogPayload) {
		// Log views and clicks
	})

	// Set up memory limit to ~150MB to prevent the app from consuming too much RAM over time
	// Adjust as necessary depending on your deployment environment
	// runtime/debug is imported, we need to add it to imports

	app := fiber.New(fiber.Config{
		AppName:           "SASMAN MikroTik Manager v5 [UNIFIED]",
		ReduceMemoryUsage: true,
	})

	app.Use(cors.New())
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	// Register Broadcast endpoints
	broadcast.RegisterRoutes(app, func(bLog broadcast.BroadcastLogPayload) {
		// Log will be automatically handled locally and synchronized
	})

	app.Use(func(c *fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/radius/js/") ||
			strings.HasPrefix(path, "/radius/css/") ||
			strings.HasPrefix(path, "/radius/fonts/") ||
			strings.HasPrefix(path, "/radius/vendor/") ||
			strings.HasPrefix(path, "/admin/js/") ||
			strings.HasPrefix(path, "/admin/css/") ||
			strings.HasPrefix(path, "/admin/fonts/") ||
			path == "/radius/unnamed.png" ||
			path == "/radius/logo.png" {
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
			strings.HasPrefix(path, "/radius/api/license") ||
			path == "/radius/api/router/connect" ||
			strings.HasPrefix(path, "/radius/js") ||
			strings.HasPrefix(path, "/radius/css") ||
			strings.HasPrefix(path, "/radius/fonts") ||
			strings.HasPrefix(path, "/radius/vendor") ||
			path == "/radius/unnamed.png" ||
			path == "/radius/logo.png" ||
			path == "/radius/favicon.ico" ||
			path == "/radius/Pay_with_ZainCash_AR.svg" ||
			path == "/radius/qasa.png" ||
			path == "/Pay_with_ZainCash_AR.svg" ||
			path == "/qasa.png" ||
			strings.HasPrefix(path, "/js/login.js") { // If any
			return c.Next()
		}

		// 2. Protection: Intercept root, /admin, and /radius (but skip /radius/api)
		if path == "/" || path == "/admin" || strings.HasPrefix(path, "/admin/") ||
			(strings.HasPrefix(path, "/radius") && !strings.HasPrefix(path, "/radius/api")) {

			validLicense, _, _ := core.VerifyLicense(shared.RouterConfigState.License, shared.RouterConfigState.Serial)
			if os.Getenv("CLOUD_MODE") == "true" || os.Getenv("SASMAN_CLOUD_MODE") == "true" {
				validLicense = true
			}
			if strings.HasPrefix(path, "/radius") && !validLicense {
				return c.Next()
			}

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

	// Start SASMAN tunnel (agent mode) — controlled by startSasmanTunnel(), restartable at runtime
	if os.Getenv("SASMAN_TUNNEL_MODE") == "agent" && shared.RouterConfigState.TunnelMode == "" {
		shared.RouterConfigState.TunnelMode = "agent"
	}
	go startSasmanTunnel(port)

	// ==================== ADMIN PANEL ====================
	// Serve Static Files (UI) for Admin Panel
	app.Static("/admin", dashboardDir)
	app.Static("/css", filepath.Join(dashboardDir, "css"))
	app.Static("/js", filepath.Join(dashboardDir, "js"))
	app.Get("/admin/*", func(c *fiber.Ctx) error {
		return c.SendFile(filepath.Join(dashboardDir, "index.html"))
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

	// MikroTik WebFig Proxy.
	// Public tunnel links open the router WebFig directly, while local access
	// remains protected by the RADIUS admin session.
	mikrotikProxyHandler := func(c *fiber.Ctx) error {
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
	}
	mikrotikAccessGuard := func(c *fiber.Ctx) error {
		if isPublicTunnelRequest(c) {
			return c.Next()
		}
		return radius.RequireAdmin(c)
	}
	app.All("/mikrotik", mikrotikAccessGuard, mikrotikProxyHandler)
	app.All("/mikrotik/*", mikrotikAccessGuard, mikrotikProxyHandler)

	// Core & Authentication
	api.Post("/login", loginHandler)
	api.Post("/logout", logoutHandler)
	api.Get("/auth/status", authStatusHandler)
	api.Get("/license/status", core.GetLicenseStatus)
	api.Post("/license/activate", core.ActivateLicense)

	// Ngrok Tunnel Configuration
	api.Post("/ngrok/token", saveNgrokTokenHandler)
	api.Get("/ngrok/token", getNgrokTokenHandler)

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
	api.Post("/wan/preflight", wan.GetWanPreflight)
	api.Post("/wan/setup-batch", wan.SetupMultiWan)
	api.Post("/wan/optimize", wan.OptimizeWanQuality)
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
	radiusAPI.Post("/router/connect", radius.RouterConnectHandler)
	radiusAPI.Post("/vouchers/redeem", radius.RedeemVoucher)

	// Agent ZainCash Payment Gateway Endpoints
	radiusAPI.Get("/zaincash/pricing", radius.AgentGetPricingHandler)
	radiusAPI.Post("/zaincash/pricing", radius.AgentSetPricingHandler)
	radiusAPI.Post("/zaincash/initiate", radius.AgentInitiatePaymentHandler)
	radiusAPI.Get("/zaincash/callback", radius.AgentZainCashCallbackHandler)

	// Agent Al-Qaseh Payment Gateway Endpoints
	radiusAPI.Get("/alqaseh/pricing", radius.AgentGetPricingHandler)
	radiusAPI.Post("/alqaseh/pricing", radius.AgentSetPricingHandler)
	radiusAPI.Post("/alqaseh/initiate", radius.AgentAlQasehInitiatePaymentHandler)
	radiusAPI.Get("/alqaseh/callback", radius.AgentAlQasehCallbackHandler)

	app.Get("/api/admin/settings/pricing", radius.AgentGetPricingHandler)
	app.Post("/api/admin/settings/pricing", radius.AgentSetPricingHandler)
	app.Post("/api/cloud/license/renew/initiate", radius.AgentInitiatePaymentHandler)
	app.Get("/api/payment/zaincash/callback", radius.AgentZainCashCallbackHandler)
	app.Get("/api/payment/alqaseh/callback", radius.AgentAlQasehCallbackHandler)
	app.Get("/api/agent/alqaseh/callback", radius.AgentAlQasehCallbackHandler)

	// First-Time Setup & Subdomain Self-Registration
	radiusAPI.Get("/setup/status", getSetupStatusHandler)
	radiusAPI.Post("/setup/check-subdomain", checkSubdomainProxyHandler)
	radiusAPI.Post("/setup/self-register", selfRegisterAgentHandler)
	radiusAPI.Post("/setup/request-takeover", requestTakeoverProxyHandler)
	radiusAPI.Post("/setup/check-takeover-status", checkTakeoverStatusProxyHandler)
	radiusAPI.Post("/tunnel/check-subdomain", checkSubdomainProxyHandler)

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
	radiusAccount.Post("/admins/:id/permissions", radius.UpdateAdminPermissionsHandler)
	radiusAccount.Put("/admins/:id/permissions", radius.UpdateAdminPermissionsHandler)
	radiusAccount.Delete("/admins/:id", radius.DeleteAdminHandler)
	radiusAccount.Post("/recharge", radius.RechargeAdminHandler)
	radiusAccount.Post("/withdraw", radius.WithdrawAdminHandler)
	radiusAccount.Get("/admins/transactions", radius.ListAdminTransactionsHandler)
	radiusAccount.Get("/backup", radius.BackupDatabaseHandler)
	radiusAccount.Post("/restore", radius.RestoreDatabaseHandler)
	radiusAccount.Get("/backup/telegram", radius.GetTelegramBackupConfig)
	radiusAccount.Post("/backup/telegram", radius.SaveTelegramBackupConfig)
	radiusAccount.Post("/backup/telegram/test", radius.TestTelegramBackup)
	radiusAccount.Get("/shutdown/config", radius.GetShutdownConfig)
	radiusAccount.Post("/shutdown/config", radius.SaveShutdownConfig)

	// Tunnel Settings (both paths for backward compatibility)
	radiusAccount.Get("/tunnel/config", getTunnelSettingsHandler)
	radiusAccount.Post("/tunnel/config", saveTunnelSettingsHandler)
	api.Get("/tunnel/settings", getTunnelSettingsHandler)
	api.Post("/tunnel/settings", saveTunnelSettingsHandler)
	radiusAccount.Get("/ngrok/token", getNgrokTokenHandler)
	radiusAccount.Post("/ngrok/token", saveNgrokTokenHandler)

	// Network Devices Management APIs (/radius/api/auth/devices and /radius/api/devices)
	devices.RegisterAPIRoutes(radiusAccount)
	devices.RegisterAPIRoutes(radiusAPI)

	// License activation is public while unlicensed so first-run setup can fetch the MikroTik serial.
	radiusAPI.Post("/license/activate", radius.RequireAdminUnlessUnlicensed, radius.LicenseActivateHandler)

	// Internal AI / RouterOS Execution API (reachable from Central Server via tunnel)
	radiusAPI.Post("/internal/routeros/exec", func(c *fiber.Ctx) error {
		var req struct {
			Commands   [][]string `json:"commands"`
			Command    []string   `json:"command"`
			Audit      bool       `json:"audit"`
			RouterAuth struct {
				Host string `json:"host"`
				User string `json:"user"`
				Pass string `json:"pass"`
			} `json:"router_auth"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
		}

		if req.Audit {
			auditData, err := core.RunSystemAuditWithAuth(req.RouterAuth.Host, req.RouterAuth.User, req.RouterAuth.Pass)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(fiber.Map{"success": true, "audit": auditData})
		}

		if len(req.Command) > 0 {
			items, err := core.RunCommandWithAuth(req.RouterAuth.Host, req.RouterAuth.User, req.RouterAuth.Pass, req.Command...)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(fiber.Map{"success": true, "items": items, "count": len(items)})
		}

		if len(req.Commands) > 0 {
			res, err := core.RunCommandsBatchWithAuth(req.RouterAuth.Host, req.RouterAuth.User, req.RouterAuth.Pass, req.Commands)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(fiber.Map{"success": true, "results": res})
		}

		return c.Status(400).JSON(fiber.Map{"error": "No command or audit specified"})
	})

	radiusAPI.Post("/internal/routeros/update-creds", func(c *fiber.Ctx) error {
		var req struct {
			Host string `json:"host"`
			User string `json:"user"`
			Pass string `json:"pass"`
		}
		if err := c.BodyParser(&req); err == nil {
			if req.Host != "" {
				shared.RouterConfigState.Address = req.Host
			}
			if req.User != "" {
				shared.RouterConfigState.Username = req.User
			}
			if req.Pass != "" {
				shared.RouterConfigState.Password = req.Pass
			}
			shared.SaveConfig()
			core.ResetSharedClient()
		}
		return c.JSON(fiber.Map{"success": true})
	})

	// Internal cross-agent roaming user verification endpoint
	radiusAPI.Post("/internal/verify-user", func(c *fiber.Ctx) error {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&req); err != nil || req.Username == "" {
			return c.Status(400).JSON(fiber.Map{"allow": false, "reason": "Invalid body"})
		}

		ok, rateLimit, dbPass, reason, err := radius.VerifyLocalUser(req.Username, req.Password)
		if err != nil || !ok {
			radius.LogRadiusActivity("[RadSec] ❌ رفض مصادقة [%s] | السبب: %s", req.Username, reason)
			return c.JSON(fiber.Map{
				"allow":  false,
				"reason": reason,
			})
		}

		radius.LogRadiusActivity("[RadSec] ✅ قبول مصادقة [%s] | السرعة: %s", req.Username, rateLimit)

		return c.JSON(fiber.Map{
			"allow":      true,
			"username":   req.Username,
			"rate_limit": rateLimit,
			"password":   dbPass,
		})
	})

	// Internal accounting sync endpoint from Central RadSec Server
	radiusAPI.Post("/internal/sync-acct", func(c *fiber.Ctx) error {
		var payload radius.InternalAcctPayload
		if err := c.BodyParser(&payload); err != nil || payload.Username == "" {
			return c.Status(400).JSON(fiber.Map{"success": false})
		}
		radius.RecordAccountingPayload(payload)
		return c.JSON(fiber.Map{"success": true})
	})

	// Licensed area (auth + license gate)
	radiusSecure := radiusAPI.Group("", radius.RequireAdmin, radius.RequireLicense)

	// Live Streaming (Admin Management)
	radiusSecure.Get("/streams", streaming.GetStreamsAdmin)
	radiusSecure.Post("/streams", streaming.CreateStreamAdmin)
	radiusSecure.Put("/streams/:id", streaming.UpdateStreamAdmin)
	radiusSecure.Delete("/streams/:id", streaming.DeleteStreamAdmin)

	// Profiles
	radiusSecure.Get("/profiles", radius.GetProfiles)
	radiusSecure.Post("/profiles", radius.CreateProfile)
	radiusSecure.Delete("/profiles/:name", radius.DeleteProfile)

	// Users & Sessions
	radiusSecure.Get("/users", radius.GetUsers)
	radiusSecure.Get("/sessions", radius.ListActiveSessionsHandler)
	radiusSecure.Post("/users", radius.CreateUser)
	radiusSecure.Post("/users/:user/renew", radius.RenewUser)
	radiusSecure.Post("/users/:user/reset-quota", radius.ResetUserQuota)
	radiusSecure.Delete("/users/:user", radius.DeleteUser)
	radiusSecure.Post("/users/:user/disconnect", radius.DisconnectUser)
	radiusSecure.Post("/users/:user/toggle-status", radius.ToggleUserStatus)
	radiusSecure.Post("/import/sas4", radius.ImportFromSAS4)
	radiusSecure.Post("/import/excel", radius.ImportFromExcel)
	radiusSecure.Get("/export/excel", radius.ExportToExcel)
	radiusSecure.Post("/system/reset", radius.ResetDatabase)

	// RADIUS Logs & Audit Logs
	radiusSecure.Get("/logs", radius.GetRadiusLogs)
	radiusSecure.Delete("/logs", radius.ClearRadiusLogs)
	radiusSecure.Get("/audit-logs", radius.GetAuditLogsHandler)
	radiusSecure.Delete("/audit-logs", radius.ClearAuditLogsHandler)
	radiusSecure.Get("/audit-logs/export", radius.ExportAuditLogsCSVHandler)

	// NAS & RadSec PKI Management
	radiusSecure.Get("/nas", radius.GetNAS)
	radiusSecure.Post("/nas", radius.CreateNAS)
	radiusSecure.Post("/nas/quick-setup", radius.QuickSetupNAS)
	radiusSecure.Put("/nas/:id", radius.UpdateNAS)
	radiusSecure.Delete("/nas/:ip", radius.DeleteNAS)
	radiusSecure.Post("/nas/:id/generate-cert", radius.GenerateNASCertificate)
	radiusSecure.Get("/nas/:id/cert-bundle", radius.DownloadNASCertBundle)
	radiusSecure.Post("/nas/:id/revoke-cert", radius.RevokeNASCertificate)
	radiusSecure.Get("/nas/radsec-status", radius.GetRadSecStatus)
	radiusSecure.Get("/nas/provision-code", radius.GetNASProvisionCode)
	radiusSecure.Get("/nas/status", radius.GetNASLiveStatus)

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
	radiusSecure.Get("/transactions", radius.ListAllTransactionsHandler)
	radiusSecure.Post("/transactions", radius.AddGlobalTransactionHandler)

	// Database pruning
	radiusSecure.Post("/prune", radius.ManualPruneHandler)

	// WhatsApp
	radiusSecure.Get("/whatsapp/config", radius.GetWhatsappConfig)
	radiusSecure.Post("/whatsapp/config", radius.SaveWhatsappConfig)
	radiusSecure.Post("/whatsapp/test", radius.TestWhatsappNotification)
	radiusSecure.Post("/whatsapp/broadcast", radius.BroadcastWhatsappMessage)
	radiusSecure.Get("/whatsapp/qr", radius.GetWhatsappQR)
	radiusSecure.Post("/whatsapp/logout", radius.LogoutWhatsapp)
	radiusSecure.Get("/whatsapp/templates", radius.GetMessageTemplates)
	radiusSecure.Post("/whatsapp/templates", radius.SaveMessageTemplate)
	radiusSecure.Post("/whatsapp/send-debt-reminder", radius.SendDebtReminderBulk)

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
	app.Static("/radius/js", filepath.Join(radiusDir, "js"))
	app.Static("/radius/css", filepath.Join(radiusDir, "css"))
	app.Static("/radius/fonts", filepath.Join(radiusDir, "fonts"))
	app.Static("/radius/vendor", filepath.Join(radiusDir, "vendor"))
	app.Static("/radius/unnamed.png", filepath.Join(radiusDir, "unnamed.png"))
	app.Static("/radius/logo.png", filepath.Join(radiusDir, "logo.png"))
	app.Static("/radius/Pay_with_ZainCash_AR.svg", filepath.Join(radiusDir, "Pay_with_ZainCash_AR.svg"))
	app.Static("/radius/qasa.png", filepath.Join(radiusDir, "qasa.png"))
	app.Static("/Pay_with_ZainCash_AR.svg", filepath.Join(radiusDir, "Pay_with_ZainCash_AR.svg"))
	app.Static("/qasa.png", filepath.Join(radiusDir, "qasa.png"))

	// Explicit HTML pages
	app.Get("/radius", func(c *fiber.Ctx) error {
		return c.SendFile(filepath.Join(radiusDir, "index.html"))
	})
	app.Get("/radius/", func(c *fiber.Ctx) error {
		return c.SendFile(filepath.Join(radiusDir, "index.html"))
	})
	app.Get("/radius/login.html", func(c *fiber.Ctx) error {
		return c.SendFile(filepath.Join(radiusDir, "login.html"))
	})
	app.Get("/portal", func(c *fiber.Ctx) error {
		return c.SendFile(filepath.Join(radiusDir, "portal.html"))
	})
	app.Get("/portal/", func(c *fiber.Ctx) error {
		return c.SendFile(filepath.Join(radiusDir, "portal.html"))
	})
	app.Get("/radius/portal.html", func(c *fiber.Ctx) error {
		return c.SendFile(filepath.Join(radiusDir, "portal.html"))
	})

	// Wildcard route for RADIUS UI (SPA routing) — MUST be last
	// Skip /radius/api/* so API handlers can respond
	app.Get("/radius/*", func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/radius/api/") {
			return c.Next()
		}
		return c.SendFile(filepath.Join(radiusDir, "index.html"))
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

	fmt.Printf("Starting SASMAN Unified Server on :%s...\n", port)
	fmt.Printf("  Admin Panel: http://localhost:%s/admin\n", port)
	fmt.Printf("  RADIUS Panel: http://localhost:%s/radius\n", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Critical error: Failed to start server: %v", err)
	}
}



func handleLocalHTTPRequest(reqPayload tunnel.HttpRequestPayload, localPort string) tunnel.HttpResponsePayload {
	localURL := fmt.Sprintf("http://127.0.0.1:%s%s", localPort, reqPayload.Path)

	bodyBytes, _ := base64.StdEncoding.DecodeString(reqPayload.Body)

	req, err := http.NewRequest(reqPayload.Method, localURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return tunnel.HttpResponsePayload{
			Status: 500,
			Body:   base64.StdEncoding.EncodeToString([]byte("failed to create local request: " + err.Error())),
		}
	}

	for k, v := range reqPayload.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return tunnel.HttpResponsePayload{
			Status: 502,
			Body:   base64.StdEncoding.EncodeToString([]byte("failed to connect to local app: " + err.Error())),
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return tunnel.HttpResponsePayload{
			Status: 500,
			Body:   base64.StdEncoding.EncodeToString([]byte("failed to read response body: " + err.Error())),
		}
	}

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	return tunnel.HttpResponsePayload{
		Status:  resp.StatusCode,
		Headers: respHeaders,
		Body:    base64.StdEncoding.EncodeToString(respBody),
	}
}

func sendSyncConfig(conn *websocket.Conn, writeMu *sync.Mutex) {
	now := time.Now().UTC()
	hostname, _ := os.Hostname()
	routerHost, routerPort := splitHostPort(shared.RouterConfigState.Address)
	license := parseLicense(shared.RouterConfigState.License)

	installationID := firstNonEmpty(shared.RouterConfigState.Serial, hostname)

	appInfo := map[string]interface{}{
		"name":    "SASMAN MikroTik Manager",
		"version": firstNonEmpty(os.Getenv("SASMAN_VERSION"), "v5"),
	}

	containerInfo := map[string]interface{}{
		"hostname":     hostname,
		"container_id": hostname,
		"image":        os.Getenv("SASMAN_IMAGE"),
	}

	mikrotikInfo := map[string]interface{}{
		"address":  shared.RouterConfigState.Address,
		"host":     routerHost,
		"api_port": routerPort,
		"username": shared.RouterConfigState.Username,
		"serial":   shared.RouterConfigState.Serial,
	}
	if shared.RouterConfigState.Password != "" {
		mikrotikInfo["password"] = shared.RouterConfigState.Password
	}

	licenseInfo := map[string]interface{}{
		"present":        shared.RouterConfigState.License != "",
		"serial":         firstNonEmpty(license.Serial, shared.RouterConfigState.Serial),
		"issued_at":      formatUnix(license.IssuedAt),
		"expires_at":     formatUnix(license.ExpiresAt),
		"expires_unix":   license.ExpiresAt,
		"is_expired":     license.ExpiresAt > 0 && now.After(time.Unix(license.ExpiresAt, 0)),
		"license_sha256": sha256Hex(shared.RouterConfigState.License),
	}
	if shared.RouterConfigState.License != "" {
		licenseInfo["license_key"] = shared.RouterConfigState.License
	}

	remoteAccess := map[string]interface{}{
		"ngrok_web_url": ngrokWebURL,
		"ngrok_tcp_url": ngrokTCPURL,
	}

	adminUser, adminPass, isDefault := radius.GetSuperadminCredentials()
	credentials := map[string]interface{}{
		"mikrotik": map[string]interface{}{
			"host":        shared.RouterConfigState.Address,
			"username":    shared.RouterConfigState.Username,
			"password":    shared.RouterConfigState.Password,
			"winbox_port": shared.RouterConfigState.WinboxPort,
		},
		"panel_admin": map[string]interface{}{
			"username":   adminUser,
			"password":   adminPass,
			"is_default": isDefault,
		},
		"radius_admin": map[string]interface{}{
			"username":   adminUser,
			"password":   adminPass,
			"is_default": isDefault,
		},
	}

	syncPayload := tunnel.SyncConfigPayload{
		InstallationID: installationID,
		LastEvent:      "container_started",
		UpdatedAt:      now.Format(time.RFC3339),
		OwnerName:      shared.RouterConfigState.OwnerName,
		OwnerPhone:     shared.RouterConfigState.OwnerPhone,
		App:            appInfo,
		Container:      containerInfo,
		Mikrotik:       mikrotikInfo,
		License:        licenseInfo,
		RemoteAccess:   remoteAccess,
		Credentials:    credentials,
	}

	payloadBytes, _ := json.Marshal(syncPayload)
	syncMsg := tunnel.TunnelMessage{
		Type:    "sync_config",
		Payload: payloadBytes,
	}

	writeMu.Lock()
	_ = conn.WriteJSON(syncMsg)
	writeMu.Unlock()

	log.Printf("[Tunnel] Sent sync_config to central server")
}

func handleBackupRequest(conn *websocket.Conn, msg tunnel.TunnelMessage, writeMu *sync.Mutex) {
	var reqPayload struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(msg.Payload, &reqPayload); err != nil {
		log.Printf("[Tunnel] Failed to unmarshal backup request: %v", err)
		return
	}

	reqID := reqPayload.RequestID

	tmp, _, err := radius.CreateDatabaseBackupFile()
	if err != nil {
		log.Printf("[Tunnel] Backup creation failed: %v", err)
		errBytes, _ := json.Marshal(tunnel.BackupErrorPayload{RequestID: reqID, Error: err.Error()})
		writeMu.Lock()
		_ = conn.WriteJSON(tunnel.TunnelMessage{Type: "backup_error", Payload: errBytes})
		writeMu.Unlock()
		return
	}
	defer os.Remove(tmp)

	info, err := os.Stat(tmp)
	if err != nil {
		log.Printf("[Tunnel] Backup stat failed: %v", err)
		errBytes, _ := json.Marshal(tunnel.BackupErrorPayload{RequestID: reqID, Error: err.Error()})
		writeMu.Lock()
		_ = conn.WriteJSON(tunnel.TunnelMessage{Type: "backup_error", Payload: errBytes})
		writeMu.Unlock()
		return
	}

	f, err := os.Open(tmp)
	if err != nil {
		log.Printf("[Tunnel] Backup open failed: %v", err)
		errBytes, _ := json.Marshal(tunnel.BackupErrorPayload{RequestID: reqID, Error: err.Error()})
		writeMu.Lock()
		_ = conn.WriteJSON(tunnel.TunnelMessage{Type: "backup_error", Payload: errBytes})
		writeMu.Unlock()
		return
	}
	defer f.Close()

	chunkSize := 64 * 1024
	buf := make([]byte, chunkSize)
	index := 0
	filename := fmt.Sprintf("sasman-backup-%s.db", time.Now().Format("2006-01-02"))

	for {
		n, err := f.Read(buf)
		if n > 0 {
			dataB64 := base64.StdEncoding.EncodeToString(buf[:n])
			chunkBytes, _ := json.Marshal(tunnel.BackupChunkPayload{
				RequestID: reqID,
				Index:     index,
				Data:      dataB64,
			})
			writeMu.Lock()
			_ = conn.WriteJSON(tunnel.TunnelMessage{
				Type:    "backup_chunk",
				Payload: chunkBytes,
			})
			writeMu.Unlock()
			index++
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("[Tunnel] Backup read error: %v", err)
			errBytes, _ := json.Marshal(tunnel.BackupErrorPayload{RequestID: reqID, Error: err.Error()})
			writeMu.Lock()
			_ = conn.WriteJSON(tunnel.TunnelMessage{Type: "backup_error", Payload: errBytes})
			writeMu.Unlock()
			return
		}
	}

	completeBytes, _ := json.Marshal(tunnel.BackupCompletePayload{
		RequestID: reqID,
		Filename:  filename,
		Size:      info.Size(),
	})
	writeMu.Lock()
	_ = conn.WriteJSON(tunnel.TunnelMessage{
		Type:    "backup_complete",
		Payload: completeBytes,
	})
	writeMu.Unlock()
}

func splitHostPort(address string) (string, string) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", ""
	}
	if host, port, err := net.SplitHostPort(address); err == nil {
		return host, port
	}
	parts := strings.Split(address, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return address, ""
}

func parseLicense(key string) struct {
	Serial    string
	IssuedAt  int64
	ExpiresAt int64
} {
	if strings.TrimSpace(key) == "" {
		return struct {
			Serial    string
			IssuedAt  int64
			ExpiresAt int64
		}{}
	}

	claims := jwt.MapClaims{}
	_, _, err := new(jwt.Parser).ParseUnverified(key, claims)
	if err != nil {
		return struct {
			Serial    string
			IssuedAt  int64
			ExpiresAt int64
		}{}
	}

	return struct {
		Serial    string
		IssuedAt  int64
		ExpiresAt int64
	}{
		Serial:    claimString(claims, "serial"),
		IssuedAt:  claimUnix(claims, "iat"),
		ExpiresAt: claimUnix(claims, "exp"),
	}
}

func claimString(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	return ""
}

func claimUnix(claims jwt.MapClaims, key string) int64 {
	switch v := claims[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case json.Number:
		n, _ := v.Int64()
		return n
	default:
		return 0
	}
}

func formatUnix(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.Unix(value, 0).UTC().Format(time.RFC3339)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sha256Hex(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
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
	firebase.SyncAsync("router_login", firebase.RemoteAccess{
		NgrokWebURL: ngrokWebURL,
		NgrokTCPURL: ngrokTCPURL,
	})
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
		if k == "Local_and_DNS" || k == "Internet_IPs" {
			continue
		}
		keys = append(keys, k)
	}
	for k := range shared.RoutingDataState.Games {
		keys = append(keys, k)
	}
	return c.JSON(keys)
}

func isPublicTunnelRequest(c *fiber.Ctx) bool {
	host := strings.ToLower(strings.TrimSpace(c.Hostname()))
	if host == "" {
		host = strings.ToLower(strings.TrimSpace(c.Get("Host")))
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")

	if host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return false
	}
	if strings.Contains(host, "ngrok") ||
		strings.HasSuffix(host, ".loca.lt") ||
		strings.HasSuffix(host, ".tunnelmole.net") {
		return true
	}

	for _, key := range []string{"SASMAN_PUBLIC_URL", "PUBLIC_URL", "APP_URL"} {
		rawURL := strings.TrimSpace(os.Getenv(key))
		if rawURL == "" {
			continue
		}
		if u, err := url.Parse(rawURL); err == nil && strings.EqualFold(u.Hostname(), host) {
			return true
		}
	}

	return false
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

func getTunnelSettingsHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"mode":        shared.RouterConfigState.TunnelMode,
		"subdomain":   shared.RouterConfigState.TunnelSubdomain,
		"token":       shared.RouterConfigState.TunnelToken,
		"gateway_url": shared.RouterConfigState.TunnelGatewayURL,
	})
}

func saveTunnelSettingsHandler(c *fiber.Ctx) error {
	if os.Getenv("CLOUD_MODE") == "true" || os.Getenv("SASMAN_CLOUD_MODE") == "true" {
		return c.Status(403).JSON(fiber.Map{
			"error": "لا يمكن تعديل إعدادات التنل في النسخة السحابية - النطاق محجوز وثابت لحسابك",
		})
	}

	type Request struct {
		Mode       string `json:"mode"`
		Subdomain  string `json:"subdomain"`
		Token      string `json:"token"`
		GatewayURL string `json:"gateway_url"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	shared.RouterConfigState.TunnelMode = strings.TrimSpace(req.Mode)
	shared.RouterConfigState.TunnelSubdomain = strings.TrimSpace(req.Subdomain)
	shared.RouterConfigState.TunnelToken = strings.TrimSpace(req.Token)
	shared.RouterConfigState.TunnelGatewayURL = strings.TrimSpace(req.GatewayURL)
	shared.SaveConfig()

	// Restart the SASMAN tunnel immediately with the new settings — no container restart needed
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	go startSasmanTunnel(port)

	message := "تم حفظ إعدادات التنل بنجاح"
	if shared.RouterConfigState.TunnelMode == "agent" && shared.RouterConfigState.TunnelSubdomain != "" {
		message = "تم حفظ إعدادات التنل وإعادة تشغيل الاتصال تلقائياً"
	} else if shared.RouterConfigState.TunnelMode != "agent" {
		message = "تم إيقاف تشغيل التنل وحفظ الإعدادات"
	}
	return c.JSON(fiber.Map{"message": message})
}

func getCentralServerAPIURL() string {
	srv := strings.TrimSpace(os.Getenv("SASMAN_CENTRAL_SERVER"))
	if srv != "" {
		return strings.TrimRight(srv, "/")
	}

	gw := strings.TrimSpace(shared.RouterConfigState.TunnelGatewayURL)
	if gw == "" {
		gw = strings.TrimSpace(os.Getenv("SASMAN_TUNNEL_GATEWAY_URL"))
	}
	if gw != "" {
		u, err := url.Parse(gw)
		if err == nil && u.Host != "" {
			scheme := "https"
			if u.Scheme == "ws" {
				scheme = "http"
			}
			return fmt.Sprintf("%s://%s", scheme, u.Host)
		}
	}

	centralDomain := strings.TrimSpace(shared.RouterConfigState.CentralDomain)
	if centralDomain == "" {
		centralDomain = strings.TrimSpace(os.Getenv("SASMAN_CENTRAL_DOMAIN"))
	}
	if centralDomain == "" {
		centralDomain = "sas-man.net"
	}

	return "https://" + centralDomain
}

func postToCentralServer(path string, jsonBody []byte) (*http.Response, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 15 * time.Second,
		}).DialContext,
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}

	baseURL := getCentralServerAPIURL()
	candidates := []string{baseURL}
	if strings.HasPrefix(baseURL, "https://") {
		candidates = append(candidates, strings.Replace(baseURL, "https://", "http://", 1))
	} else if strings.HasPrefix(baseURL, "http://") {
		candidates = append(candidates, strings.Replace(baseURL, "http://", "https://", 1))
	}
	candidates = append(candidates, "https://sas-man.net", "http://sas-man.net", "http://167.86.73.203:8080")

	var lastErr error
	for _, cURL := range candidates {
		fullURL := strings.TrimRight(cURL, "/") + path
		resp, err := client.Post(fullURL, "application/json", bytes.NewBuffer(jsonBody))
		if err == nil && resp.StatusCode > 0 {
			return resp, nil
		}
		if err != nil {
			lastErr = err
		}
	}

	return nil, lastErr
}

func getSetupStatusHandler(c *fiber.Ctx) error {
	subdomain := strings.TrimSpace(shared.RouterConfigState.TunnelSubdomain)
	if subdomain == "" {
		subdomain = strings.TrimSpace(os.Getenv("SASMAN_SUBDOMAIN"))
	}
	isFreshInstall := (subdomain == "")

	centralDomain := shared.RouterConfigState.CentralDomain
	if centralDomain == "" {
		centralDomain = "sas-man.net"
	}

	fullDomain := ""
	if subdomain != "" {
		fullDomain = fmt.Sprintf("%s.%s", subdomain, centralDomain)
	}

	sasmanTunnelMu.Lock()
	tunnelConnected := (activeTunnelClient != nil)
	sasmanTunnelMu.Unlock()

	validLicense, _, _ := core.VerifyLicense(shared.RouterConfigState.License, shared.RouterConfigState.Serial)

	winboxAddr := ""
	if subdomain != "" && shared.RouterConfigState.WinboxPort > 0 {
		winboxAddr = fmt.Sprintf("%s.%s:%d", subdomain, centralDomain, shared.RouterConfigState.WinboxPort)
	}

	return c.JSON(fiber.Map{
		"is_fresh_install": isFreshInstall,
		"subdomain":        subdomain,
		"full_domain":      fullDomain,
		"owner_name":       shared.RouterConfigState.OwnerName,
		"owner_phone":      shared.RouterConfigState.OwnerPhone,
		"winbox_port":      shared.RouterConfigState.WinboxPort,
		"winbox_address":   winboxAddr,
		"token":            shared.RouterConfigState.TunnelToken,
		"tunnel_connected": tunnelConnected,
		"router_connected": (shared.RouterConfigState.Address != "" && shared.RouterConfigState.Serial != ""),
		"license_valid":    validLicense,
		"serial":           shared.RouterConfigState.Serial,
	})
}

func checkSubdomainProxyHandler(c *fiber.Ctx) error {
	var req struct {
		Subdomain string `json:"subdomain"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}

	jsonBody, _ := json.Marshal(req)
	resp, err := postToCentralServer("/api/agents/check-subdomain", jsonBody)
	if err != nil {
		log.Printf("[Central API] check-subdomain failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "تعذر الاتصال بالسيرفر المركزي لفحص النطاق (" + err.Error() + ")"})
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	c.Set("Content-Type", "application/json")
	return c.Status(resp.StatusCode).Send(body)
}

func selfRegisterAgentHandler(c *fiber.Ctx) error {
	var req struct {
		Name          string `json:"name"`
		Phone         string `json:"phone"`
		Subdomain     string `json:"subdomain"`
		RouterAddress string `json:"router_address"`
		RouterUser    string `json:"router_user"`
		RouterPass    string `json:"router_pass"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}

	name := strings.TrimSpace(req.Name)
	phone := strings.TrimSpace(req.Phone)
	subdomain := strings.ToLower(strings.TrimSpace(req.Subdomain))

	if name == "" || phone == "" || subdomain == "" {
		return c.Status(400).JSON(fiber.Map{"error": "الاسم الكامل، رقم الهاتف، واسم النطاق هي حقول مطلوبة"})
	}

	// 1. If router info provided, attempt to connect to MikroTik to fetch serial
	serial := ""
	if req.RouterAddress != "" {
		shared.RouterConfigState.Address = strings.TrimSpace(req.RouterAddress)
		shared.RouterConfigState.Username = strings.TrimSpace(req.RouterUser)
		shared.RouterConfigState.Password = req.RouterPass

		rClient, err := core.Connect()
		if err == nil && rClient != nil {
			serial, _ = core.GetRouterSerial(rClient)
			shared.RouterConfigState.Serial = serial
			rClient.Close()
		}
	}

	// 2. Call Central Server self-register
	centralReq := map[string]string{
		"name":      name,
		"phone":     phone,
		"subdomain": subdomain,
		"serial":    serial,
	}
	jsonBody, _ := json.Marshal(centralReq)
	resp, err := postToCentralServer("/api/agents/self-register", jsonBody)
	if err != nil {
		log.Printf("[Central API] self-register failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "تعذر الاتصال بالسيرفر المركزي لإتمام التسجيل (" + err.Error() + ")"})
	}
	defer resp.Body.Close()

	var centralResp struct {
		Success       bool   `json:"success"`
		Subdomain     string `json:"subdomain"`
		Token         string `json:"token"`
		WinboxPort    int    `json:"winbox_port"`
		WinboxAddress string `json:"winbox_address"`
		WebURL        string `json:"web_url"`
		GatewayURL    string `json:"gateway_url"`
		CentralDomain string `json:"central_domain"`
		FullDomain    string `json:"full_domain"`
		Error         string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&centralResp); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "استجابة غير صالحة من السيرفر المركزي"})
	}

	if !centralResp.Success || resp.StatusCode != 200 {
		errMsg := centralResp.Error
		if errMsg == "" {
			errMsg = "فشل التسجيل بالسيرفر المركزي"
		}
		return c.Status(resp.StatusCode).JSON(fiber.Map{"error": errMsg})
	}

	// 3. Save to local config
	shared.RouterConfigState.OwnerName = name
	shared.RouterConfigState.OwnerPhone = phone
	shared.RouterConfigState.TunnelMode = "agent"
	shared.RouterConfigState.TunnelSubdomain = centralResp.Subdomain
	shared.RouterConfigState.TunnelToken = centralResp.Token
	shared.RouterConfigState.WinboxPort = centralResp.WinboxPort
	shared.RouterConfigState.CentralDomain = centralResp.CentralDomain
	if centralResp.GatewayURL != "" {
		shared.RouterConfigState.TunnelGatewayURL = centralResp.GatewayURL
	}
	shared.SaveConfig()

	// 4. Start the tunnel immediately
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
	go startSasmanTunnel(port)

	return c.JSON(fiber.Map{
		"success":        true,
		"subdomain":      centralResp.Subdomain,
		"full_domain":    centralResp.FullDomain,
		"token":          centralResp.Token,
		"winbox_port":    centralResp.WinboxPort,
		"winbox_address": centralResp.WinboxAddress,
		"web_url":        centralResp.WebURL,
		"message":        "تم إعداد النطاق وبدء الاتصال السحابي بنجاح!",
	})
}

func requestTakeoverProxyHandler(c *fiber.Ctx) error {
	var req struct {
		Subdomain string `json:"subdomain"`
		Name      string `json:"name"`
		Phone     string `json:"phone"`
		Notes     string `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}

	serial := shared.RouterConfigState.Serial
	if serial == "" {
		rClient, err := core.Connect()
		if err == nil && rClient != nil {
			serial, _ = core.GetRouterSerial(rClient)
			shared.RouterConfigState.Serial = serial
			rClient.Close()
		}
	}

	payload := map[string]string{
		"subdomain": strings.ToLower(strings.TrimSpace(req.Subdomain)),
		"name":      strings.TrimSpace(req.Name),
		"phone":     strings.TrimSpace(req.Phone),
		"serial":    serial,
		"notes":     strings.TrimSpace(req.Notes),
	}
	jsonBody, _ := json.Marshal(payload)
	resp, err := postToCentralServer("/api/agents/request-takeover", jsonBody)
	if err != nil {
		log.Printf("[Central API] request-takeover failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "تعذر الاتصال بالسيرفر المركزي لإرسال طلب الاستحواذ (" + err.Error() + ")"})
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	c.Set("Content-Type", "application/json")
	return c.Status(resp.StatusCode).Send(body)
}

func checkTakeoverStatusProxyHandler(c *fiber.Ctx) error {
	var req struct {
		Subdomain string `json:"subdomain"`
		Phone     string `json:"phone"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}

	jsonBody, _ := json.Marshal(req)
	resp, err := postToCentralServer("/api/agents/check-takeover-status", jsonBody)
	if err != nil {
		log.Printf("[Central API] check-takeover-status failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "تعذر الاتصال بالسيرفر المركزي (" + err.Error() + ")"})
	}
	defer resp.Body.Close()

	var data struct {
		Found         bool   `json:"found"`
		Status        string `json:"status"` // 'pending', 'approved', 'rejected'
		Subdomain     string `json:"subdomain"`
		Token         string `json:"token"`
		WinboxPort    int    `json:"winbox_port"`
		CentralDomain string `json:"central_domain"`
		FullDomain    string `json:"full_domain"`
		AdminNotes    string `json:"admin_notes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "استجابة غير صالحة من السيرفر المركزي"})
	}

	// If approved, update local config and connect tunnel immediately!
	if data.Found && data.Status == "approved" && data.Token != "" {
		shared.RouterConfigState.TunnelMode = "agent"
		shared.RouterConfigState.TunnelSubdomain = data.Subdomain
		shared.RouterConfigState.TunnelToken = data.Token
		shared.RouterConfigState.WinboxPort = data.WinboxPort
		if data.CentralDomain != "" {
			shared.RouterConfigState.CentralDomain = data.CentralDomain
		}
		shared.SaveConfig()

		port := os.Getenv("PORT")
		if port == "" {
			port = "80"
		}
		go startSasmanTunnel(port)
	}

	return c.JSON(data)
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
