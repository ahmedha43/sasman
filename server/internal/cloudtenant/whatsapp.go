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

	"github.com/gofiber/fiber/v2"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

var (
	cloudWAContainers = make(map[string]*sqlstore.Container)
	cloudWAClients    = make(map[string]*whatsmeow.Client)
	cloudWAMu         sync.Mutex
)

func getTenantWAClient(subdomain string, db *sql.DB, baseDir string) (*whatsmeow.Client, error) {
	cloudWAMu.Lock()
	client, ok := cloudWAClients[subdomain]
	cloudWAMu.Unlock()

	if ok && client != nil {
		return client, nil
	}

	cloudWAMu.Lock()
	defer cloudWAMu.Unlock()

	// Double check
	if client, ok := cloudWAClients[subdomain]; ok && client != nil {
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

	clientLog := waLog.Stdout("Client", "ERROR", true)

	var deviceJID string
	_ = db.QueryRow("SELECT COALESCE(device_jid, '') FROM radius_whatsapp_config WHERE admin_id = 1").Scan(&deviceJID)

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
	}

	if client == nil {
		client = whatsmeow.NewClient(container.NewDevice(), clientLog)
	}
	client.AddEventHandler(func(evt interface{}) {
		switch evt.(type) {
		case *events.Connected:
			log.Printf("[whatsapp] Tenant %s connected", subdomain)
			if client.Store.ID != nil {
				jidStr := client.Store.ID.String()
				_, _ = db.Exec(`
					INSERT INTO radius_whatsapp_config (admin_id, device_jid) VALUES (1, ?)
					ON CONFLICT(admin_id) DO UPDATE SET device_jid = excluded.device_jid
				`, jidStr)
			}
		case *events.LoggedOut:
			log.Printf("[whatsapp] Tenant %s logged out", subdomain)
			_, _ = db.Exec("UPDATE radius_whatsapp_config SET device_jid = '' WHERE admin_id = 1")
		}
	})

	cloudWAClients[subdomain] = client
	return client, nil
}

func (h *APIHandler) handleGetWhatsappConfig(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	subdomain, _ := c.Locals("subdomain").(string)

	var enabled, reminderEnabled, reminderHours int
	var phone string
	_ = db.QueryRow("SELECT enabled, COALESCE(phone_number, ''), reminder_enabled, reminder_hours FROM radius_whatsapp_config WHERE admin_id = 1").Scan(&enabled, &phone, &reminderEnabled, &reminderHours)

	status := "disconnected"
	cloudWAMu.Lock()
	client := cloudWAClients[subdomain]
	cloudWAMu.Unlock()

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
		VALUES (1, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(admin_id) DO UPDATE SET
			enabled = excluded.enabled,
			phone_number = excluded.phone_number,
			reminder_enabled = excluded.reminder_enabled,
			reminder_hours = excluded.reminder_hours,
			updated_at = CURRENT_TIMESTAMP
	`, req.Enabled, req.PhoneNumber, req.ReminderEnabled, req.ReminderHours)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true, "message": "تم حفظ إعدادات الواتساب بنجاح"})
}

func (h *APIHandler) handleGetWhatsappQR(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	subdomain, _ := c.Locals("subdomain").(string)

	client, err := getTenantWAClient(subdomain, db, ".")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل تجهيز خدمة الواتساب: " + err.Error()})
	}

	if client.IsConnected() && client.IsLoggedIn() {
		return c.JSON(fiber.Map{"status": "connected", "message": "متصل بالفعل"})
	}

	if client.Store.ID != nil {
		_ = client.Connect()
		for i := 0; i < 6; i++ {
			if client.IsConnected() && client.IsLoggedIn() {
				return c.JSON(fiber.Map{"status": "connected", "message": "تم الاتصال تلقائياً"})
			}
			time.Sleep(400 * time.Millisecond)
		}

		// Reset unauthenticated store session
		client.Disconnect()
		_ = client.Store.Delete(context.Background())
		_, _ = db.Exec("UPDATE radius_whatsapp_config SET device_jid = '' WHERE admin_id = 1")
		cloudWAMu.Lock()
		delete(cloudWAClients, subdomain)
		cloudWAMu.Unlock()

		client, err = getTenantWAClient(subdomain, db, ".")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل إعادة تعيين جلسة الواتساب: " + err.Error()})
		}
	}

	qrChan, err := client.GetQRChannel(context.Background())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "فشل بدء مسح كود QR: " + err.Error()})
	}

	_ = client.Connect()

	select {
	case evt := <-qrChan:
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
			return c.JSON(fiber.Map{"status": "connected", "message": "تم الربط بنجاح"})
		}
	case <-time.After(15 * time.Second):
		return c.Status(fiber.StatusRequestTimeout).JSON(fiber.Map{"error": "انتهت مهلة انتظار كود QR. يرجى المحاولة مرة أخرى."})
	}

	return c.JSON(fiber.Map{"status": "waiting"})
}

func (h *APIHandler) handleLogoutWhatsapp(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	subdomain, _ := c.Locals("subdomain").(string)

	cloudWAMu.Lock()
	client := cloudWAClients[subdomain]
	delete(cloudWAClients, subdomain)
	cloudWAMu.Unlock()

	if client != nil {
		client.Disconnect()
		_ = client.Store.Delete(context.Background())
	}

	_, _ = db.Exec("UPDATE radius_whatsapp_config SET device_jid = '' WHERE admin_id = 1")
	return c.JSON(fiber.Map{"success": true, "message": "تم تسجيل الخروج وفصل الواتساب بنجاح"})
}

func (h *APIHandler) handleTestWhatsapp(c *fiber.Ctx) error {
	db := c.Locals("tenant_db").(*sql.DB)
	subdomain, _ := c.Locals("subdomain").(string)

	var req struct {
		Phone   string `json:"phone"`
		Message string `json:"message"`
	}
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Phone) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "رقم الهاتف مطلوب"})
	}

	client, err := getTenantWAClient(subdomain, db, ".")
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
