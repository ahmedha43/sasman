package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"mikrotik-manager/pkg/broadcast"
	"mikrotik-manager/pkg/pki"
	"mikrotik-manager/pkg/relay"
	"mikrotik-manager/pkg/tunnel"
	"mikrotik-manager/server/internal/api"
	aiinternal "mikrotik-manager/server/internal/ai"
	"mikrotik-manager/server/internal/backup"
	"mikrotik-manager/server/internal/cloudtenant"
	otainternal "mikrotik-manager/server/internal/ota"
	"mikrotik-manager/server/internal/radsec"
	relayinternal "mikrotik-manager/server/internal/relay"
	"mikrotik-manager/server/internal/storage"
)

//go:embed index.html
var landingHTML string

//go:embed web/* web/cloud/*
var webFS embed.FS

//go:embed hotspot_template/*
var hotspotFS embed.FS

func main() {
	dbPath := os.Getenv("SASMAN_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join("data", "sasman-central.db")
	}

	repo, err := storage.NewSQLiteRepository(dbPath)
	if err != nil {
		log.Fatalf("init sqlite repo: %v", err)
	}
	if err := repo.CreateSchema(); err != nil {
		log.Fatalf("create schema: %v", err)
	}

	svc := tunnel.NewService(repo.GetDB())
	svc.RestoreSessions()

	// Handle broadcast view/click logs from agents
	svc.OnBroadcastLog = func(bLog broadcast.BroadcastLogPayload) {
		_ = repo.LogBroadcastView(storage.BroadcastLog{
			BroadcastID:    bLog.BroadcastID,
			AgentID:        bLog.AgentID,
			UserIdentifier: bLog.UserIdentifier,
			ViewedAt:       bLog.ViewedAt,
			Clicked:        bLog.Clicked,
		})
	}

	// Initialize SASMAN Service Relay Control Plane
	relayCatalog, err := relayinternal.NewCatalogManager(repo.GetDB())
	if err != nil {
		log.Printf("[Relay] Failed to init relay catalog: %v", err)
	}
	relayTelemetry := relayinternal.NewTelemetryHub()
	relayRouter := relayinternal.NewRouterEngine(relayCatalog, relayTelemetry, func(table relay.AgentRoutingTable) {
		tableJSON, _ := json.Marshal(table)
		svc.BroadcastToAgents(tunnel.TunnelMessage{
			Type:    "route_table_push",
			Payload: tableJSON,
		})
	})
	relayRouter.Start()
	relayAPI := relayinternal.NewAPIHandler(relayCatalog, relayTelemetry, relayRouter)
	syncSingleAgentFunc := func(agentSubdomain string) error {
		if relayRouter == nil {
			return nil
		}
		services := relayRouter.GetServicesForAgent(agentSubdomain)
		if services == nil {
			services = []relay.ServiceDefinition{} // Send empty slice to clean up router rules
		}
		_ = svc.SendTunnelMessage(agentSubdomain, "catalog_sync", services)

		routes := relayRouter.GetCurrentRoutingTable()
		if len(routes.Routes) > 0 {
			_ = svc.SendTunnelMessage(agentSubdomain, "route_table_push", routes)
		}
		return nil
	}

	relayAPI.SetBroadcaster(
		func(catalog []relay.ServiceDefinition) {
			agents := svc.ListAgents()
			for _, a := range agents {
				if sub, ok := a["subdomain"].(string); ok && sub != "" {
					_ = syncSingleAgentFunc(sub)
				}
			}
		},
		func(serviceID string) {
			payload, _ := json.Marshal(map[string]string{"service_id": serviceID})
			svc.BroadcastToAgents(tunnel.TunnelMessage{
				Type:    "probe_request",
				Payload: payload,
			})
		},
		func() []string {
			agents := svc.ListAgents()
			out := make([]string, 0, len(agents))
			for _, a := range agents {
				if sub, ok := a["subdomain"].(string); ok && sub != "" {
					out = append(out, sub)
				}
			}
			return out
		},
		syncSingleAgentFunc,
	)

	// Auto-push active broadcasts, license lease, and catalog to newly registered/connected agents
	svc.OnAgentRegistered = func(subdomain string) {
		// 1. Push Cloud License Lease on connect
		if licInfo, err := repo.GetAgentLicenseInfo(subdomain); err == nil && licInfo != nil {
			_ = svc.SendTunnelMessage(subdomain, "license_lease", fiber.Map{
				"status":         licInfo.Status,
				"expires_at":     licInfo.ExpiresAtStr,
				"days_remaining": licInfo.DaysRemaining,
				"is_expired":     licInfo.IsExpired,
				"valid":          !licInfo.IsExpired && licInfo.Status == "active",
			})
		}

		// 2. Push active broadcasts
		activeBroadcasts, err := repo.GetActiveBroadcastsForAgent(subdomain)
		if err == nil {
			for _, b := range activeBroadcasts {
				msg := broadcast.BroadcastMessage{
					ID:                b.ID,
					Title:             b.Title,
					Message:           b.Message,
					ImageURL:          b.ImageURL,
					ActionURL:         b.ActionURL,
					ActionText:        b.ActionText,
					DisplayType:       b.DisplayType,
					TargetType:        b.TargetType,
					TargetProfiles:    b.TargetProfiles,
					Frequency:         b.Frequency,
					SplashDurationSec: b.SplashDurationSec,
					CreatedAt:         b.CreatedAt.Format(time.RFC3339),
				}
				_ = svc.SendTunnelMessage(subdomain, "broadcast_push", msg)
			}
		}

		// 3. Push targeted Service Catalog & Routes on connect for clean MikroTik RouterOS DNS/Firewall sync
		if relayRouter != nil {
			_ = syncSingleAgentFunc(subdomain)
		}

		// 4. Persist agent arch & version to DB on every connect so OTA sends the correct binary
		if session := svc.GetAgentBySubdomain(subdomain); session != nil {
			rawArch := session.Arch
			ver := session.Version
			if rawArch != "" {
				normalizedArch := normalizeArch(rawArch)
				if ver == "" {
					ver = "v5.0.0"
				}
				if dbErr := repo.UpdateAgentVersionAndArch(subdomain, ver, normalizedArch); dbErr != nil {
					log.Printf("[OnAgentRegistered] ⚠️ Failed to persist arch for %s: %v", subdomain, dbErr)
				} else {
					log.Printf("[OnAgentRegistered] ✅ Saved arch=%s (raw: %s), ver=%s for %s", normalizedArch, rawArch, ver, subdomain)
				}
			}
		}
	}

	svc.OnRelayMessage = func(session *tunnel.AgentSession, msg tunnel.TunnelMessage) {
		if msg.Type == "telemetry_push" {
			relayAPI.IngestTelemetryMessage(session.Subdomain, msg.Payload)
		}
	}

	svc.OnSyncConfigReceived = func(subdomain string, payload tunnel.SyncConfigPayload) {
		if payload.OwnerName != "" || payload.OwnerPhone != "" {
			_ = repo.UpdateSubdomainOwner(subdomain, payload.OwnerName, payload.OwnerPhone, subdomain)
		}
		if payload.Credentials != nil {
			if credsBytes, err := json.Marshal(payload.Credentials); err == nil && len(credsBytes) > 2 {
				_ = repo.UpdateSubdomainCredentials(subdomain, string(credsBytes))
			}
		}
	}

	centralDomain := os.Getenv("SASMAN_CENTRAL_DOMAIN")
	if centralDomain == "" {
		centralDomain = "sas-man.net"
	}

	svc.OnGlobalAuthRequest = func(visitedSubdomain string, req tunnel.GlobalAuthRequestPayload) tunnel.GlobalAuthResponsePayload {
		uname := strings.TrimSpace(req.Username)
		if uname == "" {
			return tunnel.GlobalAuthResponsePayload{
				RequestID:    req.RequestID,
				Allow:        false,
				RejectReason: "اسم المستخدم فارغ (Empty username)",
			}
		}

		// 1. Identify candidate agents: visitedSubdomain, ahmed100, and other online agents
		var candidateAgents []string
		if visitedSubdomain != "" {
			candidateAgents = append(candidateAgents, visitedSubdomain)
		}
		if svc.GetAgentBySubdomain("ahmed100") != nil && visitedSubdomain != "ahmed100" {
			candidateAgents = append(candidateAgents, "ahmed100")
		}
		for _, sub := range svc.ListOnlineAgents() {
			found := false
			for _, c := range candidateAgents {
				if c == sub {
					found = true
					break
				}
			}
			if !found {
				candidateAgents = append(candidateAgents, sub)
			}
		}

		verifyReq := map[string]string{
			"username": uname,
			"password": req.Password,
		}
		verifyBytes, _ := json.Marshal(verifyReq)

		for _, ag := range candidateAgents {
			httpResp, respBytes, err := svc.SendAgentHTTPRequest(ag, "POST", "/radius/api/internal/verify-user", verifyBytes, nil)
			log.Printf("[radsec-central] 🔍 Querying agent [%s] for user [%s]: err=%v, resp=%s", ag, uname, err, string(respBytes))

			if err == nil && httpResp != nil && httpResp.Status == 200 {
				var verifyResp struct {
					Allow     bool   `json:"allow"`
					Reason    string `json:"reason"`
					RateLimit string `json:"rate_limit"`
					Password  string `json:"password"`
				}
				if err := json.Unmarshal(respBytes, &verifyResp); err == nil && verifyResp.Allow {
					rateLimit := verifyResp.RateLimit
					if rateLimit == "" {
						rateLimit = "10M/10M"
					}
					pass := verifyResp.Password
					if pass == "" {
						pass = req.Password
					}

					return tunnel.GlobalAuthResponsePayload{
						RequestID:      req.RequestID,
						Allow:          true,
						RateLimit:      rateLimit,
						SessionTimeout: 86400,
						AccountType:    "roaming_user",
						ReplyMessage:   fmt.Sprintf("مرحباً بك عبر شبكة SASMAN الموحدة (وكيل: %s)", ag),
						Password:       pass,
					}
				}
			}
		}

		// 2. Otherwise, treat as Global Voucher / Card PIN
		voucher, err := repo.ValidateAndRedeemGlobalVoucher(uname, req.UserMAC, visitedSubdomain)
		if err != nil {
			return tunnel.GlobalAuthResponsePayload{
				RequestID:    req.RequestID,
				Allow:        false,
				RejectReason: err.Error(),
			}
		}

		remSecs := 86400
		if voucher.ExpiresAt != nil {
			remSecs = int(time.Until(*voucher.ExpiresAt).Seconds())
			if remSecs <= 0 {
				remSecs = 60
			}
		}

		return tunnel.GlobalAuthResponsePayload{
			RequestID:      req.RequestID,
			Allow:          true,
			RateLimit:      voucher.RateLimit,
			SessionTimeout: remSecs,
			AccountType:    "voucher",
			ReplyMessage:   "تم تفعيل كرت SASMAN Global بنجاح",
		}
	}

	svc.OnGlobalAcctUpdate = func(visitedSubdomain string, payload tunnel.GlobalAcctPayload) {
		var stoppedAt *time.Time
		if payload.StatusType == "Stop" {
			now := time.Now().UTC()
			stoppedAt = &now
		}

		sessType := "voucher"
		homeSub := ""
		if strings.Contains(payload.Username, "@") {
			sessType = "roaming_user"
			parts := strings.Split(payload.Username, "@")
			if len(parts) > 1 {
				homeSub = parts[1]
			}
		}

		_ = repo.RecordGlobalHotspotSession(storage.GlobalHotspotSession{
			ID:               payload.SessionID,
			Username:         payload.Username,
			SessionType:      sessType,
			HomeSubdomain:    homeSub,
			VisitedSubdomain: visitedSubdomain,
			UserMAC:          payload.UserMAC,
			UserIP:           payload.UserIP,
			NasIP:            payload.NasIP,
			BytesIn:          payload.BytesIn,
			BytesOut:         payload.BytesOut,
			SessionTimeSec:   payload.SessionTimeSec,
			StoppedAt:        stoppedAt,
		})
	}

	backupScheduler := backup.NewScheduler(svc)
	backupScheduler.Start()

	otaManager := otainternal.NewManager(repo, svc)
	otaAPI := otainternal.NewAPIHandler(otaManager, repo)

	aiEngine := aiinternal.NewEngine(repo, svc)
	aiAPI := aiinternal.NewAPIHandler(aiEngine, repo)

	// Initialize Cloud Multi-Tenant Engine (Database-per-Tenant)
	cloudTenantPool := cloudtenant.NewTenantDBPool("")
	cloudTenantMgr := cloudtenant.NewManager(repo, cloudTenantPool, centralDomain, []byte("SASMAN_CLOUD_SECRET_KEY_9977_SECURE"))

	// Initialize and Start Central RadSec Server on port 2083 (RFC 6614 mTLS)
	centralRadSec := radsec.NewCentralRadSecServer(
		func(subdomain string, req tunnel.GlobalAuthRequestPayload) tunnel.GlobalAuthResponsePayload {
			if subdomain == "" {
				agents := svc.ListAgents()
				if len(agents) > 0 {
					if sub, ok := agents[0]["subdomain"].(string); ok {
						subdomain = sub
					}
				}
			}
			// 1. If agent is connected via container WebSocket tunnel
			if svc.IsAgentConnected(subdomain) {
				return svc.OnGlobalAuthRequest(subdomain, req)
			}

			// 2. Cloud Tenant fallback: Authenticate directly against isolated tenant database
			details := cloudTenantMgr.VerifyCloudUserDetails(subdomain, req.Username, req.Password)
			log.Printf("[CentralRadSec] 🔍 VerifyCloudUserDetails: Tenant=[%s], User=[%s], allow=%v, rateLimit=%s, group=%s, reason=%s, err=%v",
				subdomain, req.Username, details.Allow, details.RateLimit, details.MikrotikGroup, details.RejectReason, details.Err)

			// Record in tenant's Debug Monitor radius.log
			go func() {
				if subdomain != "" {
					tenantLogPath := filepath.Join(cloudTenantPool.GetTenantDir(subdomain), "radius.log")
					resStr := "Access-Accept ✅"
					if !details.Allow {
						resStr = fmt.Sprintf("Access-Reject ❌ (%s)", details.RejectReason)
					}
					line := fmt.Sprintf("[%s] RADIUS %s for user [%s] from NAS [%s] (MAC: %s)\n",
						time.Now().Format("2006-01-02 15:04:05"), resStr, req.Username, req.NasIP, req.UserMAC)
					f, err := os.OpenFile(tenantLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if err == nil {
						_, _ = f.WriteString(line)
						_ = f.Close()
					}
				}
			}()

			if details.Err == nil && details.Allow {
				return tunnel.GlobalAuthResponsePayload{
					RequestID:      req.RequestID,
					Allow:          true,
					Password:       details.Password,
					RateLimit:      details.RateLimit,
					MikrotikGroup:  details.MikrotikGroup,
					FramedPool:     details.FramedPool,
					SessionTimeout: 86400,
				}
			}
			return tunnel.GlobalAuthResponsePayload{
				RequestID:    req.RequestID,
				Allow:        false,
				RejectReason: details.RejectReason,
			}
		},
		func(subdomain string, req tunnel.GlobalAcctPayload) {
			if subdomain == "" {
				agents := svc.ListAgents()
				if len(agents) > 0 {
					if sub, ok := agents[0]["subdomain"].(string); ok {
						subdomain = sub
					}
				}
			}
			// 1. Container mode
			if svc.IsAgentConnected(subdomain) {
				svc.OnGlobalAcctUpdate(subdomain, req)
				if subdomain != "" {
					acctBytes, _ := json.Marshal(req)
					_, _, _ = svc.SendAgentHTTPRequest(subdomain, "POST", "/radius/api/internal/sync-acct", acctBytes, nil)
				}
				return
			}

			// 2. Cloud Tenant mode: Record accounting directly in tenant's isolated DB
			_ = cloudTenantMgr.RecordCloudAccounting(subdomain, cloudtenant.CloudAccountingPayload{
				Username:       req.Username,
				StatusType:     req.StatusType,
				SessionID:      req.SessionID,
				UserIP:         req.UserIP,
				UserMAC:        req.UserMAC,
				NasIP:          req.NasIP,
				BytesIn:        req.BytesIn,
				BytesOut:       req.BytesOut,
				SessionTimeSec: int64(req.SessionTimeSec),
			})
		},
		func(cn string, nasIP string) string {
			for _, part := range strings.Split(cn, "-") {
				part = strings.TrimSpace(part)
				if part != "" && part != "agent" && part != "SASMAN" {
					return strings.ToLower(part)
				}
			}
			online := svc.ListOnlineAgents()
			for _, sub := range online {
				if strings.Contains(strings.ToLower(cn), strings.ToLower(sub)) {
					return sub
				}
			}
			return ""
		},
	)
	if err := centralRadSec.Start(2083); err != nil {
		log.Printf("[CentralRadSec] ❌ Failed to start Central RadSec Server on :2083: %v", err)
	}
	cloudTenantMgr.SetDisconnector(centralRadSec)

	// Auto-resume dedicated cloud agent instances for active cloud tenants
	go cloudTenantMgr.EnsureAllCloudAgentsRunning()
	cloudTenantMgr.StartExpirationSweeper()
	cloudTenantMgr.StartTenantTelegramBackupScheduler()

	app := fiber.New(fiber.Config{
		AppName:   "SASMAN Central Server",
		BodyLimit: 256 * 1024 * 1024, // 256MB Max payload limit for large OTA releases and container images
	})

	// Subdomain Gateway & Routing Middleware (MUST be registered first)
	app.Use(func(c *fiber.Ctx) error {
		host := c.Get("Host")
		subdomain := tunnel.ExtractSubdomainForHost(host, centralDomain)
		if subdomain != "" {
			// 1. Container mode (Local MikroTik agent connected via WebSocket tunnel)
			if svc.IsAgentConnected(subdomain) {
				return svc.ForwardRequestToAgent(c, subdomain)
			}

			// 2. Cloud Tenant mode
			c.Locals("subdomain", subdomain)

			// Redirect root or /admin to /radius
			if c.Path() == "/" || c.Path() == "/admin" {
				return c.Redirect("/radius")
			}
		}
		return c.Next()
	})

	relayAPI.RegisterRoutes(app)
	otaAPI.RegisterRoutes(app)
	aiAPI.RegisterRoutes(app)
	cloudtenant.NewAPIHandler(cloudTenantMgr).RegisterRoutes(app)

	// Cloud Edition Public Web Pages
	app.Get("/cloud", func(c *fiber.Ctx) error {
		return c.Redirect("/cloud/register")
	})
	app.Get("/cloud/register", func(c *fiber.Ctx) error {
		content, err := webFS.ReadFile("web/cloud/register.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error loading cloud register page")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(content)
	})
	app.Get("/cloud/login", func(c *fiber.Ctx) error {
		content, err := webFS.ReadFile("web/cloud/login.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error loading cloud login page")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(content)
	})
	app.Get("/cloud/dashboard", func(c *fiber.Ctx) error {
		content, err := webFS.ReadFile("web/cloud/dashboard.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error loading cloud dashboard")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(content)
	})

	// Mount full SASMAN RADIUS UI for Cloud Tenants
	radiusDir := "web_radius"
	if _, err := os.Stat(radiusDir); os.IsNotExist(err) {
		radiusDir = "agent/web_radius"
	}
	if _, err := os.Stat(radiusDir); err == nil {
		app.Static("/radius/js", filepath.Join(radiusDir, "js"))
		app.Static("/radius/css", filepath.Join(radiusDir, "css"))
		app.Static("/radius/fonts", filepath.Join(radiusDir, "fonts"))
		app.Static("/radius/vendor", filepath.Join(radiusDir, "vendor"))
		app.Static("/radius/unnamed.png", filepath.Join(radiusDir, "unnamed.png"))
		app.Static("/radius/logo.png", filepath.Join(radiusDir, "logo.png"))
		app.Static("/radius/favicon.ico", filepath.Join(radiusDir, "favicon.ico"))

		app.Get("/radius", func(c *fiber.Ctx) error {
			return c.SendFile(filepath.Join(radiusDir, "index.html"))
		})
		app.Get("/radius/", func(c *fiber.Ctx) error {
			return c.SendFile(filepath.Join(radiusDir, "index.html"))
		})
		app.Get("/radius/login.html", func(c *fiber.Ctx) error {
			return c.SendFile(filepath.Join(radiusDir, "login.html"))
		})
		app.Get("/radius/portal.html", func(c *fiber.Ctx) error {
			return c.SendFile(filepath.Join(radiusDir, "portal.html"))
		})
		app.Get("/radius/portal", func(c *fiber.Ctx) error {
			return c.SendFile(filepath.Join(radiusDir, "portal.html"))
		})
		app.Get("/portal", func(c *fiber.Ctx) error {
			return c.SendFile(filepath.Join(radiusDir, "portal.html"))
		})
		app.Get("/portal/", func(c *fiber.Ctx) error {
			return c.SendFile(filepath.Join(radiusDir, "portal.html"))
		})
		app.Get("/portal.html", func(c *fiber.Ctx) error {
			return c.SendFile(filepath.Join(radiusDir, "portal.html"))
		})
		app.Get("/radius/*", func(c *fiber.Ctx) error {
			if strings.HasPrefix(c.Path(), "/radius/api/") {
				return c.Next()
			}
			return c.SendFile(filepath.Join(radiusDir, "index.html"))
		})
	}

	adminUser := os.Getenv("SASMAN_ADMIN_USER")
	if adminUser == "" {
		adminUser = "admin"
	}
	adminPassword := os.Getenv("SASMAN_ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "Mushtaq@Sasman#9977!"
	}

	authManager := api.NewAuthManager(adminUser, adminPassword)

	// Public Auth endpoints
	app.Get("/login", func(c *fiber.Ctx) error {
		content, err := webFS.ReadFile("web/login.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error loading login page")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(content)
	})

	app.Post("/api/login", authManager.HandleLogin)
	app.Post("/api/logout", authManager.HandleLogout)
	app.Post("/api/change-password", authManager.HandleChangePassword)

	// Helper to generate dynamic MikroTik Installer script
	generateInstaller := func(targetDisk string, host string) string {
		if host == "" {
			host = centralDomain
		}
		diskPrefix := strings.TrimSpace(targetDisk)
		if diskPrefix == "" {
			diskPrefix = "disk1"
		}
		diskPrefix = strings.TrimSuffix(diskPrefix, "/")
		pullDir := diskPrefix + "/pull"

		tpl := `# ==============================================================================
# SASMAN MikroTik Manager v5 - Ultra-Light Native Container Installer
# Auto-generated by SASMAN Central Server: {{HOST}}
# Target Storage Disk: {{DISK}}
# ==============================================================================

:put "[*] Checking MikroTik Container package..."
/container
:if ([:len [/container find]] = 0) do={
    :put "[*] Container subsystem ready."
}

:put "[*] Creating required directories on {{DISK}}..."
/file
:do {
    /file make-dir "{{DISK}}"
    /file make-dir "{{PULL}}"
    /file make-dir "{{DISK}}/data"
    /file make-dir "{{DISK}}/data/radius_db"
    /file make-dir "{{DISK}}/sasman_root"
} on-error={ :put "[!] Directories might already exist, continuing..." }

# 1. Create Dedicated Bridge (container-bridge)
:put "[*] Setting up bridge interface (container-bridge)..."
/interface/bridge
:if ([:len [/interface/bridge find name="container-bridge"]] = 0) do={
    /interface/bridge add name="container-bridge" comment="SASMAN Dedicated Bridge"
}

# 2. Add Bridge IP (172.17.0.1/24)
:put "[*] Assigning IP Address 172.17.0.1/24 to container-bridge..."
/ip/address
:if ([:len [/ip/address find address="172.17.0.1/24" interface="container-bridge"]] = 0) do={
    /ip/address add address=172.17.0.1/24 interface="container-bridge" network=172.17.0.0 comment="SASMAN Subnet Gateway"
}

# 3. Create Container VETH Interface
:put "[*] Creating container VETH interface (veth-sasman: 172.17.0.2/24)..."
/interface/veth
:if ([:len [/interface/veth find name="veth-sasman"]] = 0) do={
    /interface/veth add name="veth-sasman" address=172.17.0.2/24 gateway=172.17.0.1 comment="SASMAN Container VETH"
}

# 4. Attach VETH to Bridge
:put "[*] Adding veth-sasman to container-bridge..."
/interface/bridge/port
:if ([:len [/interface/bridge/port find interface="veth-sasman"]] = 0) do={
    /interface/bridge/port add bridge="container-bridge" interface="veth-sasman"
}

# 5. Create NAT Masquerade Rule for Internet Outbound
:put "[*] Configuring Firewall NAT Masquerade for 172.17.0.0/24..."
/ip/firewall/nat
:if ([:len [/ip/firewall/nat find chain=srcnat src-address=172.17.0.0/24 action=masquerade]] = 0) do={
    /ip/firewall/nat add chain=srcnat src-address=172.17.0.0/24 action=masquerade comment="SASMAN Container Outbound Internet NAT"
}

# 6. Create Port Forwarding DST-NAT Rule (:8080 -> 172.17.0.2:80)
:put "[*] Setting up DST-NAT Port Forward (Port 8080 -> Container Port 80)..."
:if ([:len [/ip/firewall/nat find chain=dstnat dst-port=8080 protocol=tcp action=dst-nat]] = 0) do={
    /ip/firewall/nat add chain=dstnat protocol=tcp dst-port=8080 action=dst-nat to-addresses=172.17.0.2 to-ports=80 comment="SASMAN Web UI Forward"
}

# 7. Create Container Mounts
:put "[*] Creating Container File Mounts on {{DISK}}/data..."
/container/mounts
:if ([:len [/container/mounts find name="sasman_data"]] = 0) do={
    /container/mounts add name="sasman_data" src="{{DISK}}/data" dst="/app/data"
}

# 8. Configure Docker Hub Registry and Pull Latest Multi-Arch Image
:put "[*] Setting up Container Environment & Docker Hub Registry..."
/container config set registry-url=https://registry-1.docker.io tmpdir={{PULL}}
:put "[*] Configured Docker Hub Registry: https://registry-1.docker.io"

:put "[*] Pulling latest multi-arch SASMAN image from Docker Hub (ahmedkin99/sasman-manager:latest)..."
/container
:do {
    /container remove [find comment~"sasman"]
} on-error={}

/container add remote-image="ahmedkin99/sasman-manager:latest" interface="veth-sasman" mounts="sasman_data" root-dir="{{DISK}}/sasman_root" logging=yes comment="sasman-unified-v5"

:put "[*] Installation command issued successfully!"
:put "[*] Wait for the container status to become 'stopped', then run:"
:put "    /container start [find comment~\"sasman\"]"
:put "=============================================================================="
:put "[*] Once started, access your SASMAN Manager panel at: http://<Router-IP>:8080"
:put "=============================================================================="
`
		out := strings.ReplaceAll(tpl, "{{HOST}}", host)
		out = strings.ReplaceAll(out, "{{DISK}}", diskPrefix)
		out = strings.ReplaceAll(out, "{{PULL}}", pullDir)
		return out
	}

	installScriptHandler := func(c *fiber.Ctx, defaultDisk string) error {
		targetDisk := c.Query("disk", defaultDisk)
		host := c.Query("host", centralDomain)
		script := generateInstaller(targetDisk, host)
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendString(script)
	}

	app.Get("/install-disk.rsc", func(c *fiber.Ctx) error { return installScriptHandler(c, "disk1") })
	app.Get("/install-usb.rsc", func(c *fiber.Ctx) error { return installScriptHandler(c, "usb1") })
	app.Get("/install-disk", func(c *fiber.Ctx) error { return installScriptHandler(c, "disk1") })
	app.Get("/install-usb", func(c *fiber.Ctx) error { return installScriptHandler(c, "usb1") })
	app.Get("/install.rsc", func(c *fiber.Ctx) error { return installScriptHandler(c, "disk1") })
	app.Get("/install.sh", func(c *fiber.Ctx) error { return installScriptHandler(c, "disk1") })
	app.Get("/install", func(c *fiber.Ctx) error { return installScriptHandler(c, "disk1") })
	app.Get("/installer.rsc", func(c *fiber.Ctx) error { return installScriptHandler(c, "disk1") })

	// ─── PKI RadSec MikroTik Auto-Provisioning ──────────────────────────────────
	ensureAgentCertificate := func(subdomain string) (certPEM, keyPEM, caPEM string, err error) {
		subdomain = strings.ToLower(strings.TrimSpace(subdomain))
		if subdomain == "" {
			return "", "", "", fmt.Errorf("invalid subdomain")
		}

		pkiAgentDir := filepath.Join("data", "pki", "agents", subdomain)
		if _, e := os.Stat("/app/data"); e == nil {
			pkiAgentDir = filepath.Join("/app/data", "pki", "agents", subdomain)
		}

		certPath := filepath.Join(pkiAgentDir, "agent.crt")
		keyPath := filepath.Join(pkiAgentDir, "agent.key")
		caPath := filepath.Join("data", "pki", "ca.crt")
		if _, e := os.Stat("/app/data/pki/ca.crt"); e == nil {
			caPath = "/app/data/pki/ca.crt"
		}

		caBytes, _ := os.ReadFile(caPath)
		if len(caBytes) == 0 {
			caBytes = pki.GetCACertPEM()
		}

		if _, err := os.Stat(certPath); err == nil {
			if _, err := os.Stat(keyPath); err == nil {
				cB, _ := os.ReadFile(certPath)
				kB, _ := os.ReadFile(keyPath)
				if len(cB) > 0 && len(kB) > 0 {
					return string(cB), string(kB), string(caBytes), nil
				}
			}
		}

		// Generate new certificate signed by Root CA
		_ = os.MkdirAll(pkiAgentDir, 0755)
		commonName := fmt.Sprintf("agent-%s-SASMAN", subdomain)
		bundle, err := pki.GenerateClientCertificate(commonName, 365*5)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to generate certificate: %w", err)
		}

		_ = os.WriteFile(certPath, []byte(bundle.CertPEM), 0644)
		_ = os.WriteFile(keyPath, []byte(bundle.KeyPEM), 0600)

		return bundle.CertPEM, bundle.KeyPEM, string(caBytes), nil
	}

	app.Get("/pki/cert/:subdomain/ca.crt", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")
		_, _, caPEM, err := ensureAgentCertificate(subdomain)
		if err != nil || caPEM == "" {
			return c.Status(500).SendString("CA Certificate not available")
		}
		c.Set("Content-Type", "application/x-x509-ca-cert")
		return c.SendString(caPEM)
	})

	app.Get("/pki/cert/:subdomain/agent.crt", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")
		certPEM, _, _, err := ensureAgentCertificate(subdomain)
		if err != nil || certPEM == "" {
			return c.Status(500).SendString("Agent Certificate not available")
		}
		c.Set("Content-Type", "application/x-x509-user-cert")
		return c.SendString(certPEM)
	})

	app.Get("/pki/cert/:subdomain/agent.key", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")
		_, keyPEM, _, err := ensureAgentCertificate(subdomain)
		if err != nil || keyPEM == "" {
			return c.Status(500).SendString("Agent Key not available")
		}
		c.Set("Content-Type", "application/pkcs8")
		return c.SendString(keyPEM)
	})

	radsecScriptHandler := func(c *fiber.Ctx) error {
		rawSub := c.Params("subdomain")
		subdomain := strings.TrimSuffix(rawSub, ".rsc")
		subdomain = strings.ToLower(strings.TrimSpace(subdomain))
		if subdomain == "" {
			subdomain = "default"
		}

		// Ensure certificate bundle exists
		_, _, _, _ = ensureAgentCertificate(subdomain)

		domain := centralDomain
		if domain == "" {
			domain = "sas-man.net"
		}

		script := fmt.Sprintf(`# =========================================================
#  SASMAN RadSec (RFC 6614 mTLS) Auto-Provisioning Script
#  Agent Subdomain: %[1]s
#  Central Server: 167.86.73.203:2083
# =========================================================

:put "=================================================="
:put "  [1/4] Downloading SASMAN PKI Certificates..."
:put "=================================================="

/tool fetch url="https://%[2]s/pki/cert/%[1]s/ca.crt" dst-path="ca.crt" mode=https
:delay 2s
/tool fetch url="https://%[2]s/pki/cert/%[1]s/agent.crt" dst-path="agent.crt" mode=https
:delay 2s
/tool fetch url="https://%[2]s/pki/cert/%[1]s/agent.key" dst-path="agent.key" mode=https
:delay 2s

:put "=================================================="
:put "  [2/4] Importing Certificates into RouterOS..."
:put "=================================================="

/certificate import file-name="ca.crt" passphrase=""
:delay 1s
/certificate import file-name="agent.crt" passphrase=""
:delay 1s
/certificate import file-name="agent.key" passphrase=""
:delay 1s

:put "=================================================="
:put "  [3/4] Configuring High-Speed RadSec Client..."
:put "=================================================="

:local certName "agent.crt_0"
:local caName "ca.crt_0"

:foreach c in=[/certificate find where common-name~"agent-.*"] do={
    :set certName [/certificate get $c name]
}
:foreach c in=[/certificate find where common-name~"SASMAN.*"] do={
    :set caName [/certificate get $c name]
}

# Remove existing central server RADIUS entries to avoid duplicates
/radius remove [find address="167.86.73.203"]

# Add RadSec client connected to Central Server IP with mTLS certificates
/radius add address=167.86.73.203 protocol=radsec certificate=$certName service=ppp,login,hotspot,wireless secret=radsec authentication-port=2083 accounting-port=2083 timeout=3s require-message-auth=yes-for-request-resp comment="SASMAN Central RadSec (%[1]s)"

# Enable Disconnect Messages & CoA
/radius incoming set accept=yes port=3799

# Enable RADIUS in Services
/user aaa set use-radius=yes default-group=read
/ppp aaa set use-radius=yes accounting=yes interim-update=1m
/ip hotspot profile set [find default=yes] use-radius=yes radius-accounting=yes radius-interim-update=1m

# Configure Secure Device Tunnel using the exact same certificate (Port 1194 TLS)
/interface ovpn-client remove [find name="ovpn-sasman"]
/interface ovpn-client add name="ovpn-sasman" connect-to=167.86.73.203 port=1194 mode=ip protocol=tcp user="%[1]s" password="" certificate=$certName auth=sha256 cipher=aes256-gcm verify-server-certificate=yes add-default-route=no disabled=no comment="SASMAN Cloud Device Tunnel (%[1]s)"

# Configure Firewall & NAT for Cloud Device Access Automatically
/ip firewall filter remove [find comment="Allow SASMAN Tunnel"]
/ip firewall filter add chain=input in-interface=ovpn-sasman action=accept place-before=0 comment="Allow SASMAN Tunnel"
/ip firewall nat remove [find comment="SASMAN LAN Access"]
/ip firewall nat add chain=srcnat out-interface=!ovpn-sasman src-address=10.250.0.0/24 action=masquerade comment="SASMAN LAN Access"

:put "=================================================="
:put "  [4/4] Cleaning Up Temporary Files..."
:put "=================================================="

/file remove [find name="ca.crt"]
/file remove [find name="agent.crt"]
/file remove [find name="agent.key"]
/file remove [find name="radsec.rsc"]

:put "=================================================="
:put "  [SUCCESS] SASMAN RadSec Provisioned Successfully!"
:put "  Agent: %[1]s"
:put "  Server: 167.86.73.203:2083 (RFC 6614 mTLS)"
:put "=================================================="
`, subdomain, domain)

		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendString(script)
	}

	app.Get("/pki/install/:subdomain", radsecScriptHandler)
	app.Get("/pki/install/:subdomain.rsc", radsecScriptHandler)
	app.Get("/pki/radsec/:subdomain", radsecScriptHandler)
	app.Get("/pki/radsec/:subdomain.rsc", radsecScriptHandler)

	// MikroTik RouterOS v7.21+ App Store Catalog
	appStoreHandler := func(c *fiber.Ctx) error {
		yamlContent := fmt.Sprintf(`- name: sasman-manager
  descr: SASMAN MikroTik Manager v5 - Unified Radius Server & Management
  page: https://%s
  category: networking
  default-credentials: "admin / admin"
  services:
    sasman:
      image: docker.io/ahmedkin99/sasman-manager:latest
      ports:
        - 8080:80:tcp
      volumes:
        - /disk1/data:/app/data
`, centralDomain)
		c.Set("Content-Type", "text/yaml; charset=utf-8")
		return c.SendString(yamlContent)
	}

	app.Get("/app-store.yaml", appStoreHandler)
	app.Get("/sasman.tikapp.yaml", appStoreHandler)
	app.Get("/app-store.json", func(c *fiber.Ctx) error {
		return c.JSON([]fiber.Map{
			{
				"name":        "sasman-manager",
				"title":       "SASMAN MikroTik Manager v5",
				"description": "Unified Radius Server, Network Management & High-Speed Relay Engine",
				"version":     "5.1.0",
				"image":       "ahmedkin99/sasman-manager:latest",
				"auto_update": true,
			},
		})
	})

	// 3. Direct Binary and Image Download Endpoint
	app.Get("/download/:filename", func(c *fiber.Ctx) error {
		filename := filepath.Base(c.Params("filename"))
		searchPaths := []string{
			filepath.Join("/app/data/releases", filename),
			filepath.Join("/app/data", filename),
			filepath.Join("data", "releases", filename),
			filepath.Join("data", filename),
			filepath.Join("releases", filename),
			filename,
		}
		for _, p := range searchPaths {
			if _, err := os.Stat(p); err == nil {
				return c.Download(p, filename)
			}
		}

		// Fallback: If not found as a static file, check database OTA releases!
		var targetArch string
		lower := strings.ToLower(filename)
		if strings.Contains(lower, "arm64") {
			targetArch = "linux_arm64"
		} else if strings.Contains(lower, "armv7") || strings.Contains(lower, "arm") {
			targetArch = "linux_arm"
		} else if strings.Contains(lower, "amd64") || strings.Contains(lower, "x86") {
			targetArch = "linux_amd64"
		}

		if targetArch != "" {
			releases, err := repo.ListReleases()
			if err == nil && len(releases) > 0 {
				for _, r := range releases {
					if r.TargetArch == targetArch {
						manifest, binData, err := repo.GetRelease(r.Version, r.TargetArch)
						if err == nil && len(binData) > 0 {
							c.Set("Content-Type", "application/octet-stream")
							c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
							c.Set("X-Checksum-SHA256", manifest.Sha256)
							return c.Send(binData)
						}
					}
				}
			}
		}

		return c.Status(fiber.StatusNotFound).SendString("File not found")
	})

	// Setup / Onboarding Endpoints
	app.Post("/api/agents/check-subdomain", func(c *fiber.Ctx) error {
		var req struct {
			Subdomain string `json:"subdomain"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
		}

		sub := strings.ToLower(strings.TrimSpace(req.Subdomain))
		if len(sub) < 3 || len(sub) > 30 {
			return c.JSON(fiber.Map{
				"available": false,
				"error":     "يجب أن يكون النطاق بين 3 و 30 حرفاً باللغة الإنجليزية",
			})
		}

		for _, char := range sub {
			if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
				return c.JSON(fiber.Map{
					"available": false,
					"error":     "النطاق يجب أن يحتوي على أحرف إنجليزية وأرقام وشرطة فقط",
				})
			}
		}

		available, err := repo.IsSubdomainAvailable(sub)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
		}

		if !available {
			ownerName, ownerPhone, _ := repo.GetSubdomainOwnerInfo(sub)
			maskedPhone := ""
			if len(ownerPhone) > 6 {
				maskedPhone = ownerPhone[:4] + "****" + ownerPhone[len(ownerPhone)-3:]
			} else if ownerPhone != "" {
				maskedPhone = ownerPhone
			}
			return c.JSON(fiber.Map{
				"subdomain":          sub,
				"available":          false,
				"taken":              true,
				"can_takeover":       true,
				"owner_name":         ownerName,
				"owner_phone_masked": maskedPhone,
				"error":              "هذا النطاق محجوز مسبقاً، يمكنك إرسال طلب استحواذ / نقل ملكية",
				"full_url":           fmt.Sprintf("http://%s.%s", sub, centralDomain),
			})
		}

		return c.JSON(fiber.Map{
			"subdomain": sub,
			"available": true,
			"full_url":  fmt.Sprintf("http://%s.%s", sub, centralDomain),
		})
	})

	app.Post("/api/agents/request-takeover", func(c *fiber.Ctx) error {
		var req struct {
			Subdomain string `json:"subdomain"`
			Name      string `json:"name"`
			Phone     string `json:"phone"`
			Serial    string `json:"serial"`
			Notes     string `json:"notes"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "بيانات غير صالحة"})
		}

		sub := strings.ToLower(strings.TrimSpace(req.Subdomain))
		name := strings.TrimSpace(req.Name)
		phone := strings.TrimSpace(req.Phone)

		if sub == "" || name == "" || phone == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "الاسم ورقم الهاتف واسم النطاق هي حقول مطلوبة"})
		}

		available, _ := repo.IsSubdomainAvailable(sub)
		if available {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "هذا النطاق متاح بالفعل ويمكنك تسجيله مباشرة دون الحاجة لطلب استحواذ"})
		}

		ownerName, ownerPhone, _ := repo.GetSubdomainOwnerInfo(sub)

		takeoverReq := storage.SubdomainTakeoverRequest{
			ID:                fmt.Sprintf("req-%d", time.Now().UnixNano()),
			Subdomain:         sub,
			RequesterName:     name,
			RequesterPhone:    phone,
			RequesterSerial:   strings.TrimSpace(req.Serial),
			RequesterNotes:    strings.TrimSpace(req.Notes),
			RequesterAgentID:  c.IP(),
			CurrentOwnerName:  ownerName,
			CurrentOwnerPhone: ownerPhone,
			Status:            "pending",
			RequestedAt:       time.Now().UTC(),
		}

		if err := repo.CreateTakeoverRequest(takeoverReq); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل حفظ طلب الاستحواذ: " + err.Error()})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": "تم إرسال طلب الاستحواذ بنجاح، بانتظار مراجعة وموافقة مدير السيرفر",
			"request_id": takeoverReq.ID,
			"subdomain": sub,
		})
	})

	app.Post("/api/agents/check-takeover-status", func(c *fiber.Ctx) error {
		var req struct {
			Subdomain string `json:"subdomain"`
			Phone     string `json:"phone"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
		}

		sub := strings.ToLower(strings.TrimSpace(req.Subdomain))
		takeover, err := repo.CheckTakeoverStatus(sub, req.Phone)
		if err != nil || takeover == nil {
			return c.JSON(fiber.Map{
				"found":  false,
				"status": "not_found",
			})
		}

		respMap := fiber.Map{
			"found":        true,
			"request_id":   takeover.ID,
			"subdomain":    takeover.Subdomain,
			"status":       takeover.Status, // 'pending', 'approved', 'rejected'
			"admin_notes":  takeover.AdminNotes,
			"requested_at": takeover.RequestedAt,
		}

		if takeover.Status == "approved" {
			subObj, _ := repo.GetSubdomainByName(sub)
			if subObj != nil {
				respMap["token"] = subObj.Token
				respMap["winbox_port"] = subObj.WinboxPort
				respMap["central_domain"] = centralDomain
				respMap["full_domain"] = fmt.Sprintf("%s.%s", sub, centralDomain)
			}
		}

		return c.JSON(respMap)
	})

	app.Post("/api/agents/self-register", func(c *fiber.Ctx) error {
		var req struct {
			Subdomain string `json:"subdomain"`
			Name      string `json:"name"`
			Phone     string `json:"phone"`
			Serial    string `json:"serial"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
		}

		sub := strings.ToLower(strings.TrimSpace(req.Subdomain))
		if len(sub) < 3 || len(sub) > 30 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "اسم النطاق غير صالح (3-30 حرف)"})
		}

		available, err := repo.IsSubdomainAvailable(sub)
		if err != nil || !available {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "اسم النطاق هذا محجوز مسبقاً، يرجى اختيار اسم آخر"})
		}

		token := fmt.Sprintf("tok-%d-%s", time.Now().Unix(), strings.ToLower(sub))
		customerID := fmt.Sprintf("cust-%d", time.Now().UnixNano())
		customer := storage.Customer{
			ID:          customerID,
			Name:        req.Name,
			Phone:       req.Phone,
			Email:       "",
			CompanyName: sub,
			Status:      "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		_ = repo.SaveCustomer(customer)

		licenseID := fmt.Sprintf("lic-%d", time.Now().UnixNano())
		licenseKey := fmt.Sprintf("KEY-%s-%d", strings.ToUpper(sub), time.Now().Unix())
		// Set initial active license for 30 days
		initExp := time.Now().UTC().Add(30 * 24 * time.Hour)
		_ = repo.SaveLicense(storage.License{
			ID:         licenseID,
			CustomerID: customerID,
			LicenseKey: licenseKey,
			HWUUID:     req.Serial,
			PlanName:   "basic",
			Status:     "active",
			IssuedAt:   time.Now(),
			ExpiresAt:  &initExp,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})

		agent := svc.RegisterAgent(sub, token)
		subRecord, err := repo.CreateOrGetSubdomain(customerID, licenseID, sub)
		if err == nil && subRecord != nil {
			subRecord.CustomerID = customerID
			subRecord.LicenseID = licenseID
			subRecord.Token = token
			subRecord.WinboxPort = agent.WinboxPort
			_ = repo.SaveSubdomain(*subRecord)
		}
		_ = repo.UpdateSubdomainOwner(sub, req.Name, req.Phone, sub)

		webURL := fmt.Sprintf("http://%s.%s", agent.Subdomain, centralDomain)
		winboxAddress := fmt.Sprintf("%s:%d", centralDomain, agent.WinboxPort)
		gatewayURL := fmt.Sprintf("wss://%s/ws", centralDomain)

		return c.JSON(fiber.Map{
			"success":          true,
			"subdomain":        agent.Subdomain,
			"token":            agent.Token,
			"winbox_port":      agent.WinboxPort,
			"web_url":          webURL,
			"winbox_address":   winboxAddress,
			"gateway_url":      gatewayURL,
			"central_domain":   centralDomain,
			"tunnel_mode":      "agent",
			"setup_instructions": fmt.Sprintf("الإعداد على النظام المحلي:\n1. وضع التوصيل: Agent\n2. الدومين الفرعي: %s\n3. رابط اللوحة الكامل: %s\n4. رقم منفذ Winbox: %d\n5. عنوان Winbox المباشر: %s\n6. التوكن: %s\n7. بوابة السيرفر: %s", agent.Subdomain, webURL, agent.WinboxPort, winboxAddress, agent.Token, gatewayURL),
		})
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "central-server",
			"db":      dbPath,
		})
	})

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(landingHTML)
	})

	app.Get("/admin", func(c *fiber.Ctx) error {
		content, err := webFS.ReadFile("web/admin.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error loading admin page")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(content)
	})

	app.Get("/api/admin/backup/download", func(c *fiber.Ctx) error {
		fileName := fmt.Sprintf("sasman-central-backup-%s.db", time.Now().Format("2006-01-02_15-04-05"))
		tmpBackup := filepath.Join(os.TempDir(), fileName)
		if err := repo.BackupTo(tmpBackup); err != nil {
			log.Printf("backup error: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("فشل إنشاء النسخة الاحتياطية: " + err.Error())
		}
		c.Attachment(tmpBackup, fileName)
		return c.SendFile(tmpBackup)
	})

	app.Post("/api/admin/backup/restore", func(c *fiber.Ctx) error {
		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "لم يتم اختيار ملف للرفع"})
		}

		if fileHeader.Size <= 0 || fileHeader.Size > 100*1024*1024 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "حجم الملف غير مسموح به (الحد الأقصى 100 ميجابايت)"})
		}

		tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("upload-backup-%d.db", time.Now().UnixNano()))
		if err := c.SaveFile(fileHeader, tmpFile); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل حفظ الملف المرفوع"})
		}
		defer os.Remove(tmpFile)

		buf := make([]byte, 16)
		f, err := os.Open(tmpFile)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "فشل قراءة الملف المرفوع"})
		}
		_, err = f.Read(buf)
		_ = f.Close()
		if err != nil || !strings.HasPrefix(string(buf), "SQLite format 3") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "الملف المرفوع ليس ملف قاعدة بيانات SQLite صالح"})
		}

		// Save safety backup of existing db
		safetyBackup := dbPath + ".bak." + time.Now().Format("20060102-150405")
		_ = repo.BackupTo(safetyBackup)

		if err := repo.Close(); err != nil {
			log.Printf("close db before restore: %v", err)
		}

		inputData, err := os.ReadFile(tmpFile)
		if err != nil {
			_ = repo.Reopen(dbPath)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل قراءة محتوى النسخة الاحتياطية"})
		}

		if err := os.WriteFile(dbPath, inputData, 0644); err != nil {
			_ = repo.Reopen(dbPath)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل كتابة النسخة الاحتياطية إلى قاعدة البيانات"})
		}

		if err := repo.Reopen(dbPath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل إعادة فتح قاعدة البيانات بعد الاستعادة: " + err.Error()})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": "تمت استعادة النسخة الاحتياطية بنجاح!",
		})
	})

	api.SetupBackupRoutes(app, svc)

	// Edge Static Asset Cache APIs
	app.Post("/api/admin/cache/clear", func(c *fiber.Ctx) error {
		purged := svc.ClearAssetCache()
		return c.JSON(fiber.Map{
			"success":      true,
			"message":      fmt.Sprintf("تم مسح الكاش السحابي بنجاح وتفريغ %d ملف أصول.", purged),
			"purged_count": purged,
		})
	})

	app.Get("/api/admin/cache/stats", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"stats":   svc.GetAssetCacheStats(),
		})
	})

	// Agents List API (with Owner + License Information)
	app.Get("/api/agents", func(c *fiber.Ctx) error {
		agents := svc.ListAgents()
		owners, _ := repo.GetSubdomainOwners()
		licenses, _ := repo.GetSubdomainLicensesMap()
		allCreds, _ := repo.GetSubdomainCredentialsMap()

		for _, agent := range agents {
			subdomain := agent["subdomain"].(string)
			if subdomain == "" {
				continue
			}

			session := svc.GetAgentBySubdomain(subdomain)

			if owner, ok := owners[strings.ToLower(subdomain)]; ok && (owner.Name != "" || owner.Phone != "") {
				agent["owner_name"] = owner.Name
				agent["owner_phone"] = owner.Phone
				agent["company_name"] = owner.CompanyName
			} else if session != nil && session.SyncData != nil {
				syncOwnerName, _ := session.SyncData["owner_name"].(string)
				syncOwnerPhone, _ := session.SyncData["owner_phone"].(string)
				agent["owner_name"] = syncOwnerName
				agent["owner_phone"] = syncOwnerPhone
				agent["company_name"] = subdomain
				if syncOwnerName != "" || syncOwnerPhone != "" {
					_ = repo.UpdateSubdomainOwner(subdomain, syncOwnerName, syncOwnerPhone, subdomain)
				}
			} else {
				agent["owner_name"] = ""
				agent["owner_phone"] = ""
				agent["company_name"] = ""
			}

			if lic, ok := licenses[strings.ToLower(subdomain)]; ok {
				agent["license_status"] = lic.Status
				agent["license_expires_at"] = lic.ExpiresAtStr
				agent["days_remaining"] = lic.DaysRemaining
				agent["is_expired"] = lic.IsExpired
			} else {
				agent["license_status"] = "unlicensed"
				agent["license_expires_at"] = ""
				agent["days_remaining"] = 0
				agent["is_expired"] = true
			}

			backupPath := backup.GetBackupPath(subdomain)
			if info, err := os.Stat(backupPath); err == nil {
				agent["last_backup"] = info.ModTime().Format("2006-01-02 15:04:05")
				agent["backup_size"] = info.Size()
			} else {
				agent["last_backup"] = nil
				agent["backup_size"] = int64(0)
			}

			if session != nil && session.SyncData != nil {
				agent["sync_data"] = session.SyncData
			} else {
				agent["sync_data"] = nil
			}

			// Attach Credentials (from live session or DB fallback)
			if session != nil && session.SyncData != nil && session.SyncData["credentials"] != nil {
				agent["credentials"] = session.SyncData["credentials"]
			} else if creds, ok := allCreds[strings.ToLower(subdomain)]; ok {
				agent["credentials"] = creds
			} else {
				agent["credentials"] = nil
			}

			agent["group_name"] = repo.GetSubdomainGroup(subdomain)

			arch, _ := repo.GetSubdomainArch(subdomain)
			agent["arch"] = arch
			statuses, _ := repo.GetAgentOTAStatuses()
			if st, ok := statuses[subdomain]; ok {
				if session != nil && session.Version != "" {
					agent["agent_version"] = session.Version
				} else {
					agent["agent_version"] = st.CurrentVer
				}
				agent["ota_status"] = st.Status
				agent["target_version"] = st.TargetVer
			} else {
				if session != nil && session.Version != "" {
					agent["agent_version"] = session.Version
				} else {
					agent["agent_version"] = "v5.0.0"
				}
				agent["ota_status"] = "idle"
			}

			// Detect Cloud Tenant status and mark as online
			if cloudTenantMgr != nil {
				if _, err := os.Stat(cloudTenantMgr.GetPool().GetTenantDBPath(subdomain)); err == nil {
					agent["online"] = true
					agent["connected"] = true
					agent["mode"] = "cloud"
					agent["agent_version"] = "Cloud Edition"
					if agent["last_seen"] == nil || agent["last_seen"] == "" || agent["last_seen"] == "-" {
						agent["last_seen"] = time.Now().Format(time.RFC3339)
					}
				}
			}
		}

		seen := make(map[string]bool)
		for _, a := range agents {
			if s, ok := a["subdomain"].(string); ok {
				seen[strings.ToLower(s)] = true
			}
		}

		for sub, owner := range owners {
			subLower := strings.ToLower(sub)
			if !seen[subLower] {
				cloudAgent := fiber.Map{
					"subdomain":     sub,
					"online":        true, // Cloud tenant is natively hosted
					"owner_name":    owner.Name,
					"owner_phone":   owner.Phone,
					"company_name":  owner.CompanyName,
					"mode":          "cloud",
					"agent_version": "Cloud Edition",
					"arch":          "x86_64",
					"ip":            c.IP(),
				}
				if lic, ok := licenses[subLower]; ok {
					cloudAgent["license_status"] = lic.Status
					cloudAgent["license_expires_at"] = lic.ExpiresAtStr
					cloudAgent["days_remaining"] = lic.DaysRemaining
					cloudAgent["is_expired"] = lic.IsExpired
				} else {
					cloudAgent["license_status"] = "active"
					cloudAgent["license_expires_at"] = "Active"
					cloudAgent["days_remaining"] = 365
					cloudAgent["is_expired"] = false
				}
				agents = append(agents, cloudAgent)
				seen[subLower] = true
			}
		}

		return c.JSON(agents)
	})

	// ─── Central License Management APIs ───────────────────────────────────────

	app.Post("/api/admin/agents/:subdomain/license/activate", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")
		var req struct {
			Days int `json:"days"`
		}
		_ = c.BodyParser(&req)
		if req.Days <= 0 {
			req.Days = 30
		}

		info, err := repo.ActivateAgentLicense(subdomain, req.Days)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// Push instant license lease to agent via WebSocket tunnel
		_ = svc.SendTunnelMessage(subdomain, "license_lease", fiber.Map{
			"status":         info.Status,
			"expires_at":     info.ExpiresAtStr,
			"days_remaining": info.DaysRemaining,
			"is_expired":     info.IsExpired,
			"valid":          !info.IsExpired && info.Status == "active",
		})

		return c.JSON(fiber.Map{
			"success": true,
			"license": info,
			"message": fmt.Sprintf("تم تفعيل اشتراك الوكيل بنجاح لمدة %d يوماً", req.Days),
		})
	})

	app.Post("/api/admin/agents/:subdomain/license/suspend", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")
		if err := repo.SuspendAgentLicense(subdomain); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// Push instant suspension to agent via WebSocket tunnel
		_ = svc.SendTunnelMessage(subdomain, "license_lease", fiber.Map{
			"status":         "suspended",
			"expires_at":     "",
			"days_remaining": 0,
			"is_expired":     true,
			"valid":          false,
		})

		return c.JSON(fiber.Map{
			"success": true,
			"status":  "suspended",
			"message": "تم تجميد وإيقاف اشتراك الوكيل فورياً",
		})
	})

	app.Post("/api/admin/agents/:subdomain/license/resume", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")
		if err := repo.ResumeAgentLicense(subdomain); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		info, _ := repo.GetAgentLicenseInfo(subdomain)
		// Push instant reactivation to agent via WebSocket tunnel
		_ = svc.SendTunnelMessage(subdomain, "license_lease", fiber.Map{
			"status":         info.Status,
			"expires_at":     info.ExpiresAtStr,
			"days_remaining": info.DaysRemaining,
			"is_expired":     info.IsExpired,
			"valid":          !info.IsExpired && info.Status == "active",
		})

		return c.JSON(fiber.Map{
			"success": true,
			"status":  info.Status,
			"license": info,
			"message": "تم فك التجميد واستعادة اشتراك الوكيل",
		})
	})

	app.Post("/api/agents/:subdomain/reset", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")
		token, ok := svc.RotateToken(subdomain)
		if ok {
			_ = repo.ResetSubdomain(subdomain)
		}
		return c.JSON(fiber.Map{"success": ok, "subdomain": subdomain, "token": token})
	})

	app.Post("/api/agents/:subdomain/delete", func(c *fiber.Ctx) error {
		subdomain := strings.ToLower(strings.TrimSpace(c.Params("subdomain")))
		svc.RemoveAgent(subdomain)
		if cloudTenantMgr != nil {
			_ = cloudTenantMgr.DeleteTenant(subdomain)
		}
		_ = repo.DeleteSubdomain(subdomain)
		return c.JSON(fiber.Map{"success": true, "subdomain": subdomain, "message": "تم حذف الوكيل والنطاق الفرعي وجميع بياناته نهائياً"})
	})

	app.Post("/api/agents/:subdomain/credentials", func(c *fiber.Ctx) error {
		subdomain := strings.ToLower(strings.TrimSpace(c.Params("subdomain")))
		if subdomain == "" {
			return c.Status(400).JSON(fiber.Map{"error": "اسم النطاق مطلوب"})
		}

		var payload struct {
			MikrotikHost string `json:"mikrotik_host"`
			MikrotikUser string `json:"mikrotik_user"`
			MikrotikPass string `json:"mikrotik_pass"`
			PanelUser    string `json:"panel_user"`
			PanelPass    string `json:"panel_pass"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
		}

		credsMap, _ := repo.GetSubdomainCredentialsMap()
		currentCreds := credsMap[subdomain]
		if currentCreds == nil {
			currentCreds = make(map[string]interface{})
		}

		mtCreds, ok := currentCreds["mikrotik"].(map[string]interface{})
		if !ok || mtCreds == nil {
			mtCreds = make(map[string]interface{})
		}
		if payload.MikrotikHost != "" {
			mtCreds["host"] = payload.MikrotikHost
			mtCreds["address"] = payload.MikrotikHost
		}
		if payload.MikrotikUser != "" {
			mtCreds["username"] = payload.MikrotikUser
		}
		if payload.MikrotikPass != "" {
			mtCreds["password"] = payload.MikrotikPass
		}
		currentCreds["mikrotik"] = mtCreds

		panelCreds, ok := currentCreds["panel_admin"].(map[string]interface{})
		if !ok || panelCreds == nil {
			panelCreds = make(map[string]interface{})
		}
		if payload.PanelUser != "" {
			panelCreds["username"] = payload.PanelUser
		}
		if payload.PanelPass != "" {
			panelCreds["password"] = payload.PanelPass
		}
		currentCreds["panel_admin"] = panelCreds

		credsBytes, _ := json.Marshal(currentCreds)
		_ = repo.UpdateSubdomainCredentials(subdomain, string(credsBytes))

		// Push to live agent if connected
		if agent := svc.GetAgentBySubdomain(subdomain); agent != nil {
			if agent.SyncData == nil {
				agent.SyncData = make(map[string]interface{})
			}
			agent.SyncData["credentials"] = currentCreds

			updateBody, _ := json.Marshal(map[string]interface{}{
				"host": payload.MikrotikHost,
				"user": payload.MikrotikUser,
				"pass": payload.MikrotikPass,
			})
			go func() {
				_, _, _ = svc.SendAgentHTTPRequest(subdomain, "POST", "/radius/api/internal/routeros/update-creds", updateBody, nil)
			}()
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": "تم تحديث بيانات الدخول بنجاح ومزامنتها مع الراوتر",
		})
	})

	app.Post("/api/agents/register-from-ui", func(c *fiber.Ctx) error {
		var payload struct {
			Name      string `json:"name"`
			Phone     string `json:"phone"`
			Subdomain string `json:"subdomain"`
			GroupName string `json:"group_name"`
			Token     string `json:"token"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		sub := strings.ToLower(strings.TrimSpace(payload.Subdomain))
		if len(sub) < 3 || len(sub) > 30 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "اسم النطاق غير صالح (يجب أن يكون بين 3 و 30 حرفاً)"})
		}

		available, err := repo.IsSubdomainAvailable(sub)
		if err != nil || !available {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "اسم النطاق هذا محجوز مسبقاً، يرجى اختيار اسم آخر"})
		}

		token := payload.Token
		if token == "" {
			token = fmt.Sprintf("tok-%d-%s", time.Now().Unix(), strings.ToLower(sub))
		}

		customerID := fmt.Sprintf("cust-%d", time.Now().UnixNano())
		customer := storage.Customer{
			ID:          customerID,
			Name:        payload.Name,
			Phone:       payload.Phone,
			Email:       "",
			CompanyName: sub,
			Status:      "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		_ = repo.SaveCustomer(customer)

		licenseID := fmt.Sprintf("lic-%d", time.Now().UnixNano())
		licenseKey := fmt.Sprintf("KEY-%s-%d", strings.ToUpper(sub), time.Now().Unix())
		initExp := time.Now().UTC().Add(30 * 24 * time.Hour)
		planName := payload.GroupName
		if planName == "" {
			planName = "basic"
		}
		_ = repo.SaveLicense(storage.License{
			ID:         licenseID,
			CustomerID: customerID,
			LicenseKey: licenseKey,
			PlanName:   planName,
			Status:     "active",
			IssuedAt:   time.Now(),
			ExpiresAt:  &initExp,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})

		agent := svc.RegisterAgent(sub, token)
		subRecord, err := repo.CreateOrGetSubdomain(customerID, licenseID, sub)
		if err == nil && subRecord != nil {
			subRecord.CustomerID = customerID
			subRecord.LicenseID = licenseID
			subRecord.Token = token
			subRecord.WinboxPort = agent.WinboxPort
			_ = repo.SaveSubdomain(*subRecord)
		}

		if payload.Name != "" || payload.Phone != "" {
			_ = repo.UpdateSubdomainOwner(sub, payload.Name, payload.Phone, sub)
		}

		webURL := fmt.Sprintf("http://%s.%s", agent.Subdomain, centralDomain)
		winboxAddress := fmt.Sprintf("%s:%d", centralDomain, agent.WinboxPort)
		gatewayURL := fmt.Sprintf("wss://%s/ws", centralDomain)

		return c.JSON(fiber.Map{
			"success":        true,
			"subdomain":      agent.Subdomain,
			"token":          agent.Token,
			"winbox_port":    agent.WinboxPort,
			"web_url":        webURL,
			"winbox_address": winboxAddress,
			"gateway_url":    gatewayURL,
			"central_domain": centralDomain,
		})
	})

	app.Post("/api/agents/register", func(c *fiber.Ctx) error {
		var payload struct {
			Subdomain string `json:"subdomain"`
			Token     string `json:"token"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		agent := svc.RegisterAgent(payload.Subdomain, payload.Token)

		// Create default customer & license to preserve DB relational integrity
		_ = repo.SaveCustomer(storage.Customer{ID: "customer-default", Name: "Default Customer", CreatedAt: time.Now(), UpdatedAt: time.Now()})
		_ = repo.SaveLicense(storage.License{ID: "lic-default", CustomerID: "customer-default", LicenseKey: "key-default", PlanName: "basic", Status: "active", IssuedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now()})
		sub, err := repo.CreateOrGetSubdomain("customer-default", "lic-default", payload.Subdomain)
		if err == nil && sub != nil {
			sub.Token = payload.Token
			sub.WinboxPort = agent.WinboxPort
			_ = repo.SaveSubdomain(*sub)
		}

		webURL := fmt.Sprintf("http://%s.%s", agent.Subdomain, centralDomain)
		winboxAddress := fmt.Sprintf("%s:%d", centralDomain, agent.WinboxPort)
		gatewayURL := fmt.Sprintf("wss://%s/ws", centralDomain)

		return c.JSON(fiber.Map{
			"success":          true,
			"subdomain":        agent.Subdomain,
			"token":            agent.Token,
			"winbox_port":      agent.WinboxPort,
			"web_url":          webURL,
			"winbox_address":   winboxAddress,
			"gateway_url":      gatewayURL,
			"central_domain":   centralDomain,
			"tunnel_mode":      "agent",
			"setup_instructions": fmt.Sprintf("الإعداد على النظام المحلي:\n1. وضع التوصيل: Agent\n2. الدومين الفرعي: %s\n3. رابط اللوحة الكامل: %s\n4. رقم منفذ Winbox: %d\n5. عنوان Winbox المباشر: %s\n6. التوكن: %s\n7. بوابة السيرفر: %s", agent.Subdomain, webURL, agent.WinboxPort, winboxAddress, agent.Token, gatewayURL),
		})
	})

	// ─── Subdomain Takeover & Ownership Transfer Admin APIs ─────────────────────

	app.Get("/api/admin/takeovers", func(c *fiber.Ctx) error {
		status := c.Query("status", "all")
		list, err := repo.GetTakeoverRequests(status)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		pendingCount, _ := repo.GetPendingTakeoverCount()
		return c.JSON(fiber.Map{
			"success":       true,
			"requests":      list,
			"pending_count": pendingCount,
		})
	})

	app.Get("/api/admin/takeovers/count", func(c *fiber.Ctx) error {
		count, err := repo.GetPendingTakeoverCount()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{
			"success": true,
			"count":   count,
		})
	})

	app.Post("/api/admin/takeovers/:id/approve", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			AdminNotes string `json:"admin_notes"`
		}
		_ = c.BodyParser(&body)

		subRecord, err := repo.ApproveTakeoverRequest(id, body.AdminNotes)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		// Re-register in active tunnel service memory so old connections are reset with the new token
		if subRecord != nil {
			svc.RegisterAgent(subRecord.Subdomain, subRecord.Token)
		}

		return c.JSON(fiber.Map{
			"success":   true,
			"message":   "تمت الموافقة ونقل ملكية النطاق بنجاح وتوليد توكن جديد للمستخدم",
			"subdomain": subRecord.Subdomain,
			"token":     subRecord.Token,
		})
	})

	app.Post("/api/admin/takeovers/:id/reject", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			AdminNotes string `json:"admin_notes"`
		}
		_ = c.BodyParser(&body)

		if err := repo.RejectTakeoverRequest(id, body.AdminNotes); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": "تم رفض طلب الاستحواذ بنجاح",
		})
	})

	// ─── Broadcast & Ad Campaigns REST APIs ──────────────────────────────────────

	app.Get("/api/broadcasts", func(c *fiber.Ctx) error {
		list, err := repo.ListBroadcasts()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		type broadcastWithStats struct {
			storage.Broadcast
			Impressions int `json:"impressions"`
			Clicks      int `json:"clicks"`
		}
		var res []broadcastWithStats
		for _, b := range list {
			imp, clk, _ := repo.GetBroadcastStats(b.ID)
			res = append(res, broadcastWithStats{
				Broadcast:   b,
				Impressions: imp,
				Clicks:      clk,
			})
		}
		return c.JSON(res)
	})

	app.Post("/api/broadcasts", func(c *fiber.Ctx) error {
		var req struct {
			ID                string `json:"id"`
			Title             string `json:"title"`
			Message           string `json:"message"`
			ImageURL          string `json:"image_url"`
			ActionURL         string `json:"action_url"`
			ActionText        string `json:"action_text"`
			DisplayType       string `json:"display_type"`
			TargetType        string `json:"target_type"`
			TargetAgents      string `json:"target_agents"`
			TargetProfiles    string `json:"target_profiles"`
			Frequency         string `json:"frequency"`
			SplashDurationSec int    `json:"splash_duration_sec"`
			Status            string `json:"status"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		if req.Title == "" || req.Message == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Title and Message are required"})
		}

		if req.ID == "" {
			req.ID = fmt.Sprintf("bc-%d", time.Now().UnixNano())
		}
		if req.Status == "" {
			req.Status = "active"
		}
		if req.DisplayType == "" {
			req.DisplayType = "banner"
		}
		if req.TargetType == "" {
			req.TargetType = "agents"
		}
		if req.TargetAgents == "" {
			req.TargetAgents = "ALL"
		}
		if req.Frequency == "" {
			req.Frequency = "once"
		}
		if req.SplashDurationSec <= 0 {
			req.SplashDurationSec = 10
		}

		b := storage.Broadcast{
			ID:                req.ID,
			Title:             req.Title,
			Message:           req.Message,
			ImageURL:          req.ImageURL,
			ActionURL:         req.ActionURL,
			ActionText:        req.ActionText,
			DisplayType:       req.DisplayType,
			TargetType:        req.TargetType,
			TargetAgents:      req.TargetAgents,
			TargetProfiles:    req.TargetProfiles,
			Frequency:         req.Frequency,
			SplashDurationSec: req.SplashDurationSec,
			Status:            req.Status,
			CreatedBy:         "admin",
		}

		if err := repo.SaveBroadcast(b); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true, "broadcast": b})
	})

	app.Delete("/api/broadcasts/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := repo.DeleteBroadcast(id); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "id": id})
	})

	app.Post("/api/broadcasts/:id/toggle", func(c *fiber.Ctx) error {
		id := c.Params("id")
		b, err := repo.GetBroadcast(id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		newStatus := "active"
		if b.Status == "active" {
			newStatus = "paused"
		}
		if err := repo.UpdateBroadcastStatus(id, newStatus); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "id": id, "status": newStatus})
	})

	app.Post("/api/broadcasts/:id/send", func(c *fiber.Ctx) error {
		id := c.Params("id")
		b, err := repo.GetBroadcast(id)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "broadcast not found"})
		}

		msg := broadcast.BroadcastMessage{
			ID:                b.ID,
			Title:             b.Title,
			Message:           b.Message,
			ImageURL:          b.ImageURL,
			ActionURL:         b.ActionURL,
			ActionText:        b.ActionText,
			DisplayType:       b.DisplayType,
			TargetType:        b.TargetType,
			TargetProfiles:    b.TargetProfiles,
			Frequency:         b.Frequency,
			SplashDurationSec: b.SplashDurationSec,
			CreatedAt:         b.CreatedAt.Format(time.RFC3339),
		}

		sentCount := 0
		if b.TargetAgents == "ALL" || b.TargetAgents == "" {
			rawJSON, _ := json.Marshal(msg)
			svc.BroadcastToAgents(tunnel.TunnelMessage{
				Type:    "broadcast_push",
				Payload: rawJSON,
			})
			for _, a := range svc.ListAgents() {
				if conn, ok := a["connected"].(bool); ok && conn {
					sentCount++
				}
			}
		} else {
			targets := strings.Split(b.TargetAgents, ",")
			for _, t := range targets {
				sub := strings.TrimSpace(t)
				if sub != "" {
					if err := svc.SendTunnelMessage(sub, "broadcast_push", msg); err == nil {
						sentCount++
					}
				}
			}
		}

		return c.JSON(fiber.Map{"success": true, "sent_count": sentCount})
	})

	app.Get("/api/broadcasts/:id/stats", func(c *fiber.Ctx) error {
		id := c.Params("id")
		imp, clk, err := repo.GetBroadcastStats(id)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"impressions": imp, "clicks": clk})
	})

	app.Post("/api/broadcasts/log", func(c *fiber.Ctx) error {
		var payload struct {
			BroadcastID    string `json:"broadcast_id"`
			AgentID        string `json:"agent_id"`
			UserIdentifier string `json:"user_identifier"`
			Clicked        int    `json:"clicked"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		_ = repo.LogBroadcastView(storage.BroadcastLog{
			BroadcastID:    payload.BroadcastID,
			AgentID:        payload.AgentID,
			UserIdentifier: payload.UserIdentifier,
			ViewedAt:       time.Now(),
			Clicked:        payload.Clicked,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// ==========================================
	// SASMAN Global HotSpot & Vouchers Admin APIs
	// ==========================================
	app.Get("/api/global-hotspot/vouchers", func(c *fiber.Ctx) error {
		status := c.Query("status")
		batchID := c.Query("batch_id")
		list, err := repo.GetGlobalVouchers(status, batchID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "vouchers": list, "count": len(list)})
	})

	app.Get("/api/global-hotspot/batches", func(c *fiber.Ctx) error {
		batches, err := repo.GetGlobalVoucherBatches()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "batches": batches})
	})

	app.Post("/api/global-hotspot/vouchers/generate", func(c *fiber.Ctx) error {
		var req struct {
			BatchID       string  `json:"batch_id"`
			Prefix        string  `json:"prefix"`
			Count         int     `json:"count"`
			ProfileName   string  `json:"profile_name"`
			RateLimit     string  `json:"rate_limit"`
			Price         float64 `json:"price"`
			ValidityHours int     `json:"validity_hours"`
			ValidityDays  int     `json:"validity_days"`
			DataLimitMB   int64   `json:"data_limit_mb"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if req.Count <= 0 {
			req.Count = 10
		}
		vouchers, err := repo.GenerateGlobalVouchers(
			req.BatchID, req.Prefix, req.Count, req.ProfileName, req.RateLimit,
			req.Price, req.ValidityHours, req.ValidityDays, req.DataLimitMB, "admin",
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "message": fmt.Sprintf("تم توليد %d كرت موحد بنجاح!", len(vouchers)), "vouchers": vouchers})
	})

	app.Delete("/api/global-hotspot/vouchers/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := repo.DeleteGlobalVoucher(id); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "message": "تم حذف الكرت بنجاح"})
	})

	app.Delete("/api/global-hotspot/vouchers/batch/:batch_id", func(c *fiber.Ctx) error {
		batchID := c.Params("batch_id")
		if err := repo.DeleteGlobalVoucherBatch(batchID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "message": "تم حذف الدفعة بالكامل بنجاح"})
	})

	app.Get("/api/global-hotspot/sessions", func(c *fiber.Ctx) error {
		limit := c.QueryInt("limit", 100)
		sessions, err := repo.GetGlobalHotspotSessions(limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "sessions": sessions})
	})

	app.Get("/api/global-hotspot/template.zip", func(c *fiber.Ctx) error {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)

		entries, err := hotspotFS.ReadDir("hotspot_template")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("read template dir: " + err.Error())
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fData, err := hotspotFS.ReadFile("hotspot_template/" + entry.Name())
			if err != nil {
				continue
			}
			w, err := zw.Create(entry.Name())
			if err != nil {
				continue
			}
			_, _ = w.Write(fData)
		}
		_ = zw.Close()

		c.Set("Content-Type", "application/zip")
		c.Set("Content-Disposition", `attachment; filename="sasman_global_hotspot_template.zip"`)
		return c.Send(buf.Bytes())
	})

	app.Get("/api/tunnel/route/:subdomain", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")
		if svc.GetAgentBySubdomain(subdomain) == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "offline"})
		}
		return c.JSON(fiber.Map{"status": "online", "subdomain": subdomain})
	})

	app.Get("/ws", svc.WebSocketHandler, svc.WebSocketUpgrade)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("central server listening on %s", addr)
	log.Fatal(app.Listen(addr))
}

// normalizeArch converts Go runtime arch strings (runtime.GOARCH) to the
// SASMAN OTA arch identifiers used for binary naming and release matching.
//
// MikroTik device arch reference:
//   - RB4011, RB952, RB951, RB760, RBD52  → arm  (ARM 32-bit / ARMv7)
//   - RB5009, CCR2004, CRS354              → arm64 (ARM 64-bit / AArch64)
//   - CCR1009, CCR1016, CCR1036, x86 VMs  → amd64 (x86 64-bit)
func normalizeArch(goarch string) string {
	switch strings.ToLower(strings.TrimSpace(goarch)) {
	case "arm", "armv7", "armv7l", "armhf":
		return "linux_arm" // 32-bit ARM
	case "arm64", "aarch64", "armv8":
		return "linux_arm64" // 64-bit ARM
	case "amd64", "x86_64":
		return "linux_amd64"
	case "386", "x86":
		return "linux_386"
	case "mips", "mipsel", "mipsle":
		return "linux_mips"
	default:
		if goarch == "" {
			return "linux_arm" // safest MikroTik default
		}
		return "linux_" + strings.ToLower(goarch)
	}
}
