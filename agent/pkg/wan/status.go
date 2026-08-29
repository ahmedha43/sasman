package wan

import (
	"strings"

	"mikrotik-manager/agent/pkg/core"

	"github.com/gofiber/fiber/v2"
)

func rosTruthy(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true") ||
		strings.EqualFold(strings.TrimSpace(value), "yes")
}

func dhcpClientIsRunning(values map[string]string, interfaceRunning map[string]bool) bool {
	if rosTruthy(values["disabled"]) {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(values["status"]))
	if status == "bound" {
		return true
	}
	if values["address"] != "" && (values["gateway"] != "" || interfaceRunning[values["interface"]]) {
		return true
	}
	return false
}

func GetWanStatus(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	allLines := make([]map[string]interface{}, 0)
	interfaceRunning := map[string]bool{}
	if interfacesRes, interfacesErr := core.SafeRun(client, "/interface/print"); interfacesErr == nil && interfacesRes != nil {
		for _, re := range interfacesRes.Re {
			if name := re.Map["name"]; name != "" {
				interfaceRunning[name] = rosTruthy(re.Map["running"])
			}
		}
	}

	// Fetch PPPoE
	pppoeRes, pppErr := core.SafeRun(client, "/interface/pppoe-client/print", "?comment=TM_WAN")
	if pppErr == nil && pppoeRes != nil {
		for _, re := range pppoeRes.Re {
			allLines = append(allLines, map[string]interface{}{
				"id":        re.Map[".id"],
				"interface": re.Map["name"],
				"type":      "PPPOE",
				"running":   !rosTruthy(re.Map["disabled"]) && (rosTruthy(re.Map["running"]) || rosTruthy(re.Map["connected"])),
				"disabled":  rosTruthy(re.Map["disabled"]),
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
				"running":   dhcpClientIsRunning(re.Map, interfaceRunning),
				"disabled":  rosTruthy(re.Map["disabled"]),
			})
		}
	}

	return c.JSON(allLines)
}
