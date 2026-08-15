package api

import (
	"log"
	"os"

	"mikrotik-manager/pkg/tunnel"
	"mikrotik-manager/server/internal/backup"

	"github.com/gofiber/fiber/v2"
)

// SetupBackupRoutes adds routes for agent-specific backup handling
func SetupBackupRoutes(app *fiber.App, svc *tunnel.Service) {
	app.Get("/api/agents/:subdomain/backup/download", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")

		data, filename, err := svc.RequestBackup(subdomain)
		if err != nil {
			return c.Status(fiber.StatusServiceUnavailable).SendString("فشل تحميل النسخة الاحتياطية: " + err.Error())
		}

		c.Attachment(filename)
		c.Set("Content-Type", "application/octet-stream")
		return c.Send(data)
	})

	app.Get("/api/agents/:subdomain/backups", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")

		backupPath := backup.GetBackupPath(subdomain)
		info, err := os.Stat(backupPath)
		if err != nil {
			return c.JSON(fiber.Map{"backups": []interface{}{}})
		}

		return c.JSON(fiber.Map{
			"backups": []fiber.Map{
				{
					"filename":   subdomain + ".db",
					"size":       info.Size(),
					"created_at": info.ModTime().Format("2006-01-02 15:04:05"),
				},
			},
		})
	})

	app.Get("/api/agents/:subdomain/backup/latest/download", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")

		backupPath := backup.GetBackupPath(subdomain)
		info, err := os.Stat(backupPath)
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("لا توجد نسخة احتياطية محفوظة")
		}

		c.Attachment(subdomain + "-backup-" + info.ModTime().Format("2006-01-02") + ".db")
		c.Set("Content-Type", "application/octet-stream")
		return c.SendFile(backupPath)
	})

	app.Post("/api/agents/:subdomain/backup/trigger", func(c *fiber.Ctx) error {
		subdomain := c.Params("subdomain")

		go func() {
			data, filename, err := svc.RequestBackup(subdomain)
			if err != nil {
				log.Printf("[Backup] Manual backup failed for %s: %v", subdomain, err)
				return
			}

			backupPath := backup.GetBackupPath(subdomain)
			if err := os.WriteFile(backupPath, data, 0644); err != nil {
				log.Printf("[Backup] Manual backup save failed for %s: %v", subdomain, err)
				return
			}

			log.Printf("[Backup] Manual backup saved for %s (%d bytes, filename: %s)", subdomain, len(data), filename)
		}()

		return c.JSON(fiber.Map{"message": "جاري إنشاء النسخة الاحتياطية..."})
	})
}
