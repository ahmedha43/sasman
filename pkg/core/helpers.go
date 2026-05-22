package core

import (
	"log"
	"strings"

	"github.com/go-routeros/routeros/v3"
)

func ShouldRemoveManagedEntry(path string, values map[string]string) bool {
	comment := values["comment"]
	name := values["name"]

	if strings.Contains(comment, "TM_") || strings.Contains(comment, "PCC-") || strings.Contains(comment, "ECMP-") || strings.Contains(comment, "SASMAN") || 
	   strings.Contains(comment, "Force DNS") || strings.Contains(comment, "DNS to Router") ||
	   strings.HasPrefix(comment, "Route-") || strings.HasPrefix(comment, "SNI-") || 
	   strings.HasPrefix(comment, "APP-ROUTE-") || strings.HasPrefix(comment, "GAME-ROUTE-") || 
	   strings.HasPrefix(comment, "BLOCK-") || strings.HasPrefix(comment, "GR-") || 
	   strings.HasPrefix(comment, "AR-") || strings.HasPrefix(comment, "ST-") ||
	   strings.HasPrefix(comment, "TM_Rule_") {
		return true
	}
	if strings.HasPrefix(name, "mac-") || strings.HasPrefix(name, "pppoe-") || 
	   strings.HasPrefix(name, "table-") || strings.HasPrefix(name, "to_") ||
	   strings.HasPrefix(name, "pool-pppoe-") || strings.HasPrefix(name, "hs-pool-") ||
	   strings.HasPrefix(name, "profile-pppoe-") || strings.HasPrefix(name, "hsprof-") {
		return true
	}
	if strings.HasPrefix(values["list"], "list-") || strings.HasPrefix(values["address-list"], "list-") {
		return true
	}
	if strings.HasPrefix(values["routing-table"], "table-") || strings.HasPrefix(values["routing-table"], "to_") {
		return true
	}
	if path == "/ip/firewall/nat" && values["action"] == "masquerade" {
		isManaged := values["out-interface-list"] == "WAN" || comment == "TM_WAN" || strings.Contains(comment, "TM_")
		return isManaged
	}
	return false
}

func RemoveEntriesByMatcher(client *routeros.Client, path string, shouldRemove func(map[string]string) bool) {
	reply, err := SafeRun(client, path + "/print")
	if err != nil || reply == nil {
		return
	}

	for _, s := range reply.Re {
		id := s.Map[".id"]
		if id == "" {
			continue
		}
		if shouldRemove(s.Map) {
			_, err := SafeRun(client, path+"/remove", "=.id="+id)
			if err != nil {
				log.Printf("[Purge] Error removing %s from %s: %v\n", id, path, err)
			}
		}
	}
}
