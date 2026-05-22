package routing

import (
	"strings"

	"mikrotik-manager/pkg/core"

	"github.com/gofiber/fiber/v2"
)

func GetRoutingStatus(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	res, err := core.SafeRun(client, "/ip/firewall/mangle/print")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	routes := make([]map[string]interface{}, 0)
	for _, re := range res.Re {
		comment := re.Map["comment"]
		if !strings.HasPrefix(comment, "Route-") {
			continue
		}

		target := re.Map["dst-address-list"]
		if target == "" {
			target = re.Map["src-address-list"]
		}
		if target == "" {
			target = "All Traffic"
		}

		routes = append(routes, map[string]interface{}{
			"id":      re.Map[".id"],
			"app":     target,
			"gateway": re.Map["new-routing-mark"],
			"enabled": re.Map["disabled"] == "false",
			"comment": comment,
		})
	}
	return c.JSON(routes)
}

func ToggleRouting(c *fiber.Ctx) error {
	type Request struct {
		ID       string `json:"id"`
		Disabled bool   `json:"disabled"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	val := "false"
	if req.Disabled {
		val = "true"
	}

	_, err := core.SafeRun(client, "/ip/firewall/mangle/set", "=.id="+req.ID, "=disabled="+val)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "تم تحديث حالة التوجيه بنجاح"})
}
