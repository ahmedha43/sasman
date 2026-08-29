package routing

import (
	"net"
	"strings"
	"time"

	"mikrotik-manager/agent/pkg/core"
	"mikrotik-manager/pkg/shared"

	"github.com/go-routeros/routeros/v3"
	"github.com/gofiber/fiber/v2"
)

func ApplyRouting(c *fiber.Ctx) error {
	type Request struct {
		TargetApp string   `json:"target"`
		Gateway   string   `json:"gateway"`
		Gateways  []string `json:"gateways"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	gateways := normalizeRoutingGateways(req.Gateways, req.Gateway)
	if len(gateways) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Gateway is required"})
	}

	game, isGame := shared.RoutingDataState.Games[req.TargetApp]
	items := []string{}
	if !isGame {
		if val, ok := shared.RoutingDataState.Apps[req.TargetApp]; ok {
			items = append(items, val...)
		}
		if val, ok := shared.RoutingDataState.Ips[req.TargetApp]; ok {
			items = append(items, val...)
		}

		if len(items) == 0 {
			return c.Status(404).JSON(fiber.Map{"error": "App not found"})
		}
	}

	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	RemoveRoutingForApp(client, req.TargetApp)

	tableName := routingTableName(req.TargetApp)
	routeComment := "Route-" + req.TargetApp
	core.SafeRun(client, "/routing/table/add", "=name="+tableName, "=fib", "=comment="+routeComment)
	for _, gateway := range gateways {
		gw := ResolveGateway(client, gateway)
		core.SafeRun(client, "/ip/route/add",
			"=dst-address=0.0.0.0/0",
			"=gateway="+gw,
			"=routing-table="+tableName,
			"=check-gateway=ping",
			"=comment="+routeComment)
	}

	if isGame {
		addSmartRoutingMangleRule(client,
			"=chain=prerouting",
			"=src-address-list=TM_Local_Subnets",
			"=protocol="+game.Protocol,
			"=dst-port="+game.Ports,
			"=dst-address-type=!local",
			"=action=mark-routing",
			"=new-routing-mark="+tableName,
			"=passthrough=no",
			"=comment="+routeComment,
		)
	} else {
		// Ensure TM_Local_Subnets list exists so RAW rules don't fail silently
		localSubnets := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
		for _, subnet := range localSubnets {
			reply, err := client.Run("/ip/firewall/address-list/print", "?list=TM_Local_Subnets", "?address="+subnet)
			if err != nil || reply == nil || len(reply.Re) == 0 {
				core.SafeRun(client, "/ip/firewall/address-list/add", "=list=TM_Local_Subnets", "=address="+subnet, "=comment=Auto-SASMAN-Bypass")
			}
		}

		listName := "list-" + req.TargetApp
		core.SafeRun(client, "/ip/dns/set", "=allow-remote-requests=yes", "=address-list-extra-time=1w3d")
		core.SafeRun(client, "/ip/firewall/nat/add", "=chain=dstnat", "=action=redirect", "=to-ports=53", "=protocol=udp", "=dst-port=53", "=comment=Force DNS - "+req.TargetApp)
		core.SafeRun(client, "/ip/firewall/nat/add", "=chain=dstnat", "=action=redirect", "=to-ports=53", "=protocol=tcp", "=dst-port=53", "=comment=Force DNS - "+req.TargetApp)

		for _, item := range items {
			if IsIP(item) {
				core.SafeRun(client, "/ip/firewall/address-list/add", "=list="+listName, "=address="+item, "=comment=Auto-SASMAN")
			} else {
				core.SafeRun(client, "/ip/dns/static/add", "=name="+item, "=type=FWD", "=forward-to=8.8.8.8", "=address-list="+listName, "=match-subdomain=yes", "=comment="+routeComment)
				core.SafeRun(client, "/ip/firewall/raw/add", "=chain=prerouting", "=protocol=tcp", "=dst-port=443", "=action=add-dst-to-address-list", "=address-list="+listName, "=tls-host="+item, "=src-address-list=!TM_Local_Subnets", "=comment=SNI-"+item)
				core.SafeRun(client, "/ip/firewall/raw/add", "=chain=prerouting", "=protocol=tcp", "=dst-port=443", "=action=add-dst-to-address-list", "=address-list="+listName, "=tls-host=*."+item, "=src-address-list=!TM_Local_Subnets", "=comment=SNI-*."+item)
			}
		}

		addSmartRoutingMangleRule(client,
			"=chain=prerouting",
			"=src-address-list=TM_Local_Subnets",
			"=action=mark-routing",
			"=new-routing-mark="+tableName,
			"=dst-address-list="+listName,
			"=passthrough=no",
			"=comment="+routeComment)
	}

	core.SafeRun(client, "/ip/firewall/filter/add", "=chain=forward", "=action=accept", "=routing-mark="+tableName, "=connection-state=established,related", "=place-before=0", "=comment="+routeComment)

	return c.JSON(fiber.Map{"message": "تم تطبيق التوجيه بنجاح " + req.TargetApp})
}

func RemoveRouting(c *fiber.Ctx) error {
	type Request struct {
		TargetApp string `json:"target"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	RemoveRoutingForApp(client, req.TargetApp)
	return c.JSON(fiber.Map{"message": "Routing removed successfully for " + req.TargetApp})
}

func RemoveRoutingForApp(client *routeros.Client, targetApp string) {
	listName := "list-" + targetApp
	search := targetApp

	paths := []string{"/ip/firewall/mangle", "/ip/firewall/filter", "/ip/firewall/address-list", "/ip/dns/static", "/ip/firewall/raw", "/ip/firewall/nat", "/ip/route", "/routing/table"}
	for _, path := range paths {
		core.RemoveEntriesByMatcher(client, path, func(v map[string]string) bool {
			comment := v["comment"]
			list := v["list"]
			addrList := v["address-list"]
			name := v["name"]
			return strings.Contains(comment, search) || list == listName || addrList == listName || name == routingTableName(targetApp)
		})
	}
}

func normalizeRoutingGateways(gateways []string, fallback string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(gateways)+1)
	for _, gateway := range gateways {
		gateway = strings.TrimSpace(gateway)
		if gateway == "" || seen[gateway] {
			continue
		}
		seen[gateway] = true
		out = append(out, gateway)
	}
	fallback = strings.TrimSpace(fallback)
	if len(out) == 0 && fallback != "" {
		out = append(out, fallback)
	}
	return out
}

func routingTableName(targetApp string) string {
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", ";", "-", ",", "-", "'", "", "\"", "")
	name := strings.Trim(replacer.Replace(targetApp), "-")
	if name == "" {
		name = "app"
	}
	return "table-" + name
}

func addSmartRoutingMangleRule(client *routeros.Client, args ...string) {
	if placeBefore := appRoutingInsertBefore(client); placeBefore != "" {
		args = append(args, "=place-before="+placeBefore)
	}
	core.SafeRun(client, append([]string{"/ip/firewall/mangle/add"}, args...)...)
}

func appRoutingInsertBefore(client *routeros.Client) string {
	reply, err := core.SafeRun(client, "/ip/firewall/mangle/print")
	if err != nil || reply == nil {
		return ""
	}
	for _, rule := range reply.Re {
		if rule.Map["disabled"] == "true" {
			continue
		}
		comment := rule.Map["comment"]
		if comment == "TM_Input_WAN1" ||
			strings.HasPrefix(comment, "TM_Input_WAN") ||
			comment == "TM_Sticky_Bank" ||
			strings.HasPrefix(comment, "TM_PCC_PRE_") ||
			strings.HasPrefix(comment, "TM_Route_LAN_") {
			return rule.Map[".id"]
		}
	}
	return ""
}

func ResolveGateway(client *routeros.Client, iface string) string {
	for i := 0; i < 20; i++ {
		reply, err := core.SafeRun(client, "/ip/dhcp-client/print", "?interface="+iface)
		if err == nil && reply != nil && len(reply.Re) > 0 {
			gw := reply.Re[0].Map["gateway"]
			if gw != "" {
				return gw
			}
		}
		if strings.HasPrefix(iface, "pppoe") || strings.HasPrefix(iface, "l2tp") || strings.HasPrefix(iface, "sstp") {
			return iface
		}
		time.Sleep(500 * time.Millisecond)
	}
	return iface
}

func IsIP(s string) bool {
	if strings.Contains(s, "/") {
		_, _, err := net.ParseCIDR(s)
		return err == nil
	}
	return net.ParseIP(s) != nil
}

func ApplyBlock(c *fiber.Ctx) error {
	type Request struct {
		Name    string   `json:"name"`
		Domains []string `json:"domains"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	for _, domain := range req.Domains {
		core.SafeRun(client, "/ip/dns/static/add", "=name="+domain, "=type=NXDOMAIN", "=match-subdomain=yes", "=comment=BLOCK-"+req.Name)
		core.SafeRun(client, "/ip/firewall/raw/add", "=chain=prerouting", "=protocol=tcp", "=dst-port=443", "=tls-host="+domain, "=action=drop", "=comment=BLOCK-"+req.Name)
		core.SafeRun(client, "/ip/firewall/raw/add", "=chain=prerouting", "=protocol=tcp", "=dst-port=443", "=tls-host=*."+domain, "=action=drop", "=comment=BLOCK-"+req.Name)
	}
	return c.JSON(fiber.Map{"message": "Blocking applied successfully"})
}

func PurgeRouting(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	paths := []string{"/ip/firewall/mangle", "/ip/firewall/filter", "/ip/firewall/address-list", "/ip/dns/static", "/ip/firewall/raw", "/ip/firewall/nat", "/ip/route", "/routing/table"}
	for _, path := range paths {
		core.RemoveEntriesByMatcher(client, path, func(values map[string]string) bool {
			return core.ShouldRemoveManagedEntry(path, values)
		})
	}

	return c.JSON(fiber.Map{"message": "تم تصفير كافة قواعد التوجيه بنجاح"})
}
