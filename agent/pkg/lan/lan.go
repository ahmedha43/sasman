package lan

import (
	"fmt"
	"strings"

	"mikrotik-manager/agent/pkg/core"

	"github.com/gofiber/fiber/v2"
)

func SetupBridge(c *fiber.Ctx) error {
	type Request struct {
		Name          string   `json:"name"`
		Ports         []string `json:"ports"`
		IP            string   `json:"ip"`
		EnablePppoe   bool     `json:"enablePppoe"`
		PppoeIp       string   `json:"pppoeIp"`
		EnableHotspot bool     `json:"enableHotspot"`
		HotspotIp     string   `json:"hotspotIp"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	core.SafeRun(client, "/interface/bridge/add", "=name="+req.Name, "=comment=TM_LAN")
	if req.IP != "" {
		core.SafeRun(client, "/ip/address/add", "=address="+req.IP, "=interface="+req.Name, "=comment=TM_LAN")
	}

	for _, port := range req.Ports {
		if port == "" {
			continue
		}
		reply, _ := core.SafeRun(client, "/interface/bridge/port/print", "?interface="+port)
		for _, s := range reply.Re {
			if id := s.Map[".id"]; id != "" {
				core.SafeRun(client, "/interface/bridge/port/remove", "=.id="+id)
			}
		}
		core.SafeRun(client, "/interface/bridge/port/add", "=bridge="+req.Name, "=interface="+port, "=comment=TM_LAN")
	}

	// Helpers
	parseSubnetRange := func(cidr string) (localIP string, poolRange string, subnet string) {
		localIP = "10.0.0.1"
		poolRange = "10.0.0.2-10.0.0.254"
		subnet = "10.0.0.0/24"
		if cidr != "" && strings.Contains(cidr, "/") {
			cleanIP := strings.Split(cidr, "/")[0]
			localIP = cleanIP
			parts := strings.Split(cleanIP, ".")
			if len(parts) >= 3 {
				prefix := fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[2])
				poolRange = prefix + ".2-" + prefix + ".254"
				subnet = prefix + ".0/" + strings.Split(cidr, "/")[1]
			}
		}
		return
	}

	pppoePool := "pool-pppoe-" + req.Name
	pppoeProfile := "profile-pppoe-" + req.Name
	pppoeService := "pppoe-" + req.Name

	core.RemoveEntriesByMatcher(client, "/interface/pppoe-server/server", func(v map[string]string) bool {
		return v["service-name"] == pppoeService
	})
	core.RemoveEntriesByMatcher(client, "/ip/pool", func(v map[string]string) bool {
		return v["name"] == pppoePool
	})
	core.RemoveEntriesByMatcher(client, "/ppp/profile", func(v map[string]string) bool {
		return v["name"] == pppoeProfile
	})

	if req.EnablePppoe {
		localIP, poolRange, subnet := parseSubnetRange(req.PppoeIp)
		core.SafeRun(client, "/ip/pool/add", "=name="+pppoePool, "=ranges="+poolRange)
		core.SafeRun(client, "/ppp/profile/add", "=name="+pppoeProfile, "=local-address="+localIP, "=remote-address="+pppoePool, "=dns-server=8.8.8.8,8.8.4.4")
		core.SafeRun(client, "/interface/pppoe-server/server/add", "=service-name="+pppoeService, "=interface="+req.Name, "=default-profile="+pppoeProfile, "=disabled=no", "=one-session-per-host=yes", "=authentication=pap,chap,mschap2")

		natRes, natErr := core.SafeRun(client, "/ip/firewall/nat/print", "?src-address="+subnet, "?comment=PPPoE-Server-NAT")
		if natErr == nil && natRes != nil && len(natRes.Re) == 0 {
			core.SafeRun(client, "/ip/firewall/nat/add", "=chain=srcnat", "=src-address="+subnet, "=action=masquerade", "=comment=PPPoE-Server-NAT")
		}
	}

	hsPool := "hs-pool-" + req.Name
	hsProf := "hsprof-" + req.Name
	hsName := "hs-" + req.Name

	core.RemoveEntriesByMatcher(client, "/ip/hotspot", func(v map[string]string) bool {
		return v["name"] == hsName
	})
	core.RemoveEntriesByMatcher(client, "/ip/hotspot/profile", func(v map[string]string) bool {
		return v["name"] == hsProf
	})
	core.RemoveEntriesByMatcher(client, "/ip/pool", func(v map[string]string) bool {
		return v["name"] == hsPool
	})

	if req.EnableHotspot {
		localIP, poolRange, subnet := parseSubnetRange(req.HotspotIp)
		core.SafeRun(client, "/ip/pool/add", "=name="+hsPool, "=ranges="+poolRange)
		core.SafeRun(client, "/ip/hotspot/profile/add", "=name="+hsProf, "=html-directory=hotspot", "=dns-name=login.net", "=hotspot-address="+localIP)
		core.SafeRun(client, "/ip/hotspot/add", "=name="+hsName, "=interface="+req.Name, "=address-pool="+hsPool, "=profile="+hsProf, "=disabled=no")
		core.SafeRun(client, "/ip/hotspot/user/profile/add", "=name=default", "=shared-users=1")

		natRes, natErr := core.SafeRun(client, "/ip/firewall/nat/print", "?src-address="+subnet, "?comment=Hotspot-NAT-"+req.Name)
		if natErr == nil && natRes != nil && len(natRes.Re) == 0 {
			core.SafeRun(client, "/ip/firewall/nat/add", "=chain=srcnat", "=src-address="+subnet, "=action=masquerade", "=comment=Hotspot-NAT-"+req.Name)
		}
	}

	return c.JSON(fiber.Map{"message": "تم إعداد البريج والمنافذ بنجاح"})
}

func PurgeLAN(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	paths := []string{
		"/ip/address",
		"/interface/bridge/port",
		"/interface/pppoe-server/server",
		"/ppp/profile",
		"/ip/pool",
		"/ip/hotspot",
		"/ip/hotspot/profile",
		"/ip/firewall/nat",
	}

	for _, path := range paths {
		core.RemoveEntriesByMatcher(client, path, func(values map[string]string) bool {
			comment := values["comment"]
			name := values["name"]
			serviceName := values["service-name"]

			isBridge := comment == "TM_LAN" || strings.HasPrefix(name, "bridge-") || strings.HasPrefix(name, "pool-pppoe-") ||
				strings.HasPrefix(name, "hs-pool-") || strings.HasPrefix(name, "profile-pppoe-") ||
				strings.HasPrefix(name, "hsprof-") || strings.HasPrefix(name, "hs-") ||
				strings.HasPrefix(serviceName, "pppoe-")
			isNat := strings.Contains(comment, "PPPoE-Server-NAT") || strings.Contains(comment, "Hotspot-NAT")

			return isBridge || isNat
		})
	}

	core.RemoveEntriesByMatcher(client, "/interface/bridge", func(v map[string]string) bool {
		return v["comment"] == "TM_LAN" || strings.HasPrefix(v["name"], "bridge-")
	})

	return c.JSON(fiber.Map{"message": "تم تنظيف كافة عمليات البريج والـ LAN بنجاح"})
}
