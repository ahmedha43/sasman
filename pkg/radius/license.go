package radius

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"mikrotik-manager/pkg/core"
	"mikrotik-manager/pkg/shared"
)

func LicenseStatusHandler(c *fiber.Ctx) error {
	serial := shared.RouterConfigState.Serial
	if serial == "" && shared.RouterConfigState.Address != "" {
		if client, err := core.GetSharedClient(); err == nil {
			if s, err := core.GetRouterSerial(client); err == nil {
				shared.RouterConfigState.Serial = s
				shared.SaveConfig()
				serial = s
			}
		}
	}

	if shared.RouterConfigState.Address == "" {
		return c.JSON(fiber.Map{
			"valid":            false,
			"router_connected": false,
			"message":          "يجب الاتصال بالراوتر من اللوحة الرئيسية أولاً",
			"serial":           "",
			"expires":          "",
		})
	}

	valid, msg, exp := core.VerifyLicense(shared.RouterConfigState.License, serial)
	return c.JSON(fiber.Map{
		"valid":            valid,
		"router_connected": true,
		"message":          msg,
		"serial":           serial,
		"expires":          exp.Format("2006-01-02"),
	})
}

func LicenseActivateHandler(c *fiber.Ctx) error {
	if shared.RouterConfigState.Address == "" {
		return c.Status(400).JSON(fiber.Map{"error": "يجب الاتصال بالراوتر من اللوحة الرئيسية أولاً"})
	}
	type req struct {
		Key string `json:"key"`
	}
	var body req
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "بيانات غير صالحة"})
	}
	body.Key = strings.TrimSpace(body.Key)
	if body.Key == "" {
		return c.Status(400).JSON(fiber.Map{"error": "المفتاح مطلوب"})
	}

	serial := shared.RouterConfigState.Serial
	if serial == "" {
		client, err := core.GetSharedClient()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل الاتصال بالراوتر: " + err.Error()})
		}
		s, err := core.GetRouterSerial(client)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "تعذر قراءة السيريال"})
		}
		serial = s
	}

	valid, msg, _ := core.VerifyLicense(body.Key, serial)
	if !valid {
		return c.Status(403).JSON(fiber.Map{"error": msg})
	}

	shared.RouterConfigState.License = body.Key
	shared.RouterConfigState.Serial = serial
	shared.SaveConfig()
	return c.JSON(fiber.Map{"message": "تم تفعيل النظام بنجاح"})
}

func RequireLicense(c *fiber.Ctx) error {
	valid, msg, _ := core.VerifyLicense(shared.RouterConfigState.License, shared.RouterConfigState.Serial)
	if !valid {
		return c.Status(403).JSON(fiber.Map{
			"error":            msg,
			"license_required": true,
		})
	}
	return c.Next()
}
