package cloudtenant

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

func init() {
	store.SetWAVersion(store.WAVersionContainer{2, 3000, 1047769893})
	go autoUpdateWAVersion()
}

func autoUpdateWAVersion() {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://raw.githubusercontent.com/tulir/whatsmeow/main/store/clientpayload.go")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	re := regexp.MustCompile(`WAVersionContainer\{(\d+),\s*(\d+),\s*(\d+)\}`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) == 4 {
		v1, _ := strconv.ParseUint(matches[1], 10, 32)
		v2, _ := strconv.ParseUint(matches[2], 10, 32)
		v3, _ := strconv.ParseUint(matches[3], 10, 32)
		if v1 > 0 && v2 > 0 && v3 > 0 {
			newVer := store.WAVersionContainer{uint32(v1), uint32(v2), uint32(v3)}
			store.SetWAVersion(newVer)
			log.Printf("[whatsapp] Dynamic WAVersion updated to: %s", newVer.String())
		}
	}
}

var (
	cloudWAContainers = make(map[string]*sqlstore.Container)
	cloudWAClients    = make(map[string]*whatsmeow.Client)
	cloudWAMu         sync.Mutex
)

func getTenantWAClient(subdomain string, adminID int64, db *sql.DB, baseDir string) (*whatsmeow.Client, error) {
	if adminID <= 0 {
		adminID = 1
	}
	clientKey := fmt.Sprintf("%s:%d", subdomain, adminID)

	cloudWAMu.Lock()
	client, ok := cloudWAClients[clientKey]
	cloudWAMu.Unlock()

	if ok && client != nil {
		return client, nil
	}

	cloudWAMu.Lock()
	defer cloudWAMu.Unlock()

	// Double check
	if client, ok := cloudWAClients[clientKey]; ok && client != nil {
		return client, nil
	}

	tenantDir := filepath.Join(baseDir, "data", "tenants", subdomain)
	if _, e := os.Stat("/app/data"); e == nil {
		tenantDir = filepath.Join("/app/data", "tenants", subdomain)
	}
	_ = os.MkdirAll(tenantDir, 0755)

	container, ok := cloudWAContainers[subdomain]
	if !ok || container == nil {
		dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)", filepath.ToSlash(filepath.Join(tenantDir, "whatsapp.db")))
		var err error
		container, err = sqlstore.New(context.Background(), "sqlite", dsn, waLog.Stdout("Database", "ERROR", true))
		if err != nil {
			return nil, fmt.Errorf("failed to init whatsmeow store for %s: %w", subdomain, err)
		}
		cloudWAContainers[subdomain] = container
	}

	clientLog := waLog.Stdout("WA-Client", "INFO", true)

	var deviceJID string
	_ = db.QueryRow("SELECT COALESCE(device_jid, '') FROM radius_whatsapp_config WHERE admin_id = ?", adminID).Scan(&deviceJID)

	if deviceJID != "" {
		if jid, jidErr := types.ParseJID(deviceJID); jidErr == nil {
			if allDevices, devErr := container.GetAllDevices(context.Background()); devErr == nil {
				for _, d := range allDevices {
					if d.ID != nil && d.ID.String() == jid.String() {
						client = whatsmeow.NewClient(d, clientLog)
						break
					}
				}
			}
		}
		if client == nil {
			_, _ = db.Exec("UPDATE radius_whatsapp_config SET device_jid = '' WHERE admin_id = ?", adminID)
		}
	}

	if client == nil {
		client = whatsmeow.NewClient(container.NewDevice(), clientLog)
	}

	capturedAdminID := adminID
	capturedKey := clientKey
	client.AddEventHandler(func(evt interface{}) {
		switch evt.(type) {
		case *events.Connected:
			log.Printf("[whatsapp] Tenant %s admin %d connected", subdomain, capturedAdminID)
			if client.Store.ID != nil {
				jidStr := client.Store.ID.String()
				_, _ = db.Exec(`
					INSERT INTO radius_whatsapp_config (admin_id, device_jid) VALUES (?, ?)
					ON CONFLICT(admin_id) DO UPDATE SET device_jid = excluded.device_jid
				`, capturedAdminID, jidStr)
			}
		case *events.LoggedOut:
			log.Printf("[whatsapp] Tenant %s admin %d logged out", subdomain, capturedAdminID)
			_, _ = db.Exec("UPDATE radius_whatsapp_config SET device_jid = '' WHERE admin_id = ?", capturedAdminID)
			cloudWAMu.Lock()
			delete(cloudWAClients, capturedKey)
			cloudWAMu.Unlock()
		}
	})

	cloudWAClients[clientKey] = client
	return client, nil
}

func (h *APIHandler) handleGetWhatsappConfig(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	subdomain, _ := c.Locals("subdomain").(string)
	if subdomain == "" {
		subdomain = "default"
	}
	adminID, _ := c.Locals("admin_id").(int64)
	if adminID <= 0 {
		adminID = 1
	}

	var enabled, reminderEnabled, reminderHours int
	var phone, deviceJID string
	_ = db.QueryRow("SELECT enabled, COALESCE(phone_number, ''), reminder_enabled, reminder_hours, COALESCE(device_jid, '') FROM radius_whatsapp_config WHERE admin_id = ?", adminID).Scan(&enabled, &phone, &reminderEnabled, &reminderHours, &deviceJID)

	status := "disconnected"
	clientKey := fmt.Sprintf("%s:%d", subdomain, adminID)
	cloudWAMu.Lock()
	client := cloudWAClients[clientKey]
	cloudWAMu.Unlock()

	if client == nil && deviceJID != "" {
		client, _ = getTenantWAClient(subdomain, adminID, db, ".")
	}
	if client != nil {
		if client.IsConnected() && client.IsLoggedIn() {
			status = "connected"
		}
	}

	return c.JSON(fiber.Map{
		"enabled":          enabled == 1,
		"phone_number":     phone,
		"reminder_enabled": reminderEnabled == 1,
		"reminder_hours":   reminderHours,
		"status":           status,
	})
}

func (h *APIHandler) handleSaveWhatsappConfig(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	adminID, _ := c.Locals("admin_id").(int64)
	if adminID <= 0 {
		adminID = 1
	}

	var req struct {
		Enabled         int    `json:"enabled"`
		PhoneNumber     string `json:"phone_number"`
		ReminderEnabled int    `json:"reminder_enabled"`
		ReminderHours   int    `json:"reminder_hours"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}

	if req.ReminderHours <= 0 {
		req.ReminderHours = 24
	}

	_, err := db.Exec(`
		INSERT INTO radius_whatsapp_config (admin_id, enabled, phone_number, reminder_enabled, reminder_hours, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(admin_id) DO UPDATE SET
			enabled = excluded.enabled,
			phone_number = excluded.phone_number,
			reminder_enabled = excluded.reminder_enabled,
			reminder_hours = excluded.reminder_hours,
			updated_at = CURRENT_TIMESTAMP
	`, adminID, req.Enabled, req.PhoneNumber, req.ReminderEnabled, req.ReminderHours)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true, "message": "تم حفظ إعدادات الواتساب بنجاح"})
}

func (h *APIHandler) handleGetWhatsappQR(c *fiber.Ctx) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[whatsapp-panic] handleGetWhatsappQR: %v", r)
			_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("خطأ في معالجة كود QR: %v", r)})
		}
	}()

	db := c.Locals("tenant_db").(*sql.DB)
	subdomain, _ := c.Locals("subdomain").(string)
	if subdomain == "" {
		subdomain = "default"
	}
	adminID, _ := c.Locals("admin_id").(int64)
	if adminID <= 0 {
		adminID = 1
	}
	clientKey := fmt.Sprintf("%s:%d", subdomain, adminID)

	client, err := getTenantWAClient(subdomain, adminID, db, ".")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل تجهيز خدمة الواتساب: " + err.Error()})
	}

	if client.IsConnected() && client.IsLoggedIn() {
		return c.JSON(fiber.Map{"status": "connected", "message": "متصل بالفعل"})
	}

	if client.Store.ID != nil {
		err = client.Connect()
		if err == nil || client.IsConnected() {
			for i := 0; i < 5; i++ {
				if client.IsConnected() && client.IsLoggedIn() {
					return c.JSON(fiber.Map{"status": "connected", "message": "تم الاتصال تلقائياً"})
				}
				time.Sleep(300 * time.Millisecond)
			}
			if client.IsConnected() && client.IsLoggedIn() {
				return c.JSON(fiber.Map{"status": "connected", "message": "تم الاتصال تلقائياً"})
			}
		}

		// Reset unauthenticated store session
		client.Disconnect()
		_ = client.Store.Delete(context.Background())
		_, _ = db.Exec("UPDATE radius_whatsapp_config SET device_jid = '' WHERE admin_id = ?", adminID)
		cloudWAMu.Lock()
		delete(cloudWAClients, clientKey)
		cloudWAMu.Unlock()

		client, err = getTenantWAClient(subdomain, adminID, db, ".")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل إعادة تعيين جلسة الواتساب: " + err.Error()})
		}
	} else {
		// Device is fresh/unauthenticated: ensure any active connection is cleared
		if client.IsConnected() {
			client.Disconnect()
			time.Sleep(200 * time.Millisecond)
		}
		// In whatsmeow, if Connect() was already called on this client, GetQRChannel will fail
		// So clear client from cache and create a clean device
		cloudWAMu.Lock()
		delete(cloudWAClients, clientKey)
		cloudWAMu.Unlock()

		client, err = getTenantWAClient(subdomain, adminID, db, ".")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل تجهيز جهاز الواتساب: " + err.Error()})
		}
	}

	qrChan, err := client.GetQRChannel(context.Background())
	if err != nil {
		log.Printf("[whatsapp] GetQRChannel error for %s: %v", clientKey, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل بدء مسح كود QR: " + err.Error()})
	}

	if connErr := client.Connect(); connErr != nil {
		log.Printf("[whatsapp] Connect error for %s: %v", clientKey, connErr)
	}

	deadline := time.After(25 * time.Second)
	for {
		select {
		case evt, ok := <-qrChan:
			if !ok {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "انقطعت قناة كود QR"})
			}
			switch evt.Event {
			case "code":
				png, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل توليد صورة QR"})
				}
				base64Img := base64.StdEncoding.EncodeToString(png)
				return c.JSON(fiber.Map{
					"status": "qr",
					"qr":     "data:image/png;base64," + base64Img,
				})
			case "success":
				for i := 0; i < 10; i++ {
					if client.IsConnected() && client.IsLoggedIn() {
						return c.JSON(fiber.Map{"status": "connected", "message": "تم الربط بنجاح"})
					}
					time.Sleep(300 * time.Millisecond)
				}
				return c.JSON(fiber.Map{"status": "waiting", "message": "تم استلام حدث الربط، بانتظار اكتمال تسجيل الدخول"})
			case "timeout":
				log.Printf("[whatsapp] Received QR timeout event for %s, awaiting fresh code event...", clientKey)
				continue
			}
		case <-deadline:
			return c.Status(fiber.StatusRequestTimeout).JSON(fiber.Map{"error": "انتهت مهلة انتظار كود QR. يرجى المحاولة مرة أخرى."})
		}
	}
	return nil
}

func (h *APIHandler) handleLogoutWhatsapp(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	subdomain, _ := c.Locals("subdomain").(string)
	if subdomain == "" {
		subdomain = "default"
	}
	adminID, _ := c.Locals("admin_id").(int64)
	if adminID <= 0 {
		adminID = 1
	}
	clientKey := fmt.Sprintf("%s:%d", subdomain, adminID)

	cloudWAMu.Lock()
	client := cloudWAClients[clientKey]
	delete(cloudWAClients, clientKey)
	cloudWAMu.Unlock()

	if client != nil {
		client.Disconnect()
		_ = client.Store.Delete(context.Background())
	}

	_, _ = db.Exec("UPDATE radius_whatsapp_config SET device_jid = '' WHERE admin_id = ?", adminID)
	return c.JSON(fiber.Map{"success": true, "message": "تم تسجيل الخروج وفصل الواتساب بنجاح"})
}

func (h *APIHandler) handleTestWhatsapp(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	subdomain, _ := c.Locals("subdomain").(string)
	if subdomain == "" {
		subdomain = "default"
	}
	adminID, _ := c.Locals("admin_id").(int64)
	if adminID <= 0 {
		adminID = 1
	}

	var req struct {
		Phone   string `json:"phone"`
		Message string `json:"message"`
	}
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Phone) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "رقم الهاتف مطلوب"})
	}

	client, err := getTenantWAClient(subdomain, adminID, db, ".")
	if err != nil || client == nil || !client.IsConnected() || !client.IsLoggedIn() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "الواتساب غير متصل. يرجى مسح كود QR أولاً."})
	}

	cleanPhone := strings.ReplaceAll(req.Phone, "+", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
	if strings.HasPrefix(cleanPhone, "07") {
		cleanPhone = "964" + cleanPhone[1:]
	}

	targetJID := types.NewJID(cleanPhone, types.DefaultUserServer)
	msgText := req.Message
	if msgText == "" {
		msgText = "🔔 رسالة اختبار من نظام SASMAN Cloud"
	}

	_, sendErr := client.SendMessage(context.Background(), targetJID, &waE2E.Message{
		Conversation: proto.String(msgText),
	})

	if sendErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل إرسال الرسالة: " + sendErr.Error()})
	}

	return c.JSON(fiber.Map{"success": true, "message": "تم إرسال رسالة الاختبار بنجاح ✅"})
}

func (h *APIHandler) handleWhatsappBroadcast(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	subdomain, _ := c.Locals("subdomain").(string)
	if subdomain == "" {
		subdomain = "default"
	}
	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)
	if adminID <= 0 {
		adminID = 1
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Message) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "نص الرسالة مطلوب"})
	}

	client, err := getTenantWAClient(subdomain, adminID, db, ".")
	if err != nil || client == nil || !client.IsConnected() || !client.IsLoggedIn() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "الواتساب غير متصل. يرجى مسح كود QR أولاً."})
	}

	var rows *sql.Rows
	if role == "superadmin" {
		rows, err = db.Query("SELECT DISTINCT phone FROM radius_user_meta WHERE phone IS NOT NULL AND phone != ''")
	} else {
		rows, err = db.Query("SELECT DISTINCT phone FROM radius_user_meta WHERE (admin_id = ? OR admin_id IN (SELECT id FROM radius_admins WHERE parent_id = ?)) AND phone IS NOT NULL AND phone != ''", adminID, adminID)
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل جلب أرقام الهواتف: " + err.Error()})
	}
	defer rows.Close()

	var phones []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			p = strings.TrimSpace(p)
			if p != "" {
				phones = append(phones, p)
			}
		}
	}

	if len(phones) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "لا يوجد مشتركون بأرقام هواتف صالحة"})
	}

	go func(targetPhones []string, msg string, waClient *whatsmeow.Client) {
		for _, p := range targetPhones {
			cleanPhone := strings.ReplaceAll(p, "+", "")
			cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
			cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
			if strings.HasPrefix(cleanPhone, "07") {
				cleanPhone = "964" + cleanPhone[1:]
			}
			targetJID := types.NewJID(cleanPhone, types.DefaultUserServer)
			_, _ = waClient.SendMessage(context.Background(), targetJID, &waE2E.Message{
				Conversation: proto.String(msg),
			})
			time.Sleep(1500 * time.Millisecond)
		}
	}(phones, req.Message, client)

	return c.JSON(fiber.Map{"success": true, "message": fmt.Sprintf("بدأ إرسال الرسالة إلى %d مشترك في الخلفية.", len(phones))})
}

func (h *APIHandler) handleWhatsappSendDebtReminder(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	subdomain, _ := c.Locals("subdomain").(string)
	if subdomain == "" {
		subdomain = "default"
	}
	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)
	if adminID <= 0 {
		adminID = 1
	}

	client, err := getTenantWAClient(subdomain, adminID, db, ".")
	if err != nil || client == nil || !client.IsConnected() || !client.IsLoggedIn() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "الواتساب غير متصل. يرجى مسح كود QR أولاً."})
	}

	var templateText string
	_ = db.QueryRow("SELECT template_text FROM radius_whatsapp_templates WHERE event_type = 'debt_reminder' AND enabled = 1").Scan(&templateText)
	if templateText == "" {
		templateText = "تذكير بالديون المستحقة 📋\nعزيزي {full_name}، نود تذكيرك بأن لديك ديوناً مستحقة بمبلغ {balance} د.ع.\nيرجى التواصل مع الوكيل لتسوية الحساب في أقرب وقت ممكن.\nشكراً لتعاملكم معنا 🙏"
	}

	var rows *sql.Rows
	if role == "superadmin" {
		rows, err = db.Query("SELECT username, COALESCE(phone, ''), COALESCE(full_name, ''), balance FROM radius_user_meta WHERE balance > 0 AND phone IS NOT NULL AND phone != ''")
	} else {
		rows, err = db.Query("SELECT username, COALESCE(phone, ''), COALESCE(full_name, ''), balance FROM radius_user_meta WHERE (admin_id = ? OR admin_id IN (SELECT id FROM radius_admins WHERE parent_id = ?)) AND balance > 0 AND phone IS NOT NULL AND phone != ''", adminID, adminID)
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل جلب المستخدمين: " + err.Error()})
	}
	defer rows.Close()

	type debtUser struct {
		Username string
		Phone    string
		FullName string
		Balance  float64
	}
	var debtUsers []debtUser
	for rows.Next() {
		var u debtUser
		if err := rows.Scan(&u.Username, &u.Phone, &u.FullName, &u.Balance); err == nil {
			debtUsers = append(debtUsers, u)
		}
	}

	if len(debtUsers) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "لا يوجد مشتركون عليهم ديون بأرقام هواتف مسجلة"})
	}

	go func(users []debtUser, tmpl string, waClient *whatsmeow.Client) {
		for _, u := range users {
			msg := tmpl
			msg = strings.ReplaceAll(msg, "{username}", u.Username)
			msg = strings.ReplaceAll(msg, "{full_name}", u.FullName)
			msg = strings.ReplaceAll(msg, "{balance}", fmt.Sprintf("%.0f", u.Balance))

			cleanPhone := strings.ReplaceAll(u.Phone, "+", "")
			cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
			cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
			if strings.HasPrefix(cleanPhone, "07") {
				cleanPhone = "964" + cleanPhone[1:]
			}
			targetJID := types.NewJID(cleanPhone, types.DefaultUserServer)
			_, _ = waClient.SendMessage(context.Background(), targetJID, &waE2E.Message{
				Conversation: proto.String(msg),
			})
			time.Sleep(1500 * time.Millisecond)
		}
	}(debtUsers, templateText, client)

	return c.JSON(fiber.Map{"success": true, "message": fmt.Sprintf("بدأ إرسال تذكير الديون إلى %d مشترك في الخلفية.", len(debtUsers))})
}

func (h *APIHandler) SendTenantWhatsappNotification(subdomain string, db *sql.DB, username string, templateKey string, vars map[string]string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[whatsapp-panic] SendTenantWhatsappNotification: %v", r)
			}
		}()

		if subdomain == "" {
			subdomain = "default"
		}

		var phone string
		var adminID sql.NullInt64
		var fullName string
		err := db.QueryRow(
			"SELECT COALESCE(phone, ''), admin_id, COALESCE(full_name, '') FROM radius_user_meta WHERE username = ?",
			username,
		).Scan(&phone, &adminID, &fullName)

		phone = strings.TrimSpace(phone)
		if err != nil || phone == "" {
			return
		}

		targetAdmin := int64(1)
		if adminID.Valid && adminID.Int64 > 0 {
			targetAdmin = adminID.Int64
		}

		var enabled int
		_ = db.QueryRow("SELECT enabled FROM radius_whatsapp_config WHERE admin_id = ?", targetAdmin).Scan(&enabled)
		if enabled != 1 {
			return
		}

		var templateText string
		_ = db.QueryRow("SELECT template_text FROM radius_whatsapp_templates WHERE event_type = ? AND enabled = 1", templateKey).Scan(&templateText)
		if templateText == "" {
			return
		}

		client, err := getTenantWAClient(subdomain, targetAdmin, db, ".")
		if err != nil || client == nil || !client.IsConnected() || !client.IsLoggedIn() {
			return
		}

		msg := templateText
		for k, v := range vars {
			msg = strings.ReplaceAll(msg, "{"+k+"}", v)
		}
		msg = strings.ReplaceAll(msg, "{username}", username)
		msg = strings.ReplaceAll(msg, "{full_name}", fullName)

		cleanPhone := strings.ReplaceAll(phone, "+", "")
		cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
		cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
		if strings.HasPrefix(cleanPhone, "07") {
			cleanPhone = "964" + cleanPhone[1:]
		}

		targetJID := types.NewJID(cleanPhone, types.DefaultUserServer)
		_, _ = client.SendMessage(context.Background(), targetJID, &waE2E.Message{
			Conversation: proto.String(msg),
		})
	}()
}

