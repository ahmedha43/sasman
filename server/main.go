package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

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
		loginHTML := `<!doctype html>
<html lang="ar" dir="rtl">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>تسجيل الدخول - SASMAN Central Admin</title>
  <style>
    :root { color-scheme: dark; }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: 'Segoe UI', Tahoma, Arial, sans-serif; background: radial-gradient(circle at top right, #0f172a, #020617); color: #e2e8f0; min-height: 100vh; display: flex; justify-content: center; align-items: center; padding: 20px; }
    .login-card { background: rgba(15, 23, 42, 0.85); backdrop-filter: blur(16px); border: 1px solid #334155; border-radius: 20px; padding: 36px; width: 100%; max-width: 420px; box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.6); }
    h1 { font-size: 1.6rem; font-weight: 800; margin-bottom: 8px; text-align: center; background: linear-gradient(135deg, #38bdf8, #818cf8); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
    p.subtitle { text-align: center; color: #94a3b8; font-size: 13px; margin-bottom: 28px; }
    .form-group { margin-bottom: 18px; }
    label { display: block; font-size: 13px; font-weight: 600; color: #cbd5e1; margin-bottom: 6px; }
    input { width: 100%; padding: 12px 16px; border-radius: 10px; border: 1px solid #475569; background: #0b1329; color: #f8fafc; font-size: 14px; outline: none; transition: border-color 0.2s; }
    input:focus { border-color: #38bdf8; box-shadow: 0 0 0 3px rgba(56, 189, 248, 0.15); }
    button { width: 100%; padding: 13px; border-radius: 10px; border: none; background: linear-gradient(135deg, #0284c7, #2563eb); color: #fff; font-size: 15px; font-weight: bold; cursor: pointer; transition: transform 0.15s, box-shadow 0.15s; margin-top: 10px; }
    button:hover { transform: translateY(-1px); box-shadow: 0 6px 20px rgba(37, 99, 235, 0.4); }
    .error-msg { background: rgba(239, 68, 68, 0.15); border: 1px solid #ef4444; border-radius: 8px; padding: 10px 14px; font-size: 13px; color: #fca5a5; margin-bottom: 16px; display: none; }
    .footer { text-align: center; margin-top: 24px; font-size: 11px; color: #64748b; }
  </style>
</head>
<body>
  <div class="login-card">
    <h1>SASMAN Cloud Server</h1>
    <p class="subtitle">لوحة التحكم المركزية وإدارة شبكة الوكلاء</p>
    
    <div id="error-box" class="error-msg"></div>

    <form onsubmit="handleLogin(event)">
      <div class="form-group">
        <label>اسم المستخدم (Username):</label>
        <input type="text" id="username" placeholder="admin" required autofocus>
      </div>
      <div class="form-group">
        <label>كلمة المرور (Password):</label>
        <input type="password" id="password" placeholder="••••••••" required>
      </div>
      <button type="submit" id="submit-btn">🔐 تسجيل الدخول</button>
    </form>
    <div class="footer">SASMAN Central Architecture v5.0 • Protected with Anti-BruteForce & JWT</div>
  </div>

  <script>
    async function handleLogin(e) {
      e.preventDefault();
      const errBox = document.getElementById('error-box');
      const btn = document.getElementById('submit-btn');
      errBox.style.display = 'none';
      btn.disabled = true;
      btn.innerText = 'جاري التحقق...';

      const username = document.getElementById('username').value.trim();
      const password = document.getElementById('password').value;

      try {
        const res = await fetch('/api/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ username, password })
        });
        const data = await res.json();
        if (data.success) {
          window.location.href = '/admin';
        } else {
          errBox.innerText = data.error || 'فشل تسجيل الدخول';
          errBox.style.display = 'block';
        }
      } catch (err) {
        errBox.innerText = 'خطأ في الاتصال بالسيرفر';
        errBox.style.display = 'block';
      } finally {
        btn.disabled = false;
        btn.innerText = '🔐 تسجيل الدخول';
      }
    }
  </script>
</body>
</html>`
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(loginHTML)
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
		htmlContent := `<!doctype html>
<html lang="ar" dir="rtl">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>SASMAN Central Admin</title>
  <style>
    :root { color-scheme: dark; }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: 'Segoe UI', Tahoma, Arial, sans-serif; padding: 24px; background: linear-gradient(135deg, #020617, #111827); color: #e2e8f0; min-height: 100vh; }
    .card { background: rgba(15, 23, 42, 0.85); border: 1px solid #334155; border-radius: 16px; padding: 24px; box-shadow: 0 10px 30px rgba(0,0,0,0.25); margin-bottom: 24px; }
    h1 { font-size: 1.8rem; margin-bottom: 8px; background: linear-gradient(135deg, #22c55e, #3b82f6); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
    h3 { font-size: 1.1rem; margin-bottom: 12px; color: #94a3b8; border-bottom: 1px solid #334155; padding-bottom: 8px; }
    .muted { color: #94a3b8; font-size: 14px; margin-bottom: 16px; }
    input { padding: 12px 16px; border-radius: 10px; border: 1px solid #475569; margin: 6px 0; width: 100%; max-width: 320px; background: #0f172a; color: #e2e8f0; font-size: 14px; }
    input::placeholder { color: #64748b; }
    button { padding: 12px 24px; border-radius: 10px; border: none; cursor: pointer; color: white; font-weight: 700; font-size: 14px; transition: transform 0.2s, box-shadow 0.2s; }
    button:hover { transform: translateY(-1px); box-shadow: 0 4px 12px rgba(0,0,0,0.3); }
    .btn-primary { background: linear-gradient(135deg, #22c55e, #16a34a); }
    .btn-warning { background: linear-gradient(135deg, #f59e0b, #d97706); }
    .btn-danger { background: linear-gradient(135deg, #ef4444, #dc2626); }
    .btn-copy { background: linear-gradient(135deg, #3b82f6, #2563eb); width: auto; padding: 8px 16px; font-size: 13px; }
    .btn-copy-all { background: linear-gradient(135deg, #8b5cf6, #7c3aed); width: auto; padding: 12px 28px; font-size: 15px; }
    .small-btn { width: auto; padding: 7px 12px; margin: 2px; font-size: 12px; }
    table { width: 100%; border-collapse: collapse; margin-top: 12px; }
    th, td { padding: 12px; border-bottom: 1px solid #334155; text-align: right; }
    th { color: #94a3b8; font-size: 13px; text-transform: uppercase; letter-spacing: .06em; }
    tbody tr:hover { background: rgba(51, 65, 85, 0.4); }
    .badge { display: inline-block; padding: 4px 12px; border-radius: 999px; font-size: 12px; font-weight: 700; }
    .online { background: #14532d; color: #dcfce7; }
    .offline { background: #7f1d1d; color: #fee2e2; }
    .result-box { background: #0f172a; border: 2px solid #22c55e; border-radius: 12px; padding: 20px; margin-top: 16px; display: none; }
    .result-box h3 { color: #22c55e; border-color: #14532d; }
    .field-row { display: flex; align-items: center; gap: 10px; margin: 8px 0; flex-wrap: wrap; }
    .field-label { min-width: 160px; color: #94a3b8; font-weight: 600; font-size: 14px; }
    .field-value { flex: 1; background: #1e293b; padding: 10px 14px; border-radius: 8px; font-family: monospace; font-size: 14px; color: #22c55e; word-break: break-all; border: 1px solid #334155; min-width: 200px; }
    .copy-success { background: #14532d; color: #dcfce7; padding: 4px 10px; border-radius: 6px; font-size: 12px; display: none; }
    .setup-box { background: #1e293b; border-radius: 12px; padding: 16px; margin-top: 12px; border: 1px solid #334155; white-space: pre-wrap; font-family: monospace; font-size: 13px; line-height: 1.8; color: #e2e8f0; direction: ltr; text-align: left; }
    .error-box { background: rgba(239,68,68,0.15); border: 1px solid #ef4444; border-radius: 10px; padding: 14px; margin-top: 12px; display: none; color: #fca5a5; }
    .web-link { color: #38bdf8; text-decoration: none; font-weight: bold; }
    .web-link:hover { text-decoration: underline; }
  </style>
</head>
<body>
  <!-- Top Session Navigation Bar -->
  <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px; background:#0f172a; padding:12px 20px; border-radius:12px; border:1px solid #334155; flex-wrap:wrap; gap:10px;">
    <div style="display:flex; align-items:center; gap:10px;">
      <span style="background:#14532d; color:#86efac; padding:4px 12px; border-radius:6px; font-size:12px; font-weight:bold;">🟢 متصل (Admin Session)</span>
      <span style="color:#94a3b8; font-size:13px;">SASMAN Central Management Cloud v5.0</span>
    </div>
    <div style="display:flex; gap:8px;">
      <button class="small-btn" style="background:#334155;" onclick="openChangePasswordModal()">🔑 تغيير كلمة المرور</button>
      <button class="small-btn btn-danger" onclick="logout()">🚪 تسجيل الخروج</button>
    </div>
  </div>

  <div class="card">
    <h1>SASMAN Central Admin</h1>
    <p class="muted">إدارة النطاقات الفرعية، التوكنات، وروابط اللوحات ومنافذ Winbox المباشرة للوكلاء (Agents).</p>
    
    <h3>➕ تسجيل وكيل جديد (Register Agent)</h3>
    <p class="muted" style="margin-bottom:12px;">أدخل الدومين والتوكن (اختياري - سيتم توليدها تلقائياً إذا تركتها فارغة).</p>
    <div style="display:flex; gap:10px; flex-wrap:wrap; align-items:flex-end;">
      <div>
        <label style="display:block; color:#94a3b8; font-size:13px; margin-bottom:4px;">الدومين الفرعي (Subdomain):</label>
        <input type="text" id="reg-subdomain" placeholder="customer-xyz (أو اتركه فارغ)">
      </div>
      <div>
        <label style="display:block; color:#94a3b8; font-size:13px; margin-bottom:4px;">التوكن (Token):</label>
        <input type="text" id="reg-token" placeholder="tok-xxxx (أو اتركه فارغ)">
      </div>
      <button class="btn-primary" onclick="registerAgent()">🚀 تسجيل الآن</button>
    </div>
    
    <div class="error-box" id="error-box"></div>
    
    <!-- Result Box - shows after registration -->
    <div class="result-box" id="result-box">
      <h3>✅ تم التسجيل بنجاح! انسخ البيانات التالية وأرسلها للمستخدم</h3>
      <p class="muted" style="margin-top:8px;">هذه هي جميع الحقول المطلوبة لإعداد الاتصال المباشر على النظام المحلي، رابط لوحة التحكم، وتطبيق Winbox.</p>
      
      <div class="field-row">
        <span class="field-label">1. وضع التوصيل (Mode):</span>
        <span class="field-value" id="res-tunnel-mode">agent</span>
        <button class="btn-copy" onclick="copyField('res-tunnel-mode')">📋 نسخ</button>
      </div>
      <div class="field-row">
        <span class="field-label">2. الدومين الفرعي (Subdomain):</span>
        <span class="field-value" id="res-subdomain">-</span>
        <button class="btn-copy" onclick="copyField('res-subdomain')">📋 نسخ</button>
      </div>
      <div class="field-row">
        <span class="field-label">3. رابط اللوحة الكامل (Web Dashboard URL):</span>
        <span class="field-value" id="res-web-url">-</span>
        <button class="btn-copy" onclick="copyField('res-web-url')">📋 نسخ الرابط</button>
      </div>
      <div class="field-row">
        <span class="field-label">4. عنوان Winbox المباشر (Direct Winbox):</span>
        <span class="field-value" id="res-winbox-address">-</span>
        <button class="btn-copy" onclick="copyField('res-winbox-address')">📋 نسخ Winbox</button>
      </div>
      <div class="field-row">
        <span class="field-label">5. رقم منفذ Winbox (Port):</span>
        <span class="field-value" id="res-winbox-port">-</span>
        <button class="btn-copy" onclick="copyField('res-winbox-port')">📋 نسخ المنفذ</button>
      </div>
      <div class="field-row">
        <span class="field-label">6. التوكن (Token):</span>
        <span class="field-value" id="res-token">-</span>
        <button class="btn-copy" onclick="copyField('res-token')">📋 نسخ</button>
      </div>
      <div class="field-row">
        <span class="field-label">7. بوابة السيرفر (Gateway URL):</span>
        <span class="field-value" id="res-gateway-url">-</span>
        <button class="btn-copy" onclick="copyField('res-gateway-url')">📋 نسخ</button>
      </div>
      <div class="field-row">
        <span class="field-label">8. الدومين المركزي (Central Domain):</span>
        <span class="field-value" id="res-central-domain">-</span>
        <button class="btn-copy" onclick="copyField('res-central-domain')">📋 نسخ</button>
      </div>
      <div class="field-row">
        <span class="field-label">9. معرف الوكيل (Agent ID):</span>
        <span class="field-value" id="res-agent-id">-</span>
        <button class="btn-copy" onclick="copyField('res-agent-id')">📋 نسخ</button>
      </div>
      
      <div style="margin-top:16px; padding-top:16px; border-top:1px solid #334155;">
        <h3 style="color:#3b82f6;">📋 نص الإعداد الكامل (للنسخ دفعة واحدة):</h3>
        <div class="setup-box" id="res-setup-instructions"></div>
        <button class="btn-copy-all" style="margin-top:12px;" onclick="copyAllSetup()">📋 نسخ كل الإعدادات دفعة واحدة</button>
      </div>
    </div>
  </div>
  
  <div class="card">
    <h3>📡 الوكلاء المتصلون حالياً (Live Agents)</h3>
    <div id="agents"></div>
  </div>

  <!-- SASMAN Service Relay Smart Mesh Card -->
  <div class="card" style="border: 1px solid #3b82f6; background: rgba(15, 23, 42, 0.95); box-shadow: 0 0 25px rgba(59, 130, 246, 0.15);">
    <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; margin-bottom:12px; border-bottom:1px solid #334155; padding-bottom:10px;">
      <div>
        <h2 style="font-size:1.4rem; color:#38bdf8; display:flex; align-items:center; gap:8px;">
          🌐 SASMAN Service Relay <span class="badge online" style="font-size:11px;">Smart Mesh Engine Active</span>
        </h2>
        <p class="muted" style="margin-bottom:0;">نظام التوجيه الذكي للخدمات المحجوبة أو الحصرية (سينمانا، شبكتي، CDNs) عبر أفضل Agent مزوّد وبأقل Latency.</p>
      </div>
      <div style="display:flex; gap:8px; margin-top:8px;">
        <button class="btn-primary" onclick="openAddServiceModal()" style="padding:8px 16px; font-size:13px;">➕ إضافة خدمة جديدة</button>
        <button class="btn-warning" onclick="recalculateRelayRoutes()" style="padding:8px 16px; font-size:13px;">⚡ إعادة حساب التوجيه (Recalculate)</button>
      </div>
    </div>

    <!-- Live Services & Active Egress Table -->
    <h3 style="color:#94a3b8; font-size:1rem; margin-top:16px;">📋 جدول الخدمات النشطة ومسار التوجيه (Active Egress Routes)</h3>
    <div id="relay-services-table" style="margin-top:8px;">
      <p class="muted">جاري تحميل الخدمات...</p>
    </div>

    <!-- Agent Telemetry Matrix -->
    <h3 style="color:#94a3b8; font-size:1rem; margin-top:24px;">📊 مصفوفة فحص الصحة ومقاييس الوكلاء (Live Agents Telemetry Matrix)</h3>
    <p class="muted" style="font-size:12px; margin-bottom:8px;">نتائج فحص الـ Health Checks الدورية (Ping، Packet Loss، CPU) المرسلة من الوكلاء إلى الـ Cloud.</p>
    <div id="relay-telemetry-matrix" style="margin-top:8px;">
      <p class="muted">جاري تحميل مصفوفة المقاييس...</p>
    </div>
  </div>

  <!-- Add Service Modal -->
  <div id="add-service-modal" style="display:none; position:fixed; top:0; left:0; width:100vw; height:100vh; background:rgba(0,0,0,0.75); z-index:9999; justify-content:center; align-items:center;">
    <div style="background:#0f172a; border:1px solid #38bdf8; border-radius:16px; padding:24px; max-width:600px; width:92%; box-shadow:0 20px 50px rgba(0,0,0,0.5); max-height:90vh; overflow-y:auto;">
      <h3 style="color:#38bdf8; margin-bottom:12px;">➕ إضافة أو تعديل خدمة في SASMAN Relay</h3>
      <div style="display:flex; flex-direction:column; gap:10px;">
        <div style="display:flex; gap:10px;">
          <div style="flex:1;">
            <label style="font-size:12px; color:#94a3b8;">معرّف الخدمة (ID بالإنجليزية):</label>
            <input type="text" id="modal-svc-id" placeholder="cinemana" style="width:100%; max-width:100%;">
          </div>
          <div style="flex:1;">
            <label style="font-size:12px; color:#94a3b8;">اسم الخدمة (Display Name):</label>
            <input type="text" id="modal-svc-name" placeholder="Shabakaty Cinemana" style="width:100%; max-width:100%;">
          </div>
        </div>

        <div style="display:flex; gap:10px;">
          <div style="flex:1;">
            <label style="font-size:12px; color:#94a3b8;">التصنيف (Category):</label>
            <input type="text" id="modal-svc-cat" placeholder="Streaming, CDN, Gaming" value="Streaming" style="width:100%; max-width:100%;">
          </div>
          <div style="flex:1;">
            <label style="font-size:12px; color:#94a3b8;">نطاق الاستفادة (Access Control):</label>
            <select id="modal-svc-scope" onchange="toggleScopeInputs()" style="padding:12px 16px; border-radius:10px; border:1px solid #475569; width:100%; background:#0f172a; color:#e2e8f0; font-size:14px; margin-top:6px;">
              <option value="all">🌐 متاح لجميع الوكلاء (All Agents)</option>
              <option value="selected">🎯 وكلاء أو مجموعات محددة فقط (Selected Targets)</option>
            </select>
          </div>
        </div>

        <div id="consumers-field-box" style="display:none; background:#1e293b; padding:12px; border-radius:8px; border:1px dashed #38bdf8;">
          <label style="font-size:12px; color:#38bdf8; font-weight:bold;">1. المجموعات المستهدفة (Target Groups):</label>
          <p class="muted" style="font-size:11px; margin-bottom:4px;">أدخل أسماء المجموعات مفصولة بفواصل (مثال: VIP, Baghdad, South)</p>
          <input type="text" id="modal-svc-groups" placeholder="VIP, Baghdad" style="width:100%; max-width:100%; margin-bottom:10px;">

          <label style="font-size:12px; color:#38bdf8; font-weight:bold;">2. وكلاء محددون بالاسم (Allowed Consumer Subdomains):</label>
          <p class="muted" style="font-size:11px; margin-bottom:4px;">أدخل الدومينات الفرعية مفصولة بفواصل (مثال: customer-1, customer-2)</p>
          <input type="text" id="modal-svc-consumers" placeholder="customer-1, customer-2" style="width:100%; max-width:100%;">
        </div>

        <div>
          <label style="font-size:12px; color:#94a3b8; font-weight:bold;">النطاقات والعناوين المشمولة (Domains):</label>
          <p class="muted" style="font-size:11px; margin-bottom:4px;">يمكنك لصق قائمة عناوين كاملة (كل عنوان في سطر، أو مفصولة بفواصل أو مسافات):</p>
          <textarea id="modal-svc-domains" rows="6" placeholder="cdn.shabakaty.cc&#10;se.shabakaty.cc&#10;cinemana.shabakaty.cc&#10;*.shabakaty.cc" style="width:100%; max-width:100%; font-family:monospace; font-size:12px; line-height:1.5; padding:8px; border-radius:8px; border:1px solid #334155; background:#020617; color:#38bdf8; resize:vertical;"></textarea>
        </div>

        <div>
          <label style="font-size:12px; color:#94a3b8;">رابط فحص الصحة الدوري (Health Probe URL):</label>
          <input type="text" id="modal-svc-probe" placeholder="https://cinemana.shabakaty.cc (أو اتركه فارغاً للتحقق التلقائي)" style="width:100%; max-width:100%;">
        </div>

        <div>
          <label style="font-size:12px; color:#94a3b8;">الوكلاء المزودون المسموحون (Allowed Egress Providers - اختياري، اتركه فارغاً لأي مزود):</label>
          <input type="text" id="modal-svc-providers" placeholder="اتركه فارغاً للاختيار التلقائي من أي مزود" style="width:100%; max-width:100%;">
        </div>
      </div>
      <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:20px;">
        <button onclick="closeAddServiceModal()" style="background:#334155;">إلغاء</button>
        <button class="btn-primary" onclick="submitAddService()">💾 حفظ وتطبيق التحكم فوراً</button>
      </div>
    </div>
  </div>

  <!-- Change Password Modal -->
  <div id="change-password-modal" style="display:none; position:fixed; top:0; left:0; width:100vw; height:100vh; background:rgba(0,0,0,0.75); z-index:9999; justify-content:center; align-items:center;">
    <div style="background:#0f172a; border:1px solid #f59e0b; border-radius:16px; padding:24px; max-width:440px; width:90%; box-shadow:0 20px 50px rgba(0,0,0,0.5);">
      <h3 style="color:#f59e0b; margin-bottom:12px;">🔑 تغيير كلمة مرور لوحة السيرفر</h3>
      <div style="display:flex; flex-direction:column; gap:10px;">
        <div>
          <label style="font-size:12px; color:#94a3b8;">كلمة المرور الحالية:</label>
          <input type="password" id="modal-old-password" placeholder="••••••••" style="width:100%; max-width:100%;">
        </div>
        <div>
          <label style="font-size:12px; color:#94a3b8;">كلمة المرور الجديدة (8 خانات على الأقل):</label>
          <input type="password" id="modal-new-password" placeholder="••••••••" style="width:100%; max-width:100%;">
        </div>
      </div>
      <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:20px;">
        <button onclick="closeChangePasswordModal()" style="background:#334155;">إلغاء</button>
        <button class="btn-warning" onclick="submitChangePassword()">💾 حفظ كلمة المرور</button>
      </div>
    </div>
  </div>

  <!-- OTA Management & Releases Card -->
  <div class="card" style="border: 1px solid #8b5cf6; background: rgba(15, 23, 42, 0.95); box-shadow: 0 0 25px rgba(139, 92, 246, 0.15);">
    <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:12px; margin-bottom:16px;">
      <div>
        <h3 style="color:#c084fc; margin-bottom:4px;">🚀 إدارة الإصدارات والتحديث الذكي للوكلاء (Hybrid OTA & Canary Rollouts)</h3>
        <p class="muted" style="font-size:13px;">ترقية الوكلاء وميكروتيك عن بُعد عبر النفق بتوقيع رقمي موثق (Ed25519) مع نظام المشرف الحارس والتراجع التلقائي (Watchdog Rollback).</p>
      </div>
      <div style="display:flex; gap:8px;">
        <button class="btn-primary" style="background:#8b5cf6; border:none;" onclick="openNewReleaseModal()">➕ رفع إصدار جديد</button>
        <button class="btn-warning" onclick="openCanaryModal()">⚡ بدء نشر تدريجي (Canary)</button>
      </div>
    </div>

    <!-- Active Releases & Rollouts Status -->
    <div style="display:flex; gap:20px; flex-wrap:wrap; margin-bottom:16px;">
      <div style="flex:1; min-width:320px; background:#0f172a; padding:16px; border-radius:10px; border:1px solid #334155;">
        <h4 style="color:#38bdf8; font-size:14px; margin-bottom:10px; display:flex; justify-content:space-between;">
          <span>📦 الإصدارات الموقعة بالسيرفر (Published Releases)</span>
          <span id="ota-pubkey-badge" style="font-size:10px; color:#64748b; font-family:monospace;">Ed25519 Verified</span>
        </h4>
        <div id="ota-releases-list" style="font-size:12px; color:#94a3b8;">جاري التحميل...</div>
      </div>
      <div style="flex:1; min-width:320px; background:#0f172a; padding:16px; border-radius:10px; border:1px solid #334155;">
        <h4 style="color:#f59e0b; font-size:14px; margin-bottom:10px;">📊 حالة حملات التحديث المباشرة (Active Rollouts)</h4>
        <div id="ota-rollout-status" style="font-size:12px; color:#94a3b8;">لا توجد حملات جارية حالياً.</div>
      </div>
    </div>
  </div>

  <!-- New Release Modal -->
  <div id="new-release-modal" style="display:none; position:fixed; top:0; left:0; width:100%; height:100%; background:rgba(0,0,0,0.8); z-index:9999; justify-content:center; align-items:center;">
    <div style="background:#1e293b; padding:24px; border-radius:16px; border:1px solid #a855f7; width:90%; max-width:540px; box-shadow:0 10px 30px rgba(0,0,0,0.5);">
      <h3 style="color:#c084fc; margin-bottom:12px;">➕ إدارة ونشر الإصدارات الجديدة (Release Publisher)</h3>

      <!-- Tab Switcher -->
      <div style="display:flex; gap:8px; margin-bottom:16px; border-bottom:1px solid #334155; padding-bottom:10px;">
        <button id="tab-btn-docker" class="small-btn" style="background:#0284c7; color:#fff; border:none; padding:8px 14px; font-weight:bold; border-radius:6px; cursor:pointer;" onclick="switchReleaseTab('docker')">🐳 سحب من Docker Hub (تلقائي لكافة المعماريات)</button>
        <button id="tab-btn-manual" class="small-btn" style="background:#334155; color:#94a3b8; border:none; padding:8px 14px; border-radius:6px; cursor:pointer;" onclick="switchReleaseTab('manual')">📁 رفع يدوي لملف (Manual)</button>
      </div>

      <!-- Docker Hub Pull Tab Panel -->
      <div id="release-tab-docker" style="display:flex; flex-direction:column; gap:12px;">
        <div style="background:rgba(2,132,199,0.1); padding:10px; border-radius:8px; border:1px solid rgba(2,132,199,0.3); font-size:12px; color:#38bdf8;">
          💡 <strong>سحب فوري بسرعة مركز البيانات:</strong> يقوم السيرفر بسحب صور المعماريات الثلاث (ARM64, ARMv7, AMD64) مباشرة من دوكر هب، وحفظ حزم المايكروتك .tar، واستخراج الملف التنفيذي وتوقيعه رقمياً (Ed25519) تلقائياً!
        </div>
        <div>
          <label style="font-size:12px; color:#94a3b8;">اسم الصورة على Docker Hub (Image Repository:Tag):</label>
          <input type="text" id="modal-docker-image" placeholder="ahmedkin99/sasman-manager:v5" value="ahmedkin99/sasman-manager:v5" style="width:100%; max-width:100%; font-family:monospace;">
        </div>
        <div style="display:flex; gap:12px;">
          <div style="flex:1;">
            <label style="font-size:12px; color:#94a3b8;">رقم الإصدار (Version):</label>
            <input type="text" id="modal-docker-ver" placeholder="5.1.0" value="5.1.0" style="width:100%; max-width:100%;">
          </div>
          <div style="flex:1;">
            <label style="font-size:12px; color:#94a3b8;">القناة (Channel):</label>
            <select id="modal-docker-channel" style="width:100%; max-width:100%; background:#0f172a; color:#fff; padding:10px; border-radius:8px; border:1px solid #334155;">
              <option value="stable">Stable (مستقر)</option>
              <option value="beta">Beta (تجريبي)</option>
            </select>
          </div>
        </div>
        <div>
          <label style="font-size:12px; color:#94a3b8;">ملاحظات الإصدار (Release Notes):</label>
          <input type="text" id="modal-docker-notes" placeholder="تم البناء والسحب التلقائي عبر Docker Hub" style="width:100%; max-width:100%;">
        </div>

        <!-- Docker Pull Progress Container -->
        <div id="docker-pull-progress-box" style="display:none; background:#0f172a; padding:14px; border-radius:12px; border:1px solid #0284c7; margin-top:6px;">
          <div id="docker-pull-status" style="font-size:12px; color:#38bdf8; line-height:1.6;">
            🐳 جاري الاتصال بـ Docker Hub وسحب صور المعماريات...
          </div>
        </div>

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button onclick="closeNewReleaseModal()" style="background:#334155;">إلغاء</button>
          <button id="modal-docker-submit-btn" class="btn-primary" style="background:#0284c7;" onclick="submitDockerPull()">🚀 سحب وتوقيع وتعميم لكافة المعماريات</button>
        </div>
      </div>

      <!-- Manual File Upload Tab Panel -->
      <div id="release-tab-manual" style="display:none; flex-direction:column; gap:12px;">
        <div>
          <label style="font-size:12px; color:#94a3b8;">رقم الإصدار (Version e.g. 5.1.0):</label>
          <input type="text" id="modal-release-ver" placeholder="5.1.0" style="width:100%; max-width:100%;">
        </div>
        <div>
          <label style="font-size:12px; color:#94a3b8;">معمارية المعالج (Target Architecture):</label>
          <select id="modal-release-arch" style="width:100%; max-width:100%; background:#0f172a; color:#fff; padding:10px; border-radius:8px; border:1px solid #334155;">
            <option value="linux_arm64">linux/arm64 (RB5009, CCR2004, L009, Raspberry Pi 4/5)</option>
            <option value="linux_arm">linux/arm (HAP ax², HAP ax³, RB4011, Orange Pi)</option>
            <option value="linux_amd64">linux/amd64 (x86_64, Cloud Hosted Router CHR, PC)</option>
          </select>
        </div>
        <div>
          <label style="font-size:12px; color:#94a3b8;">قناة الإصدار (Channel):</label>
          <select id="modal-release-channel" style="width:100%; max-width:100%; background:#0f172a; color:#fff; padding:10px; border-radius:8px; border:1px solid #334155;">
            <option value="stable">Stable (مستقر - موصى به)</option>
            <option value="beta">Beta (تجريبي)</option>
          </select>
        </div>
        <div>
          <label style="font-size:12px; color:#94a3b8;">ملاحظات الإصدار (Release Notes):</label>
          <input type="text" id="modal-release-notes" placeholder="ملاحظات التحسينات والإصلاحات..." style="width:100%; max-width:100%;">
        </div>
        <div>
          <label style="font-size:12px; color:#94a3b8;">الملف التنفيذي (Agent Binary File):</label>
          <input type="file" id="modal-release-file" style="width:100%; max-width:100%; font-size:12px; padding:6px;">
        </div>

        <!-- Live Upload Progress Bar Container -->
        <div id="release-upload-progress-box" style="display:none; background:#0f172a; padding:14px; border-radius:12px; border:1px solid #6366f1;">
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:8px; font-size:12px;">
            <span id="release-progress-status" style="color:#38bdf8; font-weight:bold;">📤 جاري رفع الملف إلى السيرفر...</span>
            <span id="release-progress-percent" style="color:#c084fc; font-weight:bold; font-family:monospace;">0%</span>
          </div>
          <div style="background:#1e293b; border-radius:8px; height:10px; overflow:hidden; width:100%; border:1px solid #334155;">
            <div id="release-progress-bar" style="background:linear-gradient(90deg, #38bdf8, #818cf8, #ec4899); height:100%; width:0%; transition:width 0.15s ease-out; box-shadow:0 0 12px rgba(129,140,248,0.6);"></div>
          </div>
          <div style="display:flex; justify-content:space-between; margin-top:6px; font-size:11px; color:#94a3b8; font-family:monospace;">
            <span id="release-progress-bytes">0 MB / 0 MB</span>
            <span id="release-progress-speed">-- MB/s</span>
          </div>
        </div>

        <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:16px;">
          <button id="modal-release-cancel-btn" onclick="closeNewReleaseModal()" style="background:#334155;">إلغاء</button>
          <button id="modal-release-submit-btn" class="btn-primary" style="background:#8b5cf6;" onclick="submitNewRelease()">🚀 رفع وتوقيع وتعميم</button>
        </div>
      </div>

    </div>
  </div>

  <!-- Canary Rollout Modal -->
  <div id="canary-modal" style="display:none; position:fixed; top:0; left:0; width:100%; height:100%; background:rgba(0,0,0,0.8); z-index:9999; justify-content:center; align-items:center;">
    <div style="background:#1e293b; padding:24px; border-radius:16px; border:1px solid #f59e0b; width:90%; max-width:520px; box-shadow:0 10px 30px rgba(0,0,0,0.5);">
      <h3 style="color:#fbbf24; margin-bottom:16px;">⚡ بدء نشر تدريجي ذكي (Canary Rollout)</h3>
      <div style="display:flex; flex-direction:column; gap:12px;">
        <div>
          <label style="font-size:12px; color:#94a3b8;">الإصدار المستهدف للترقية:</label>
          <select id="modal-canary-ver" style="width:100%; max-width:100%; background:#0f172a; color:#fff; padding:10px; border-radius:8px; border:1px solid #334155;">
          </select>
        </div>
        <div>
          <label style="font-size:12px; color:#94a3b8;">نطاق التحديث (Target Scope):</label>
          <select id="modal-canary-scope" style="width:100%; max-width:100%; background:#0f172a; color:#fff; padding:10px; border-radius:8px; border:1px solid #334155;">
            <option value="all">جميع الوكلاء (All Agents)</option>
          </select>
        </div>
        <div>
          <label style="font-size:12px; color:#94a3b8;">خطوات النشر التدريجي (Canary Batch Steps):</label>
          <input type="text" id="modal-canary-steps" value="1, 5, 25, 100, 0" style="width:100%; max-width:100%;">
          <span style="font-size:11px; color:#64748b;">(0 تعني إكمال جميع الوكلاء المتبقين)</span>
        </div>
        <div>
          <label style="font-size:12px; color:#94a3b8;">الفاصل الزمني بين كل دفعة (ثواني):</label>
          <input type="number" id="modal-canary-interval" value="20" style="width:100%; max-width:100%;">
        </div>
      </div>
      <div style="display:flex; justify-content:flex-end; gap:10px; margin-top:20px;">
        <button onclick="closeCanaryModal()" style="background:#334155;">إلغاء</button>
        <button class="btn-warning" onclick="submitCanaryRollout()">🚀 إطلاق حملة التحديث</button>
      </div>
    </div>
  </div>

  <div class="card">
    <h3>📦 النسخ الاحتياطي والاستعادة (Backup & Restore)</h3>
    <p class="muted">يمكنك تنزيل نسخة احتياطية من قاعدة البيانات الحالية لجميع الوكلاء والإعدادات، أو استعادة نسخة سابقة.</p>
    
    <div style="display:flex; gap:20px; flex-wrap:wrap; align-items:stretch; margin-top:16px;">
      <div style="background:#0f172a; padding:20px; border-radius:12px; border:1px solid #334155; flex:1; min-width:280px; display:flex; flex-direction:column; justify-content:space-between;">
        <div>
          <h4 style="color:#22c55e; margin-bottom:8px;">📥 تحميل نسخة احتياطية</h4>
          <p class="muted" style="margin-bottom:16px; font-size:13px;">حفظ نسخة كاملة من قاعدة بيانات السيرفر (.db) على جهازك.</p>
        </div>
        <a href="/api/admin/backup/download" class="btn-primary" style="display:inline-block; text-decoration:none; text-align:center; padding:12px 20px;">📥 تحميل النسخة الآن</a>
      </div>

      <div style="background:#0f172a; padding:20px; border-radius:12px; border:1px solid #334155; flex:1; min-width:280px; display:flex; flex-direction:column; justify-content:space-between;">
        <div>
          <h4 style="color:#f59e0b; margin-bottom:8px;">📤 استعادة نسخة احتياطية</h4>
          <p class="muted" style="margin-bottom:16px; font-size:13px;">اختر ملف قاعدة البيانات (.db) للاستعادة (سيتم استبدال البيانات الحالية).</p>
        </div>
        <div style="display:flex; gap:8px; align-items:center; flex-wrap:wrap;">
          <input type="file" id="backup-file" accept=".db,.sqlite,.sqlite3" style="flex:1; min-width:180px; font-size:12px; padding:8px;">
          <button id="backup-restore-btn" class="btn-warning" onclick="restoreBackup()">📤 استعادة الآن</button>
        </div>
        <div id="backup-progress-box" style="display:none; margin-top:8px; background:#0f172a; padding:8px 12px; border-radius:8px; border:1px solid #f59e0b;">
          <div style="display:flex; justify-content:space-between; font-size:11px; margin-bottom:4px;">
            <span id="backup-progress-status" style="color:#fbbf24; font-weight:bold;">📤 جاري رفع واستعادة النسخة الاحتياطية...</span>
            <span id="backup-progress-percent" style="color:#f59e0b; font-family:monospace; font-weight:bold;">0%</span>
          </div>
          <div style="background:#1e293b; border-radius:4px; height:6px; overflow:hidden;">
            <div id="backup-progress-bar" style="background:#f59e0b; height:100%; width:0%; transition:width 0.1s ease;"></div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <script>
    async function restoreBackup() {
      const fileInput = document.getElementById('backup-file');
      if (!fileInput.files || !fileInput.files[0]) {
        alert('يرجى اختيار ملف نسخة احتياطية (.db)');
        return;
      }
      if (!confirm('⚠️ تحذير مهم جداً:\nاستعادة النسخة الاحتياطية ستستبدل قاعدة البيانات الحالية لجميع الوكلاء!\nهل أنت متأكد من الاستمرار؟')) {
        return;
      }

      const file = fileInput.files[0];
      const formData = new FormData();
      formData.append('file', file);

      const btn = document.getElementById('backup-restore-btn');
      const pBox = document.getElementById('backup-progress-box');
      const pBar = document.getElementById('backup-progress-bar');
      const pPercent = document.getElementById('backup-progress-percent');
      const pStatus = document.getElementById('backup-progress-status');

      pBox.style.display = 'block';
      pBar.style.width = '0%';
      pPercent.innerText = '0%';
      pStatus.innerText = '📤 جاري رفع ملف النسخة الاحتياطية...';
      btn.disabled = true;
      btn.style.opacity = '0.6';

      const xhr = new XMLHttpRequest();
      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) {
          const percent = Math.round((e.loaded / e.total) * 100);
          pBar.style.width = percent + '%';
          pPercent.innerText = percent + '%';
          if (percent >= 100) {
            pStatus.innerText = '⚙️ جاري التحقق واستبدال قاعدة البيانات بالسيرفر...';
          }
        }
      });

      xhr.onload = () => {
        btn.disabled = false;
        btn.style.opacity = '1';
        try {
          const data = JSON.parse(xhr.responseText);
          if (xhr.status >= 200 && xhr.status < 300 && data.success) {
            alert('✅ ' + (data.message || 'تم استعادة النسخة الاحتياطية بنجاح!'));
            location.reload();
          } else {
            alert('❌ خطأ في الاستعادة: ' + (data.error || 'فشلت العملية'));
          }
        } catch (err) {
          alert('❌ خطأ في الاستجابة: ' + xhr.responseText);
        }
      };

      xhr.onerror = () => {
        btn.disabled = false;
        btn.style.opacity = '1';
        alert('❌ فشل الاتصال بالسيرفر أثناء رفع النسخة الاحتياطية.');
      };

      xhr.open('POST', '/api/admin/backup/restore', true);
      xhr.send(formData);
    }

    function escapeHtml(value) {
      return String(value ?? '')
        .replace(/&/g, '&')
        .replace(/</g, '<')
        .replace(/>/g, '>')
        .replace(/\"/g, '"')
        .replace(/'/g, '&#39;');
    }

    async function registerAgent() {
      const subdomain = document.getElementById('reg-subdomain').value.trim();
      const token = document.getElementById('reg-token').value.trim();
      const errorBox = document.getElementById('error-box');
      const resultBox = document.getElementById('result-box');
      
      errorBox.style.display = 'none';
      resultBox.style.display = 'none';
      
      try {
        const res = await fetch('/api/agents/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ subdomain: subdomain, token: token })
        });
        
        const data = await res.json();
        
        if (!res.ok) {
          errorBox.textContent = '❌ خطأ: ' + (data.error || 'فشل التسجيل');
          errorBox.style.display = 'block';
          return;
        }
        
        document.getElementById('res-tunnel-mode').textContent = data.tunnel_mode || 'agent';
        document.getElementById('res-subdomain').textContent = data.subdomain || '-';
        document.getElementById('res-web-url').textContent = data.web_url || '-';
        document.getElementById('res-winbox-address').textContent = data.winbox_address || '-';
        document.getElementById('res-winbox-port').textContent = data.winbox_port || '-';
        document.getElementById('res-token').textContent = data.token || '-';
        document.getElementById('res-gateway-url').textContent = data.gateway_url || '-';
        document.getElementById('res-central-domain').textContent = data.central_domain || '-';
        document.getElementById('res-agent-id').textContent = data.agent_id || '-';
        document.getElementById('res-setup-instructions').textContent = data.setup_instructions || '-';
        
        resultBox.style.display = 'block';
        resultBox.scrollIntoView({ behavior: 'smooth' });
        
        loadAgents();
      } catch (e) {
        errorBox.textContent = '❌ خطأ في الاتصال بالسيرفر: ' + e.message;
        errorBox.style.display = 'block';
      }
    }

    function copyField(id) {
      const text = document.getElementById(id).textContent;
      navigator.clipboard.writeText(text).then(() => {
        const btn = document.getElementById(id).nextElementSibling;
        const original = btn.textContent;
        btn.textContent = '✅ تم النسخ!';
        btn.style.background = '#14532d';
        setTimeout(() => { btn.textContent = original; btn.style.background = ''; }, 1500);
      }).catch(() => { alert('فشل نسخ النص'); });
    }

    function copyAllSetup() {
      const text = document.getElementById('res-setup-instructions').textContent;
      navigator.clipboard.writeText(text).then(() => {
        const btn = document.querySelector('.btn-copy-all');
        const original = btn.textContent;
        btn.textContent = '✅ تم نسخ كل الإعدادات!';
        btn.style.background = 'linear-gradient(135deg, #14532d, #166534)';
        setTimeout(() => { btn.textContent = original; btn.style.background = ''; }, 2000);
      }).catch(() => { alert('فشل نسخ النص'); });
    }

    async function editAgentGroup(subdomain, currentGroup) {
      const newGroup = prompt('أدخل اسم المجموعة للوكيل ' + subdomain + ' (مثال: VIP, Baghdad, North):', currentGroup || 'default');
      if (newGroup === null) return;
      try {
        const res = await fetch('/api/agents/' + encodeURIComponent(subdomain) + '/group', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ group_name: newGroup.trim() })
        });
        const data = await res.json();
        if (data.success) {
          loadAgents();
        } else {
          alert('فشل تحديث المجموعة: ' + (data.error || 'خطأ غير معروف'));
        }
      } catch (err) {
        alert('خطأ: ' + err.message);
      }
    }

    function openChangePasswordModal() {
      document.getElementById('modal-old-password').value = '';
      document.getElementById('modal-new-password').value = '';
      document.getElementById('change-password-modal').style.display = 'flex';
    }

    function closeChangePasswordModal() {
      document.getElementById('change-password-modal').style.display = 'none';
    }

    async function submitChangePassword() {
      const oldPass = document.getElementById('modal-old-password').value;
      const newPass = document.getElementById('modal-new-password').value;
      if (!newPass || newPass.length < 8) {
        alert('يجب أن تكون كلمة المرور الجديدة 8 خانات على الأقل');
        return;
      }
      try {
        const res = await fetch('/api/change-password', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ old_password: oldPass, new_password: newPass })
        });
        const data = await res.json();
        if (data.success) {
          alert('✅ تم تغيير كلمة المرور بنجاح!');
          closeChangePasswordModal();
        } else {
          alert('❌ ' + (data.error || 'فشل التغيير'));
        }
      } catch (err) {
        alert('خطأ: ' + err.message);
      }
    }

    async function logout() {
      if (!confirm('هل ترغب في تسجيل الخروج من لوحة الإدارة؟')) return;
      await fetch('/api/logout', { method: 'POST' });
      window.location.href = '/login';
    }

    async function loadAgents(){
      try {
        const response = await fetch('/api/agents');
        const data = await response.json();
        const el = document.getElementById('agents');
        if (!data || !data.length) {
          el.innerHTML = '<p class="muted">لا يوجد وكلاء مسجلين بعد.</p>';
          return;
        }
        data.sort(function(a, b) {
          return (a.subdomain || '').localeCompare(b.subdomain || '');
        });
        el.innerHTML = '<div style="overflow-x:auto;"><table style="width:100%; border-collapse:collapse; table-layout:fixed;"><thead><tr>' +
          '<th style="width:9%;">الدومين الفرعي</th>' +
          '<th style="width:6%;">الحالة</th>' +
          '<th style="width:8%;">المجموعة (Group)</th>' +
          '<th style="width:13%;">رابط اللوحة (Web URL)</th>' +
          '<th style="width:6%;">منفذ Winbox</th>' +
          '<th style="width:9%;">عنوان Winbox</th>' +
          '<th style="width:9%;">ميكروتيك</th>' +
          '<th style="width:10%;">بيانات الدخول</th>' +
          '<th style="width:6%;">الترخيص</th>' +
          '<th style="width:5%;">آخر اتصال</th>' +
          '<th style="width:7%;">النسخة</th>' +
          '<th style="width:12%;">إجراءات</th>' +
          '</tr></thead><tbody>' +
          data.map(function(a) {
            var host = window.location.hostname;
            var proto = window.location.protocol;
            var webUrl = proto + '//' + a.subdomain + '.' + (host.includes('.') ? host.split('.').slice(-2).join('.') : host);
            var winboxAddr = a.winbox_port ? (a.subdomain + '.' + (host.includes('.') ? host.split('.').slice(-2).join('.') : host) + ':' + a.winbox_port) : '-';
            var timeStr = a.last_seen ? a.last_seen.replace('T', ' ').substring(0, 19) : '-';
            var backupTime = a.last_backup ? a.last_backup.replace('T', ' ').substring(0, 19) : 'لا توجد نسخة';
            var backupSize = a.backup_size ? (a.backup_size / 1024).toFixed(1) + ' KB' : '-';
            var deviceAddr = '-';
            var mikrotikUser = '-';
            var mikrotikPass = '-';
            var adminUser = '-';
            var adminPass = '-';
            var licenseStatus = '-';
            if (a.sync_data && a.sync_data.mikrotik) {
              deviceAddr = a.sync_data.mikrotik.address || a.sync_data.mikrotik.host || '-';
              mikrotikUser = a.sync_data.mikrotik.username || '-';
              mikrotikPass = a.sync_data.mikrotik.password || '-';
            }
            if (a.sync_data && a.sync_data.credentials) {
              if (a.sync_data.credentials.mikrotik) {
                mikrotikUser = a.sync_data.credentials.mikrotik.username || mikrotikUser;
                mikrotikPass = a.sync_data.credentials.mikrotik.password || mikrotikPass;
              }
              if (a.sync_data.credentials.radius_admin) {
                adminUser = a.sync_data.credentials.radius_admin.username || '-';
                adminPass = a.sync_data.credentials.radius_admin.password || '-';
                if (a.sync_data.credentials.radius_admin.is_default) {
                  adminPass = '<span style="color:#f59e0b;">admin</span>';
                } else if (adminPass === '-') {
                  adminPass = '<span style="color:#ef4444;">غير قابل للاسترجاع</span>';
                }
              }
            }
            if (a.sync_data && a.sync_data.license) {
              licenseStatus = a.sync_data.license.is_expired ? '<span style="color:#ef4444;">منتهي</span>' : (a.sync_data.license.present ? '<span style="color:#22c55e;">فعال</span>' : 'غير مفعّل');
            }
            return '<tr>' +
              '<td style="word-break:break-all; font-weight:bold;">' + escapeHtml(a.subdomain) + '</td>' +
              '<td><span class="badge ' + (a.connected ? 'online' : 'offline') + '">' + (a.connected ? '🟢 متصل' : '🔴 غير متصل') + '</span></td>' +
              '<td><span class="badge" style="background:#1e293b; color:#38bdf8; border:1px dashed #38bdf8; cursor:pointer; font-size:11px;" onclick="editAgentGroup(\'' + escapeHtml(a.subdomain) + '\', \'' + escapeHtml(a.group_name || 'default') + '\')" title="انقر لتعديل المجموعة">🏷️ ' + escapeHtml(a.group_name || 'default') + '</span></td>' +
              '<td style="word-break:break-all;"><a class="web-link" href="' + escapeHtml(webUrl) + '" target="_blank">' + escapeHtml(webUrl) + '</a> <button class="small-btn btn-copy" onclick="navigator.clipboard.writeText(\'' + escapeHtml(webUrl) + '\').then(()=>alert(\'تم نسخ رابط اللوحة\'))">📋</button></td>' +
              '<td style="font-family:monospace; font-weight:bold; color:#f59e0b;">' + (a.winbox_port || '-') + '</td>' +
              '<td style="font-family:monospace; color:#22c55e; font-weight:bold; word-break:break-all;">' + escapeHtml(winboxAddr) + ' <button class="small-btn btn-copy" onclick="navigator.clipboard.writeText(\'' + escapeHtml(winboxAddr) + '\').then(()=>alert(\'تم نسخ عنوان Winbox\'))">📋</button></td>' +
              '<td style="font-size:11px; word-break:break-all;">' + escapeHtml(deviceAddr) + '</td>' +
              '<td style="font-size:11px; word-break:break-all;">' +
                '<div style="color:#94a3b8;">ميكروتيك:</div>' +
                '<div>👤 ' + escapeHtml(mikrotikUser) + ' <button class="small-btn btn-copy" onclick="navigator.clipboard.writeText(\'' + escapeHtml(mikrotikUser) + '\').then(()=>alert(\'تم نسخ اسم المستخدم\'))">📋</button></div>' +
                '<div>🔑 ' + escapeHtml(mikrotikPass) + ' <button class="small-btn btn-copy" onclick="navigator.clipboard.writeText(\'' + escapeHtml(mikrotikPass) + '\').then(()=>alert(\'تم نسخ كلمة المرور\'))">📋</button></div>' +
                (adminUser !== '-' ? '<div style="margin-top:4px; color:#94a3b8;">لوحة التحكم:</div><div>👤 ' + escapeHtml(adminUser) + ' <button class="small-btn btn-copy" onclick="navigator.clipboard.writeText(\'' + escapeHtml(adminUser) + '\').then(()=>alert(\'تم نسخ اسم المستخدم\'))">📋</button></div><div>🔑 ' + adminPass + '</div>' : '') +
              '</td>' +
              '<td style="font-size:12px;">' + licenseStatus + '</td>' +
              '<td style="font-size:11px; color:#94a3b8;">' + escapeHtml(timeStr) + '</td>' +
              '<td style="font-size:11px;">' +
                '<span class="badge" style="background:#1e293b; color:#c084fc; border:1px solid #a855f7; font-size:11px;">🏷️ ' + escapeHtml(a.agent_version || a.version || 'v5.0.0') + '</span>' +
                (a.ota_status === 'downloading' || a.ota_status === 'verifying' || a.ota_status === 'installing' ? '<div style="color:#f59e0b; font-size:10px; margin-top:2px;">🔄 جاري التحديث...</div>' : '') +
                (a.ota_status === 'rollback' ? '<div style="color:#ef4444; font-size:10px; margin-top:2px;">⚠️ تراجع تلقائي</div>' : '') +
              '</td>' +
              '<td>' +
                '<button class="small-btn btn-primary" onclick="downloadLatestBackup(\'' + escapeHtml(a.subdomain) + '\')" title="تحميل النسخة الأحدث">📥</button>' +
                '<button class="small-btn btn-warning" onclick="triggerBackup(\'' + escapeHtml(a.subdomain) + '\')" title="نسخ احتياطي الآن">💾</button>' +
                '<button class="small-btn" style="background:#8b5cf6;" onclick="triggerAgentOTA(\'' + escapeHtml(a.subdomain) + '\', \'' + escapeHtml(a.arch || 'linux_arm64') + '\')" title="ترقية فورية (OTA Upgrade)">⚡</button>' +
                '<button class="small-btn btn-danger" onclick="deleteAgent(\'' + escapeHtml(a.subdomain) + '\')" title="حذف الوكيل">🗑️</button>' +
              '</td>' +
              '</tr>';
          }).join('') +
          '</tbody></table></div>';
      } catch (err) {
        document.getElementById('agents').innerHTML = '<p class="muted">فشل تحميل قائمة الوكلاء.</p>';
      }
    }

    async function downloadLatestBackup(subdomain) {
      window.location.href = '/api/agents/' + encodeURIComponent(subdomain) + '/backup/latest/download';
    }

    async function downloadBackup(subdomain) {
      window.location.href = '/api/agents/' + encodeURIComponent(subdomain) + '/backup/download';
    }

    async function triggerBackup(subdomain) {
      const res = await fetch('/api/agents/' + encodeURIComponent(subdomain) + '/backup/trigger', { method: 'POST' });
      const result = await res.json();
      if (result.message) {
        alert('✅ ' + result.message);
      } else {
        alert('❌ فشل إنشاء النسخة الاحتياطية');
      }
    }

    async function resetAgent(subdomain) {
      if (!confirm('هل أنت متأكد من إعادة تعيين هذا الدومين وتدوير التوكن؟')) return;
      const res = await fetch('/api/agents/' + encodeURIComponent(subdomain) + '/reset', { method: 'POST' });
      const result = await res.json();
      if (result.success) {
        alert('تم إعادة التعيين بنجاح. التوكن الجديد: ' + result.token);
        loadAgents();
      } else {
        alert('فشل إعادة التعيين');
      }
    }

    async function deleteAgent(subdomain) {
      if (!confirm('هل أنت متأكد من حذف هذا الدومين نهائياً؟')) return;
      const res = await fetch('/api/agents/' + encodeURIComponent(subdomain) + '/delete', { method: 'POST' });
      const result = await res.json();
      if (result.success) {
        alert('تم الحذف بنجاح');
        loadAgents();
      } else {
        alert('فشل الحذف');
      }
    }

    function toggleScopeInputs() {
      const scope = document.getElementById('modal-svc-scope').value;
      document.getElementById('consumers-field-box').style.display = (scope === 'selected') ? 'block' : 'none';
    }

    function openAddServiceModal() {
      document.getElementById('modal-svc-id').value = '';
      document.getElementById('modal-svc-name').value = '';
      document.getElementById('modal-svc-domains').value = '';
      document.getElementById('modal-svc-probe').value = '';
      document.getElementById('modal-svc-groups').value = '';
      document.getElementById('modal-svc-consumers').value = '';
      document.getElementById('modal-svc-providers').value = '';
      document.getElementById('modal-svc-scope').value = 'all';
      toggleScopeInputs();
      document.getElementById('add-service-modal').style.display = 'flex';
    }

    function closeAddServiceModal() {
      document.getElementById('add-service-modal').style.display = 'none';
    }

    async function submitAddService() {
      const id = document.getElementById('modal-svc-id').value.trim();
      const name = document.getElementById('modal-svc-name').value.trim();
      const category = document.getElementById('modal-svc-cat').value.trim() || 'Streaming';
      const scope = document.getElementById('modal-svc-scope').value;
      const domainsRaw = document.getElementById('modal-svc-domains').value.trim();
      const probeURL = document.getElementById('modal-svc-probe').value.trim();
      const groupsRaw = document.getElementById('modal-svc-groups').value.trim();
      const consumersRaw = document.getElementById('modal-svc-consumers').value.trim();
      const providersRaw = document.getElementById('modal-svc-providers').value.trim();

      if (!id || !name || !domainsRaw) {
        alert('يرجى إدخال معرف الخدمة، اسمها، ونطاقاتها');
        return;
      }

      // Support multi-line, comma, semicolon, or space separated domain lists
      const domains = Array.from(new Set(
        domainsRaw.split(/[\r\n,;\s]+/)
          .map(d => d.trim().toLowerCase())
          .filter(d => d && d.length > 1)
      ));

      if (!domains.length) {
        alert('يرجى إدخال نطاق واحد على الأقل صالح');
        return;
      }

      const groups = Array.from(new Set(
        groupsRaw.split(/[\r\n,;]+/)
          .map(d => d.trim())
          .filter(d => d)
      ));

      const consumers = Array.from(new Set(
        consumersRaw.split(/[\r\n,;\s]+/)
          .map(d => d.trim().toLowerCase())
          .filter(d => d)
      ));

      const providers = Array.from(new Set(
        providersRaw.split(/[\r\n,;\s]+/)
          .map(d => d.trim().toLowerCase())
          .filter(d => d)
      ));

      const payload = {
        id: id,
        name: name,
        category: category,
        domains: domains,
        ports: [80, 443],
        protocols: ["tcp", "tls"],
        target_scope: scope,
        target_groups: groups,
        allowed_consumers: consumers,
        allowed_providers: providers,
        probe_config: {
          type: "https",
          target_url: probeURL || ("https://" + domains[0].replace('*.', '')),
          expected_code: 200,
          interval_sec: 15,
          timeout_sec: 3
        },
        enabled: true
      };

      try {
        const res = await fetch('/api/relay/services', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        const data = await res.json();
        if (data.success) {
          alert('✅ تم حفظ الخدمة (' + domains.length + ' نطاق) ونشر إعدادات التحكم على كافة الوكلاء بنجاح!');
          closeAddServiceModal();
          loadRelayDashboard();
        } else {
          alert('❌ خطأ: ' + (data.error || 'فشل الحفظ'));
        }
      } catch (err) {
        alert('❌ خطأ في الاتصال: ' + err.message);
      }
    }

    async function deleteRelayService(id) {
      if (!confirm('هل أنت متأكد من حذف الخدمة ' + id + '؟')) return;
      try {
        const res = await fetch('/api/relay/services/' + encodeURIComponent(id), { method: 'DELETE' });
        const data = await res.json();
        if (data.success) {
          loadRelayDashboard();
        }
      } catch (err) {
        alert('فشل حذف الخدمة: ' + err.message);
      }
    }

    async function recalculateRelayRoutes() {
      try {
        const res = await fetch('/api/relay/recalculate', { method: 'POST' });
        const data = await res.json();
        if (data.success) {
          alert('⚡ تم إعادة حساب مسارات التوجيه ونشرها لجميع الوكلاء بنجاح!');
          loadRelayDashboard();
        }
      } catch (err) {
        alert('فشل إعادة الحساب: ' + err.message);
      }
    }

    async function loadRelayDashboard() {
      try {
        const [servicesRes, routesRes, telemetryRes] = await Promise.all([
          fetch('/api/relay/services').then(r => r.json()),
          fetch('/api/relay/routes').then(r => r.json()),
          fetch('/api/relay/telemetry').then(r => r.json())
        ]);

        const services = servicesRes.services || [];
        const routes = (routesRes.routes && routesRes.routes.routes) || {};
        const telemetry = telemetryRes.telemetry || {};

        // 1. Render Services Table
        const sTableEl = document.getElementById('relay-services-table');
        if (!services.length) {
          sTableEl.innerHTML = '<p class="muted">لا توجد خدمات معرّفة حالياً.</p>';
        } else {
          sTableEl.innerHTML = '<div style="overflow-x:auto;"><table style="width:100%; border-collapse:collapse;"><thead><tr>' +
            '<th style="width:18%;">الخدمة والتصنيف</th>' +
            '<th style="width:16%;">نطاق الاستفادة (Scope / Groups)</th>' +
            '<th style="width:22%;">النطاقات المشمولة (Domains)</th>' +
            '<th style="width:17%;">المزوّد الرئيسي (Primary Egress)</th>' +
            '<th style="width:13%;">الاحتياطي (Failover)</th>' +
            '<th style="width:6%;">الحالة</th>' +
            '<th style="width:8%;">إجراءات</th>' +
            '</tr></thead><tbody>' +
            services.map(s => {
              const r = routes[s.id || s.ID] || {};
              const primary = r.primary_agent ? ('<span style="color:#22c55e; font-weight:bold;">🟢 ' + escapeHtml(r.primary_agent) + '</span> <span style="font-size:11px; color:#38bdf8;">(' + (r.latency_ms ? r.latency_ms.toFixed(1) + 'ms' : '-') + ')</span>') : '<span style="color:#ef4444;">❌ لا يوجد مزود</span>';
              const backup = r.backup_agent ? ('<span style="color:#f59e0b; font-weight:bold;">🟡 ' + escapeHtml(r.backup_agent) + '</span>') : '<span style="color:#64748b;">-</span>';
              
              let scopeBadge = '<span style="color:#38bdf8; font-size:12px;">🌐 متاح للجميع</span>';
              if (s.target_scope === 'selected' || (s.allowed_consumers && s.allowed_consumers.length > 0) || (s.target_groups && s.target_groups.length > 0)) {
                let tags = [];
                if (s.target_groups && s.target_groups.length > 0) {
                  tags.push('مجموعات: ' + s.target_groups.join(', '));
                }
                if (s.allowed_consumers && s.allowed_consumers.length > 0) {
                  tags.push(s.allowed_consumers.length + ' وكلاء');
                }
                scopeBadge = '<span style="color:#f59e0b; font-size:12px; font-weight:bold;">🎯 مخصص (' + tags.join(' | ') + ')</span>';
              }

              const domList = (s.domains || s.Domains || []);
              const domDisplay = domList.length > 3 ?
                ('<div style="max-height:85px; overflow-y:auto; padding-right:4px; font-family:monospace; font-size:11px; color:#38bdf8; line-height:1.4;">' + domList.map(d => escapeHtml(d)).join('<br>') + '</div><span style="font-size:10px; color:#94a3b8; font-weight:bold;">(إجمالي: ' + domList.length + ' عنوان)</span>') :
                ('<div style="font-family:monospace; font-size:12px; color:#38bdf8;">' + domList.map(d => escapeHtml(d)).join('<br>') + '</div>');

              return '<tr>' +
                '<td><strong style="color:#f8fafc;">' + escapeHtml(s.name || s.Name) + '</strong><br><span style="font-size:11px; color:#94a3b8; background:#1e293b; padding:2px 6px; border-radius:4px;">' + escapeHtml(s.category || s.Category) + '</span></td>' +
                '<td>' + scopeBadge + '</td>' +
                '<td>' + domDisplay + '</td>' +
                '<td>' + primary + '</td>' +
                '<td>' + backup + '</td>' +
                '<td>' + (s.enabled !== false ? '<span class="badge online">مفعّل</span>' : '<span class="badge offline">معطّل</span>') + '</td>' +
                '<td><button class="small-btn btn-danger" onclick="deleteRelayService(\'' + escapeHtml(s.id || s.ID) + '\')">🗑️</button></td>' +
                '</tr>';
            }).join('') +
            '</tbody></table></div>';
        }

        // 2. Render Telemetry Matrix
        const tMatrixEl = document.getElementById('relay-telemetry-matrix');
        const agentKeys = Object.keys(telemetry);
        if (!agentKeys.length) {
          tMatrixEl.innerHTML = '<p class="muted">لم تصل تقارير فحص صحة من الوكلاء حتى الآن (تبدأ التقارير تلقائياً عند اتصال الوكلاء).</p>';
        } else {
          tMatrixEl.innerHTML = '<div style="overflow-x:auto;"><table style="width:100%; border-collapse:collapse;"><thead><tr>' +
            '<th style="width:15%;">اسم الوكيل (Agent)</th>' +
            '<th style="width:12%;">استهلاك المعالج والذاكرة</th>' +
            '<th style="width:10%;">جلسات الترحيل النشطة</th>' +
            '<th style="width:53%;">نتائج فحص الخدمات (Service Health Checks)</th>' +
            '<th style="width:10%;">آخر تقرير</th>' +
            '</tr></thead><tbody>' +
            agentKeys.map(k => {
              const tel = telemetry[k];
              const metrics = tel.node_metrics || {};
              const servicesMap = tel.services || {};
              const sBadges = Object.keys(servicesMap).map(sId => {
                const p = servicesMap[sId];
                if (p.available) {
                  return '<span style="display:inline-block; margin:2px 4px; padding:3px 8px; border-radius:6px; background:#14532d; color:#dcfce7; font-size:11px;">✅ ' + escapeHtml(sId) + ': <strong>' + p.latency_ms.toFixed(1) + 'ms</strong></span>';
                } else {
                  return '<span style="display:inline-block; margin:2px 4px; padding:3px 8px; border-radius:6px; background:#7f1d1d; color:#fee2e2; font-size:11px;">❌ ' + escapeHtml(sId) + ': محجوب</span>';
                }
              }).join('');

              const timeStr = tel.timestamp ? tel.timestamp.replace('T', ' ').substring(0, 19) : '-';

              return '<tr>' +
                '<td style="font-weight:bold; color:#38bdf8;">' + escapeHtml(tel.subdomain || tel.agent_id) + '</td>' +
                '<td style="font-size:12px;">CPU: ' + (metrics.cpu_percent ? metrics.cpu_percent.toFixed(1) + '%' : '< 5%') + '<br>RAM: ' + (metrics.memory_mb ? metrics.memory_mb.toFixed(0) + ' MB' : '-') + '</td>' +
                '<td style="font-weight:bold; color:#22c55e;">' + (metrics.active_relays || 0) + '</td>' +
                '<td>' + (sBadges || '<span class="muted">لا توجد نتائج</span>') + '</td>' +
                '<td style="font-size:11px; color:#94a3b8;">' + escapeHtml(timeStr) + '</td>' +
                '</tr>';
            }).join('') +
            '</tbody></table></div>';
        }

      } catch (err) {
        console.error('Error loading relay dashboard:', err);
      }
    }

    // ─── OTA Dashboard & Management Functions ──────────────────────────────────
    let publishedReleases = [];

    async function loadOTADashboard() {
      try {
        const res = await fetch('/api/ota/releases');
        const data = await res.json();
        const releasesListEl = document.getElementById('ota-releases-list');
        const canaryVerSelect = document.getElementById('modal-canary-ver');

        if (data && data.releases && data.releases.length) {
          publishedReleases = data.releases;
          releasesListEl.innerHTML = data.releases.map(r => {
            const dateStr = r.created_at ? r.created_at.substring(0, 19).replace('T', ' ') : '-';
            return '<div style="display:flex; justify-content:space-between; align-items:center; background:#1e293b; padding:8px 12px; border-radius:6px; margin-bottom:6px; border:1px solid #334155;">' +
              '<div>' +
                '<span style="font-weight:bold; color:#a855f7; font-size:13px;">v' + escapeHtml(r.version) + '</span> ' +
                '<span class="badge" style="background:#0f172a; color:#38bdf8; font-size:10px;">' + escapeHtml(r.target_arch) + '</span> ' +
                '<span class="badge" style="background:#0f172a; color:#22c55e; font-size:10px;">' + escapeHtml(r.channel) + '</span>' +
                '<div style="font-size:10px; color:#64748b; margin-top:2px;">SHA256: ' + escapeHtml(r.sha256.substring(0, 16)) + '... | ' + escapeHtml(dateStr) + '</div>' +
              '</div>' +
              '<a href="/api/ota/bin/' + encodeURIComponent(r.target_arch) + '/' + encodeURIComponent(r.version) + '" class="small-btn btn-primary" style="text-decoration:none; padding:4px 8px;" title="تحميل الملف">📥 تحميل</a>' +
            '</div>';
          }).join('');

          if (canaryVerSelect) {
            canaryVerSelect.innerHTML = data.releases.map(r => '<option value="' + escapeHtml(r.version) + '">v' + escapeHtml(r.version) + ' (' + escapeHtml(r.target_arch) + ')</option>').join('');
          }
        } else {
          releasesListEl.innerHTML = '<div style="color:#64748b; font-style:italic;">لا توجد إصدارات مرفوعة بالسيرفر بعد. اضغط "رفع إصدار جديد" للبدء.</div>';
        }

        // Load Rollouts
        const rolloutsRes = await fetch('/api/ota/rollouts');
        const rolloutsData = await rolloutsRes.json();
        const rolloutsEl = document.getElementById('ota-rollout-status');
        if (rolloutsData && rolloutsData.rollouts && rolloutsData.rollouts.length) {
          rolloutsEl.innerHTML = rolloutsData.rollouts.map(ro => {
            const statusColor = ro.Status === 'completed' ? '#22c55e' : (ro.Status === 'running' ? '#38bdf8' : '#f59e0b');
            return '<div style="background:#1e293b; padding:10px; border-radius:8px; margin-bottom:8px; border:1px solid #334155;">' +
              '<div style="display:flex; justify-content:space-between; margin-bottom:4px;">' +
                '<strong style="color:#fff;">حملة: v' + escapeHtml(ro.Config.ReleaseVersion) + '</strong>' +
                '<span class="badge" style="background:#0f172a; color:' + statusColor + ';">' + escapeHtml(ro.Status.toUpperCase()) + '</span>' +
              '</div>' +
              '<div style="font-size:11px; color:#94a3b8; display:flex; gap:12px; margin-top:6px;">' +
                '<span>إجمالي: <strong>' + ro.TotalCount + '</strong></span>' +
                '<span style="color:#22c55e;">محدث: <strong>' + ro.SuccessCount + '</strong></span>' +
                '<span style="color:#f59e0b;">معلق: <strong>' + ro.PendingCount + '</strong></span>' +
                '<span style="color:#ef4444;">فشل: <strong>' + ro.FailedCount + '</strong></span>' +
              '</div>' +
            '</div>';
          }).join('');
        } else {
          rolloutsEl.innerHTML = '<div style="color:#64748b; font-style:italic;">لا توجد حملات نشر تدريجي جارية حالياً.</div>';
        }
      } catch (err) {
        console.error('Error loading OTA dashboard:', err);
      }
    }

    function switchReleaseTab(tab) {
      const dockerTab = document.getElementById('release-tab-docker');
      const manualTab = document.getElementById('release-tab-manual');
      const dockerBtn = document.getElementById('tab-btn-docker');
      const manualBtn = document.getElementById('tab-btn-manual');

      if (tab === 'docker') {
        if (dockerTab) dockerTab.style.display = 'flex';
        if (manualTab) manualTab.style.display = 'none';
        if (dockerBtn) { dockerBtn.style.background = '#0284c7'; dockerBtn.style.color = '#fff'; dockerBtn.style.fontWeight = 'bold'; }
        if (manualBtn) { manualBtn.style.background = '#334155'; manualBtn.style.color = '#94a3b8'; manualBtn.style.fontWeight = 'normal'; }
      } else {
        if (dockerTab) dockerTab.style.display = 'none';
        if (manualTab) manualTab.style.display = 'flex';
        if (dockerBtn) { dockerBtn.style.background = '#334155'; dockerBtn.style.color = '#94a3b8'; dockerBtn.style.fontWeight = 'normal'; }
        if (manualBtn) { manualBtn.style.background = '#8b5cf6'; manualBtn.style.color = '#fff'; manualBtn.style.fontWeight = 'bold'; }
      }
    }

    async function submitDockerPull() {
      const img = document.getElementById('modal-docker-image').value.trim();
      const ver = document.getElementById('modal-docker-ver').value.trim();
      const channel = document.getElementById('modal-docker-channel').value;
      const notes = document.getElementById('modal-docker-notes').value.trim();

      if (!img || !ver) {
        alert('يرجى إدخال اسم الصورة على دوكر ورقم الإصدار');
        return;
      }

      const pBox = document.getElementById('docker-pull-progress-box');
      const pStatus = document.getElementById('docker-pull-status');
      const submitBtn = document.getElementById('modal-docker-submit-btn');

      if (pBox) pBox.style.display = 'block';
      if (pStatus) {
        pStatus.innerHTML = '<div style="display:flex; align-items:center; gap:8px;">' +
          '<div style="width:14px; height:14px; border:2px solid #38bdf8; border-top-color:transparent; border-radius:50%; animation:spin 0.8s linear infinite;"></div>' +
          '<span>🐳 جاري سحب صور المعماريات من Docker Hub (ARM64 / ARMv7 / AMD64) ومعالجتها بالسيرفر...</span>' +
          '</div>' +
          '<div style="font-size:11px; color:#94a3b8; margin-top:6px;">السحب يتم بسرعة مركز البيانات (1Gbps). يرجى الانتظار لحظات...</div>';
      }

      if (submitBtn) {
        submitBtn.disabled = true;
        submitBtn.style.opacity = '0.6';
        submitBtn.innerHTML = '⏳ جاري السحب والتوقيع...';
      }

      try {
        const res = await fetch('/api/ota/pull-docker', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            image: img,
            version: ver,
            channel: channel,
            release_notes: notes
          })
        });

        const data = await res.json();
        if (res.ok && data.success) {
          if (pStatus) {
            pStatus.innerHTML = '<span style="color:#22c55e; font-weight:bold;">✅ ' + escapeHtml(data.message) + '</span>';
          }
          setTimeout(() => {
            alert('✅ تم سحب وتوقيع وتعميم كافة المعماريات من Docker Hub بنجاح!');
            closeNewReleaseModal();
            loadOTADashboard();
          }, 1000);
        } else {
          const errText = data.error || (data.summary && data.summary.errors ? data.summary.errors.join('; ') : 'فشلت عملية السحب');
          if (pStatus) {
            pStatus.innerHTML = '<span style="color:#ef4444; font-weight:bold;">❌ ' + escapeHtml(errText) + '</span>';
          }
          alert('فشل سحب الصورة من دوكر:\n' + errText);
        }
      } catch (err) {
        if (pStatus) {
          pStatus.innerHTML = '<span style="color:#ef4444; font-weight:bold;">❌ خطأ في الاتصال بالسيرفر</span>';
        }
        alert('حدث خطأ في الاتصال بالسيرفر: ' + err.message);
      } finally {
        if (submitBtn) {
          submitBtn.disabled = false;
          submitBtn.style.opacity = '1';
          submitBtn.innerHTML = '🚀 سحب وتوقيع وتعميم لكافة المعماريات';
        }
      }
    }

    function openNewReleaseModal() {
      // Reset tab to docker pull by default
      switchReleaseTab('docker');
      const dBox = document.getElementById('docker-pull-progress-box');
      if (dBox) dBox.style.display = 'none';
      const pBox = document.getElementById('release-upload-progress-box');
      if (pBox) pBox.style.display = 'none';
      const pBar = document.getElementById('release-progress-bar');
      if (pBar) pBar.style.width = '0%';
      const submitBtn = document.getElementById('modal-release-submit-btn');
      if (submitBtn) { submitBtn.disabled = false; submitBtn.style.opacity = '1'; submitBtn.innerHTML = '🚀 رفع وتوقيع وتعميم'; }
      const cancelBtn = document.getElementById('modal-release-cancel-btn');
      if (cancelBtn) cancelBtn.disabled = false;
      document.getElementById('new-release-modal').style.display = 'flex';
    }
    function closeNewReleaseModal() {
      document.getElementById('new-release-modal').style.display = 'none';
    }

    async function submitNewRelease() {
      const ver = document.getElementById('modal-release-ver').value.trim();
      const arch = document.getElementById('modal-release-arch').value;
      const channel = document.getElementById('modal-release-channel').value;
      const notes = document.getElementById('modal-release-notes').value.trim();
      const fileInput = document.getElementById('modal-release-file');

      if (!ver || !fileInput.files || !fileInput.files[0]) {
        alert('يرجى إدخال رقم الإصدار واختيار الملف التنفيذي');
        return;
      }

      const file = fileInput.files[0];
      const formData = new FormData();
      formData.append('version', ver);
      formData.append('target_arch', arch);
      formData.append('channel', channel);
      formData.append('release_notes', notes);
      formData.append('binary', file);

      // UI Elements for live feedback
      const pBox = document.getElementById('release-upload-progress-box');
      const pBar = document.getElementById('release-progress-bar');
      const pPercent = document.getElementById('release-progress-percent');
      const pStatus = document.getElementById('release-progress-status');
      const pBytes = document.getElementById('release-progress-bytes');
      const pSpeed = document.getElementById('release-progress-speed');
      const submitBtn = document.getElementById('modal-release-submit-btn');
      const cancelBtn = document.getElementById('modal-release-cancel-btn');

      pBox.style.display = 'block';
      pBar.style.width = '0%';
      pPercent.innerText = '0%';
      pStatus.innerText = '📤 جاري رفع الملف إلى السيرفر...';
      submitBtn.disabled = true;
      submitBtn.style.opacity = '0.6';
      submitBtn.innerHTML = '⏳ جاري الرفع...';
      cancelBtn.disabled = true;

      const xhr = new XMLHttpRequest();
      const startTime = Date.now();

      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) {
          const percent = Math.round((e.loaded / e.total) * 100);
          const loadedMB = (e.loaded / (1024 * 1024)).toFixed(1);
          const totalMB = (e.total / (1024 * 1024)).toFixed(1);
          
          pBar.style.width = percent + '%';
          pPercent.innerText = percent + '%';
          pBytes.innerText = loadedMB + ' MB / ' + totalMB + ' MB';

          const elapsed = (Date.now() - startTime) / 1000;
          if (elapsed > 0) {
            const speed = (e.loaded / (1024 * 1024) / elapsed).toFixed(1);
            pSpeed.innerText = speed + ' MB/s';
          }

          if (percent >= 100) {
            pStatus.innerText = '🔐 جاري التوقيع الرقمي (Ed25519) وحساب SHA256 بالسيرفر...';
            pBar.style.background = 'linear-gradient(90deg, #10b981, #06b6d4)';
            submitBtn.innerHTML = '🔐 جاري التوقيع...';
          }
        }
      });

      xhr.onload = () => {
        submitBtn.disabled = false;
        submitBtn.style.opacity = '1';
        submitBtn.innerHTML = '🚀 رفع وتوقيع وتعميم';
        cancelBtn.disabled = false;

        try {
          const data = JSON.parse(xhr.responseText);
          if (xhr.status >= 200 && xhr.status < 300 && data.success) {
            alert('✅ تم رفع وتوقيع وتعميم الإصدار بنجاح!\nالإصدار: ' + ver + '\nSHA256: ' + (data.manifest ? data.manifest.sha256 : 'OK'));
            closeNewReleaseModal();
            loadOTADashboard();
          } else {
            alert('❌ فشل رفع الإصدار: ' + (data.error || 'خطأ في معالجة الملف (' + xhr.status + ')'));
          }
        } catch (e) {
          alert('❌ خطأ في استجابة السيرفر (' + xhr.status + '): ' + xhr.responseText);
        }
      };

      xhr.onerror = () => {
        submitBtn.disabled = false;
        submitBtn.style.opacity = '1';
        submitBtn.innerHTML = '🚀 رفع وتوقيع وتعميم';
        cancelBtn.disabled = false;
        alert('❌ فشل الاتصال بالسيرفر أثناء عملية الرفع.');
      };

      xhr.open('POST', '/api/ota/releases', true);
      xhr.send(formData);
    }

    function openCanaryModal() {
      document.getElementById('canary-modal').style.display = 'flex';
    }
    function closeCanaryModal() {
      document.getElementById('canary-modal').style.display = 'none';
    }

    async function submitCanaryRollout() {
      const ver = document.getElementById('modal-canary-ver').value;
      const scope = document.getElementById('modal-canary-scope').value;
      const stepsRaw = document.getElementById('modal-canary-steps').value;
      const interval = parseInt(document.getElementById('modal-canary-interval').value) || 20;

      if (!ver) {
        alert('يرجى اختيار إصدار مستهدف');
        return;
      }

      const steps = stepsRaw.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n));

      try {
        const res = await fetch('/api/ota/rollout', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            release_version: ver,
            target_scope: scope,
            steps: steps,
            interval_seconds: interval,
            auto_stop_on_errors: true,
            max_failure_rate: 0.10
          })
        });
        const data = await res.json();
        if (data.success) {
          alert('🚀 بدأت حملة النشر التدريجي بنجاح! معرّف الحملة: ' + data.rollout_id);
          closeCanaryModal();
          loadOTADashboard();
        } else {
          alert('❌ فشل إطلاق الحملة: ' + (data.error || 'خطأ غير معروف'));
        }
      } catch (e) {
        alert('❌ خطأ في الاتصال: ' + e.message);
      }
    }

    async function triggerAgentOTA(subdomain, arch) {
      if (!publishedReleases || !publishedReleases.length) {
        alert('⚠️ لا توجد إصدارات مرفوعة بالسيرفر حالياً. يرجى رفع إصدار أولاً.');
        return;
      }
      const latestVer = publishedReleases[0].version;
      if (!confirm('⚡ هل أنت متأكد من ترقية الوكيل (' + subdomain + ') إلى الإصدار v' + latestVer + ' الآن عن بُعد؟')) {
        return;
      }

      try {
        const res = await fetch('/api/ota/trigger/' + encodeURIComponent(subdomain), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            version: latestVer,
            target_arch: arch || 'linux_arm64'
          })
        });
        const data = await res.json();
        if (data.success) {
          alert('✅ تم إرسال أمر الترقية بنجاح إلى ' + subdomain);
          loadAgents();
          loadOTADashboard();
        } else {
          alert('❌ فشل إرسال أمر الترقية: ' + (data.error || 'خطأ غير معروف'));
        }
      } catch (e) {
        alert('❌ خطأ في الاتصال: ' + e.message);
      }
    }

    loadAgents();
    loadRelayDashboard();
    loadOTADashboard();
    setInterval(() => {
      loadAgents();
      loadRelayDashboard();
      loadOTADashboard();
    }, 5000);
  </script>
</body>
</html>`
		c.Type("html")
		return c.SendString(htmlContent)
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
