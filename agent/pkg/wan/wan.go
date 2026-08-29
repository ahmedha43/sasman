package wan

import (
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"mikrotik-manager/agent/pkg/core"
	"mikrotik-manager/agent/pkg/routing"

	"github.com/go-routeros/routeros/v3"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type WanLine struct {
	Interface string `json:"interface"`
	Name      string `json:"name"`
	Gateway   string `json:"gateway"`
	Weight    int    `json:"weight"`
}

// waitForInterface polls the RouterOS interface list to confirm the interface
// is fully created and visible before we proceed to attach services to it.
// Helps to avoid race conditions on busy routers.
func waitForInterface(client *routeros.Client, name string) bool {
	for i := 0; i < 10; i++ {
		reply, err := core.SafeRun(client, "/interface/print", "?name="+name)
		if err == nil && reply != nil && len(reply.Re) > 0 {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func normalizeWeight(weight int) int {
	if weight <= 0 {
		return 1
	}
	return weight
}

func isInterfaceGateway(name string) bool {
	return strings.HasPrefix(name, "pppoe") ||
		strings.HasPrefix(name, "l2tp") ||
		strings.HasPrefix(name, "sstp") ||
		strings.HasPrefix(name, "ovpn")
}

func resolveWanGateway(client *routeros.Client, line WanLine) (string, error) {
	if line.Gateway != "" {
		return line.Gateway, nil
	}
	if isInterfaceGateway(line.Interface) {
		return line.Interface, nil
	}

	gw := routing.ResolveGateway(client, line.Interface)
	if gw == "" || gw == line.Interface {
		if staticGW := resolveStaticRouteGateway(client, line.Interface); staticGW != "" {
			return staticGW, nil
		}
		return "", fmt.Errorf("لم يتم العثور على gateway صالح للواجهة %s", line.Interface)
	}
	return gw, nil
}

func resolveStaticRouteGateway(client *routeros.Client, iface string) string {
	reply, err := core.SafeRun(client, "/ip/route/print", "?dst-address=0.0.0.0/0")
	if err != nil || reply == nil {
		return ""
	}
	for _, route := range reply.Re {
		if route.Map["disabled"] == "true" {
			continue
		}
		gateway := route.Map["gateway"]
		if gateway == "" {
			continue
		}
		if strings.Contains(gateway, "%"+iface) {
			return strings.Split(gateway, "%")[0]
		}
		if route.Map["immediate-gw"] != "" && strings.Contains(route.Map["immediate-gw"], "%"+iface) {
			return gateway
		}
		if route.Map["gateway-status"] != "" && strings.Contains(route.Map["gateway-status"], iface) {
			return gateway
		}
	}
	return ""
}

func hasMainDefaultRoute(client *routeros.Client, gateway string) bool {
	reply, err := core.SafeRun(client, "/ip/route/print", "?dst-address=0.0.0.0/0", "?routing-table=main")
	if err != nil || reply == nil {
		return false
	}
	for _, route := range reply.Re {
		routeGateway := strings.Split(route.Map["gateway"], "%")[0]
		routeGateway = strings.Split(routeGateway, "@")[0]
		if routeGateway == gateway && route.Map["disabled"] != "true" {
			return true
		}
	}
	return false
}

func scopedGateway(gateway string, iface string) string {
	if gateway == "" || iface == "" || isInterfaceGateway(gateway) || strings.Contains(gateway, "%") || strings.Contains(gateway, "@") {
		return gateway
	}
	return gateway + "%" + iface
}

func routeLookupGateway(gateway string, table string) string {
	if gateway == "" || table == "" || strings.Contains(gateway, "@") || strings.Contains(gateway, "%") || isInterfaceGateway(gateway) {
		return gateway
	}
	return gateway + "@" + table
}

func flushManagedWanConnections(client *routeros.Client) {
	reply, err := core.SafeRun(client, "/ip/firewall/connection/print")
	if err != nil || reply == nil {
		return
	}
	for _, conn := range reply.Re {
		mark := conn.Map["connection-mark"]
		if !strings.HasPrefix(mark, "conn_WAN") {
			continue
		}
		id := conn.Map[".id"]
		if id != "" {
			core.SafeRun(client, "/ip/firewall/connection/remove", "=.id="+id)
		}
	}
}

func managedWanLineCount(client *routeros.Client) int {
	count := 0
	if reply, err := core.SafeRun(client, "/ip/dhcp-client/print", "?comment=TM_WAN"); err == nil && reply != nil {
		for _, re := range reply.Re {
			if re.Map["disabled"] != "true" {
				count++
			}
		}
	}
	if reply, err := core.SafeRun(client, "/interface/pppoe-client/print", "?comment=TM_WAN"); err == nil && reply != nil {
		for _, re := range reply.Re {
			if re.Map["disabled"] != "true" {
				count++
			}
		}
	}
	return count
}

func setPPPoEDefaultRoute(client *routeros.Client, name string, distance int, usePeerDNS string) bool {
	reply, err := core.SafeRun(client, "/interface/pppoe-client/print", "?name="+name)
	if err != nil || reply == nil || len(reply.Re) == 0 {
		return false
	}
	id := reply.Re[0].Map[".id"]
	if id == "" {
		return false
	}
	args := []string{
		"=.id=" + id,
		"=add-default-route=yes",
		"=use-peer-dns=" + usePeerDNS,
		"=comment=TM_WAN",
		"=disabled=no",
	}
	if distance > 0 {
		args = append(args, "=default-route-distance="+strconv.Itoa(distance))
	}
	core.SafeRun(client, append([]string{"/interface/pppoe-client/set"}, args...)...)
	return true
}

func disableFastTrackRules(client *routeros.Client) {
	reply, err := core.SafeRun(client, "/ip/firewall/filter/print")
	if err != nil || reply == nil {
		return
	}
	for _, rule := range reply.Re {
		if rule.Map["action"] != "fasttrack-connection" || rule.Map["disabled"] == "true" {
			continue
		}
		id := rule.Map[".id"]
		if id != "" {
			core.SafeRun(client, "/ip/firewall/filter/set", "=.id="+id, "=disabled=yes")
		}
	}
}

func GetInterfaces(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	reply, err := core.SafeRun(client, "/interface/print")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل قراءة واجهات الشبكة: " + err.Error()})
	}

	var list []map[string]string
	for _, s := range reply.Re {
		m := s.Map
		list = append(list, map[string]string{
			"name":     m["name"],
			"type":     m["type"],
			"disabled": m["disabled"],
			"running":  m["running"],
		})
	}
	return c.JSON(list)
}

func GetPPPoE(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	reply, err := core.SafeRun(client, "/interface/pppoe-client/print")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل قراءة خطوط PPPoE: " + err.Error()})
	}

	var list []map[string]string
	for _, s := range reply.Re {
		list = append(list, s.Map)
	}
	return c.JSON(list)
}

func GetDhcpClients(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	reply, err := core.SafeRun(client, "/ip/dhcp-client/print")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل قراءة DHCP Clients: " + err.Error()})
	}

	var list []map[string]string
	for _, s := range reply.Re {
		list = append(list, s.Map)
	}
	return c.JSON(list)
}

func AddPPPoE(c *fiber.Ctx) error {
	type Request struct {
		Name       string `json:"name"`
		User       string `json:"user"`
		Password   string `json:"pass"`
		Interface  string `json:"interface"`
		UseMacvlan bool   `json:"useMacvlan"`
	}
	var req Request
	c.BodyParser(&req)

	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	targetInterface := req.Interface
	if req.UseMacvlan {
		suffix := fmt.Sprintf("%03d", time.Now().UnixNano()%1000)
		targetInterface = "mac-" + req.Interface + "-" + suffix
		_, err := core.SafeRun(client, "/interface/macvlan/add",
			"=name="+targetInterface, "=interface="+req.Interface,
			"=mode=private", "=disabled=no", "=comment=TM_WAN")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل إنشاء واجهة MAC-VLAN: " + err.Error()})
		}

		// Wait until interface is fully loaded on the router
		if !waitForInterface(client, targetInterface) {
			return c.Status(500).JSON(fiber.Map{"error": "فشل تهيئة واجهة MAC-VLAN (تأخر في استجابة المايكروتك)"})
		}
	}

	usePeerDNS := "yes"
	if managedWanLineCount(client) > 0 {
		usePeerDNS = "no"
	}

	_, err := core.SafeRun(client, "/interface/pppoe-client/add",
		"=name="+req.Name, "=user="+req.User, "=password="+req.Password, "=interface="+targetInterface,
		"=add-default-route=yes", "=use-peer-dns="+usePeerDNS, "=disabled=no", "=comment=TM_WAN")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل إنشاء PPPoE Client: " + err.Error()})
	}

	core.SafeRun(client, "/interface/list/add", "=name=WAN")
	core.SafeRun(client, "/interface/list/add", "=name=WAN-PPP")
	core.SafeRun(client, "/interface/list/member/add", "=interface="+req.Name, "=list=WAN", "=comment=TM_WAN")
	core.SafeRun(client, "/interface/list/member/add", "=interface="+req.Name, "=list=WAN-PPP", "=comment=TM_WAN")

	return c.JSON(fiber.Map{"message": "PPPoE added successfully"})
}

func AddMacvlan(c *fiber.Ctx) error {
	type Request struct{ Name, Parent string }
	var req Request
	c.BodyParser(&req)
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	_, err := core.SafeRun(client, "/interface/macvlan/add", "=name="+req.Name, "=interface="+req.Parent, "=mode=private", "=disabled=no")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل إنشاء MAC-VLAN: " + err.Error()})
	}
	return c.JSON(fiber.Map{"message": "MAC-VLAN created"})
}

func AddDhcpClient(c *fiber.Ctx) error {
	type Request struct {
		Interface string `json:"interface"`
	}
	var req Request
	c.BodyParser(&req)
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	var err error
	dhcpPrint, printErr := core.SafeRun(client, "/ip/dhcp-client/print", "?interface="+req.Interface)
	if printErr == nil && dhcpPrint != nil && len(dhcpPrint.Re) > 0 {
		id := dhcpPrint.Re[0].Map[".id"]
		_, err = core.SafeRun(client, "/ip/dhcp-client/set",
			"=.id="+id, "=add-default-route=no", "=comment=TM_WAN", "=disabled=no")
	} else {
		_, err = core.SafeRun(client, "/ip/dhcp-client/add",
			"=interface="+req.Interface, "=disabled=no",
			"=add-default-route=no", "=comment=TM_WAN")
	}
	if err == nil {
		core.SafeRun(client, "/interface/list/add", "=name=WAN")
		core.SafeRun(client, "/interface/list/member/add", "=interface="+req.Interface, "=list=WAN", "=comment=TM_WAN")
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل تهيئة DHCP Client: " + err.Error()})
	}
	return c.JSON(fiber.Map{"message": "DHCP Client configured successfully"})
}

func SetupMultiWan(c *fiber.Ctx) error {
	type WanSession struct {
		User   string `json:"user"`
		Pass   string `json:"pass"`
		Weight int    `json:"weight"`
	}
	type PortConfig struct {
		Interface string       `json:"interface"`
		Type      string       `json:"type"`
		Sessions  []WanSession `json:"sessions"`
	}
	type Request struct {
		Configs      []PortConfig `json:"configs"`
		LanInterface string       `json:"lan"`
		Classifier   string       `json:"classifier"`
		Mode         string       `json:"mode"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	PurgeSASMANInternal(client)
	isECMPMode := strings.EqualFold(req.Mode, "default-route") ||
		strings.EqualFold(req.Mode, "normal") ||
		strings.EqualFold(req.Mode, "ecmp") ||
		strings.EqualFold(req.Mode, "default-route-ecmp")
	isDefaultRouteMode := isECMPMode
	dhcpDefaultRoute := "no"

	var allLines []WanLine
	wanCounter := 1

	for _, config := range req.Configs {
		if config.Type == "DHCP" {
			var err error

			dhcpPrint, printErr := core.SafeRun(client, "/ip/dhcp-client/print", "?interface="+config.Interface)
			if printErr == nil && dhcpPrint != nil && len(dhcpPrint.Re) > 0 {
				id := dhcpPrint.Re[0].Map[".id"]
				if isDefaultRouteMode {
					// First WAN: use-peer-dns=yes, subsequent WANs: use-peer-dns=no
					usePeerDNS := "yes"
					if wanCounter > 1 {
						usePeerDNS = "no"
					}
					_, err = core.SafeRun(client, "/ip/dhcp-client/set",
						"=.id="+id, "=add-default-route="+dhcpDefaultRoute, "=use-peer-dns="+usePeerDNS, "=comment=TM_WAN", "=disabled=no")
				} else {
					_, err = core.SafeRun(client, "/ip/dhcp-client/set",
						"=.id="+id, "=add-default-route=no", "=comment=TM_WAN", "=disabled=no")
				}
			} else {
				if isDefaultRouteMode {
					usePeerDNS := "yes"
					if wanCounter > 1 {
						usePeerDNS = "no"
					}
					_, err = core.SafeRun(client, "/ip/dhcp-client/add",
						"=interface="+config.Interface, "=disabled=no",
						"=add-default-route="+dhcpDefaultRoute, "=use-peer-dns="+usePeerDNS, "=comment=TM_WAN")
				} else {
					_, err = core.SafeRun(client, "/ip/dhcp-client/add",
						"=interface="+config.Interface, "=disabled=no",
						"=add-default-route=no", "=comment=TM_WAN")
				}
			}
			if err == nil {
				core.SafeRun(client, "/interface/list/add", "=name=WAN")
				core.SafeRun(client, "/interface/list/member/add", "=interface="+config.Interface, "=list=WAN", "=comment=TM_WAN")
			}
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "فشل تهيئة DHCP Client على واجهة " + config.Interface + ": " + err.Error()})
			}
			allLines = append(allLines, WanLine{
				Interface: config.Interface,
				Name:      fmt.Sprintf("WAN%d", wanCounter),
				Weight:    1,
			})
			if len(config.Sessions) > 0 {
				allLines[len(allLines)-1].Weight = normalizeWeight(config.Sessions[0].Weight)
			}
			wanCounter++
		} else {
			for i, sess := range config.Sessions {
				suffix := fmt.Sprintf("%03d", (time.Now().UnixNano()/1000000%1000)+int64(i))
				macName := "mac-" + config.Interface + "-" + suffix
				pppoeName := "pppoe-out-" + suffix

				_, err := core.SafeRun(client, "/interface/macvlan/add",
					"=name="+macName, "=interface="+config.Interface,
					"=mode=private", "=disabled=no", "=comment=TM_WAN")
				if err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "فشل إضافة MAC-VLAN على واجهة " + config.Interface + ": " + err.Error()})
				}

				// Poll to ensure the macvlan interface is fully generated before attaching PPPoE
				if !waitForInterface(client, macName) {
					return c.Status(500).JSON(fiber.Map{"error": "فشل تهيئة واجهة MAC-VLAN " + macName + " (تأخر في استجابة الراوتر)"})
				}

				pppoeDefaultRoute := "no"
				pppoeUsePeerDNS := "no"
				if isDefaultRouteMode {
					pppoeDefaultRoute = "yes"
					if wanCounter == 1 {
						pppoeUsePeerDNS = "yes"
					}
				}

				_, err = core.SafeRun(client, "/interface/pppoe-client/add",
					"=name="+pppoeName, "=user="+sess.User, "=password="+sess.Pass,
					"=interface="+macName, "=add-default-route="+pppoeDefaultRoute, "=use-peer-dns="+pppoeUsePeerDNS,
					"=disabled=no", "=comment=TM_WAN")
				if err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "فشل إضافة PPPoE Client " + pppoeName + ": " + err.Error()})
				}

				core.SafeRun(client, "/interface/list/add", "=name=WAN")
				core.SafeRun(client, "/interface/list/add", "=name=WAN-PPP")
				core.SafeRun(client, "/interface/list/member/add", "=interface="+pppoeName, "=list=WAN", "=comment=TM_WAN")
				core.SafeRun(client, "/interface/list/member/add", "=interface="+pppoeName, "=list=WAN-PPP", "=comment=TM_WAN")

				allLines = append(allLines, WanLine{
					Interface: pppoeName,
					Name:      fmt.Sprintf("WAN%d", wanCounter),
					Weight:    normalizeWeight(sess.Weight),
				})
				wanCounter++
			}
		}
	}

	preflight, err := RunWanPreflight(client, wanPreflightOptions{
		Mode:         req.Mode,
		LanInterface: req.LanInterface,
		Lines:        allLines,
		AutoFix:      true,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "preflight": preflight})
	}
	req.LanInterface = preflight.LanInterface
	allLines = preflight.Lines

	if isDefaultRouteMode {
		if err := ApplyEcmpWanInternal(client, allLines); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error(), "preflight": preflight})
		}
		return c.JSON(fiber.Map{"message": "WAN ECMP balancing configured successfully", "preflight": preflight})
	}

	if err := ApplyPccInternal(client, allLines, req.LanInterface, req.Classifier); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "preflight": preflight})
	}
	return c.JSON(fiber.Map{"message": "Multi-WAN setup complete", "preflight": preflight})
}

func ApplyPcc(c *fiber.Ctx) error {
	type Request struct {
		Lines        []WanLine `json:"lines"`
		LanInterface string    `json:"lan"`
	}
	var req Request
	c.BodyParser(&req)

	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	preflight, err := RunWanPreflight(client, wanPreflightOptions{
		Mode:         "pcc",
		LanInterface: req.LanInterface,
		Lines:        req.Lines,
		AutoFix:      true,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "preflight": preflight})
	}

	if err := ApplyPccInternal(client, preflight.Lines, preflight.LanInterface, "both-addresses"); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "preflight": preflight})
	}
	return c.JSON(fiber.Map{"message": "Applied Advanced PCC successfully", "preflight": preflight})
}

func DeletePPPoE(c *fiber.Ctx) error {
	name := c.Params("name")
	client, responded := core.ConnectOrReply(c)
	if responded {
		return nil
	}

	reply, err := core.SafeRun(client, "/interface/pppoe-client/print")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	var pppoeID string
	var parentInterface string
	for _, s := range reply.Re {
		if s.Map["name"] != name {
			continue
		}
		if !core.ShouldRemoveManagedEntry("/interface/pppoe-client", s.Map) {
			continue
		}
		pppoeID = s.Map[".id"]
		parentInterface = s.Map["interface"]
		break
	}

	if pppoeID == "" {
		return c.Status(404).JSON(fiber.Map{"error": "PPPoE not found"})
	}

	core.RemoveEntriesByMatcher(client, "/interface/list/member", func(v map[string]string) bool {
		return v["interface"] == name
	})

	_, err = core.SafeRun(client, "/interface/pppoe-client/remove", "=.id="+pppoeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل إزالة PPPoE: " + err.Error()})
	}
	if parentInterface != "" {
		core.RemoveEntriesByMatcher(client, "/interface/macvlan", func(v map[string]string) bool {
			return v["name"] == parentInterface
		})
	}
	return c.JSON(fiber.Map{"message": "PPPoE deleted"})
}

func DeleteDhcpClient(c *fiber.Ctx) error {
	id := c.Params("id")
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	// Clean up WAN list/member first
	reply, _ := core.SafeRun(client, "/ip/dhcp-client/print")
	if reply != nil {
		for _, s := range reply.Re {
			if s.Map[".id"] == id {
				iface := s.Map["interface"]
				if iface != "" {
					core.RemoveEntriesByMatcher(client, "/interface/list/member", func(v map[string]string) bool {
						return v["interface"] == iface && v["list"] == "WAN"
					})
				}
				break
			}
		}
	}

	_, err := core.SafeRun(client, "/ip/dhcp-client/remove", "=.id="+id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل إزالة DHCP Client: " + err.Error()})
	}
	return c.JSON(fiber.Map{"message": "DHCP Client removed"})
}

func PurgeSASMAN(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	purgeSASMANInternal(client, false)
	return c.JSON(fiber.Map{"message": "System purged successfully"})
}

// Private helpers
func PurgeSASMANInternal(client *routeros.Client) {
	purgeSASMANInternal(client, true)
}

func purgeSASMANInternal(client *routeros.Client, preserveAppRouting bool) {
	paths := []string{
		"/interface/pppoe-client",
		"/interface/macvlan",
		"/ip/firewall/mangle", "/ip/route", "/routing/table",
		"/interface/list/member", "/interface/list", "/ip/firewall/nat",
		"/ip/firewall/address-list", "/tool/netwatch",
		"/routing/rule", "/ip/firewall/filter",
		"/ip/dns/static",
		// NOTE: /ip/dhcp-client is intentionally excluded from purge.
		// Purging DHCP clients breaks internet on DHCP WAN lines during optimization cycles.
	}
	for _, path := range paths {
		core.RemoveEntriesByMatcher(client, path, func(values map[string]string) bool {
			if preserveAppRouting && isAppRoutingEntry(values) {
				return false
			}
			return core.ShouldRemoveManagedEntry(path, values)
		})
	}
	time.Sleep(500 * time.Millisecond)
}

func isAppRoutingEntry(values map[string]string) bool {
	comment := values["comment"]
	name := values["name"]
	list := values["list"]
	addrList := values["address-list"]
	if strings.HasPrefix(comment, "Route-") ||
		strings.HasPrefix(comment, "SNI-") ||
		strings.HasPrefix(comment, "Force DNS - ") ||
		strings.HasPrefix(comment, "BLOCK-") {
		return true
	}
	return strings.HasPrefix(name, "table-") ||
		strings.HasPrefix(list, "list-") ||
		strings.HasPrefix(addrList, "list-")
}

func ApplyDefaultRouteWanInternal(client *routeros.Client, lines []WanLine) error {
	activeLines := make([]WanLine, 0, len(lines))
	for _, line := range lines {
		line.Interface = strings.TrimSpace(line.Interface)
		if line.Interface != "" {
			activeLines = append(activeLines, line)
		}
	}
	if len(activeLines) == 0 {
		return fmt.Errorf("لا توجد خطوط WAN صالحة لتطبيق الدمج العادي")
	}

	disableFastTrackRules(client)
	core.SafeRun(client, "/ip/settings/set", "=rp-filter=no")
	core.SafeRun(client, "/ip/dns/set", "=allow-remote-requests=yes", "=servers=1.1.1.1,1.0.0.1")

	for i, line := range activeLines {
		distance := strconv.Itoa(i + 1)
		core.SafeRun(client, "/interface/list/add", "=name=WAN")
		core.SafeRun(client, "/interface/list/member/add", "=interface="+line.Interface, "=list=WAN", "=comment=TM_WAN")
		core.SafeRun(client, "/ip/firewall/nat/add",
			"=chain=srcnat", "=out-interface="+line.Interface,
			"=action=masquerade", "=comment=TM_Masq_WAN"+strconv.Itoa(i+1))

		if isInterfaceGateway(line.Interface) {
			usePeerDNS := "yes"
			if i > 0 {
				usePeerDNS = "no"
			}
			if !setPPPoEDefaultRoute(client, line.Interface, i+1, usePeerDNS) {
				core.SafeRun(client, "/ip/route/add",
					"=dst-address=0.0.0.0/0", "=gateway="+line.Interface,
					"=distance="+distance, "=comment=TM_Default_WAN"+strconv.Itoa(i+1))
			}
			continue
		}

		// First WAN uses use-peer-dns=yes, rest use use-peer-dns=no
		usePeerDNS := "yes"
		if i > 0 {
			usePeerDNS = "no"
		}

		reply, err := core.SafeRun(client, "/ip/dhcp-client/print", "?interface="+line.Interface)
		if err == nil && reply != nil && len(reply.Re) > 0 {
			id := reply.Re[0].Map[".id"]
			if id != "" {
				core.SafeRun(client, "/ip/dhcp-client/set",
					"=.id="+id,
					"=add-default-route=yes",
					"=default-route-distance="+distance,
					"=use-peer-dns="+usePeerDNS,
					"=comment=TM_WAN",
					"=disabled=no")
			}
			continue
		}

		gw, err := resolveWanGateway(client, line)
		if err != nil {
			return err
		}
		core.SafeRun(client, "/ip/route/add",
			"=dst-address=0.0.0.0/0", "=gateway="+scopedGateway(gw, line.Interface),
			"=distance="+distance, "=comment=TM_Default_WAN"+strconv.Itoa(i+1))
	}

	core.SafeRun(client, "/ip/dns/cache/flush")
	flushManagedWanConnections(client)
	return nil
}

func ApplyEcmpWanInternal(client *routeros.Client, lines []WanLine) error {
	activeLines := make([]WanLine, 0, len(lines))
	for _, line := range lines {
		line.Interface = strings.TrimSpace(line.Interface)
		if line.Interface != "" {
			activeLines = append(activeLines, line)
		}
	}
	if len(activeLines) == 0 {
		return fmt.Errorf("لا توجد خطوط WAN صالحة لتطبيق ECMP")
	}

	disableFastTrackRules(client)
	core.SafeRun(client, "/ip/settings/set", "=rp-filter=no", "=ipv4-multipath-hash-policy=l4")
	core.SafeRun(client, "/ip/dns/set", "=allow-remote-requests=yes", "=servers=1.1.1.1,1.0.0.1")

	for i, line := range activeLines {
		wanName := fmt.Sprintf("WAN%d", i+1)
		core.SafeRun(client, "/interface/list/add", "=name=WAN")
		core.SafeRun(client, "/interface/list/member/add", "=interface="+line.Interface, "=list=WAN", "=comment=TM_WAN")
		core.SafeRun(client, "/ip/firewall/nat/add",
			"=chain=srcnat", "=out-interface="+line.Interface,
			"=action=masquerade", "=comment=TM_Masq_"+wanName)

		if isInterfaceGateway(line.Interface) {
			setPPPoEDefaultRoute(client, line.Interface, 1, "no")
			continue
		}
		if reply, err := core.SafeRun(client, "/ip/dhcp-client/print", "?interface="+line.Interface); err == nil && reply != nil && len(reply.Re) > 0 {
			if id := reply.Re[0].Map[".id"]; id != "" {
				core.SafeRun(client, "/ip/dhcp-client/set",
					"=.id="+id,
					"=add-default-route=no",
					"=use-peer-dns=no",
					"=comment=TM_WAN",
					"=disabled=no")
			}
		}

		gw, err := resolveWanGateway(client, line)
		if err != nil {
			return err
		}
		core.SafeRun(client, "/ip/route/add",
			"=dst-address=0.0.0.0/0",
			"=gateway="+scopedGateway(gw, line.Interface),
			"=distance=1",
			"=check-gateway=ping",
			"=comment=TM_ECMP_"+wanName)
	}

	core.SafeRun(client, "/ip/dns/cache/flush")
	flushManagedWanConnections(client)
	return nil
}

// ApplyPccInternal builds the complete Multi-WAN PCC load-balancing ruleset.
//
// Final Mangle rule order (top to bottom in RouterOS):
//
//	[0] TM_Bypass_Local           — drop-out local-dst traffic immediately (prerouting accept)
//	[1] TM_Local_to_Local_Bypass  — drop-out LAN-to-LAN traffic
//	[2] TM_Input_WAN1..N          — mark incoming connections per WAN (input chain)
//	[N] TM_Sticky_Bank            — always pin bank connections to WAN1
//	[N] TM_PCC_PRE_WAN1..N        — distribute new connections across WANs
//	[N] TM_Route_LAN_WAN1..N      — stamp routing-mark so packets exit the right WAN
//
// Rules are appended in their final order. This is intentionally explicit:
// relying on place-before=0 proved inconsistent across RouterOS/API behavior
// and can put route-mark rules above the bypass/PCC rules, which breaks browsing.
func ApplyPccInternal(client *routeros.Client, lines []WanLine, lan string, classifier string) error {
	if classifier == "" {
		classifier = "both-addresses"
	}
	if lan == "" {
		lan = "bridge"
	}

	activeLines := make([]WanLine, 0, len(lines))
	for _, line := range lines {
		line.Interface = strings.TrimSpace(line.Interface)
		if line.Interface == "" {
			continue
		}
		line.Weight = normalizeWeight(line.Weight)
		activeLines = append(activeLines, line)
	}
	if len(activeLines) == 0 {
		return fmt.Errorf("لا توجد خطوط WAN صالحة لتطبيق PCC")
	}
	if len(activeLines) == 1 {
		activeLines[0].Weight = 1
	}
	lines = activeLines

	// Create TM_LAN early because MSS clamping and management restrictions use it.
	core.SafeRun(client, "/interface/list/add", "=name=TM_LAN", "=comment=TM_LAN")
	core.SafeRun(client, "/interface/list/member/add", "=list=TM_LAN", "=interface="+lan, "=comment=TM_LAN")
	disableFastTrackRules(client)
	core.SafeRun(client, "/ip/settings/set", "=rp-filter=no")

	// ── Address lists ────────────────────────────────────────────────────────
	subnets := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	for _, s := range subnets {
		core.SafeRun(client, "/ip/firewall/address-list/add",
			"=list=TM_Local_Subnets", "=address="+s, "=comment=TM_Bypass")
	}
	banks := []string{"cibeg.com", "ahly.net", "fawry.com"}
	for _, b := range banks {
		core.SafeRun(client, "/ip/firewall/address-list/add",
			"=list=TM_Banks", "=address="+b, "=comment=TM_Sticky")
	}
	// ── Configure DNS with DNS over HTTPS (DoH) ──────────────────────────────
	// verify-doh-cert=no prevents failures caused by router clock de-sync during startup
	core.SafeRun(client, "/ip/dns/set", "=allow-remote-requests=yes", "=servers=1.1.1.1,1.0.0.1", "=use-doh-server=https://cloudflare-dns.com/dns-query", "=verify-doh-cert=no")

	// Add static DNS records for DoH bootstrap resolution (required before DoH can resolve its own server)
	core.SafeRun(client, "/ip/dns/static/add", "=name=cloudflare-dns.com", "=address=1.1.1.1", "=comment=TM_DoH")
	core.SafeRun(client, "/ip/dns/static/add", "=name=cloudflare-dns.com", "=address=1.0.0.1", "=comment=TM_DoH")

	// ── Force DNS NAT Redirection ────────────────────────────────────────────
	// Redirect plain-text DNS traffic ONLY from local LAN clients to the router's DoH resolver.
	// Scoped to TM_Local_Subnets to prevent hijacking VPN/VLAN/Docker internal DNS traffic.
	core.SafeRun(client, "/ip/firewall/nat/add",
		"=chain=dstnat",
		"=protocol=udp",
		"=src-address-list=TM_Local_Subnets",
		"=dst-port=53",
		"=action=redirect",
		"=comment=TM_Force_DNS")
	core.SafeRun(client, "/ip/firewall/nat/add",
		"=chain=dstnat",
		"=protocol=tcp",
		"=src-address-list=TM_Local_Subnets",
		"=dst-port=53",
		"=action=redirect",
		"=comment=TM_Force_DNS")

	totalWeight := 0
	for _, l := range lines {
		totalWeight += l.Weight
	}
	if totalWeight == 0 {
		return fmt.Errorf("مجموع أوزان WAN يساوي صفر")
	}
	singleWan := len(lines) == 1

	// ── Routing tables + NAT + routes + Netwatch (order doesn't matter) ──────
	for i, line := range lines {
		wanName := fmt.Sprintf("WAN%d", i+1)
		routingTable := "to_" + wanName
		pingHost := fmt.Sprintf("1.1.1.%d", i+1)

		if !singleWan {
			core.SafeRun(client, "/routing/table/add", "=name="+routingTable, "=fib")
		}

		gw, err := resolveWanGateway(client, line)
		if err != nil {
			return err
		}

		core.SafeRun(client, "/interface/list/add", "=name=WAN")
		core.SafeRun(client, "/interface/list/member/add", "=interface="+line.Interface, "=list=WAN", "=comment=TM_WAN")
		if isInterfaceGateway(line.Interface) {
			core.SafeRun(client, "/interface/list/add", "=name=WAN-PPP")
			core.SafeRun(client, "/interface/list/member/add", "=interface="+line.Interface, "=list=WAN-PPP", "=comment=TM_WAN")
		}

		core.SafeRun(client, "/ip/firewall/nat/add",
			"=chain=srcnat", "=out-interface="+line.Interface,
			"=action=masquerade", "=comment=TM_Masq_"+wanName)
		if !hasMainDefaultRoute(client, gw) {
			core.SafeRun(client, "/ip/route/add",
				"=dst-address=0.0.0.0/0", "=gateway="+scopedGateway(gw, line.Interface),
				"=distance="+strconv.Itoa(i+1), "=comment=TM_Main_"+wanName)
		}
		if !singleWan {
			core.SafeRun(client, "/ip/route/add",
				"=dst-address="+pingHost+"/32", "=gateway="+scopedGateway(gw, line.Interface),
				"=scope=10", "=comment=TM_Rec_Host_"+wanName)
			core.SafeRun(client, "/ip/route/add",
				"=dst-address="+pingHost+"/32", "=gateway="+scopedGateway(gw, line.Interface),
				"=routing-table="+routingTable, "=scope=10", "=comment=TM_Rec_Host_"+wanName)
			core.SafeRun(client, "/ip/route/add",
				"=dst-address=0.0.0.0/0", "=gateway="+routeLookupGateway(pingHost, "main"),
				"=routing-table="+routingTable, "=check-gateway=ping", "=target-scope=12")
		}

		// Netwatch with DNS cache flush on failover events
		upScript := `/ip/dns/cache/clear; :log info "SASMAN: ` + wanName + ` is UP, DNS Cache cleared."`
		downScript := `/ip/dns/cache/clear; :log warning "SASMAN: ` + wanName + ` is DOWN, DNS Cache cleared."`
		core.SafeRun(client, "/tool/netwatch/add",
			"=host="+pingHost,
			"=interval=10s",
			"=timeout=2s",
			"=up-script="+upScript,
			"=down-script="+downScript,
			"=comment=TM_Monitor_"+wanName)
	}

	// ── Mangle rules — appended in the desired display/evaluation order ─────
	core.SafeRun(client, "/ip/firewall/mangle/add",
		"=chain=prerouting",
		"=dst-address-type=local",
		"=action=accept",
		"=comment=TM_Bypass_Local")

	core.SafeRun(client, "/ip/firewall/mangle/add",
		"=chain=prerouting",
		"=src-address-list=TM_Local_Subnets",
		"=dst-address-list=TM_Local_Subnets",
		"=action=accept",
		"=comment=TM_Local_to_Local_Bypass")

	// Clamp TCP MSS early for routed LAN clients. This fixes the classic PCC
	// symptom where ICMP works but HTTPS stalls because the WAN path MTU is lower.
	core.SafeRun(client, "/ip/firewall/mangle/add",
		"=chain=forward",
		"=in-interface-list=TM_LAN",
		"=protocol=tcp",
		"=tcp-flags=syn",
		"=action=change-mss",
		"=new-mss=clamp-to-pmtu",
		"=passthrough=yes",
		"=comment=TM_Fix_MSS")

	if !singleWan {
		for i := 0; i < len(lines); i++ {
			wanName := fmt.Sprintf("WAN%d", i+1)
			core.SafeRun(client, "/ip/firewall/mangle/add",
				"=chain=input",
				"=in-interface="+lines[i].Interface,
				"=action=mark-connection",
				"=new-connection-mark=conn_"+wanName,
				"=passthrough=yes",
				"=comment=TM_Input_"+wanName)
		}

		core.SafeRun(client, "/ip/firewall/mangle/add",
			"=chain=prerouting",
			"=src-address-list=TM_Local_Subnets",
			"=connection-mark=no-mark",
			"=dst-address-list=TM_Banks",
			"=action=mark-connection",
			"=new-connection-mark=conn_WAN1",
			"=passthrough=yes",
			"=comment=TM_Sticky_Bank")

		currentRemainder := 0
		for i := 0; i < len(lines); i++ {
			line := lines[i]
			wanName := fmt.Sprintf("WAN%d", i+1)
			comment := "TM_PCC_PRE_" + wanName
			for w := 0; w < line.Weight; w++ {
				pcc := fmt.Sprintf("%s:%d/%d", classifier, totalWeight, currentRemainder)
				core.SafeRun(client, "/ip/firewall/mangle/add",
					"=chain=prerouting",
					"=src-address-list=TM_Local_Subnets",
					"=connection-mark=no-mark",
					"=dst-address-type=!local",
					"=dst-address-list=!TM_Local_Subnets",
					"=per-connection-classifier="+pcc,
					"=action=mark-connection",
					"=new-connection-mark=conn_"+wanName,
					"=passthrough=yes",
					"=comment="+comment)
				currentRemainder++
			}
		}

		for i := 0; i < len(lines); i++ {
			wanName := fmt.Sprintf("WAN%d", i+1)
			routingTable := "to_" + wanName
			core.SafeRun(client, "/ip/firewall/mangle/add",
				"=chain=prerouting",
				"=src-address-list=TM_Local_Subnets",
				"=connection-mark=conn_"+wanName,
				"=action=mark-routing",
				"=new-routing-mark="+routingTable,
				"=passthrough=no",
				"=comment=TM_Route_LAN_"+wanName)
		}
	}

	// ── Security & Stealth Rules ─────────────────────────────────────────────

	// 1. Output-chain rules — ensure router's own responses exit via the same WAN they came in on
	// Only applies to established,related state to avoid marking fresh router-originated traffic incorrectly
	if !singleWan {
		for i := 0; i < len(lines); i++ {
			wanName := fmt.Sprintf("WAN%d", i+1)
			routingTable := "to_" + wanName
			core.SafeRun(client, "/ip/firewall/mangle/add",
				"=chain=output",
				"=connection-mark=conn_"+wanName,
				"=connection-state=established,related",
				"=action=mark-routing",
				"=new-routing-mark="+routingTable,
				"=passthrough=no",
				"=comment=TM_Route_Out_"+wanName)
		}
	}

	// 2. Change TTL of outgoing WAN packets to 64 to bypass ISP detection of multiple devices (tethering/DPI)
	// Scoped to TM_Local_Subnets only to avoid modifying router-originated control traffic (BGP/OSPF/VPN)
	for i := 0; i < len(lines); i++ {
		core.SafeRun(client, "/ip/firewall/mangle/add",
			"=chain=postrouting",
			"=out-interface="+lines[i].Interface,
			"=src-address-list=TM_Local_Subnets",
			"=action=change-ttl",
			"=new-ttl=set:64",
			"=passthrough=yes",
			"=comment=TM_Anti_DPI_TTL")
	}

	// 3. Firewall input chain baseline — ensure established/related traffic is always allowed
	// before applying the stealth drop rule to avoid breaking DHCP renew, VPN replies, and ICMP
	core.SafeRun(client, "/ip/firewall/filter/add",
		"=chain=input",
		"=in-interface-list=WAN",
		"=connection-state=established,related",
		"=action=accept",
		"=comment=TM_Accept_Established_WAN")
	core.SafeRun(client, "/ip/firewall/filter/add",
		"=chain=input",
		"=in-interface-list=WAN",
		"=connection-state=invalid",
		"=action=drop",
		"=comment=TM_Drop_Invalid_WAN")
	core.SafeRun(client, "/ip/firewall/filter/add",
		"=chain=input",
		"=in-interface-list=WAN",
		"=protocol=udp",
		"=src-port=67",
		"=dst-port=68",
		"=action=accept",
		"=comment=TM_Accept_DHCP_WAN")
	// Block external DNS queries to prevent router becoming a public resolver
	core.SafeRun(client, "/ip/firewall/filter/add",
		"=chain=input",
		"=in-interface-list=WAN",
		"=protocol=udp",
		"=dst-port=53",
		"=action=drop",
		"=comment=TM_Block_DNS_WAN")
	core.SafeRun(client, "/ip/firewall/filter/add",
		"=chain=input",
		"=in-interface-list=WAN",
		"=protocol=tcp",
		"=dst-port=53",
		"=action=drop",
		"=comment=TM_Block_DNS_WAN")
	// Drop any remaining new connections from WAN (Stealth Firewall)
	core.SafeRun(client, "/ip/firewall/filter/add",
		"=chain=input",
		"=in-interface-list=WAN",
		"=connection-state=new",
		"=action=drop",
		"=comment=TM_Block_ISP_Access")

	// 4. Restrict Neighbor Discovery (MNDP) and MAC services to TM_LAN explicitly
	// Using explicit list instead of !WAN to ensure deterministic behavior on RouterOS v7
	lanList := "TM_LAN"
	core.SafeRun(client, "/ip/neighbor/discovery-settings/set", "=discover-interface-list="+lanList)
	core.SafeRun(client, "/tool/mac-server/set", "=allowed-interface-list="+lanList)
	core.SafeRun(client, "/tool/mac-server/mac-winbox/set", "=allowed-interface-list="+lanList)
	core.SafeRun(client, "/tool/mac-server/ping/set", "=enabled=no")
	core.SafeRun(client, "/tool/bandwidth-server/set", "=enabled=no")
	core.SafeRun(client, "/ip/dns/cache/flush")
	flushManagedWanConnections(client)
	return nil
}

// LineQuality holds measurement parameters and quality score of a WAN line.
type LineQuality struct {
	Interface  string  `json:"interface"`
	LatencyMS  float64 `json:"latency_ms"`
	JitterMS   float64 `json:"jitter_ms"`
	PacketLoss float64 `json:"packet_loss"`
	Score      int     `json:"score"` // Score from 100
}

func parseRosDuration(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(s, "ms") {
		val := strings.TrimSuffix(s, "ms")
		return strconv.ParseFloat(val, 64)
	}
	// Try parsing hh:mm:ss.xxx or mm:ss.xxx
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		h, _ := strconv.ParseFloat(parts[0], 64)
		m, _ := strconv.ParseFloat(parts[1], 64)
		sPart := parts[2]
		sec, _ := strconv.ParseFloat(sPart, 64)
		return (h*3600 + m*60 + sec) * 1000, nil
	}
	d, err := time.ParseDuration(s)
	if err == nil {
		return float64(d.Milliseconds()), nil
	}
	return strconv.ParseFloat(s, 64)
}

func quickPing(host string) error {
	dst, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout("ip4:icmp", dst.String(), 400*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()

	m := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: []byte("SASMAN_PROBE"),
		},
	}
	b, err := m.Marshal(nil)
	if err != nil {
		return err
	}
	_, err = conn.Write(b)
	if err != nil {
		return err
	}

	reply := make([]byte, 1500)
	conn.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	_, err = conn.Read(reply)
	return err
}

func probeLineQuality(client *routeros.Client, line WanLine, pingHost string) LineQuality {
	const totalPackets = 10
	var latencies []float64
	lostPackets := 0

	// 0. First check if the interface is actually running
	// This prevents routing leaks when interface is down but still configured
	interfaceReply, err := core.SafeRun(client, "/interface/print", "?name="+line.Interface)
	if err == nil && interfaceReply != nil && len(interfaceReply.Re) > 0 {
		// If the interface is not running, consider it completely down
		if interfaceReply.Re[0].Map["running"] != "true" {
			return LineQuality{PacketLoss: 100, Score: 0}
		}
	} else {
		// If we can't find the interface at all, consider it down
		return LineQuality{PacketLoss: 100, Score: 0}
	}

	// 1. Run ping directly on the RouterOS device scoped to the specific WAN interface.
	// Using /ping with =interface= is lighter on CPU than traceroute (especially with many WANs)
	// while still providing proper isolation from the default routing table.
	reply, err := core.SafeRun(client, "/ping",
		"=address="+pingHost,
		"=interface="+line.Interface,
		"=count=10",
		"=interval=0.1",
		"=timeout=400ms")
	if err == nil && reply != nil {
		totalCount := 0
		for _, re := range reply.Re {
			totalCount++
			if re.Map["status"] == "timeout" || re.Map["time"] == "" {
				lostPackets++
				continue
			}
			ms, parseErr := parseRosDuration(re.Map["time"])
			if parseErr != nil {
				lostPackets++
				continue
			}
			latencies = append(latencies, ms)
		}
		if totalCount == 0 {
			totalCount = totalPackets
		}
		lossPercent := (float64(lostPackets) / float64(totalCount)) * 100
		if len(latencies) == 0 {
			return LineQuality{PacketLoss: 100, Score: 0}
		}
		var sum, maxL, minL float64
		minL = 99999.0
		for _, l := range latencies {
			sum += l
			if l > maxL {
				maxL = l
			}
			if l < minL {
				minL = l
			}
		}
		avgLatency := sum / float64(len(latencies))
		jitter := maxL - minL
		score := 100 - int(lossPercent*0.6) - int(avgLatency*0.2) - int(jitter*0.2)
		if score < 0 {
			score = 0
		}
		return LineQuality{
			LatencyMS:  math.Round(avgLatency*100) / 100,
			JitterMS:   math.Round(jitter*100) / 100,
			PacketLoss: lossPercent,
			Score:      score,
		}
	}

	// 2. Fallback to Go Local ICMP Ping (without interface binding - may leak to default route)
	// Only use this if RouterOS ping fails completely
	for i := 0; i < totalPackets; i++ {
		start := time.Now()
		err := quickPing(pingHost)
		if err != nil {
			lostPackets++
		} else {
			duration := time.Since(start).Seconds() * 1000
			latencies = append(latencies, duration)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(latencies) == 0 {
		return LineQuality{PacketLoss: 100, Score: 0}
	}

	var sum, max, min float64
	min = 99999.0
	for _, l := range latencies {
		sum += l
		if l > max {
			max = l
		}
		if l < min {
			min = l
		}
	}
	avgLatency := sum / float64(len(latencies))
	jitter := max - min
	lossPercent := (float64(lostPackets) / float64(totalPackets)) * 100

	score := 100 - int(lossPercent*0.6) - int(avgLatency*0.2) - int(jitter*0.2)
	if score < 0 {
		score = 0
	}

	return LineQuality{
		LatencyMS:  math.Round(avgLatency*100) / 100,
		JitterMS:   math.Round(jitter*100) / 100,
		PacketLoss: lossPercent,
		Score:      score,
	}
}

// OptimizeWanQuality is the main HTTP controller handler for the WAN on-demand optimization.
func OptimizeWanQuality(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	type Request struct {
		LanInterface string    `json:"lan"`
		Classifier   string    `json:"classifier"`
		Lines        []WanLine `json:"lines"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		// Proceed with empty defaults if parsing fails
	}

	// 1. Auto-discover active WAN lines if request didn't specify them
	if len(req.Lines) == 0 {
		natPrint, err := core.SafeRun(client, "/ip/firewall/nat/print")
		if err == nil && natPrint != nil {
			discoveredMap := make(map[string]string) // name -> interface
			for _, re := range natPrint.Re {
				comment := re.Map["comment"]
				if strings.HasPrefix(comment, "TM_Masq_WAN") {
					wanName := strings.TrimPrefix(comment, "TM_Masq_") // WAN1, WAN2...
					outInterface := re.Map["out-interface"]
					discoveredMap[wanName] = outInterface
				}
			}

			// Sort by numeric order of WAN suffix
			var names []string
			for name := range discoveredMap {
				names = append(names, name)
			}
			sort.Slice(names, func(i, j int) bool {
				numI := extractNumber(names[i])
				numJ := extractNumber(names[j])
				return numI < numJ
			})

			for _, name := range names {
				req.Lines = append(req.Lines, WanLine{
					Interface: discoveredMap[name],
					Name:      name,
					Weight:    1, // Default weight fallback
				})
			}
		}
	}

	if len(req.Lines) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "لم يتم العثور على خطوط WAN نشطة لتجهيزها أو تهيئتها",
		})
	}

	// 2. Auto-discover active classifier from mangle rules if not provided
	if req.Classifier == "" {
		manglePrint, err := core.SafeRun(client, "/ip/firewall/mangle/print", "?comment=TM_PCC_PRE_WAN1")
		if err != nil || manglePrint == nil || len(manglePrint.Re) == 0 {
			manglePrint, err = core.SafeRun(client, "/ip/firewall/mangle/print", "?comment=TM_PCC_WAN1")
		}
		if err == nil && manglePrint != nil && len(manglePrint.Re) > 0 {
			pccVal := manglePrint.Re[0].Map["per-connection-classifier"]
			if pccVal != "" {
				parts := strings.Split(pccVal, ":")
				if len(parts) > 0 {
					req.Classifier = parts[0]
				}
			}
		}
		if req.Classifier == "" || req.Classifier == "both-addresses-and-ports" {
			req.Classifier = "both-addresses"
		}
	}

	if req.LanInterface == "" {
		req.LanInterface = "bridge"
	}

	// 3. Perform parallel probing using goroutines to minimize latency
	type probeResult struct {
		index   int
		quality LineQuality
	}
	ch := make(chan probeResult, len(req.Lines))

	for i, line := range req.Lines {
		go func(idx int, ln WanLine) {
			pingHost := fmt.Sprintf("1.1.1.%d", idx+1)
			q := probeLineQuality(client, ln, pingHost)
			q.Interface = ln.Interface
			ch <- probeResult{index: idx, quality: q}
		}(i, line)
	}

	results := make([]LineQuality, len(req.Lines))
	for i := 0; i < len(req.Lines); i++ {
		res := <-ch
		results[res.index] = res.quality
	}

	var optimizedLines []WanLine
	var report []map[string]interface{}

	for i, quality := range results {
		line := req.Lines[i]

		// Dynamic optimization: adjust weights based on health rating
		newWeight := 1
		var actionMessage string
		if quality.Score > 85 {
			newWeight = 3
			actionMessage = "خط ممتاز جداً (تم رفع حصة الدمج إلى 3)"
		} else if quality.Score > 60 {
			newWeight = 2
			actionMessage = "خط متوسط (تم تحديد حصة الدمج بـ 2)"
		} else if quality.Score <= 30 || quality.PacketLoss > 20 {
			newWeight = 0
			actionMessage = "خط يعاني من تدهور حاد أو فقدان حزم (تم عزله مؤقتاً بوزن 0)"
		} else {
			newWeight = 1
			actionMessage = "خط مستقر (تم تحديد حصة الدمج بـ 1)"
		}

		line.Weight = newWeight
		optimizedLines = append(optimizedLines, line)

		report = append(report, map[string]interface{}{
			"interface":   line.Interface,
			"name":        line.Name,
			"score":       quality.Score,
			"latency_ms":  quality.LatencyMS,
			"jitter_ms":   quality.JitterMS,
			"packet_loss": quality.PacketLoss,
			"new_weight":  newWeight,
			"message":     actionMessage,
		})
	}

	// 4. Filter out weight=0 lines — these are degraded/dead WANs.
	// Including them in PCC causes remainder math errors and zombie routing tables.
	var activeLines []WanLine
	for _, l := range optimizedLines {
		if l.Weight > 0 {
			activeLines = append(activeLines, l)
		}
	}
	// Fallback: if ALL lines are degraded, keep the best-scoring one at weight=1
	// to avoid a total routing blackout.
	if len(activeLines) == 0 && len(optimizedLines) > 0 {
		bestIdx := 0
		for j := 1; j < len(results); j++ {
			if results[j].Score > results[bestIdx].Score {
				bestIdx = j
			}
		}
		fallback := optimizedLines[bestIdx]
		fallback.Weight = 1
		activeLines = []WanLine{fallback}
	}

	preflight, err := RunWanPreflight(client, wanPreflightOptions{
		Mode:         "pcc-optimize",
		LanInterface: req.LanInterface,
		Lines:        activeLines,
		AutoFix:      true,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "report": report, "preflight": preflight})
	}
	req.LanInterface = preflight.LanInterface
	activeLines = preflight.Lines

	// 5. Re-apply PCC rules with the new optimized weights (active lines only)
	PurgeSASMANInternal(client)
	if err := ApplyPccInternal(client, activeLines, req.LanInterface, req.Classifier); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "report": report, "preflight": preflight})
	}

	return c.JSON(fiber.Map{
		"status":    "success",
		"message":   "تمت إعادة موازنة وتحسين جودة الخطوط بنجاح كلي",
		"report":    report,
		"preflight": preflight,
	})
}

// helper to extract digit/number from string
func extractNumber(s string) int {
	var numStr strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			numStr.WriteByte(s[i])
		}
	}
	val, err := strconv.Atoi(numStr.String())
	if err != nil {
		return 0
	}
	return val
}
