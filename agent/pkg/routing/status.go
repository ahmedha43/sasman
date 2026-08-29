package routing

import (
	"strings"

	"mikrotik-manager/agent/pkg/core"

	"github.com/go-routeros/routeros/v3"
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
	routeRes, _ := core.SafeRun(client, "/ip/route/print")

	routes := make([]map[string]interface{}, 0)
	for _, re := range res.Re {
		comment := re.Map["comment"]
		if !strings.HasPrefix(comment, "Route-") {
			continue
		}

		target := ""
		if strings.HasPrefix(comment, "Route-") {
			target = strings.TrimPrefix(comment, "Route-")
		} else {
			target = re.Map["dst-address-list"]
			if target == "" {
				target = re.Map["src-address-list"]
			}
			if target == "" {
				target = "All Traffic"
			}
		}

		routingMark := re.Map["new-routing-mark"]
		gateways := routingStatusGateways(routeRes, routingMark, comment)
		routes = append(routes, map[string]interface{}{
			"id":       re.Map[".id"],
			"app":      target,
			"gateway":  routingMark,
			"gateways": gateways,
			"enabled":  re.Map["disabled"] != "true",
			"comment":  comment,
		})
	}
	return c.JSON(routes)
}

func routingStatusGateways(routeRes *routeros.Reply, routingMark string, comment string) []string {
	if routeRes == nil {
		return nil
	}
	seen := map[string]bool{}
	gateways := []string{}
	for _, re := range routeRes.Re {
		route := re.Map
		if route["disabled"] == "true" || route["dst-address"] != "0.0.0.0/0" {
			continue
		}
		if routeRoutingTable(route) != routingMark && route["comment"] != comment {
			continue
		}
		gateway := strings.TrimSpace(route["gateway"])
		if gateway == "" || seen[gateway] {
			continue
		}
		seen[gateway] = true
		gateways = append(gateways, gateway)
	}
	return gateways
}

func routeRoutingTable(route map[string]string) string {
	table := strings.TrimSpace(route["routing-table"])
	if table == "" {
		table = strings.TrimSpace(route["vrf-interface"])
	}
	if table == "" {
		table = "main"
	}
	return table
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
