package wan

import (
	"fmt"
	"time"

	"mikrotik-manager/pkg/core"
	"mikrotik-manager/pkg/routing"

	"github.com/go-routeros/routeros/v3"
	"github.com/gofiber/fiber/v2"
)

type WanLine struct {
	Interface string `json:"interface"`
	Name      string `json:"name"`
	Gateway   string `json:"gateway"`
	Weight    int    `json:"weight"`
}

func GetInterfaces(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	reply, _ := core.SafeRun(client, "/interface/print")

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

	reply, _ := core.SafeRun(client, "/interface/pppoe-client/print")

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

	reply, _ := core.SafeRun(client, "/ip/dhcp-client/print")

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
		core.SafeRun(client, "/interface/macvlan/add", "=name="+targetInterface, "=interface="+req.Interface, "=mode=private", "=disabled=no", "=comment=TM_WAN")
	}

	core.SafeRun(client, "/interface/pppoe-client/add",
		"=name="+req.Name, "=user="+req.User, "=password="+req.Password, "=interface="+targetInterface,
		"=add-default-route=no", "=use-peer-dns=yes", "=disabled=no", "=comment=TM_WAN")

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

	core.SafeRun(client, "/interface/macvlan/add", "=name="+req.Name, "=interface="+req.Parent, "=mode=private", "=disabled=no")
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

	core.SafeRun(client, "/ip/dhcp-client/add", "=interface="+req.Interface, "=disabled=no", "=add-default-route=no", "=comment=TM_WAN")
	return c.JSON(fiber.Map{"message": "DHCP Client added"})
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

	var allLines []WanLine
	wanCounter := 1

	for _, config := range req.Configs {
		if config.Type == "DHCP" {
			core.SafeRun(client, "/ip/dhcp-client/add", "=interface="+config.Interface, "=disabled=no", "=add-default-route=no", "=comment=TM_WAN")
			allLines = append(allLines, WanLine{
				Interface: config.Interface,
				Name:      fmt.Sprintf("WAN%d", wanCounter),
				Weight:    config.Sessions[0].Weight,
			})
			wanCounter++
		} else {
			for i, sess := range config.Sessions {
				suffix := fmt.Sprintf("%03d", (time.Now().UnixNano()/1000000%1000)+int64(i))
				macName := "mac-" + config.Interface + "-" + suffix
				pppoeName := "pppoe-out-" + suffix

				core.SafeRun(client, "/interface/macvlan/add", "=name="+macName, "=interface="+config.Interface, "=mode=private", "=disabled=no", "=comment=TM_WAN")
				time.Sleep(500 * time.Millisecond)

				core.SafeRun(client, "/interface/pppoe-client/add",
					"=name="+pppoeName, "=user="+sess.User, "=password="+sess.Pass, "=interface="+macName,
					"=add-default-route=no", "=use-peer-dns=yes", "=disabled=no", "=comment=TM_WAN")

				core.SafeRun(client, "/interface/list/member/add", "=interface="+pppoeName, "=list=WAN", "=comment=TM_WAN")
				core.SafeRun(client, "/interface/list/member/add", "=interface="+pppoeName, "=list=WAN-PPP", "=comment=TM_WAN")

				allLines = append(allLines, WanLine{
					Interface: pppoeName,
					Name:      fmt.Sprintf("WAN%d", wanCounter),
					Weight:    sess.Weight,
				})
				wanCounter++
			}
		}
	}

	ApplyPccInternal(client, allLines, req.LanInterface, req.Classifier)
	return c.JSON(fiber.Map{"message": "Multi-WAN setup complete"})
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

	ApplyPccInternal(client, req.Lines, req.LanInterface, "both-addresses-and-ports")
	return c.JSON(fiber.Map{"message": "Applied Advanced PCC successfully"})
}

func DeletePPPoE(c *fiber.Ctx) error {
	name := c.Params("name")
	client, responded := core.ConnectOrReply(c)
	if responded {
		return nil
	}

	reply, _ := core.SafeRun(client, "/interface/pppoe-client/print")
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

	core.SafeRun(client, "/interface/remove", "=.id="+pppoeID)
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

	core.SafeRun(client, "/ip/dhcp-client/remove", "=numbers="+id)
	return c.JSON(fiber.Map{"message": "DHCP Client removed"})
}

func PurgeSASMAN(c *fiber.Ctx) error {
	client, responded := core.RestrictedConnectOrReply(c)
	if responded {
		return nil
	}

	PurgeSASMANInternal(client)
	return c.JSON(fiber.Map{"message": "System purged successfully"})
}

// Private helpers
func PurgeSASMANInternal(client *routeros.Client) {
	paths := []string{"/ip/firewall/mangle", "/ip/route", "/routing/table", "/interface/list/member", "/ip/firewall/nat", "/ip/firewall/address-list", "/tool/netwatch", "/routing/rule", "/ip/dhcp-client"}
	for _, path := range paths {
		core.RemoveEntriesByMatcher(client, path, func(values map[string]string) bool {
			return core.ShouldRemoveManagedEntry(path, values)
		})
	}
	time.Sleep(500 * time.Millisecond)
	interfacePaths := []string{"/interface/pppoe-client", "/interface/macvlan"}
	for _, path := range interfacePaths {
		core.RemoveEntriesByMatcher(client, path, func(values map[string]string) bool {
			return core.ShouldRemoveManagedEntry(path, values)
		})
	}
}

func ApplyPccInternal(client *routeros.Client, lines []WanLine, lan string, classifier string) {
	if classifier == "" {
		classifier = "both-addresses"
	}
	subnets := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	for _, s := range subnets {
		core.SafeRun(client, "/ip/firewall/address-list/add", "=list=TM_Local_Subnets", "=address="+s, "=comment=TM_Bypass")
	}
	banks := []string{"cibeg.com", "ahly.net", "fawry.com"}
	for _, b := range banks {
		core.SafeRun(client, "/ip/firewall/address-list/add", "=list=TM_Banks", "=address="+b, "=comment=TM_Sticky")
	}
	core.SafeRun(client, "/ip/dns/set", "=allow-remote-requests=yes", "=servers=8.8.8.8,1.1.1.1")
	core.SafeRun(client, "/ip/firewall/mangle/add", "=chain=prerouting", "=dst-address-type=local", "=action=accept", "=comment=TM_Bypass_Local")
	core.SafeRun(client, "/ip/firewall/mangle/add", "=chain=prerouting", "=src-address-list=TM_Local_Subnets", "=dst-address-list=TM_Local_Subnets", "=action=accept", "=comment=TM_Local_to_Local_Bypass")
	core.SafeRun(client, "/ip/firewall/mangle/add", "=chain=prerouting", "=src-address-list=TM_Local_Subnets", "=connection-mark=no-mark", "=dst-address-list=TM_Banks", "=action=mark-connection", "=new-connection-mark=conn_WAN1", "=passthrough=yes", "=comment=TM_Sticky_Bank")

	totalWeight := 0
	for _, l := range lines {
		totalWeight += l.Weight
	}
	if totalWeight == 0 {
		return
	}

	currentRemainder := 0
	for i, line := range lines {
		wanName := fmt.Sprintf("WAN%d", i+1)
		routingTable := "to_" + wanName
		pingHost := fmt.Sprintf("1.1.1.%d", i+1)
		core.SafeRun(client, "/routing/table/add", "=name="+routingTable, "=fib")
		gw := line.Gateway
		if gw == "" {
			gw = routing.ResolveGateway(client, line.Interface)
		}
		core.SafeRun(client, "/ip/firewall/mangle/add", "=chain=input", "=in-interface="+line.Interface, "=action=mark-connection", "=new-connection-mark=conn_"+wanName, "=passthrough=yes", "=comment=TM_Input_"+wanName)
		for w := 0; w < line.Weight; w++ {
			pcc := fmt.Sprintf("%s:%d/%d", classifier, totalWeight, currentRemainder)
			core.SafeRun(client, "/ip/firewall/mangle/add", "=chain=prerouting", "=src-address-list=TM_Local_Subnets", "=connection-mark=no-mark", "=dst-address-type=!local", "=dst-address-list=!TM_Local_Subnets", "=per-connection-classifier="+pcc, "=action=mark-connection", "=new-connection-mark=conn_"+wanName, "=passthrough=yes", "=comment=TM_PCC_PRE_"+wanName)
			currentRemainder++
		}
		core.SafeRun(client, "/ip/firewall/mangle/add", "=chain=prerouting", "=src-address-list=TM_Local_Subnets", "=connection-mark=conn_"+wanName, "=action=mark-routing", "=new-routing-mark="+routingTable, "=passthrough=no", "=comment=TM_Route_LAN_"+wanName)
		core.SafeRun(client, "/ip/firewall/nat/add", "=chain=srcnat", "=out-interface="+line.Interface, "=action=masquerade", "=comment=TM_Masq_"+wanName)
		core.SafeRun(client, "/ip/route/add", "=dst-address="+pingHost+"/32", "=gateway="+gw, "=scope=10", "=check-gateway=ping", "=comment=TM_Rec_Host_"+wanName)
		core.SafeRun(client, "/ip/route/add", "=dst-address=0.0.0.0/0", "=gateway="+pingHost, "=routing-table="+routingTable, "=target-scope=12")
		core.SafeRun(client, "/tool/netwatch/add", "=host="+pingHost, "=interval=10s", "=timeout=2s", "=comment=TM_Monitor_"+wanName)
	}
}
