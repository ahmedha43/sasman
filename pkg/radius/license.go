package radius

import (
	"strings"

	"github.com/go-routeros/routeros/v3"
	"github.com/gofiber/fiber/v2"

	"mikrotik-manager/pkg/core"
	"mikrotik-manager/pkg/firebase"
	"mikrotik-manager/pkg/shared"
)

type routerConnectRequest struct {
	Address string `json:"address"`
	User    string `json:"user"`
	Pass    string `json:"pass"`
}

func connectAndSaveRouter(req routerConnectRequest) (string, error) {
	req.Address = strings.TrimSpace(req.Address)
	req.User = strings.TrimSpace(req.User)
	if req.Address == "" || req.User == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "Router address and username are required")
	}

	client, err := routeros.Dial(req.Address, req.User, req.Pass)
	if err != nil {
		return "", fiber.NewError(fiber.StatusUnauthorized, "Failed to connect to MikroTik: "+err.Error())
	}
	defer client.Close()

	serial, err := core.GetRouterSerial(client)
	if err != nil {
		return "", fiber.NewError(fiber.StatusInternalServerError, "Failed to read MikroTik serial: "+err.Error())
	}

	shared.RouterConfigState.Address = req.Address
	shared.RouterConfigState.Username = req.User
	shared.RouterConfigState.Password = req.Pass
	shared.RouterConfigState.Serial = serial
	shared.SaveConfig()
	core.ResetSharedClient()
	firebase.SyncAsync("radius_router_connected", firebase.RemoteAccess{})

	return serial, nil
}

func RouterConnectHandler(c *fiber.Ctx) error {
	var req routerConnectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	serial, err := connectAndSaveRouter(req)
	if err != nil {
		if e, ok := err.(*fiber.Error); ok {
			return c.Status(e.Code).JSON(fiber.Map{"error": e.Message})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message":          "MikroTik connected and serial fetched successfully",
		"router_connected": true,
		"serial":           serial,
	})
}

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
			"message":          "Enter MikroTik details first so SASMAN can fetch the router serial for activation",
			"serial":           "",
			"expires":          "",
		})
	}

	valid, msg, exp := core.VerifyLicense(shared.RouterConfigState.License, serial)
	expStr := ""
	if !exp.IsZero() {
		expStr = exp.Format("2006-01-02 15:04:05")
	} else if shared.RouterConfigState.CloudLicenseExpiresAt != "" {
		expStr = shared.RouterConfigState.CloudLicenseExpiresAt
	}

	return c.JSON(fiber.Map{
		"valid":            valid,
		"router_connected": true,
		"message":          msg,
		"serial":           serial,
		"expires":          expStr,
		"status":           shared.RouterConfigState.CloudLicenseStatus,
		"days_remaining":   shared.RouterConfigState.CloudLicenseDaysLeft,
	})
}

func LicenseActivateHandler(c *fiber.Ctx) error {
	type req struct {
		Key     string `json:"key"`
		Address string `json:"address"`
		User    string `json:"user"`
		Pass    string `json:"pass"`
	}
	var body req
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	body.Key = strings.TrimSpace(body.Key)
	if body.Key == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Activation key is required"})
	}

	if strings.TrimSpace(body.Address) != "" {
		if _, err := connectAndSaveRouter(routerConnectRequest{Address: body.Address, User: body.User, Pass: body.Pass}); err != nil {
			if e, ok := err.(*fiber.Error); ok {
				return c.Status(e.Code).JSON(fiber.Map{"error": e.Message})
			}
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if shared.RouterConfigState.Address == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Enter MikroTik details first so SASMAN can fetch the router serial"})
	}

	serial := shared.RouterConfigState.Serial
	if serial == "" {
		client, err := core.GetSharedClient()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to MikroTik: " + err.Error()})
		}
		s, err := core.GetRouterSerial(client)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to read MikroTik serial"})
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
	firebase.SyncAsync("radius_license_activated", firebase.RemoteAccess{})
	return c.JSON(fiber.Map{"message": "System activated successfully"})
}

func RequireAdminUnlessUnlicensed(c *fiber.Ctx) error {
	valid, _, _ := core.VerifyLicense(shared.RouterConfigState.License, shared.RouterConfigState.Serial)
	if valid {
		return RequireAdmin(c)
	}
	return c.Next()
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
