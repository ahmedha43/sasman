package relay

import (
	"fmt"
	"log"
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

	// 2. Ensure DNS settings allow remote requests
	core.SafeRun(client, "/ip/dns/set",
		"=allow-remote-requests=yes",
		"=address-list-extra-time=1w3d")

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

// reconcileServiceRules queries existing rules for a service, compares them with desired rules,
// and ensures exactly one valid rule exists for each domain/port without duplicates.
func reconcileServiceRules(client *routeros.Client, svc ServiceDefinition, interceptorPort int) {
	listName := "SASMAN_RELAY_" + strings.ToUpper(svc.ID)
	comment := "SASMAN-Relay-" + svc.ID

	// ─── A. DNS Static FWD Rules Reconcile ────────────────────────────────────
	desiredDNS := make(map[string]bool)
	for _, domain := range svc.Domains {
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
	for _, domain := range svc.Domains {
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
			if tlsHost == "" || id == "" {
				continue
			}

			// If duplicate or not desired, remove it
			if _, exists := existingRAW[tlsHost]; exists || !desiredRAW[tlsHost] {
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
				"=src-address-list=!TM_Local_Subnets",
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

	existingNAT := make(map[string]string) // "dstPort->toPorts" -> first .id seen
	natReply, err := client.Run("/ip/firewall/nat/print", "?comment="+comment)
	if err == nil && natReply != nil {
		for _, re := range natReply.Re {
			dstPort := re.Map["dst-port"]
			toPorts := re.Map["to-ports"]
			id := re.Map[".id"]
			if dstPort == "" || id == "" {
				continue
			}

			key := fmt.Sprintf("%s->%s", dstPort, toPorts)
			// If duplicate or not desired, remove it
			if _, exists := existingNAT[key]; exists || !desiredNAT[key] {
				_, _ = client.Run("/ip/firewall/nat/remove", "=.id="+id)
			} else {
				existingNAT[key] = id
			}
		}
	}

	// Add missing NAT redirect rules
	for _, p := range ports {
		key := fmt.Sprintf("%d->%d", p, interceptorPort)
		if _, exists := existingNAT[key]; !exists {
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
