package wan

import (
	"mikrotik-manager/pkg/core"

	"github.com/gofiber/fiber/v2"
)

func GetWanStatus(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	allLines := make([]map[string]interface{}, 0)

	// Fetch PPPoE
	pppoeRes, pppErr := core.SafeRun(client, "/interface/pppoe-client/print", "?comment=TM_WAN")
	if pppErr == nil && pppoeRes != nil {
		for _, re := range pppoeRes.Re {
			allLines = append(allLines, map[string]interface{}{
				"id":        re.Map[".id"],
				"interface": re.Map["name"],
				"type":      "PPPOE",
				"running":   re.Map["running"] == "true",
				"disabled":  re.Map["disabled"] == "true",
			})
		}
	}

	// Fetch DHCP
	dhcpRes, dhcpErr := core.SafeRun(client, "/ip/dhcp-client/print", "?comment=TM_WAN")
	if dhcpErr == nil && dhcpRes != nil {
		for _, re := range dhcpRes.Re {
			allLines = append(allLines, map[string]interface{}{
				"id":        re.Map[".id"],
				"interface": re.Map["interface"],
				"type":      "DHCP",
				"running":   re.Map["status"] == "bound",
				"disabled":  re.Map["disabled"] == "true",
			})
		}
	}

	return c.JSON(allLines)
}
