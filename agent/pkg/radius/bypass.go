package radius

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const bypassFlagFile = "/app/data/bypass_all.flag"

// GetBypassStatus returns whether global bypass is enabled
func GetBypassStatus(c *fiber.Ctx) error {
	enabled := isBypassEnabled()
	return c.JSON(fiber.Map{
		"enabled": enabled,
	})
}

// SetBypassStatus enables or disables global blind accept bypass
func SetBypassStatus(c *fiber.Ctx) error {
	type req struct {
		Enabled bool `json:"enabled"`
	}
	var body req
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}

	if err := setBypassEnabled(body.Enabled); err != nil {
		log.Printf("[bypass] Failed to set bypass status: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "فشل تحديث الإعداد"})
	}

	status := "معطل"
	if body.Enabled {
		status = "مفعّل"
	}
	log.Printf("[bypass] Global blind accept bypass %s", status)
	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("تم %s وضع البايپاس العمياء", status),
		"enabled": body.Enabled,
	})
}

// isBypassEnabled checks the bypass flag from both file and DB fallback
func isBypassEnabled() bool {
	// Check file flag first (for FreeRADIUS unlang to read)
	data, err := os.ReadFile(bypassFlagFile)
	if err == nil {
		return strings.TrimSpace(string(data)) == "1"
	}
	// Fallback to DB
	var val string
	err = DB.QueryRow("SELECT value FROM radius_system_settings WHERE `key`='bypass_all' LIMIT 1").Scan(&val)
	if err == nil {
		return strings.TrimSpace(val) == "1"
	}
	return false
}

// setBypassEnabled atomically updates bypass state in both file and DB
func setBypassEnabled(enabled bool) error {
	// Write file flag (for FreeRADIUS unlang)
	val := "0"
	if enabled {
		val = "1"
	}
	if err := os.WriteFile(bypassFlagFile, []byte(val), 0644); err != nil {
		return fmt.Errorf("failed to write bypass flag file: %w", err)
	}

	// Ensure DB table exists
	ensureSystemSettingsTable()

	// Update DB
	_, err := DB.Exec("INSERT INTO radius_system_settings (key, value, updated_at) "+
		"VALUES ('bypass_all', ?, CURRENT_TIMESTAMP) "+
		"ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP", val)
	if err != nil {
		return fmt.Errorf("failed to update bypass DB: %w", err)
	}

	return nil
}

// EnsureBypassMigration ensures the system settings table and bypass state exist
func EnsureBypassMigration() {
	ensureSystemSettingsTable()
}

func ensureSystemSettingsTable() {
	_, _ = DB.Exec("CREATE TABLE IF NOT EXISTS radius_system_settings (" +
		"key TEXT PRIMARY KEY, " +
		"value TEXT NOT NULL DEFAULT '', " +
		"updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)")
}

// EnsureBypassFlagFile creates the flag file if missing (called on startup)
func EnsureBypassFlagFile() {
	if _, err := os.Stat(bypassFlagFile); os.IsNotExist(err) {
		_ = os.WriteFile(bypassFlagFile, []byte("0"), 0644)
	}
}
