package relay

import (
	"fmt"
	"log"
	"net"
	"strings"

	"mikrotik-manager/pkg/core"

	"github.com/go-routeros/routeros/v3"
)

// SyncMikroTikRelayRules intelligently queries, compares, reconciles, and deduplicates
// MikroTik RouterOS firewall and DNS rules for all active services without creating duplicate rules.
func SyncMikroTikRelayRules(client *routeros.Client, services []ServiceDefinition, interceptorPort int) error {
	if client == nil {
		return fmt.Errorf("routeros client is nil")
	}

	if interceptorPort <= 0 {
		interceptorPort = 18443
	}

	// 1. Reconcile Local Subnets (TM_Local_Subnets) without duplicates
	reconcileLocalSubnets(client)

	// 2. Ensure DNS settings allow remote requests & force local DNS capture for PPPoE/Broadband clients
	core.SafeRun(client, "/ip/dns/set",
		"=allow-remote-requests=yes",
		"=address-list-extra-time=1w3d")
	reconcileDNSNATRules(client)

	// 3. Build active service map
	activeServiceMap := make(map[string]ServiceDefinition)
	for _, svc := range services {
		if svc.Enabled && len(svc.Domains) > 0 {
			activeServiceMap[svc.ID] = svc
		}
	}

	// 4. Clean up any orphaned relay rules on the router whose services are disabled or removed
	cleanupOrphanedRelayRules(client, activeServiceMap)

	// 5. For each active service, query existing rules, compare against desired state, and reconcile
	for _, svc := range activeServiceMap {
		reconcileServiceRules(client, svc, interceptorPort)
	}

	log.Printf("[Relay MikroTik Sync] Reconciled and deduplicated %d active services on RouterOS", len(activeServiceMap))
	return nil
}

// reconcileDNSNATRules ensures local PPPoE/Broadband users' DNS requests are captured by RouterOS DNS FWD rules
func reconcileDNSNATRules(client *routeros.Client) {
	comment := "SASMAN-Force-DNS"
	desiredProtocols := []string{"udp", "tcp"}

	for _, proto := range desiredProtocols {
		reply, err := client.Run("/ip/firewall/nat/print",
			"?chain=dstnat",
			"?protocol="+proto,
			"?dst-port=53",
			"?comment="+comment,
		)
		if err == nil && reply != nil && len(reply.Re) > 0 {
			continue // Already exists
		}

		core.SafeRun(client, "/ip/firewall/nat/add",
			"=chain=dstnat",
			"=protocol="+proto,
			"=dst-port=53",
			"=src-address-list=TM_Local_Subnets",
			"=action=redirect",
			"=to-ports=53",
			"=comment="+comment,
		)
	}
}

// reconcileLocalSubnets queries existing TM_Local_Subnets entries, eliminates duplicates, and adds missing ones
func reconcileLocalSubnets(client *routeros.Client) {
	desiredSubnets := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	existingSubnets := make(map[string]string) // address -> first .id seen

	reply, err := client.Run("/ip/firewall/address-list/print", "?list=TM_Local_Subnets")
	if err == nil && reply != nil {
		for _, re := range reply.Re {
			addr := re.Map["address"]
			id := re.Map[".id"]
			if addr == "" || id == "" {
				continue
			}
			if _, exists := existingSubnets[addr]; exists {
				// Duplicate entry found! Remove it to keep router clean
				_, _ = client.Run("/ip/firewall/address-list/remove", "=.id="+id)
			} else {
				existingSubnets[addr] = id
			}
		}
	}

	// Add only missing subnets
	for _, subnet := range desiredSubnets {
		if _, exists := existingSubnets[subnet]; !exists {
			core.SafeRun(client, "/ip/firewall/address-list/add",
				"=list=TM_Local_Subnets",
				"=address="+subnet,
				"=comment=Auto-SASMAN-Bypass")
		}
	}
}

// cleanupOrphanedRelayRules queries all rules with SASMAN-Relay- comment and removes those for inactive services
func cleanupOrphanedRelayRules(client *routeros.Client, activeServices map[string]ServiceDefinition) {
	paths := []string{
		"/ip/firewall/nat",
		"/ip/firewall/raw",
		"/ip/dns/static",
		"/ip/firewall/address-list",
	}

	for _, path := range paths {
		reply, err := client.Run(path + "/print")
		if err != nil || reply == nil {
			continue
		}

		for _, re := range reply.Re {
			comment := re.Map["comment"]
			if !strings.HasPrefix(comment, "SASMAN-Relay-") {
				continue
			}

			svcID := strings.TrimPrefix(comment, "SASMAN-Relay-")
			if _, isActive := activeServices[svcID]; !isActive {
				id := re.Map[".id"]
				if id != "" {
					_, _ = client.Run(path+"/remove", "=.id="+id)
				}
			}
		}
	}
}

// normalizeDomains splits any multi-line or comma-separated strings into individual clean domains
func normalizeDomains(raw []string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, d := range raw {
		fields := strings.FieldsFunc(d, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ',' || r == ';' || r == ' ' || r == '\t'
		})
		for _, f := range fields {
			clean := strings.ToLower(strings.TrimSpace(f))
			if clean != "" && !seen[clean] {
				seen[clean] = true
				result = append(result, clean)
			}
		}
	}
	return result
}

// reconcileServiceRules queries existing rules for a service, compares them with desired rules,
// and ensures exactly one valid rule exists for each domain/port without duplicates.
func reconcileServiceRules(client *routeros.Client, svc ServiceDefinition, interceptorPort int) {
	listName := "SASMAN_RELAY_" + strings.ToUpper(svc.ID)
	comment := "SASMAN-Relay-" + svc.ID

	cleanDomains := normalizeDomains(svc.Domains)

	// ─── A. DNS Static FWD Rules Reconcile ────────────────────────────────────
	desiredDNS := make(map[string]bool)
	for _, domain := range cleanDomains {
		cleanDomain := strings.TrimPrefix(domain, "*.")
		desiredDNS[cleanDomain] = true
	}

	existingDNS := make(map[string]string) // name -> first .id seen
	dnsReply, err := client.Run("/ip/dns/static/print", "?comment="+comment)
	if err == nil && dnsReply != nil {
		for _, re := range dnsReply.Re {
			name := re.Map["name"]
			id := re.Map[".id"]
			if name == "" || id == "" {
				continue
			}

			// If duplicate or not in desired list, remove it
			if _, exists := existingDNS[name]; exists || !desiredDNS[name] {
				_, _ = client.Run("/ip/dns/static/remove", "=.id="+id)
			} else {
				existingDNS[name] = id
			}
		}
	}

	// Add missing DNS rules
	for domain := range desiredDNS {
		if _, exists := existingDNS[domain]; !exists {
			core.SafeRun(client, "/ip/dns/static/add",
				"=name="+domain,
				"=type=FWD",
				"=forward-to=8.8.8.8",
				"=address-list="+listName,
				"=match-subdomain=yes",
				"=comment="+comment)
		}
	}

	// ─── B. RAW TLS-Host Prerouting Rules Reconcile ───────────────────────────
	desiredRAW := make(map[string]bool)
	for _, domain := range cleanDomains {
		cleanDomain := strings.TrimPrefix(domain, "*.")
		desiredRAW[cleanDomain] = true
		if strings.HasPrefix(domain, "*.") {
			desiredRAW["*."+cleanDomain] = true
		}
	}

	existingRAW := make(map[string]string) // tls-host -> first .id seen
	rawReply, err := client.Run("/ip/firewall/raw/print", "?comment="+comment)
	if err == nil && rawReply != nil {
		for _, re := range rawReply.Re {
			tlsHost := re.Map["tls-host"]
			id := re.Map[".id"]
			srcList := re.Map["src-address-list"]
			if tlsHost == "" || id == "" {
				continue
			}

			// If duplicate, not desired, or has the old inverted '!TM_Local_Subnets', remove it
			if _, exists := existingRAW[tlsHost]; exists || !desiredRAW[tlsHost] || srcList == "!TM_Local_Subnets" {
				_, _ = client.Run("/ip/firewall/raw/remove", "=.id="+id)
			} else {
				existingRAW[tlsHost] = id
			}
		}
	}

	// Add missing RAW rules
	for tlsHost := range desiredRAW {
		if _, exists := existingRAW[tlsHost]; !exists {
			core.SafeRun(client, "/ip/firewall/raw/add",
				"=chain=prerouting",
				"=protocol=tcp",
				"=dst-port=443",
				"=action=add-dst-to-address-list",
				"=address-list="+listName,
				"=tls-host="+tlsHost,
				"=src-address-list=TM_Local_Subnets",
				"=dst-address-list=!TM_Local_Subnets",
				"=comment="+comment)
		}
	}

	// ─── C. DST-NAT Redirect Rules Reconcile ──────────────────────────────────
	ports := svc.Ports
	if len(ports) == 0 {
		ports = []int{443}
	}

	desiredNAT := make(map[string]bool)
	for _, p := range ports {
		key := fmt.Sprintf("%d->%d", p, interceptorPort)
		desiredNAT[key] = true
	}

	agentIP := detectAgentTargetAddress(client)

	existingNAT := make(map[string]string) // "dstPort->toPorts" -> first .id seen
	natReply, err := client.Run("/ip/firewall/nat/print", "?comment="+comment)
	if err == nil && natReply != nil {
		for _, re := range natReply.Re {
			dstPort := re.Map["dst-port"]
			toPorts := re.Map["to-ports"]
			toAddresses := re.Map["to-addresses"]
			action := re.Map["action"]
			id := re.Map[".id"]
			if dstPort == "" || id == "" {
				continue
			}

			key := fmt.Sprintf("%s->%s", dstPort, toPorts)
			// If duplicate, not desired, or old 'redirect' without to-addresses, remove it
			if _, exists := existingNAT[key]; exists || !desiredNAT[key] || action == "redirect" || (agentIP != "" && toAddresses != agentIP) {
				_, _ = client.Run("/ip/firewall/nat/remove", "=.id="+id)
			} else {
				existingNAT[key] = id
			}
		}
	}

	// Add missing NAT redirect/dst-nat rules pointing to container/agent IP
	for _, p := range ports {
		key := fmt.Sprintf("%d->%d", p, interceptorPort)
		if _, exists := existingNAT[key]; !exists {
			if agentIP != "" && agentIP != "127.0.0.1" {
				core.SafeRun(client, "/ip/firewall/nat/add",
					"=chain=dstnat",
					"=protocol=tcp",
					fmt.Sprintf("=dst-port=%d", p),
					"=dst-address-list="+listName,
					"=src-address-list=TM_Local_Subnets",
					"=action=dst-nat",
					"=to-addresses="+agentIP,
					fmt.Sprintf("=to-ports=%d", interceptorPort),
					"=comment="+comment)
			} else {
				core.SafeRun(client, "/ip/firewall/nat/add",
					"=chain=dstnat",
					"=protocol=tcp",
					fmt.Sprintf("=dst-port=%d", p),
					"=dst-address-list="+listName,
					"=src-address-list=TM_Local_Subnets",
					"=action=redirect",
					fmt.Sprintf("=to-ports=%d", interceptorPort),
					"=comment="+comment)
			}
		}
	}
}

// RemoveMikroTikRelayRules cleans up rules for a specific service
func RemoveMikroTikRelayRules(client *routeros.Client, serviceID string) {
	if client == nil {
		return
	}

	comment := "SASMAN-Relay-" + serviceID

	removeWithComment := func(path string) {
		reply, err := client.Run(path+"/print", "?comment="+comment)
		if err != nil || reply == nil {
			return
		}
		for _, re := range reply.Re {
			id := re.Map[".id"]
			if id != "" {
				_, _ = client.Run(path+"/remove", "=.id="+id)
			}
		}
	}

	removeWithComment("/ip/firewall/nat")
	removeWithComment("/ip/firewall/raw")
	removeWithComment("/ip/dns/static")
	removeWithComment("/ip/firewall/address-list")
}

func detectAgentTargetAddress(client *routeros.Client) string {
	// 1. Check local network interfaces for active container / LAN IP
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				ipStr := ipnet.IP.String()
				// Match Container VETH (172.17.0.X) or local router subnet (192.168.X.X / 10.X.X.X)
				if strings.HasPrefix(ipStr, "172.17.0.") || strings.HasPrefix(ipStr, "192.168.10.") || strings.HasPrefix(ipStr, "192.168.") || strings.HasPrefix(ipStr, "10.") {
					return ipStr
				}
			}
		}
	}

	// 2. Query router for veth-sasman address
	reply, err := client.Run("/interface/veth/print")
	if err == nil && reply != nil {
		for _, re := range reply.Re {
			addr := re.Map["address"]
			if addr != "" {
				return strings.Split(addr, "/")[0]
			}
		}
	}

	return "172.17.0.2"
}
