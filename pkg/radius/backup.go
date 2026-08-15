package radius

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

var telegramBackupMu sync.Mutex

type telegramBackupConfig struct {
	Enabled       bool   `json:"enabled"`
	BotEnabled    bool   `json:"bot_enabled"`
	BotToken      string `json:"bot_token,omitempty"`
	ChatID        string `json:"chat_id"`
	IntervalHours int    `json:"interval_hours"`
	LastSentAt    int64  `json:"last_sent_at"`
	Configured    bool   `json:"configured"`
}

func BackupDatabaseHandler(c *fiber.Ctx) error {
	if _, ok := c.Locals("admin_id").(int64); !ok {
		return c.Status(401).JSON(fiber.Map{"error": "غير مسجل"})
	}

	tmp, filename, err := CreateDatabaseBackupFile()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل النسخ الاحتياطي لقاعدة البيانات: " + err.Error()})
	}
	defer os.Remove(tmp)

	data, err := os.ReadFile(tmp)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	LogActivityFromCtx(c, "نسخ احتياطي", "قاعدة البيانات", fmt.Sprintf("تم تحميل نسخة احتياطية من قاعدة البيانات (%s)", filename))
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

func CreateDatabaseBackupFile() (string, string, error) {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("sasman_backup_%d.sqlite", time.Now().UnixNano()))
	filename := fmt.Sprintf("sasman_sqlite_%s.db", time.Now().Format("2006-01-02_1504"))

	if DB == nil {
		return "", "", fmt.Errorf("SQLite connection is not initialized")
	}

	// WAL mode keeps recent writes in sasman.db-wal. VACUUM INTO asks SQLite
	// itself to write a complete and consistent database backup.
	checkpointSQLiteWAL(false)
	if _, err := DB.Exec("VACUUM main INTO " + sqliteQuote(tmp)); err != nil {
		return "", "", fmt.Errorf("failed to create consistent SQLite backup: %w", err)
	}
	if err := pruneBackupFile(tmp); err != nil {
		log.Printf("[backup] Warning: Failed to prune sessions from backup: %v", err)
	}
	if err := validateSQLiteDatabase(tmp); err != nil {
		_ = os.Remove(tmp)
		return "", "", err
	}
	return tmp, filename, nil
}

func RestoreDatabaseHandler(c *fiber.Ctx) error {
	if _, ok := c.Locals("admin_id").(int64); !ok {
		return c.Status(401).JSON(fiber.Map{"error": "غير مسجل"})
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "لم يتم رفع أي ملف"})
	}

	dbPath := sqliteDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل تجهيز مسار قاعدة البيانات: " + err.Error()})
	}

	tempRestoreFile := filepath.Join(filepath.Dir(dbPath), fmt.Sprintf(".restore_%d.db", time.Now().UnixNano()))
	if err := c.SaveFile(fh, tempRestoreFile); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل حفظ الملف: " + err.Error()})
	}

	log.Println("[restore] Starting SQLite restore process...")

	defer os.Remove(tempRestoreFile)

	restoreDBFile, cleanup, err := normalizeRestoreFile(tempRestoreFile)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "ملف النسخة غير صالح: " + err.Error()})
	}
	if err := validateSQLiteDatabase(restoreDBFile); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "ملف النسخة غير صالح: " + err.Error()})
	}
	if err := replaceSQLiteDatabase(dbPath, restoreDBFile); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل استبدال قاعدة البيانات: " + err.Error()})
	}

	LogActivityFromCtx(c, "استعادة نسخة احتياطية", "قاعدة البيانات", fmt.Sprintf("تم استعادة قاعدة البيانات من الملف %s", fh.Filename))

	log.Println("[restore] Database restore completed. Restarting service...")
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()

	return c.JSON(fiber.Map{"message": "تمت استعادة القاعدة بنجاح. سيقوم النظام بإعادة التشغيل الآن لتطبيق التغييرات."})
}

func GetTelegramBackupConfig(c *fiber.Ctx) error {
	cfg := loadTelegramBackupConfig()
	cfg.BotToken = maskTelegramToken(cfg.BotToken)
	return c.JSON(cfg)
}

func SaveTelegramBackupConfig(c *fiber.Ctx) error {
	type request struct {
		Enabled       bool   `json:"enabled"`
		BotEnabled    bool   `json:"bot_enabled"`
		BotToken      string `json:"bot_token"`
		ChatID        string `json:"chat_id"`
		IntervalHours int    `json:"interval_hours"`
	}

	var req request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}

	if req.IntervalHours != 1 && req.IntervalHours != 12 && req.IntervalHours != 24 {
		return c.Status(400).JSON(fiber.Map{"error": "اختر فترة صحيحة: 1 أو 12 أو 24 ساعة"})
	}

	current := loadTelegramBackupConfig()
	token := strings.TrimSpace(req.BotToken)
	if token == "" {
		token = current.BotToken
	}
	chatID := strings.TrimSpace(req.ChatID)
	if chatID == "" {
		chatID = current.ChatID
	}

	if (req.Enabled || req.BotEnabled) && (token == "" || chatID == "") {
		return c.Status(400).JSON(fiber.Map{"error": "توكن البوت و Chat ID مطلوبان عند التفعيل"})
	}

	ensureSystemSettingsTable()
	settings := map[string]string{
		"telegram_backup_enabled":        boolString(req.Enabled),
		"telegram_bot_enabled":           boolString(req.BotEnabled),
		"telegram_backup_chat_id":        chatID,
		"telegram_backup_interval_hours": strconv.Itoa(req.IntervalHours),
	}
	if token != "" && !strings.Contains(token, "***") {
		settings["telegram_backup_bot_token"] = token
	}

	for key, value := range settings {
		if err := setSystemSetting(key, value); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل حفظ الإعدادات: " + err.Error()})
		}
	}

	return c.JSON(fiber.Map{"message": "تم حفظ إعدادات النسخ التلقائي إلى تيليگرام"})
}

func TestTelegramBackup(c *fiber.Ctx) error {
	cfg := loadTelegramBackupConfig()
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "أدخل توكن البوت و Chat ID أولاً"})
	}

	if err := sendTelegramDatabaseBackup(cfg, "نسخة اختبارية من SASMAN"); err != nil {
		log.Printf("[telegram-backup] Manual test failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "فشل إرسال النسخة: " + err.Error()})
	}

	_ = setSystemSetting("telegram_backup_last_sent_at", strconv.FormatInt(time.Now().Unix(), 10))
	return c.JSON(fiber.Map{"message": "تم إرسال نسخة احتياطية إلى تيليگرام بنجاح"})
}

func StartTelegramBackupScheduler() {
	go func() {
		time.Sleep(30 * time.Second)
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			runTelegramBackupIfDue()
			<-ticker.C
		}
	}()
}

func runTelegramBackupIfDue() {
	cfg := loadTelegramBackupConfig()
	if !cfg.Enabled || cfg.BotToken == "" || cfg.ChatID == "" || cfg.IntervalHours <= 0 {
		return
	}

	now := time.Now().Unix()
	interval := int64(cfg.IntervalHours) * 3600
	if cfg.LastSentAt > 0 && now-cfg.LastSentAt < interval {
		return
	}

	if err := sendTelegramDatabaseBackup(cfg, "نسخة احتياطية تلقائية من SASMAN"); err != nil {
		log.Printf("[telegram-backup] Scheduled backup failed: %v", err)
		return
	}

	_ = setSystemSetting("telegram_backup_last_sent_at", strconv.FormatInt(now, 10))
	log.Printf("[telegram-backup] Scheduled backup sent successfully")
}

func sendTelegramDatabaseBackup(cfg telegramBackupConfig, caption string) error {
	telegramBackupMu.Lock()
	defer telegramBackupMu.Unlock()

	tmp, filename, err := CreateDatabaseBackupFile()
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("chat_id", cfg.ChatID); err != nil {
		return err
	}
	if err := writer.WriteField("caption", fmt.Sprintf("%s\n%s", caption, time.Now().Format("2006-01-02 15:04:05"))); err != nil {
		return err
	}

	fileWriter, err := writer.CreateFormFile("document", filepath.Base(filename))
	if err != nil {
		return err
	}

	file, err := os.Open(tmp)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(fileWriter, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", cfg.BotToken)
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 90 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Telegram API returned %s: %s", res.Status, strings.TrimSpace(string(respBody)))
	}

	return nil
}

func loadTelegramBackupConfig() telegramBackupConfig {
	ensureSystemSettingsTable()
	interval, _ := strconv.Atoi(getSystemSetting("telegram_backup_interval_hours", "24"))
	if interval != 1 && interval != 12 && interval != 24 {
		interval = 24
	}
	lastSentAt, _ := strconv.ParseInt(getSystemSetting("telegram_backup_last_sent_at", "0"), 10, 64)
	token := getSystemSetting("telegram_backup_bot_token", "")
	chatID := getSystemSetting("telegram_backup_chat_id", "")

	return telegramBackupConfig{
		Enabled:       getSystemSetting("telegram_backup_enabled", "1") == "1",
		BotEnabled:    getSystemSetting("telegram_bot_enabled", "1") == "1",
		BotToken:      token,
		ChatID:        chatID,
		IntervalHours: interval,
		LastSentAt:    lastSentAt,
		Configured:    token != "" && chatID != "",
	}
}

func setSystemSetting(key, value string) error {
	_, err := DB.Exec("INSERT INTO radius_system_settings (key, value, updated_at) "+
		"VALUES (?, ?, CURRENT_TIMESTAMP) "+
		"ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP", key, value)
	return err
}

func getSystemSetting(key, fallback string) string {
	var value string
	err := DB.QueryRow("SELECT value FROM radius_system_settings WHERE key=? LIMIT 1", key).Scan(&value)
	if err != nil {
		return fallback
	}
	return value
}

func boolString(enabled bool) string {
	if enabled {
		return "1"
	}
	return "0"
}

func maskTelegramToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 10 {
		return "***"
	}
	return token[:6] + "***" + token[len(token)-4:]
}

func sqliteDBPath() string {
	if dbPath := os.Getenv("SQLITE_DB_PATH"); dbPath != "" {
		return dbPath
	}
	if _, err := os.Stat("/app/data/sasman.db"); err == nil {
		return "/app/data/sasman.db"
	}
	return "data/sasman.db"
}

func sqliteQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "''") + "'"
}

func pruneBackupFile(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("DELETE FROM radacct; DELETE FROM radpostauth; VACUUM;")
	return err
}

func normalizeRestoreFile(path string) (string, func(), error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	if len(data) >= 16 && string(data[:16]) == "SQLite format 3\x00" {
		return path, nil, nil
	}

	probeLen := len(data)
	if probeLen > 4096 {
		probeLen = 4096
	}
	probe := string(data[:probeLen])
	probeUpper := strings.ToUpper(probe)
	
	isSQL := strings.Contains(probeUpper, "PRAGMA") ||
		strings.Contains(probeUpper, "BEGIN ") ||
		strings.Contains(probeUpper, "CREATE TABLE") ||
		strings.Contains(probeUpper, "INSERT INTO") ||
		strings.Contains(probeUpper, "--") ||
		strings.Contains(probeUpper, "/*")

	if !isSQL {
		return "", nil, fmt.Errorf("الملف المرفوع لا يبدو كملف قاعدة بيانات SQLite أو ملف SQL صالح")
	}

	imported := filepath.Join(os.TempDir(), fmt.Sprintf("sasman_restore_import_%d.sqlite", time.Now().UnixNano()))
	cmd := exec.Command("sqlite3", imported)
	cmd.Stdin = bytes.NewReader(data)
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(imported)
		return "", nil, fmt.Errorf("SQL dump import failed: %v %s", err, strings.TrimSpace(string(output)))
	}
	return imported, func() { _ = os.Remove(imported) }, nil
}

func validateSQLiteDatabase(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()

	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return err
	}
	if !strings.EqualFold(integrity, "ok") {
		return fmt.Errorf("integrity_check=%s", integrity)
	}

	requiredTables := []string{"radcheck", "radreply", "radusergroup", "radius_user_meta", "radius_admins", "nas"}
	for _, table := range requiredTables {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("missing table %s", table)
		}
	}
	return nil
}

func replaceSQLiteDatabase(dbPath, restoreDBFile string) error {
	if DB != nil {
		_ = DB.Close()
		DB = nil
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}

	backupPath := fmt.Sprintf("%s.before_restore_%d", dbPath, time.Now().Unix())
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Rename(dbPath, backupPath); err != nil {
			return err
		}
	}

	if err := copyFile(restoreDBFile, dbPath); err != nil {
		if _, statErr := os.Stat(backupPath); statErr == nil {
			_ = os.Rename(backupPath, dbPath)
		}
		return err
	}
	return nil
}

func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Sync()
}

func CleanOldTransactions() error {
	_, err := DB.Exec("DELETE FROM radius_user_transactions WHERE created_at < datetime('now', '-90 days')")
	return err
}

func GetDatabaseSizeHandler(c *fiber.Ctx) error {
	dbPath := "/app/data/sasman.db"
	if os.Getenv("SQLITE_DB_PATH") != "" {
		dbPath = os.Getenv("SQLITE_DB_PATH")
	}

	info, err := os.Stat(dbPath)
	if err != nil {
		return c.JSON(fiber.Map{"size": "0 MB"})
	}

	sizeMB := float64(info.Size()) / 1024 / 1024
	return c.JSON(fiber.Map{
		"size": fmt.Sprintf("%.2f MB", sizeMB),
	})
}
