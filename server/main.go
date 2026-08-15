package main

import (
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
	"mikrotik-manager/pkg/relay"
	"mikrotik-manager/pkg/tunnel"
	"mikrotik-manager/server/internal/api"
	"mikrotik-manager/server/internal/backup"
	otainternal "mikrotik-manager/server/internal/ota"
	relayinternal "mikrotik-manager/server/internal/relay"
	"mikrotik-manager/server/internal/storage"
)

//go:embed index.html
var landingHTML string

//go:embed web/*
var webFS embed.FS

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

	// Auto-push active broadcasts and license lease to newly registered/connected agents
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

	svc.OnRelayMessage = func(session *tunnel.AgentSession, msg tunnel.TunnelMessage) {
		if msg.Type == "telemetry_push" {
			relayAPI.IngestTelemetryMessage(session.Subdomain, msg.Payload)
		}
	}

	backupScheduler := backup.NewScheduler(svc)
	backupScheduler.Start()

	otaManager := otainternal.NewManager(repo, svc)
	otaAPI := otainternal.NewAPIHandler(otaManager, repo)

	app := fiber.New(fiber.Config{
		AppName:   "SASMAN Central Server",
		BodyLimit: 256 * 1024 * 1024, // 256MB Max payload limit for large OTA releases and container images
	})

	relayAPI.RegisterRoutes(app)
	otaAPI.RegisterRoutes(app)

	centralDomain := os.Getenv("SASMAN_CENTRAL_DOMAIN")
	if centralDomain == "" {
		centralDomain = "sas-man.net"
	}

	app.Use(func(c *fiber.Ctx) error {
		host := c.Get("Host")
		subdomain := tunnel.ExtractSubdomainForHost(host, centralDomain)
		if subdomain != "" {
			return svc.ForwardRequestToAgent(c, subdomain)
		}
		return c.Next()
	})

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
		diskPrefix := targetDisk
		if diskPrefix == "" {
			diskPrefix = "disk1"
		}
		diskPrefix = strings.TrimSuffix(diskPrefix, "/")
		pullDir := diskPrefix + "/pull"

		script := fmt.Sprintf(`# ==============================================================================
# SASMAN MikroTik Manager v5 - Ultra-Light Native Container Installer
# Auto-generated by SASMAN Central Server: %s
# Target Storage Disk: %s
# ==============================================================================

:put "[*] Checking MikroTik Container package..."
/container
:if ([:len [/container find]] = 0) do={
    :put "[*] Container subsystem ready."
}

:put "[*] Creating required directories on %s..."
/file
:do {
    /file make-dir "%s"
    /file make-dir "%s/data"
    /file make-dir "%s/data/radius_db"
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
:put "[*] Creating Container File Mounts on %s/data..."
/container/mounts
:if ([:len [/container/mounts find name="sasman_data"]] = 0) do={
    /container/mounts add name="sasman_data" src="%s/data" dst="/app/data"
}

# 8. Configure Docker Hub Registry and Pull Latest Multi-Arch Image
:put "[*] Setting up Container Environment & Docker Hub Registry..."
/container config set registry-url=https://registry-1.docker.io tmpdir=%s
:put "[*] Configured Docker Hub Registry: https://registry-1.docker.io"

:put "[*] Pulling latest multi-arch SASMAN image from Docker Hub (ahmedkin99/sasman-manager:latest)..."
/container
:do {
    /container remove [find comment~"sasman"]
} on-error={}

/container add remote-image="ahmedkin99/sasman-manager:latest" interface="veth-sasman" mounts="sasman_data" root-dir="%s/sasman_root" logging=yes comment="sasman-unified-v5"

:put "[*] Installation command issued successfully!"
:put "[*] Wait for the container status to become 'stopped', then run:"
:put "    /container start [find comment~\"sasman\"]"
:put "=============================================================================="
:put "[*] Once started, access your SASMAN Manager panel at: http://<Router-IP>:8080"
:put "=============================================================================="
`, host, diskPrefix, diskPrefix, pullDir, diskPrefix, diskPrefix, diskPrefix, diskPrefix, pullDir, diskPrefix)
		return script
	}

	app.Get("/install.rsc", func(c *fiber.Ctx) error {
		targetDisk := c.Query("disk", "disk1")
		host := c.Query("host", centralDomain)
		script := generateInstaller(targetDisk, host)
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendString(script)
	})

	app.Get("/install.sh", func(c *fiber.Ctx) error {
		targetDisk := c.Query("disk", "disk1")
		host := c.Query("host", centralDomain)
		script := generateInstaller(targetDisk, host)
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendString(script)
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

		return c.JSON(fiber.Map{
			"subdomain": sub,
			"available": available,
			"full_url":  fmt.Sprintf("http://%s.%s", sub, centralDomain),
		})
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
			subRecord.Token = token
			subRecord.WinboxPort = agent.WinboxPort
			_ = repo.SaveSubdomain(*subRecord)
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

	// Agents List API (with Owner + License Information)
	app.Get("/api/agents", func(c *fiber.Ctx) error {
		agents := svc.ListAgents()
		owners, _ := repo.GetSubdomainOwners()
		licenses, _ := repo.GetSubdomainLicensesMap()

		for _, agent := range agents {
			subdomain := agent["subdomain"].(string)
			if subdomain == "" {
				continue
			}

			if owner, ok := owners[strings.ToLower(subdomain)]; ok {
				agent["owner_name"] = owner.Name
				agent["owner_phone"] = owner.Phone
				agent["company_name"] = owner.CompanyName
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

			session := svc.GetAgentBySubdomain(subdomain)
			if session != nil && session.SyncData != nil {
				agent["sync_data"] = session.SyncData
			} else {
				agent["sync_data"] = nil
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
		subdomain := c.Params("subdomain")
		deleted := svc.RemoveAgent(subdomain)
		if deleted {
			_ = repo.DeleteSubdomain(subdomain)
		}
		return c.JSON(fiber.Map{"success": deleted, "subdomain": subdomain})
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
