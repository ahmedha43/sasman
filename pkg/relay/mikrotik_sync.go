package relay

import (
	"fmt"
	"log"
	"strings"

	"mikrotik-manager/pkg/core"

	"github.com/go-routeros/routeros/v3"
)

// SyncMikroTikRelayRules updates MikroTik RouterOS firewall and DNS rules for the active services
func SyncMikroTikRelayRules(client *routeros.Client, services []ServiceDefinition, interceptorPort int) error {
	if client == nil {
		return fmt.Errorf("routeros client is nil")
	}

	if interceptorPort <= 0 {
		interceptorPort = 18443
	}

	// 1. Ensure Local Subnets list exists to avoid looping
	localSubnets := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	for _, subnet := range localSubnets {
		core.SafeRun(client, "/ip/firewall/address-list/add",
			"=list=TM_Local_Subnets",
			"=address="+subnet,
			"=comment=Auto-SASMAN-Bypass")
	}

	// 2. Allow remote requests on DNS so FWD rules work
	core.SafeRun(client, "/ip/dns/set",
		"=allow-remote-requests=yes",
		"=address-list-extra-time=1w3d")

	for _, svc := range services {
		if !svc.Enabled || len(svc.Domains) == 0 {
			continue
		}

		listName := "SASMAN_RELAY_" + strings.ToUpper(svc.ID)
		comment := "SASMAN-Relay-" + svc.ID

		// Add DNS Static FWD and RAW TLS-Host matchers
		for _, domain := range svc.Domains {
			cleanDomain := strings.TrimPrefix(domain, "*.")

			// Add DNS static forward rule
			core.SafeRun(client, "/ip/dns/static/add",
				"=name="+cleanDomain,
				"=type=FWD",
				"=forward-to=8.8.8.8",
				"=address-list="+listName,
				"=match-subdomain=yes",
				"=comment="+comment)

			// Add RAW prerouting rule for TLS SNI detection
			core.SafeRun(client, "/ip/firewall/raw/add",
				"=chain=prerouting",
				"=protocol=tcp",
				"=dst-port=443",
				"=action=add-dst-to-address-list",
				"=address-list="+listName,
				"=tls-host="+cleanDomain,
				"=src-address-list=!TM_Local_Subnets",
				"=comment="+comment)

			if strings.HasPrefix(domain, "*.") {
				core.SafeRun(client, "/ip/firewall/raw/add",
					"=chain=prerouting",
					"=protocol=tcp",
					"=dst-port=443",
					"=action=add-dst-to-address-list",
					"=address-list="+listName,
					"=tls-host=*."+cleanDomain,
					"=src-address-list=!TM_Local_Subnets",
					"=comment="+comment)
			}
		}

		// Add DST-NAT Redirect Rule for port 443 to the local Agent interceptor port
		core.SafeRun(client, "/ip/firewall/nat/add",
			"=chain=dstnat",
			"=protocol=tcp",
			"=dst-port=443",
			"=dst-address-list="+listName,
			"=src-address-list=TM_Local_Subnets",
			"=action=redirect",
			fmt.Sprintf("=to-ports=%d", interceptorPort),
			"=comment="+comment)
	}

	log.Printf("[Relay MikroTik Sync] Successfully synced %d active services to RouterOS", len(services))
	return nil
}

// RemoveMikroTikRelayRules cleans up rules for a specific service
func RemoveMikroTikRelayRules(client *routeros.Client, serviceID string) {
	if client == nil {
		return
	}

	comment := "SASMAN-Relay-" + serviceID

	// Helper to remove entries with comment
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
