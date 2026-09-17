package radius

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

	_ "github.com/go-sql-driver/mysql"
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
}

var (
	waContainer *sqlstore.Container
	waClients   = make(map[int64]*whatsmeow.Client)
	waMu        sync.Mutex
)

type WhatsappConfig struct {
	ID              int    `json:"id"`
	AdminID         int64  `json:"admin_id"`
	Enabled         int    `json:"enabled"`
	PhoneNumber     string `json:"phone_number"`
	ReminderEnabled int    `json:"reminder_enabled"`
	ReminderHours   int    `json:"reminder_hours"`
	DeviceJID       string `json:"device_jid"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type MessageTemplate struct {
	ID           int    `json:"id"`
	TemplateKey  string `json:"template_key"`
	TemplateText string `json:"template_text"`
	UpdatedAt    string `json:"updated_at"`
}

func initWAStore() {
	waMu.Lock()
	defer waMu.Unlock()

	if waContainer != nil {
		return
	}
	dbLog := waLog.Stdout("Database", "ERROR", true)

	// Ensure tables exist - Use the global DB handle
	if DB == nil {
		log.Printf("[whatsapp] Global DB handle is nil, cannot ensure tables exist")
		return
	}

	_, err := DB.Exec(`CREATE TABLE IF NOT EXISTS radius_whatsapp_config (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		admin_id INTEGER NOT NULL UNIQUE,
		enabled INTEGER DEFAULT 0,
		phone_number TEXT,
		reminder_enabled INTEGER DEFAULT 0,
		reminder_hours INTEGER DEFAULT 24,
		device_jid TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Printf("[whatsapp] Failed to create radius_whatsapp_config: %v", err)
	}

	// Migration: Check for columns using PRAGMA table_info
	columns := make(map[string]bool)
	rows, err := DB.Query("PRAGMA table_info(radius_whatsapp_config)")
	if err == nil {
		for rows.Next() {
			var cid int
			var name, dtype string
			var notnull, pk int
			var dflt interface{}
			rows.Scan(&cid, &name, &dtype, &notnull, &dflt, &pk)
			columns[name] = true
		}
		rows.Close()
	}

	if !columns["reminder_enabled"] {
		_, _ = DB.Exec("ALTER TABLE radius_whatsapp_config ADD COLUMN reminder_enabled INTEGER NOT NULL DEFAULT 0")
	}
	if !columns["reminder_hours"] {
		_, _ = DB.Exec("ALTER TABLE radius_whatsapp_config ADD COLUMN reminder_hours INTEGER NOT NULL DEFAULT 24")
	}
	if !columns["device_jid"] {
		_, _ = DB.Exec("ALTER TABLE radius_whatsapp_config ADD COLUMN device_jid TEXT DEFAULT ''")
	}

	// Ensure admin_id uniqueness for ON CONFLICT to work
	_, _ = DB.Exec("DELETE FROM radius_whatsapp_config WHERE id NOT IN (SELECT MAX(id) FROM radius_whatsapp_config GROUP BY admin_id)")
	_, _ = DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_whatsapp_admin ON radius_whatsapp_config(admin_id)")

	// Migration: radius_user_meta columns
	userMetaCols := make(map[string]bool)
	rows, err = DB.Query("PRAGMA table_info(radius_user_meta)")
	if err == nil {
		for rows.Next() {
			var cid int
			var name, dtype string
			var notnull, pk int
			var dflt interface{}
			rows.Scan(&cid, &name, &dtype, &notnull, &dflt, &pk)
			userMetaCols[name] = true
		}
		rows.Close()
	}
	if !userMetaCols["last_reminder_at"] {
		_, _ = DB.Exec("ALTER TABLE radius_user_meta ADD COLUMN last_reminder_at DATETIME DEFAULT NULL")
	}
	if !userMetaCols["reminder_sent_for"] {
		_, _ = DB.Exec("ALTER TABLE radius_user_meta ADD COLUMN reminder_sent_for INTEGER DEFAULT 0")
	}

	_, err = DB.Exec(`CREATE TABLE IF NOT EXISTS radius_message_templates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		_template_key TEXT UNIQUE,
		template_text TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Printf("[whatsapp] Failed to create radius_message_templates: %v", err)
	}

	seedDefaultTemplates()

	storePath := os.Getenv("WHATSAPP_STORE_PATH")
	if storePath == "" {
		dataDir := os.Getenv("SASMAN_DATA_DIR")
		if dataDir == "" {
			dataDir = "data"
		}
		_ = os.MkdirAll(dataDir, 0755)
		storePath = filepath.Join(dataDir, "whatsapp.db")
	} else {
		_ = os.MkdirAll(filepath.Dir(storePath), 0755)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", filepath.ToSlash(storePath))

	waContainer, err = sqlstore.New(context.Background(), "sqlite", dsn, dbLog)
	if err != nil {
		log.Printf("[whatsapp] Failed to initialize whatsmeow sqlstore: %v", err)
		waContainer = nil // Ensure it's nil if failed
		return
	}
}

func getWAClient(adminID int64) (*whatsmeow.Client, error) {
	waMu.Lock()
	client, ok := waClients[adminID]
	waMu.Unlock()

	if ok && client != nil {
		return client, nil
	}

	initWAStore()

	waMu.Lock()
	client, ok = waClients[adminID]
	container := waContainer
	waMu.Unlock()

	if ok && client != nil {
		return client, nil
	}

	if container == nil {
		return nil, fmt.Errorf("WA store not initialized (check logs for errors)")
	}

	clientLog := waLog.Stdout("Client", "ERROR", true)

	// Per-admin device selection: each admin has their own WhatsApp session
	var adminDeviceJID string
	_ = DB.QueryRow("SELECT COALESCE(device_jid,'') FROM radius_whatsapp_config WHERE admin_id = ?", adminID).Scan(&adminDeviceJID)

	if adminDeviceJID != "" {
		if jid, jidErr := types.ParseJID(adminDeviceJID); jidErr == nil {
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
			// Stored JID no longer in device store, clear it and create new
			log.Printf("[whatsapp] Device JID for admin %d not found in store, creating new device", adminID)
			_, _ = DB.Exec("UPDATE radius_whatsapp_config SET device_jid = '' WHERE admin_id = ?", adminID)
		}
	}

	if client == nil {
		// No device for this admin yet: allocate a fresh unregistered device
		client = whatsmeow.NewClient(container.NewDevice(), clientLog)
	}

	client.AddEventHandler(func(evt interface{}) {
		switch evt.(type) {
		case *events.Connected:
			log.Printf("[whatsapp] Admin %d connected", adminID)
			// Persist device JID so future restarts reload the correct device
			if client.Store.ID != nil {
				jidStr := client.Store.ID.String()
				_, _ = DB.Exec(
					`INSERT INTO radius_whatsapp_config (admin_id, device_jid) VALUES (?, ?)
					ON CONFLICT(admin_id) DO UPDATE SET device_jid = excluded.device_jid`,
					adminID, jidStr,
				)
			}
		case *events.LoggedOut:
			log.Printf("[whatsapp] Admin %d logged out", adminID)
			_, _ = DB.Exec("UPDATE radius_whatsapp_config SET device_jid = '' WHERE admin_id = ?", adminID)
			waMu.Lock()
			delete(waClients, adminID)
			waMu.Unlock()
		}
	})

	waMu.Lock()
	if existing := waClients[adminID]; existing != nil {
		waMu.Unlock()
		return existing, nil
	}
	waClients[adminID] = client
	waMu.Unlock()

	if client.Store.ID != nil {
		err := client.Connect()
		if err != nil {
			waMu.Lock()
			delete(waClients, adminID)
			waMu.Unlock()
			return nil, err
		}
	}

	return client, nil
}

func loadWhatsappConfig(adminID int64) (WhatsappConfig, error) {
	var config WhatsappConfig
	var createdAt, updatedAt sql.NullString
	var err error

	initWAStore()

	err = DB.QueryRow(
		"SELECT id, admin_id, enabled, phone_number, reminder_enabled, reminder_hours, COALESCE(device_jid,''), created_at, updated_at FROM radius_whatsapp_config WHERE admin_id = ? LIMIT 1",
		adminID,
	).Scan(&config.ID, &config.AdminID, &config.Enabled, &config.PhoneNumber, &config.ReminderEnabled, &config.ReminderHours, &config.DeviceJID, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return WhatsappConfig{AdminID: adminID, Enabled: 0}, nil
	}
	if err != nil {
		return config, err
	}

	config.CreatedAt = createdAt.String
	config.UpdatedAt = updatedAt.String
	return config, nil
}

func loadMessageTemplate(key string) string {
	var text string
	err := DB.QueryRow("SELECT template_text FROM radius_message_templates WHERE _template_key=?", key).Scan(&text)
	if err != nil {
		defaults := map[string]string{
			"renew_paid": "تم تجديد اشتراكك بنجاح ✅\nالمستخدم: {username}\nالباقة: {profile}\nالسعر: {price} د.ع\nالمدة: {validity_days} يوم\nحالة الدفع: مدفوع",
			"renew_debt": "تم تجديد اشتراكك ⏳\nالمستخدم: {username}\nالباقة: {profile}\nالسعر: {price} د.ع\nالمدة: {validity_days} يوم\nملاحظة: تمت إضافة المبلغ كديون\nرصيدك الحالي: {balance} د.ع",
			"add_debt":   "تم إضافة ديون 📋\nالمستخدم: {username}\nالمبلغ: {amount} د.ع\nالملاحظات: {notes}\nرصيدك الحالي: {balance} د.ع",
			"payment":    "تم تسديد ديون ✅\nالمستخدم: {username}\nالمبلغ: {amount} د.ع\nالملاحظات: {notes}\nرصيدك الحالي: {balance} د.ع",
		"expiry_reminder": "تنبيه انتهاء الاشتراك ⚠️\nعزيزي {full_name}، نود إعلامك أن اشتراكك في باقة {profile} سينتهي قريباً.\nتاريخ الانتهاء: {expiry_date}\nيرجى التجديد لضمان استمرار الخدمة.",
			"debt_reminder": "تذكير بالديون المستحقة 📋\nعزيزي {username}، نود تذكيرك بأن لديك ديوناً مستحقة بمبلغ {balance} د.ع.\nيرجى التواصل مع الوكيل لتسوية الحساب في أقرب وقت ممكن.\nشكراً لتعاملكم معنا 🙏",
		}
		if t, ok := defaults[key]; ok {
			return t
		}
		return ""
	}
	return text
}

func SendWhatsappNotification(username, templateKey string, vars map[string]string) {
	var phone string
	var adminID sql.NullInt64
	var fullName string
	err := DB.QueryRow(
		"SELECT phone, admin_id, COALESCE(full_name,'') FROM radius_user_meta WHERE username=?",
		username,
	).Scan(&phone, &adminID, &fullName)

	if err != nil || phone == "" {
		return
	}

	targetAdmin := int64(0)
	if adminID.Valid {
		targetAdmin = adminID.Int64
	}

	config, err := loadWhatsappConfig(targetAdmin)
	if err != nil || config.Enabled == 0 {
		return
	}

	templateText := loadMessageTemplate(templateKey)
	if templateText == "" {
		return
	}

	// Merge vars, always including full_name so every template can use {full_name}
	varsWithFullName := make(map[string]string)
	if vars != nil {
		for k, v := range vars {
			varsWithFullName[k] = v
		}
	}
	if _, has := varsWithFullName["full_name"]; !has {
		varsWithFullName["full_name"] = fullName
	}

	message := templateText
	for k, v := range varsWithFullName {
		message = strings.ReplaceAll(message, "{"+k+"}", v)
	}

	client, err := getWAClient(targetAdmin)
	if err != nil || client == nil || !client.IsConnected() {
		return
	}

	// Clean phone number (remove +, spaces, leading zeros)
	cleanPhone := strings.TrimLeft(phone, "+")
	cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")

	targetJID := types.NewJID(cleanPhone, types.DefaultUserServer)
	go func() {
		_, err := client.SendMessage(context.Background(), targetJID, &waE2E.Message{
			Conversation: proto.String(message),
		})
		if err != nil {
			log.Printf("[whatsapp] Failed to send to %s: %v", phone, err)
		}
	}()
}

func GetWhatsappConfig(c *fiber.Ctx) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[whatsapp-panic] GetWhatsappConfig: %v", r)
			_ = c.Status(500).JSON(fiber.Map{"error": "Internal server crash in WhatsApp module"})
		}
	}()

	adminID, _ := c.Locals("admin_id").(int64)
	config, err := loadWhatsappConfig(adminID)
	if err != nil {
		log.Printf("[whatsapp] GetWhatsappConfig error for admin %d: %v", adminID, err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to load config: " + err.Error()})
	}

	status := "disconnected"

	waMu.Lock()
	client := waClients[adminID]
	waMu.Unlock()
	if client != nil && client.IsConnected() && client.IsLoggedIn() {
		status = "connected"
	}

	return c.JSON(fiber.Map{
		"enabled":          config.Enabled,
		"phone_number":     config.PhoneNumber,
		"reminder_enabled": config.ReminderEnabled,
		"reminder_hours":   config.ReminderHours,
		"status":           status,
	})
}

func SaveWhatsappConfig(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(int64)
	type Request struct {
		Enabled         int    `json:"enabled"`
		PhoneNumber     string `json:"phone_number"`
		ReminderEnabled int    `json:"reminder_enabled"`
		ReminderHours   int    `json:"reminder_hours"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	initWAStore()

	if DB == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Database connection not ready"})
	}

	_, err := DB.Exec(
		`INSERT INTO radius_whatsapp_config (admin_id, enabled, phone_number, reminder_enabled, reminder_hours, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(admin_id) DO UPDATE SET 
		 	enabled=excluded.enabled, 
		 	phone_number=excluded.phone_number, 
		 	reminder_enabled=excluded.reminder_enabled,
		 	reminder_hours=excluded.reminder_hours,
		 	updated_at=CURRENT_TIMESTAMP`,
		adminID, req.Enabled, req.PhoneNumber, req.ReminderEnabled, req.ReminderHours,
	)
	if err != nil {
		log.Printf("[whatsapp] SaveWhatsappConfig error for admin %d: %v", adminID, err)
		return c.Status(500).JSON(fiber.Map{"error": "Database error: " + err.Error()})
	}

	return c.JSON(fiber.Map{"message": "تم حفظ إعدادات الواتساب بنجاح"})
}

func GetMessageTemplates(c *fiber.Ctx) error {
	if DB == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Database connection not ready"})
	}
	rows, err := DB.Query("SELECT id, _template_key, template_text, updated_at FROM radius_message_templates ORDER BY _template_key")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query templates: " + err.Error()})
	}
	defer rows.Close()

	templates := make([]MessageTemplate, 0)
	for rows.Next() {
		var t MessageTemplate
		if err := rows.Scan(&t.ID, &t.TemplateKey, &t.TemplateText, &t.UpdatedAt); err != nil {
			continue
		}
		templates = append(templates, t)
	}
	return c.JSON(templates)
}

func SaveMessageTemplate(c *fiber.Ctx) error {
	type Request struct {
		TemplateKey  string `json:"template_key"`
		TemplateText string `json:"template_text"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	if req.TemplateKey == "" || req.TemplateText == "" {
		return c.Status(400).JSON(fiber.Map{"error": "مفتاح القالب والنص مطلوبان"})
	}

	if DB == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Database connection not ready"})
	}

	_, err := DB.Exec(
		`INSERT INTO radius_message_templates (_template_key, template_text, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(_template_key) DO UPDATE SET
		 	template_text=excluded.template_text,
		 	updated_at=CURRENT_TIMESTAMP`,
		req.TemplateKey, req.TemplateText,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save template: " + err.Error()})
	}

	return c.JSON(fiber.Map{"message": "تم حفظ القالب بنجاح"})
}

func seedDefaultTemplates() {
	defaults := map[string]string{
		"renew_paid":      "تم تجديد اشتراكك بنجاح ✅\nالمستخدم: {username}\nالباقة: {profile}\nالسعر: {price} د.ع\nالمدة: {validity_days} يوم\nحالة الدفع: مدفوع",
		"renew_debt":      "تم تجديد اشتراكك ⏳\nالمستخدم: {username}\nالباقة: {profile}\nالسعر: {price} د.ع\nالمدة: {validity_days} يوم\nملاحظة: تمت إضافة المبلغ كديون\nرصيدك الحالي: {balance} د.ع",
		"add_debt":        "تم إضافة ديون 📋\nالمستخدم: {username}\nالمبلغ: {amount} د.ع\nالملاحظات: {notes}\nرصيدك الحالي: {balance} د.ع",
		"payment":         "تم تسديد ديون ✅\nالمستخدم: {username}\nالمبلغ: {amount} د.ع\nالملاحظات: {notes}\nرصيدك الحالي: {balance} د.ع",
		"expiry_reminder": "تنبيه انتهاء الاشتراك ⚠️\nعزيزي {full_name}، نود إعلامك أن اشتراكك في باقة {profile} سينتهي قريباً.\nتاريخ الانتهاء: {expiry_date}\nيرجى التجديد لضمان استمرار الخدمة.",
		"debt_reminder":   "تذكير بالديون المستحقة 📋\nعزيزي {full_name}، نود تذكيرك بأن لديك ديوناً مستحقة بمبلغ {balance} د.ع.\nيرجى التواصل مع الوكيل لتسوية الحساب في أقرب وقت ممكن.\nشكراً لتعاملكم معنا 🙏",
	}

	for key, text := range defaults {
		_, _ = DB.Exec("INSERT OR IGNORE INTO radius_message_templates (_template_key, template_text) VALUES (?, ?)", key, text)
	}
}

func TestWhatsappNotification(c *fiber.Ctx) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[whatsapp-panic] TestWhatsappNotification: %v", r)
			_ = c.Status(500).JSON(fiber.Map{"error": "Internal server crash during WhatsApp test"})
		}
	}()

	adminID, _ := c.Locals("admin_id").(int64)
	type Request struct {
		Phone   string `json:"phone"`
		Message string `json:"message"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	req.Phone = strings.TrimSpace(req.Phone)
	req.Message = strings.TrimSpace(req.Message)
	if req.Phone == "" || req.Message == "" {
		return c.Status(400).JSON(fiber.Map{"error": "رقم الهاتف ونص الرسالة مطلوبان"})
	}

	client, err := getWAClient(adminID)
	if err != nil {
		log.Printf("[whatsapp] Test getWAClient error for admin %d: %v", adminID, err)
		return c.Status(500).JSON(fiber.Map{"error": "فشل فتح جلسة الواتساب: " + err.Error()})
	}
	if client == nil || !client.IsConnected() || !client.IsLoggedIn() {
		return c.Status(400).JSON(fiber.Map{"error": "الواتساب غير متصل. يرجى مسح كود QR أولاً."})
	}

	cleanPhone := strings.TrimLeft(req.Phone, "+")
	cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
	targetJID := types.NewJID(cleanPhone, types.DefaultUserServer)

	_, err = client.SendMessage(context.Background(), targetJID, &waE2E.Message{
		Conversation: proto.String(req.Message),
	})
	if err != nil {
		log.Printf("[whatsapp] Test send failed for admin %d to %s: %v", adminID, req.Phone, err)
		return c.Status(500).JSON(fiber.Map{"error": "فشل إرسال الرسالة: " + err.Error()})
	}

	return c.JSON(fiber.Map{"message": "تم إرسال رسالة الاختبار"})
}

func BroadcastWhatsappMessage(c *fiber.Ctx) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[whatsapp-panic] BroadcastWhatsappMessage: %v", r)
			_ = c.Status(500).JSON(fiber.Map{"error": "Internal server crash during WhatsApp broadcast"})
		}
	}()

	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	type Request struct {
		Message string `json:"message"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		return c.Status(400).JSON(fiber.Map{"error": "نص الرسالة مطلوب"})
	}

	client, err := getWAClient(adminID)
	if err != nil {
		log.Printf("[whatsapp] Broadcast getWAClient error for admin %d: %v", adminID, err)
		return c.Status(500).JSON(fiber.Map{"error": "فشل فتح جلسة الواتساب: " + err.Error()})
	}
	if client == nil || !client.IsConnected() || !client.IsLoggedIn() {
		return c.Status(400).JSON(fiber.Map{"error": "الواتساب غير متصل. يرجى مسح كود QR أولاً."})
	}

	// Fetch target phone numbers
	var rows *sql.Rows
	if role == "superadmin" {
		rows, err = DB.Query("SELECT DISTINCT phone FROM radius_user_meta WHERE phone IS NOT NULL AND phone != ''")
	} else {
		rows, err = DB.Query("SELECT DISTINCT phone FROM radius_user_meta WHERE admin_id = ? AND phone IS NOT NULL AND phone != ''", adminID)
	}

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل جلب أرقام الهواتف: " + err.Error()})
	}
	defer rows.Close()

	var phones []string
	for rows.Next() {
		var phone string
		if err := rows.Scan(&phone); err == nil {
			phone = strings.TrimSpace(phone)
			if phone != "" {
				phones = append(phones, phone)
			}
		}
	}

	if len(phones) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "لا يوجد مستخدمين مسجلين بأرقام هواتف صالحة لإرسال الرسائل إليهم"})
	}

	// Send in background to prevent HTTP timeout
	go func(targetPhones []string, msg string, waClient *whatsmeow.Client, currentAdminID int64) {
		log.Printf("[whatsapp-broadcast] Starting broadcast of %d messages for admin %d", len(targetPhones), currentAdminID)
		for i, phone := range targetPhones {
			// Clean phone number (remove +, spaces, leading zeros)
			cleanPhone := strings.TrimLeft(phone, "+")
			cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
			if cleanPhone == "" {
				continue
			}

			targetJID := types.NewJID(cleanPhone, types.DefaultUserServer)
			_, err := waClient.SendMessage(context.Background(), targetJID, &waE2E.Message{
				Conversation: proto.String(msg),
			})
			if err != nil {
				log.Printf("[whatsapp-broadcast] Failed to send to %s (index: %d): %v", phone, i, err)
			} else {
				log.Printf("[whatsapp-broadcast] Message sent to %s successfully (%d/%d)", phone, i+1, len(targetPhones))
			}

			// Delay to avoid spam filters
			time.Sleep(1500 * time.Millisecond)
		}
		log.Printf("[whatsapp-broadcast] Finished broadcast of %d messages for admin %d", len(targetPhones), currentAdminID)
	}(phones, req.Message, client, adminID)

	return c.JSON(fiber.Map{"message": fmt.Sprintf("بدأ إرسال الرسالة إلى %d مستخدم في الخلفية تلافياً للحظر.", len(phones))})
}


// SendDebtReminderBulk sends WhatsApp reminders to all users with outstanding debts (balance > 0)
func SendDebtReminderBulk(c *fiber.Ctx) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[whatsapp-panic] SendDebtReminderBulk: %v", r)
			_ = c.Status(500).JSON(fiber.Map{"error": "Internal server crash during debt reminder"})
		}
	}()

	adminID, _ := c.Locals("admin_id").(int64)
	role, _ := c.Locals("role").(string)

	client, err := getWAClient(adminID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل فتح جلسة الواتساب: " + err.Error()})
	}
	if client == nil || !client.IsConnected() || !client.IsLoggedIn() {
		return c.Status(400).JSON(fiber.Map{"error": "الواتساب غير متصل. يرجى مسح كود QR أولاً."})
	}

	templateText := loadMessageTemplate("debt_reminder")
	if templateText == "" {
		return c.Status(500).JSON(fiber.Map{"error": "قالب تذكير الديون غير موجود"})
	}

	// Fetch all users with outstanding debt (balance > 0 means they owe money)
	var rows *sql.Rows
	if role == "superadmin" {
		rows, err = DB.Query(
			"SELECT username, COALESCE(phone,''), COALESCE(full_name,''), balance FROM radius_user_meta WHERE balance > 0 AND phone IS NOT NULL AND phone != ''")
	} else {
		rows, err = DB.Query(
			"SELECT username, COALESCE(phone,''), COALESCE(full_name,''), balance FROM radius_user_meta WHERE admin_id = ? AND balance > 0 AND phone IS NOT NULL AND phone != ''",
			adminID)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل جلب المستخدمين: " + err.Error()})
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
		if scanErr := rows.Scan(&u.Username, &u.Phone, &u.FullName, &u.Balance); scanErr == nil {
			debtUsers = append(debtUsers, u)
		}
	}

	if len(debtUsers) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "لا يوجد مستخدمون عليهم ديون أو لا تتوفر أرقام هواتفهم"})
	}

	// Send reminders in background
	go func(users []debtUser, tmpl string, waClient *whatsmeow.Client) {
		log.Printf("[whatsapp-debt-reminder] Starting debt reminders for %d users (admin %d)", len(users), adminID)
		for i, u := range users {
			msg := tmpl
			msg = strings.ReplaceAll(msg, "{username}", u.Username)
			msg = strings.ReplaceAll(msg, "{full_name}", u.FullName)
			msg = strings.ReplaceAll(msg, "{balance}", fmt.Sprintf("%.0f", u.Balance))

			cleanPhone := strings.TrimLeft(u.Phone, "+")
			cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
			if cleanPhone == "" {
				continue
			}

			targetJID := types.NewJID(cleanPhone, types.DefaultUserServer)
			_, sendErr := waClient.SendMessage(context.Background(), targetJID, &waE2E.Message{
				Conversation: proto.String(msg),
			})
			if sendErr != nil {
				log.Printf("[whatsapp-debt-reminder] Failed to send to %s: %v", u.Phone, sendErr)
			} else {
				log.Printf("[whatsapp-debt-reminder] Reminder sent to %s (%d/%d)", u.Username, i+1, len(users))
			}
			time.Sleep(1500 * time.Millisecond)
		}
		log.Printf("[whatsapp-debt-reminder] Finished sending debt reminders")
	}(debtUsers, templateText, client)

	return c.JSON(fiber.Map{"message": fmt.Sprintf("بدأ إرسال تذكير الديون إلى %d مستخدم في الخلفية.", len(debtUsers))})
}

func clearLocalWASession(adminID int64) {
	initWAStore()

	waMu.Lock()
	container := waContainer
	waMu.Unlock()

	if container == nil {
		log.Printf("[whatsapp] Cannot clear local session for admin %d: WA store is not initialized", adminID)
		return
	}

	// Find and delete only this admin's device (not the first/shared device)
	var adminDeviceJID string
	_ = DB.QueryRow("SELECT COALESCE(device_jid,'') FROM radius_whatsapp_config WHERE admin_id = ?", adminID).Scan(&adminDeviceJID)

	if adminDeviceJID != "" {
		if jid, jidErr := types.ParseJID(adminDeviceJID); jidErr == nil {
			if allDevices, devErr := container.GetAllDevices(context.Background()); devErr == nil {
				for _, device := range allDevices {
					if device.ID != nil && device.ID.String() == jid.String() {
						if delErr := device.Delete(context.Background()); delErr != nil {
							log.Printf("[whatsapp] Failed to delete local session for admin %d: %v", adminID, delErr)
						}
						break
					}
				}
			}
		}
	}

	// Always clear the stored JID from the config
	_, _ = DB.Exec("UPDATE radius_whatsapp_config SET device_jid = '' WHERE admin_id = ?", adminID)
}

func GetWhatsappQR(c *fiber.Ctx) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[whatsapp-panic] GetWhatsappQR: %v", r)
			_ = c.Status(500).JSON(fiber.Map{"error": "Internal server crash during QR generation"})
		}
	}()

	adminID, _ := c.Locals("admin_id").(int64)
	log.Printf("[whatsapp] Generating QR for admin %d", adminID)

	client, err := getWAClient(adminID)
	if err != nil {
		log.Printf("[whatsapp] getWAClient error for admin %d: %v", adminID, err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get client: " + err.Error()})
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
				time.Sleep(500 * time.Millisecond)
			}
			if client.IsConnected() && client.IsLoggedIn() {
				return c.JSON(fiber.Map{"status": "connected", "message": "تم الاتصال تلقائياً"})
			}
		}

		log.Printf("[whatsapp] Stored session for admin %d is not authenticated, clearing it before QR pairing", adminID)
		client.Disconnect()
		if deleteErr := client.Store.Delete(context.Background()); deleteErr != nil {
			log.Printf("[whatsapp] Failed to delete unauthenticated session for admin %d: %v", adminID, deleteErr)
		}
		// Clear stored device JID since it's now invalid
		_, _ = DB.Exec("UPDATE radius_whatsapp_config SET device_jid = '' WHERE admin_id = ?", adminID)
		waMu.Lock()
		delete(waClients, adminID)
		waMu.Unlock()

		client, err = getWAClient(adminID)
		if err != nil {
			log.Printf("[whatsapp] getWAClient after reset error for admin %d: %v", adminID, err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to reset WhatsApp session: " + err.Error()})
		}
	}

	qrChan, err := client.GetQRChannel(context.Background())
	if err != nil {
		log.Printf("[whatsapp] GetQRChannel error for admin %d: %v", adminID, err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to start QR pairing: " + err.Error()})
	}

	// If already connecting, Connect() might return an error we can ignore or handle
	_ = client.Connect()

	select {
	case evt := <-qrChan:
		switch evt.Event {
		case "code":
			png, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to generate QR image"})
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
				time.Sleep(500 * time.Millisecond)
			}
			log.Printf("[whatsapp] QR success event for admin %d received before authenticated login", adminID)
			return c.JSON(fiber.Map{"status": "waiting", "message": "تم استلام حدث الربط، بانتظار اكتمال تسجيل الدخول"})
		}
	case <-time.After(15 * time.Second):
		return c.Status(408).JSON(fiber.Map{"error": "انتهت مهلة انتظار كود QR. يرجى المحاولة مرة أخرى."})
	}

	return c.JSON(fiber.Map{"status": "waiting"})
}

func StartReminderWorker() {
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			processReminders()
		}
	}()
}

func processReminders() {
	if DB == nil {
		return
	}

	// 1. Get all admins with reminders enabled
	rows, err := DB.Query("SELECT admin_id, reminder_hours FROM radius_whatsapp_config WHERE enabled = 1 AND reminder_enabled = 1")
	if err != nil {
		log.Printf("[whatsapp-reminders] Failed to query configs: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var adminID int64
		var reminderHours int
		if err := rows.Scan(&adminID, &reminderHours); err != nil {
			continue
		}

		// 2. Find users for this admin that are expiring soon and haven't been reminded for this cycle
		now := currentBaghdadTime().Unix()
		targetTime := now + int64(reminderHours*3600)

		uRows, err := DB.Query(`
			SELECT m.username, m.expiration_unix, m.phone, m.full_name, COALESCE(g.groupname, 'بدون باقة')
			FROM radius_user_meta m
			LEFT JOIN radusergroup g ON m.username = g.username
			WHERE m.admin_id = ? 
			  AND m.expiration_unix > ? 
			  AND m.expiration_unix <= ? 
			  AND (m.reminder_sent_for IS NULL OR m.reminder_sent_for != m.expiration_unix)
			  AND m.phone != ''`, adminID, now, targetTime)

		if err != nil {
			log.Printf("[whatsapp-reminders] Failed to query users for admin %d: %v", adminID, err)
			continue
		}

		for uRows.Next() {
			var username, phone, fullName, profile string
			var expirationUnix int64
			if err := uRows.Scan(&username, &expirationUnix, &phone, &fullName, &profile); err != nil {
				continue
			}

			// Send Reminder
			expiryDate := time.Unix(expirationUnix, 0).In(baghdadLocation).Format("2006-01-02 15:04")
			
			log.Printf("[whatsapp-reminders] Sending reminder to %s (expiring at %s)", username, expiryDate)

			SendWhatsappNotification(username, "expiry_reminder", map[string]string{
				"username":    username,
				"full_name":   fullName,
				"profile":     profile,
				"expiry_date": expiryDate,
			})

			// Mark as sent for this expiration date
			_, _ = DB.Exec("UPDATE radius_user_meta SET last_reminder_at = CURRENT_TIMESTAMP, reminder_sent_for = ? WHERE username = ?", expirationUnix, username)
		}
		uRows.Close()
	}
}

func LogoutWhatsapp(c *fiber.Ctx) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[whatsapp-panic] LogoutWhatsapp: %v", r)
			_ = c.Status(500).JSON(fiber.Map{"error": "Internal server crash during WhatsApp logout"})
		}
	}()

	adminID, _ := c.Locals("admin_id").(int64)

	waMu.Lock()
	client, ok := waClients[adminID]
	waMu.Unlock()

	if ok && client != nil {
		err := client.Logout(context.Background())
		if err != nil {
			log.Printf("[whatsapp] Logout request failed for admin %d, clearing local session anyway: %v", adminID, err)
			client.Disconnect()
			if client.Store != nil && client.Store.ID != nil {
				if deleteErr := client.Store.Delete(context.Background()); deleteErr != nil {
					log.Printf("[whatsapp] Failed to delete local session for admin %d: %v", adminID, deleteErr)
				}
			}
		}
		waMu.Lock()
		delete(waClients, adminID)
		waMu.Unlock()
	}
	clearLocalWASession(adminID)

	return c.JSON(fiber.Map{"message": "تم تسجيل الخروج بنجاح"})
}
