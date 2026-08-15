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

	// Auto-push active broadcasts to newly registered/connected agents
	svc.OnAgentRegistered = func(subdomain string) {
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
		// Strip port if present in host header for cleaner URL
		if idx := strings.Index(host, ":"); idx != -1 {
			host = host[:idx]
		}

		return fmt.Sprintf(`# ==============================================================================
#           SASMAN Automatic Container & Full MikroTik Setup for RouterOS v7
# ==============================================================================
:put "================================================================="
:put "         Starting SASMAN Unified Auto-Installer & Setup        "
:put "================================================================="

# 1. Check RouterOS Version
:local rosVersion [/system resource get version]
:put ("[*] RouterOS Version: " . $rosVersion)

# 2. Detect Hardware Architecture and Select Matching Image
:local arch [/system resource get architecture-name]
:put ("[*] Hardware Architecture: " . $arch)

:local imageName "sasman-arm64.tar"
:if ($arch = "arm") do={
    :set imageName "sasman-armv7.tar"
} else={
    :if ($arch = "x86_64" || $arch = "x86" || $arch = "tile" || $arch = "mmips") do={
        :set imageName "sasman-amd64.tar"
    }
}
:put ("[*] Selected Target Image: " . $imageName)

# 3. Target Storage Configuration
:local targetDisk "%s"
:put ("[*] Target Storage Location: " . $targetDisk)

# 4. Configure Networking for Container (VETH & Bridge)
:if ([:len [/interface veth find name="veth-sasman"]] = 0) do={
    /interface veth add name=veth-sasman address=172.17.0.2/24 gateway=172.17.0.1
    :put "[+] Created VETH interface: veth-sasman (172.17.0.2)"
}

:if ([:len [/interface bridge find name="bridge-container"]] = 0) do={
    /interface bridge add name=bridge-container
    /ip address add address=172.17.0.1/24 interface=bridge-container
    /interface bridge port add bridge=bridge-container interface=veth-sasman
    :put "[+] Created Bridge interface: bridge-container (172.17.0.1)"
} else={
    :if ([:len [/interface bridge port find interface="veth-sasman"]] = 0) do={
        /interface bridge port add bridge=bridge-container interface=veth-sasman
    }
}

# 5. Configure Firewall NAT Rules (Masquerade + Port Forwarding)
:if ([:len [/ip firewall nat find comment="Container Internet Access"]] = 0) do={
    /ip firewall nat add chain=srcnat src-address=172.17.0.0/24 action=masquerade comment="Container Internet Access"
    :put "[+] Added NAT Masquerade rule for Container"
}

:if ([:len [/ip firewall nat find comment="SASMAN Web Panel"]] = 0) do={
    /ip firewall nat add chain=dstnat dst-port=88 protocol=tcp action=dst-nat to-addresses=172.17.0.2 to-ports=80 comment="SASMAN Web Panel"
    :put "[+] Added NAT Port Forward: TCP 88 -> 172.17.0.2:80 (Web Panel)"
}

:if ([:len [/ip firewall nat find comment="RADIUS Auth"]] = 0) do={
    /ip firewall nat add chain=dstnat dst-port=1812 protocol=udp action=dst-nat to-addresses=172.17.0.2 to-ports=1812 comment="RADIUS Auth"
    :put "[+] Added NAT Port Forward: UDP 1812 -> 172.17.0.2:1812 (RADIUS Auth)"
}

:if ([:len [/ip firewall nat find comment="RADIUS Acct"]] = 0) do={
    /ip firewall nat add chain=dstnat dst-port=1813 protocol=udp action=dst-nat to-addresses=172.17.0.2 to-ports=1813 comment="RADIUS Acct"
    :put "[+] Added NAT Port Forward: UDP 1813 -> 172.17.0.2:1813 (RADIUS Acct)"
}

# 6. Configure Firewall Filter Rules (Input Accept)
:if ([:len [/ip firewall filter find comment="Allow RADIUS"]] = 0) do={
    /ip firewall filter add chain=input action=accept protocol=udp dst-port=1812,1813 comment="Allow RADIUS"
    :put "[+] Added Firewall Filter: Accept UDP 1812, 1813"
}

:if ([:len [/ip firewall filter find comment="Allow SASMAN Web"]] = 0) do={
    /ip firewall filter add chain=input action=accept protocol=tcp dst-port=88 comment="Allow SASMAN Web"
    :put "[+] Added Firewall Filter: Accept TCP 88"
}

# 7. Configure MikroTik RADIUS Client & PPP AAA
:if ([:len [/radius find address="172.17.0.2"]] = 0) do={
    /radius add address=172.17.0.2 secret="123456" service=ppp,hotspot,login timeout=3000ms authentication-port=1812 accounting-port=1813 comment="SASMAN RADIUS"
    :put "[+] Configured MikroTik RADIUS Client pointing to 172.17.0.2"
}

/ppp aaa set use-radius=yes accounting=yes interim-update=1m
:put "[+] Enabled RADIUS Accounting & Authentication for PPP/PPPoE"

# 8. Configure Docker Hub Registry and Pull Latest Multi-Arch Image
:local pullDir ($targetDisk . "/pull")
:local rootDir ($targetDisk . "/sasman-data")

/container config set registry-url=https://registry-1.docker.io tmpdir=$pullDir
:put "[*] Configured Docker Hub Registry: https://registry-1.docker.io"

:put "[*] Pulling latest multi-arch SASMAN image from Docker Hub (ahmedkin99/sasman-manager:latest)..."
:if ([:len [/container find comment="SASMAN Manager"]] = 0) do={
    /container add remote-image="ahmedkin99/sasman-manager:latest" interface=veth-sasman root-dir=$rootDir start-on-boot=yes logging=yes comment="SASMAN Manager"
}

:put "[*] Waiting for download and container initialization..."
:delay 35s

:local cId [/container find comment="SASMAN Manager"]
:if ([:len $cId] > 0) do={
    /container start $cId
    :put "================================================================="
    :put " [✔] SASMAN Container successfully installed and started!"
    :put "     Open http://[Router-IP]:88 in your browser to access web panel."
    :put "================================================================="
} else={
    :put "[!] Container is initializing in background. Run '/container print' to check status."
}
`, targetDisk)
	}

	// 1. Script for Internal Disk Installation (disk1)
	app.Get("/install-disk.rsc", func(c *fiber.Ctx) error {
		host := c.Get("Host")
		script := generateInstaller("disk1", host)
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendString(script)
	})

	// Default alias for /install.rsc -> disk1
	app.Get("/install.rsc", func(c *fiber.Ctx) error {
		host := c.Get("Host")
		script := generateInstaller("disk1", host)
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendString(script)
	})

	// 2. Script for External USB Installation (usb1)
	app.Get("/install-usb.rsc", func(c *fiber.Ctx) error {
		host := c.Get("Host")
		script := generateInstaller("usb1", host)
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
			if err == nil {
				for _, rel := range releases {
					if rel.TargetArch == targetArch {
						_, binData, err := repo.GetRelease(rel.Version, rel.TargetArch)
						if err == nil && len(binData) > 0 {
							c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
							c.Set("Content-Type", "application/octet-stream")
							return c.Send(binData)
						}
					}
				}
			}
		}

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "File not found: " + filename,
		})
	})

	// ─── Public Self-Service Agent Registration & Subdomain Validation ──────────

	app.Post("/api/agents/check-subdomain", func(c *fiber.Ctx) error {
		var req struct {
			Subdomain string `json:"subdomain"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		subdomain := strings.ToLower(strings.TrimSpace(req.Subdomain))
		if subdomain == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"available": false, "error": "يرجى كتابة اسم النطاق المطلوب"})
		}

		// Validation rules: 3-30 chars, alphanumeric + hyphens only
		if len(subdomain) < 3 || len(subdomain) > 30 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"available": false, "error": "يجب أن يكون طول النطاق بين 3 و 30 حرفاً"})
		}

		// Check reserved names
		reserved := map[string]bool{"admin": true, "api": true, "ws": true, "mail": true, "vpn": true, "radius": true, "portal": true, "system": true, "root": true}
		if reserved[subdomain] {
			return c.JSON(fiber.Map{"available": false, "subdomain": subdomain, "error": "هذا الاسم محجوز للنظام، يرجى اختيار اسم آخر"})
		}

		for _, r := range subdomain {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"available": false, "error": "يجب أن يحتوي النطاق على أحرف إنجليزية وأرقام وشرطة فقط"})
			}
		}

		avail, err := repo.IsSubdomainAvailable(subdomain)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		fullDomain := fmt.Sprintf("%s.%s", subdomain, centralDomain)
		if !avail {
			return c.JSON(fiber.Map{
				"available":   false,
				"subdomain":   subdomain,
				"full_domain": fullDomain,
				"error":       "هذا النطاق مستخدم بالفعل من قبل وكيل آخر، يرجى اختيار اسم مختلف",
			})
		}

		return c.JSON(fiber.Map{
			"available":   true,
			"subdomain":   subdomain,
			"full_domain": fullDomain,
			"message":     "النطاق متاح وجاهز للاستخدام!",
		})
	})

	app.Post("/api/agents/self-register", func(c *fiber.Ctx) error {
		var req struct {
			Name      string `json:"name"`
			Phone     string `json:"phone"`
			Subdomain string `json:"subdomain"`
			Serial    string `json:"serial"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		name := strings.TrimSpace(req.Name)
		phone := strings.TrimSpace(req.Phone)
		subdomain := strings.ToLower(strings.TrimSpace(req.Subdomain))

		if name == "" || phone == "" || subdomain == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "الاسم الكامل، رقم الهاتف، واسم النطاق هي حقول مطلوبة"})
		}

		if len(subdomain) < 3 || len(subdomain) > 30 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "يجب أن يكون طول النطاق بين 3 و 30 حرفاً"})
		}

		for _, r := range subdomain {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "يجب أن يحتوي النطاق على أحرف إنجليزية وأرقام وشرطة فقط"})
			}
		}

		avail, err := repo.IsSubdomainAvailable(subdomain)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if !avail {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "اسم النطاق مستخدم بالفعل، يرجى اختيار اسم آخر",
			})
		}

		// 1. Create or Save Customer record with Name & Phone
		custID := fmt.Sprintf("cust-%d", time.Now().UnixNano())
		_ = repo.SaveCustomer(storage.Customer{
			ID:          custID,
			Name:        name,
			Phone:       phone,
			CompanyName: name,
			Status:      "active",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		})

		// 2. Generate secure token & Register Agent session
		token := fmt.Sprintf("tok-%d-%d", time.Now().UnixNano(), time.Now().Unix()%100000)
		agent := svc.RegisterAgent(subdomain, token)

		// 3. Create License record
		licenseID := fmt.Sprintf("license-%s", agent.ID)
		_ = repo.SaveLicense(storage.License{
			ID:         licenseID,
			CustomerID: custID,
			LicenseKey: fmt.Sprintf("KEY-%s", agent.ID),
			HWUUID:     req.Serial,
			Status:     "active",
			IssuedAt:   time.Now(),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})

		// 4. Create Subdomain record in SQLite
		_, _ = repo.CreateOrGetSubdomain(custID, licenseID, agent.Subdomain)

		webURL := fmt.Sprintf("http://%s.%s", agent.Subdomain, centralDomain)
		winboxAddress := fmt.Sprintf("%s.%s:%d", agent.Subdomain, centralDomain, agent.WinboxPort)
		gatewayURL := fmt.Sprintf("wss://%s/ws", centralDomain)

		return c.JSON(fiber.Map{
			"success":        true,
			"agent_id":       agent.ID,
			"subdomain":      agent.Subdomain,
			"token":          agent.Token,
			"winbox_port":    agent.WinboxPort,
			"winbox_address": winboxAddress,
			"web_url":        webURL,
			"gateway_url":    gatewayURL,
			"central_domain": centralDomain,
			"full_domain":    fmt.Sprintf("%s.%s", agent.Subdomain, centralDomain),
			"owner_name":     name,
			"owner_phone":    phone,
			"message":        "تم حجز النطاق وتسجيل الوكيل بنجاح!",
		})
	})

	// Protect all administrative and agent management APIs with JWT Auth Middleware
	app.Use(authManager.Middleware())

	// Group management endpoints
	app.Get("/api/groups", func(c *fiber.Ctx) error {
		groups, err := repo.ListAgentGroups()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"groups": groups})
	})

	app.Post("/api/agents/:subdomain/group", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")
		var payload struct {
			GroupName string `json:"group_name"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if err := repo.UpdateSubdomainGroup(subdomain, payload.GroupName); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "subdomain": subdomain, "group_name": payload.GroupName})
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

	app.Get("/api/agents", func(c *fiber.Ctx) error {
		agents := svc.ListAgents()

		for _, agent := range agents {
			subdomain := agent["subdomain"].(string)
			if subdomain == "" {
				continue
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
		_ = repo.SaveLicense(storage.License{ID: fmt.Sprintf("license-%s", agent.ID), CustomerID: "customer-default", LicenseKey: fmt.Sprintf("KEY-%s", agent.ID), Status: "active", IssuedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now()})

		_, _ = repo.CreateOrGetSubdomain("customer-default", fmt.Sprintf("license-%s", agent.ID), agent.Subdomain)

		webURL := fmt.Sprintf("http://%s.%s", agent.Subdomain, centralDomain)
		winboxAddress := fmt.Sprintf("%s.%s:%d", agent.Subdomain, centralDomain, agent.WinboxPort)
		gatewayURL := fmt.Sprintf("ws://%s/ws", c.Hostname())

		return c.JSON(fiber.Map{
			"agent_id":         agent.ID,
			"subdomain":        agent.Subdomain,
			"token":            agent.Token,
			"web_url":          webURL,
			"winbox_port":      agent.WinboxPort,
			"winbox_address":   winboxAddress,
			"gateway_url":      gatewayURL,
			"central_domain":   centralDomain,
			"tunnel_mode":      "agent",
			"setup_instructions": fmt.Sprintf("الإعداد على النظام المحلي:\n1. وضع التوصيل: Agent\n2. الدومين الفرعي: %s\n3. رابط اللوحة الكامل: %s\n4. رقم منفذ Winbox: %d\n5. عنوان Winbox المباشر: %s\n6. التوكن: %s\n7. بوابة السيرفر: %s", agent.Subdomain, webURL, agent.WinboxPort, winboxAddress, agent.Token, gatewayURL),
		})
	})

	app.Post("/api/agents/register-from-ui", func(c *fiber.Ctx) error {
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
		_ = repo.SaveLicense(storage.License{ID: fmt.Sprintf("license-%s", agent.ID), CustomerID: "customer-default", LicenseKey: fmt.Sprintf("KEY-%s", agent.ID), Status: "active", IssuedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now()})

		_, _ = repo.CreateOrGetSubdomain("customer-default", fmt.Sprintf("license-%s", agent.ID), agent.Subdomain)

		webURL := fmt.Sprintf("http://%s.%s", agent.Subdomain, centralDomain)
		winboxAddress := fmt.Sprintf("%s.%s:%d", agent.Subdomain, centralDomain, agent.WinboxPort)
		gatewayURL := fmt.Sprintf("ws://%s/ws", c.Hostname())

		return c.JSON(fiber.Map{
			"agent_id":         agent.ID,
			"subdomain":        agent.Subdomain,
			"token":            agent.Token,
			"web_url":          webURL,
			"winbox_port":      agent.WinboxPort,
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
