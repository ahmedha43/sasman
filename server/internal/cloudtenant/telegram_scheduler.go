package cloudtenant

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TenantTelegramBackupConfig struct {
	Enabled       bool
	BotToken      string
	ChatID        string
	IntervalHours int
	LastSentAt    int64
}

var tgBackupMu sync.Mutex

func (m *Manager) StartTenantTelegramBackupScheduler() {
	go func() {
		time.Sleep(45 * time.Second)
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()

		for {
			<-ticker.C
			m.runTenantTelegramBackupsIfDue()
		}
	}()
}

func (m *Manager) runTenantTelegramBackupsIfDue() {
	tenantsDir := m.pool.baseDir
	entries, err := os.ReadDir(tenantsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subdomain := entry.Name()
		db, err := m.pool.Get(subdomain)
		if err != nil {
			continue
		}

		cfg := loadTenantTelegramConfig(db)
		if !cfg.Enabled || cfg.BotToken == "" || cfg.ChatID == "" || cfg.IntervalHours <= 0 {
			continue
		}

		now := time.Now().Unix()
		intervalSec := int64(cfg.IntervalHours) * 3600
		if cfg.LastSentAt > 0 && now-cfg.LastSentAt < intervalSec {
			continue
		}

		caption := fmt.Sprintf("نسخة احتياطية تلقائية من SASMAN Cloud\nالوكيل: %s\nالتاريخ: %s", subdomain, time.Now().Format("2006-01-02 15:04:05"))
		if err := m.SendTenantTelegramBackup(subdomain, cfg, caption); err != nil {
			log.Printf("[telegram-backup] Scheduled backup for tenant [%s] failed: %v", subdomain, err)
			continue
		}

		_ = setTenantSetting(db, "telegram_backup_last_sent_at", strconv.FormatInt(now, 10))
		log.Printf("[telegram-backup] ✅ Scheduled backup for tenant [%s] sent to Telegram successfully", subdomain)
	}
}

func (m *Manager) SendTenantTelegramBackup(subdomain string, cfg TenantTelegramBackupConfig, caption string) error {
	tgBackupMu.Lock()
	defer tgBackupMu.Unlock()

	dbPath := m.pool.GetTenantDBPath(subdomain)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("tenant database file does not exist")
	}

	tmpBackup := filepath.Join(os.TempDir(), fmt.Sprintf("sasman_%s_backup_%d.db", subdomain, time.Now().UnixNano()))
	defer os.Remove(tmpBackup)

	db, err := m.pool.Get(subdomain)
	if err == nil && db != nil {
		_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		_, _ = db.Exec(fmt.Sprintf("VACUUM INTO '%s'", tmpBackup))
	}

	if _, err := os.Stat(tmpBackup); os.IsNotExist(err) {
		data, readErr := os.ReadFile(dbPath)
		if readErr != nil {
			return fmt.Errorf("failed to read database: %w", readErr)
		}
		if writeErr := os.WriteFile(tmpBackup, data, 0644); writeErr != nil {
			return fmt.Errorf("failed to create backup file: %w", writeErr)
		}
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("chat_id", cfg.ChatID); err != nil {
		return err
	}
	if err := writer.WriteField("caption", caption); err != nil {
		return err
	}

	filename := fmt.Sprintf("sasman_%s_backup_%s.db", subdomain, time.Now().Format("2006-01-02_1504"))
	fileWriter, err := writer.CreateFormFile("document", filename)
	if err != nil {
		return err
	}

	file, err := os.Open(tmpBackup)
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

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", cfg.BotToken)
	req, err := http.NewRequest(http.MethodPost, apiURL, &body)
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

func loadTenantTelegramConfig(db *sql.DB) TenantTelegramBackupConfig {
	cfg := TenantTelegramBackupConfig{
		Enabled:       false,
		IntervalHours: 24,
	}
	if db == nil {
		return cfg
	}

	var enabledStr, token, chatID, intervalStr, lastSentStr string
	_ = db.QueryRow("SELECT value FROM radius_settings WHERE key = 'telegram_backup_enabled'").Scan(&enabledStr)
	_ = db.QueryRow("SELECT value FROM radius_settings WHERE key = 'telegram_backup_bot_token'").Scan(&token)
	_ = db.QueryRow("SELECT value FROM radius_settings WHERE key = 'telegram_backup_chat_id'").Scan(&chatID)
	_ = db.QueryRow("SELECT value FROM radius_settings WHERE key = 'telegram_backup_interval_hours'").Scan(&intervalStr)
	_ = db.QueryRow("SELECT value FROM radius_settings WHERE key = 'telegram_backup_last_sent_at'").Scan(&lastSentStr)

	cfg.Enabled = (enabledStr == "true" || enabledStr == "1")
	cfg.BotToken = token
	cfg.ChatID = chatID
	if n, err := strconv.Atoi(intervalStr); err == nil && n > 0 {
		cfg.IntervalHours = n
	}
	if t, err := strconv.ParseInt(lastSentStr, 10, 64); err == nil {
		cfg.LastSentAt = t
	}
	return cfg
}

func setTenantSetting(db *sql.DB, key string, value string) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	_, err := db.Exec(`
		INSERT INTO radius_settings (key, value, updated_at) 
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}
