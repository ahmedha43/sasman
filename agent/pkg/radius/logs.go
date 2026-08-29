package radius

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
)

func GetRadiusLogs(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "غير مصرح (للمدراء الأساسيين فقط)"})
	}
	file, err := os.Open("data/radius.log")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "تعذر قراءة السجل: " + err.Error()})
	}
	defer file.Close()

	// Read last 100KB for performance
	stat, _ := file.Stat()
	size := stat.Size()
	start := int64(0)
	if size > 100000 {
		start = size - 100000
	}

	buf := make([]byte, size-start)
	_, err = file.ReadAt(buf, start)
	if err != nil && err.Error() != "EOF" {
		return c.Status(500).JSON(fiber.Map{"error": "فشل في قراءة السجل"})
	}

	return c.SendString(string(buf))
}

func ClearRadiusLogs(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "غير مصرح (للمدراء الأساسيين فقط)"})
	}
	err := os.Truncate("data/radius.log", 0)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "تعذر تصفير السجل"})
	}
	return c.JSON(fiber.Map{"message": "تم تصفير السجل بنجاح"})
}

// WatchAndRotateLogs keeps the log file size under control (e.g., 5MB)
func WatchAndRotateLogs() {
	ticker := time.NewTicker(30 * time.Second)
	logPath := "data/radius.log"

	for range ticker.C {
		stat, err := os.Stat(logPath)
		if err != nil {
			continue
		}

		// If larger than 5MB, keep only the last 1MB
		if stat.Size() > 5*1024*1024 {
			log.Printf("[system] Rotating log file (current size: %d bytes)", stat.Size())
			data, err := os.ReadFile(logPath)
			if err != nil {
				continue
			}

			keepSize := int64(1 * 1024 * 1024)
			start := int64(len(data)) - keepSize
			if start < 0 {
				start = 0
			}

			err = os.WriteFile(logPath, data[start:], 0644)
			if err != nil {
				log.Printf("[system] Log rotation failed: %v", err)
			}
		}
	}
}
