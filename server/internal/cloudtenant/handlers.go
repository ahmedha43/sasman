package cloudtenant

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"mikrotik-manager/pkg/tunnel"
)

type APIHandler struct {
	mgr *Manager
}

func NewAPIHandler(mgr *Manager) *APIHandler {
	return &APIHandler{mgr: mgr}
}

func (h *APIHandler) RegisterRoutes(app fiber.Router) {
	cloud := app.Group("/cloud")

	// Public API
	cloud.Get("/api/check-subdomain", h.handleCheckSubdomain)
	cloud.Post("/api/check-subdomain", h.handleCheckSubdomain)
	cloud.Post("/api/register", h.handleRegister)
	cloud.Post("/api/login", h.handleLogin)
	cloud.Get("/api/script/:subdomain", h.handleGetInstallScript)

	// Protected Tenant APIs
	tenant := cloud.Group("/api/tenant", h.TenantAuthMiddleware())
	tenant.Get("/stats", h.handleGetStats)
	tenant.Get("/users", h.handleListUsers)
	tenant.Post("/users", h.handleCreateUser)
	tenant.Delete("/users/:username", h.handleDeleteUser)
	tenant.Get("/profiles", h.handleListProfiles)
	tenant.Get("/vouchers", h.handleListVouchers)
	tenant.Post("/vouchers/generate", h.handleGenerateVouchers)
	tenant.Get("/sessions", h.handleListActiveSessions)
	tenant.Post("/sessions/disconnect", h.handleDisconnectSession)

	// =========================================================================
	// Compatibility routes for web_radius when running in multi-tenant cloud mode
	// =========================================================================
	radiusAPI := app.Group("/radius/api")

	// License & Router Status
	radiusAPI.Get("/license/status", h.handleCloudLicenseStatus)
	radiusAPI.Post("/license/activate", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "message": "License managed by SASMAN Cloud", "valid": true})
	})
	radiusAPI.Post("/router/connect", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Connected via RadSec TLS", "router_connected": true})
	})
	radiusAPI.Get("/setup/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"installed": true, "cloud_mode": true})
	})

	// Auth APIs
	radiusAPI.Post("/auth/login", h.handleCloudAuthLogin)
	app.Post("/radius/login", h.handleCloudAuthLogin)
	radiusAPI.Get("/auth/me", h.handleCloudAuthMe)
	radiusAPI.Post("/auth/logout", h.handleCloudAuthLogout)

	// Protected Data APIs for web_radius
	protectedRadius := radiusAPI.Group("", h.TenantAuthMiddleware())
	protectedRadius.Get("/users", h.handleListUsers)
	protectedRadius.Post("/users", h.handleCreateUser)
	protectedRadius.Delete("/users/:username", h.handleDeleteUser)
	protectedRadius.Get("/profiles", h.handleListProfiles)
	protectedRadius.Get("/vouchers", h.handleListVouchers)
	protectedRadius.Post("/vouchers/generate", h.handleGenerateVouchers)
	protectedRadius.Get("/sessions", h.handleListActiveSessions)
	protectedRadius.Post("/sessions/disconnect", h.handleDisconnectSession)
}

func (h *APIHandler) handleCheckSubdomain(c *fiber.Ctx) error {
	sub := c.Query("sub")
	if sub == "" {
		var req struct {
			Subdomain string `json:"subdomain"`
		}
		_ = c.BodyParser(&req)
		sub = req.Subdomain
	}

	available, reason := h.mgr.IsSubdomainAvailable(sub)
	return c.JSON(fiber.Map{
		"subdomain": strings.ToLower(strings.TrimSpace(sub)),
		"available": available,
		"message":   reason,
	})
}

func (h *APIHandler) handleRegister(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "بيانات غير صالحة: " + err.Error(),
		})
	}

	tenant, err := h.mgr.RegisterTenant(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Authenticate and issue token
	_, token, _ := h.mgr.AuthenticateTenant(tenant.Subdomain, req.Password)

	c.Cookie(&fiber.Cookie{
		Name:     "sasman_cloud_token",
		Value:    token,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: true,
		SameSite: "Lax",
	})

	installScriptURL := fmt.Sprintf("https://%s/pki/install/%s.rsc", h.mgr.domain, tenant.Subdomain)
	radsecAddress := fmt.Sprintf("%s:2083", h.mgr.domain)

	return c.JSON(LoginResponse{
		Success:          true,
		Token:            token,
		Tenant:           tenant,
		InstallScriptURL: installScriptURL,
		RadSecAddress:    radsecAddress,
	})
}

func (h *APIHandler) handleLogin(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "بيانات غير صالحة",
		})
	}

	tenant, token, err := h.mgr.AuthenticateTenant(req.Subdomain, req.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "sasman_cloud_token",
		Value:    token,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: true,
		SameSite: "Lax",
	})

	installScriptURL := fmt.Sprintf("https://%s/pki/install/%s.rsc", h.mgr.domain, tenant.Subdomain)
	radsecAddress := fmt.Sprintf("%s:2083", h.mgr.domain)

	return c.JSON(LoginResponse{
		Success:          true,
		Token:            token,
		Tenant:           tenant,
		InstallScriptURL: installScriptURL,
		RadSecAddress:    radsecAddress,
	})
}

func (h *APIHandler) handleGetInstallScript(c *fiber.Ctx) error {
	sub := strings.ToLower(strings.TrimSpace(c.Params("subdomain")))
	if sub == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid subdomain")
	}

	script := fmt.Sprintf(`# ==============================================================================
# SASMAN Cloud Edition - MikroTik 1-Click Provisioning Script
# Subdomain: %[1]s
# Central Host: %[2]s
# ==============================================================================

:put "[*] Downloading SASMAN mTLS Security Certificates for %[1]s..."
/tool fetch url="https://%[2]s/pki/cert/%[1]s/ca.crt" dst-path="sasman-ca.crt" mode=https
/tool fetch url="https://%[2]s/pki/cert/%[1]s/agent.crt" dst-path="sasman-agent.crt" mode=https
/tool fetch url="https://%[2]s/pki/cert/%[1]s/agent.key" dst-path="sasman-agent.key" mode=https

:delay 2s
:put "[*] Importing Certificates into RouterOS Security Store..."
/certificate import file-name="sasman-ca.crt" passphrase=""
/certificate import file-name="sasman-agent.crt" passphrase=""
/certificate import file-name="sasman-agent.key" passphrase=""

:delay 1s
:put "[*] Configuring High-Speed RadSec RFC 6614 Client..."
/radius remove [find comment="SASMAN_CLOUD"]

/radius add address=%[2]s protocol=radsec authentication-port=2083 accounting-port=2083 \
    service=hotspot,ppp,login,wireless \
    certificate="agent-%[1]s-SASMAN" \
    secret="radsec" \
    timeout=3000ms \
    comment="SASMAN_CLOUD"

:put "[*] Configuring AAA Accounting Updates..."
/radius incoming set accept=yes port=3799
/ppp aaa set use-radius=yes interim-update=2m
/ip hotspot profile set [find] use-radius=yes radius-interim-update=2m

:put "[SUCCESS] ✅ SASMAN Cloud Edition is now actively connected to %[2]s!"
`, sub, h.mgr.domain)

	c.Set("Content-Type", "text/plain; charset=utf-8")
	return c.SendString(script)
}

func (h *APIHandler) TenantAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenString := c.Get("Authorization")
		if strings.HasPrefix(tokenString, "Bearer ") {
			tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		} else {
			tokenString = c.Cookies("sasman_cloud_token")
			if tokenString == "" {
				tokenString = c.Cookies("sasman_admin_session")
			}
		}

		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success":       false,
				"error":         "غير مصرح - الرجاء تسجيل الدخول",
				"auth_required": true,
			})
		}

		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return h.mgr.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success":       false,
				"error":         "جلسة غير صالحة أو منتهية",
				"auth_required": true,
			})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "بيانات الجلسة غير صالحة",
			})
		}

		subdomain, _ := claims["subdomain"].(string)
		if subdomain == "" {
			if sub, ok := c.Locals("subdomain").(string); ok && sub != "" {
				subdomain = sub
			} else {
				subdomain = tunnel.ExtractSubdomainForHost(c.Get("Host"), h.mgr.domain)
			}
		}

		if subdomain == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "نطاق المستأجر مفقود",
			})
		}

		db, err := h.mgr.pool.Get(subdomain)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "فشل الاتصال بقاعدة بيانات المستأجر",
			})
		}

		c.Locals("subdomain", subdomain)
		c.Locals("tenant_db", db)
		return c.Next()
	}
}

func (h *APIHandler) handleCloudLicenseStatus(c *fiber.Ctx) error {
	subdomain := ""
	if sub, ok := c.Locals("subdomain").(string); ok && sub != "" {
		subdomain = sub
	} else {
		subdomain = tunnel.ExtractSubdomainForHost(c.Get("Host"), h.mgr.domain)
	}

	valid := false
	status := "unlicensed"
	msg := "الحساب السحابي غير مرخص، يرجى التواصل مع الإدارة لتفعيل الاشتراك"
	expStr := "Unlicensed"
	daysRemaining := 0

	if h.mgr.repo != nil && subdomain != "" {
		lic, err := h.mgr.repo.GetAgentLicenseInfo(subdomain)
		if err == nil && lic != nil {
			status = lic.Status
			expStr = lic.ExpiresAtStr
			daysRemaining = lic.DaysRemaining
			if lic.Status == "active" && !lic.IsExpired {
				valid = true
				msg = "SASMAN Cloud Edition (RadSec RFC 6614)"
			} else if lic.Status == "suspended" {
				msg = "تم تجميد حساب الوكيل مؤقتاً"
			} else if lic.IsExpired {
				msg = "انتهت فترة اشتراك الوكيل، يرجى التجديد"
			}
		}
	}

	return c.JSON(fiber.Map{
		"valid":            valid,
		"router_connected": true,
		"cloud_mode":       true,
		"subdomain":        subdomain,
		"message":          msg,
		"serial":           "CLOUD-" + subdomain,
		"expires":          expStr,
		"status":           status,
		"days_remaining":   daysRemaining,
	})
}

func (h *APIHandler) handleCloudAuthMe(c *fiber.Ctx) error {
	subdomain := ""
	if sub, ok := c.Locals("subdomain").(string); ok && sub != "" {
		subdomain = sub
	} else {
		subdomain = tunnel.ExtractSubdomainForHost(c.Get("Host"), h.mgr.domain)
	}

	return c.JSON(fiber.Map{
		"username":  "admin",
		"role":      "superadmin",
		"subdomain": subdomain,
		"name":      "Admin (" + subdomain + ")",
	})
}

func (h *APIHandler) handleCloudAuthLogout(c *fiber.Ctx) error {
	c.ClearCookie("sasman_admin_session", "sasman_cloud_token")
	return c.JSON(fiber.Map{"success": true})
}

func (h *APIHandler) handleCloudAuthLogin(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "بيانات الدخول غير صالحة",
		})
	}

	subdomain := ""
	if sub, ok := c.Locals("subdomain").(string); ok && sub != "" {
		subdomain = sub
	} else {
		subdomain = tunnel.ExtractSubdomainForHost(c.Get("Host"), h.mgr.domain)
	}

	if subdomain == "" {
		subdomain = c.Query("sub")
	}

	if subdomain == "" && strings.Contains(req.Username, "@") {
		parts := strings.Split(req.Username, "@")
		req.Username = parts[0]
		subdomain = parts[1]
	}

	if subdomain == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "تعذر تحديد النطاق الفرعي",
		})
	}

	db, err := h.mgr.pool.Get(subdomain)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "المستأجر غير موجود",
		})
	}

	var hash, role string
	err = db.QueryRow("SELECT password, role FROM radius_admins WHERE username = ?", req.Username).Scan(&hash, &role)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "اسم المستخدم أو كلمة المرور غير صحيحة",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "كلمة المرور غير صحيحة",
		})
	}

	claims := jwt.MapClaims{
		"subdomain": subdomain,
		"username":  req.Username,
		"role":      role,
		"exp":       time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(h.mgr.jwtSecret)

	c.Cookie(&fiber.Cookie{
		Name:     "sasman_admin_session",
		Value:    tokenString,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: false,
		SameSite: "Lax",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "sasman_cloud_token",
		Value:    tokenString,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: true,
		SameSite: "Lax",
	})

	return c.JSON(fiber.Map{
		"success":  true,
		"token":    tokenString,
		"username": req.Username,
		"role":     role,
	})
}

func (h *APIHandler) handleGetStats(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	subdomain := c.Locals("subdomain").(string)

	var userCount, onlineCount, voucherCount, profileCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM radcheck WHERE attribute = 'Cleartext-Password'").Scan(&userCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM radacct WHERE acctstoptime IS NULL").Scan(&onlineCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM radius_vouchers WHERE is_used = 0").Scan(&voucherCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM radius_profile_meta").Scan(&profileCount)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"subdomain":      subdomain,
			"users":          userCount,
			"online":         onlineCount,
			"vouchers":       voucherCount,
			"profiles":       profileCount,
			"install_script": fmt.Sprintf("https://%s/pki/install/%s.rsc", h.mgr.domain, subdomain),
			"radsec_server":  fmt.Sprintf("%s:2083", h.mgr.domain),
		},
	})
}

func (h *APIHandler) handleListUsers(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)

	rows, err := db.Query(`
		SELECT rc.username, rc.value, COALESCE(rum.full_name, ''), COALESCE(rum.phone, ''), 
		       COALESCE(rum.expiration_unix, 0), COALESCE(rum.enabled, 1),
		       COALESCE((SELECT groupname FROM radusergroup WHERE username = rc.username LIMIT 1), 'default')
		FROM radcheck rc
		LEFT JOIN radius_user_meta rum ON rc.username = rum.username
		WHERE rc.attribute = 'Cleartext-Password'
		ORDER BY rc.id DESC
		LIMIT 200
	`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	defer rows.Close()

	type UserItem struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		FullName   string `json:"full_name"`
		Phone      string `json:"phone"`
		Expiration int64  `json:"expiration"`
		Enabled    bool   `json:"enabled"`
		Profile    string `json:"profile"`
	}

	var users []UserItem
	for rows.Next() {
		var u UserItem
		var enabledInt int
		if err := rows.Scan(&u.Username, &u.Password, &u.FullName, &u.Phone, &u.Expiration, &enabledInt, &u.Profile); err == nil {
			u.Enabled = (enabledInt == 1)
			users = append(users, u)
		}
	}

	return c.JSON(fiber.Map{"success": true, "users": users})
}

func (h *APIHandler) handleCreateUser(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		FullName string `json:"full_name"`
		Phone    string `json:"phone"`
		Profile  string `json:"profile"`
		Days     int    `json:"days"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "بيانات غير صالحة"})
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "اسم المستخدم مطلوب"})
	}

	if req.Profile == "" {
		req.Profile = "10M"
	}
	if req.Days <= 0 {
		req.Days = 30
	}

	expUnix := time.Now().Add(time.Duration(req.Days) * 24 * time.Hour).Unix()

	// Insert into radcheck
	_, _ = db.Exec("DELETE FROM radcheck WHERE username = ?", username)
	_, err := db.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES (?, 'Cleartext-Password', ':=', ?)", username, req.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}

	// Insert into radusergroup
	_, _ = db.Exec("DELETE FROM radusergroup WHERE username = ?", username)
	_, _ = db.Exec("INSERT INTO radusergroup (username, groupname, priority) VALUES (?, ?, 1)", username, req.Profile)

	// Insert into radius_user_meta
	_, _ = db.Exec(`
		INSERT OR REPLACE INTO radius_user_meta (username, full_name, phone, expiration_unix, enabled)
		VALUES (?, ?, ?, ?, 1)
	`, username, req.FullName, req.Phone, expUnix)

	return c.JSON(fiber.Map{"success": true, "message": "تم إضافة المشترك بنجاح"})
}

func (h *APIHandler) handleDeleteUser(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	username := c.Params("username")

	_, _ = db.Exec("DELETE FROM radcheck WHERE username = ?", username)
	_, _ = db.Exec("DELETE FROM radreply WHERE username = ?", username)
	_, _ = db.Exec("DELETE FROM radusergroup WHERE username = ?", username)
	_, _ = db.Exec("DELETE FROM radius_user_meta WHERE username = ?", username)

	return c.JSON(fiber.Map{"success": true, "message": "تم حذف المشترك"})
}

func (h *APIHandler) handleListProfiles(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)

	rows, err := db.Query(`
		SELECT groupname, validity_days, price 
		FROM radius_profile_meta
		ORDER BY groupname ASC
	`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	defer rows.Close()

	type ProfileItem struct {
		Name         string  `json:"name"`
		ValidityDays int     `json:"validity_days"`
		Price        float64 `json:"price"`
		RateLimit    string  `json:"rate_limit"`
	}

	var list []ProfileItem
	for rows.Next() {
		var p ProfileItem
		if err := rows.Scan(&p.Name, &p.ValidityDays, &p.Price); err == nil {
			_ = db.QueryRow("SELECT value FROM radgroupreply WHERE groupname = ? AND attribute = 'Mikrotik-Rate-Limit'", p.Name).Scan(&p.RateLimit)
			list = append(list, p)
		}
	}

	return c.JSON(fiber.Map{"success": true, "profiles": list})
}

func (h *APIHandler) handleListVouchers(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)

	rows, err := db.Query(`
		SELECT id, batch_id, code, profile_name, validity_days, price, is_used, used_by, used_at, created_at
		FROM radius_vouchers
		ORDER BY id DESC
		LIMIT 200
	`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	defer rows.Close()

	type VoucherItem struct {
		ID           int64   `json:"id"`
		BatchID      string  `json:"batch_id"`
		Code         string  `json:"code"`
		ProfileName  string  `json:"profile_name"`
		ValidityDays int     `json:"validity_days"`
		Price        float64 `json:"price"`
		IsUsed       bool    `json:"is_used"`
		UsedBy       *string `json:"used_by"`
		UsedAt       *string `json:"used_at"`
		CreatedAt    string  `json:"created_at"`
	}

	var vouchers []VoucherItem
	for rows.Next() {
		var v VoucherItem
		var isUsedInt int
		if err := rows.Scan(&v.ID, &v.BatchID, &v.Code, &v.ProfileName, &v.ValidityDays, &v.Price, &isUsedInt, &v.UsedBy, &v.UsedAt, &v.CreatedAt); err == nil {
			v.IsUsed = (isUsedInt == 1)
			vouchers = append(vouchers, v)
		}
	}

	return c.JSON(fiber.Map{"success": true, "vouchers": vouchers})
}

func (h *APIHandler) handleGenerateVouchers(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)

	var req struct {
		Count        int     `json:"count"`
		ProfileName  string  `json:"profile_name"`
		ValidityDays int     `json:"validity_days"`
		Price        float64 `json:"price"`
	}
	if err := c.BodyParser(&req); err != nil || req.Count <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "العدد غير صالح"})
	}

	if req.Count > 1000 {
		req.Count = 1000
	}
	if req.ProfileName == "" {
		req.ProfileName = "10M"
	}
	if req.ValidityDays <= 0 {
		req.ValidityDays = 30
	}

	batchID := fmt.Sprintf("batch_%d", time.Now().Unix())
	tx, err := db.Begin()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO radius_vouchers (batch_id, code, profile_name, validity_days, price, created_by, is_used)
		VALUES (?, ?, ?, ?, ?, 1, 0)
	`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	defer stmt.Close()

	for i := 0; i < req.Count; i++ {
		code := fmt.Sprintf("%d%05d", time.Now().Unix()%100000, i)
		_, _ = stmt.Exec(batchID, code, req.ProfileName, req.ValidityDays, req.Price)
	}

	if err := tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"batch_id": batchID,
		"count":    req.Count,
		"message":  fmt.Sprintf("تم توليد %d قسيمة بنجاح", req.Count),
	})
}

func (h *APIHandler) handleListActiveSessions(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)

	rows, err := db.Query(`
		SELECT radacctid, acctsessionid, username, nasipaddress, acctstarttime, 
		       COALESCE(framedipaddress, ''), COALESCE(callingstationid, ''),
		       COALESCE(acctinputoctets, 0), COALESCE(acctoutputoctets, 0)
		FROM radacct
		WHERE acctstoptime IS NULL
		ORDER BY radacctid DESC
		LIMIT 200
	`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	defer rows.Close()

	type SessionItem struct {
		RadAcctID   int64  `json:"radacctid"`
		SessionID   string `json:"session_id"`
		Username    string `json:"username"`
		NasIP       string `json:"nas_ip"`
		StartTime   string `json:"start_time"`
		UserIP      string `json:"user_ip"`
		UserMAC     string `json:"user_mac"`
		BytesIn     int64  `json:"bytes_in"`
		BytesOut    int64  `json:"bytes_out"`
	}

	var sessions []SessionItem
	for rows.Next() {
		var s SessionItem
		if err := rows.Scan(&s.RadAcctID, &s.SessionID, &s.Username, &s.NasIP, &s.StartTime, &s.UserIP, &s.UserMAC, &s.BytesIn, &s.BytesOut); err == nil {
			sessions = append(sessions, s)
		}
	}

	return c.JSON(fiber.Map{"success": true, "sessions": sessions})
}

func (h *APIHandler) handleDisconnectSession(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)

	var req struct {
		Username  string `json:"username"`
		SessionID string `json:"session_id"`
	}
	_ = c.BodyParser(&req)

	now := time.Now()
	_, err := db.Exec(`
		UPDATE radacct SET acctstoptime = ?, acctterminatecause = 'Admin-Reset'
		WHERE (username = ? OR acctsessionid = ?) AND acctstoptime IS NULL
	`, now, req.Username, req.SessionID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true, "message": "تم إغلاق الجلسة"})
}
