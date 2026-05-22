package radius

import (
	"fmt"
	"net"
	"strings"

	"mikrotik-manager/pkg/core"

	"github.com/go-routeros/routeros/v3"
	layehRadius "layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
)

func buildDisconnectPacket(username string, info SessionInfo, secret string) ([]byte, error) {
	packet := layehRadius.New(layehRadius.CodeDisconnectRequest, []byte(secret))
	rfc2865.UserName_AddString(packet, username)
	if info.NASIP != "" {
		if ip := net.ParseIP(info.NASIP).To4(); ip != nil {
			if err := rfc2865.NASIPAddress_Add(packet, ip); err != nil {
				return nil, err
			}
		}
	}
	if info.IP != "" {
		if ip := net.ParseIP(info.IP).To4(); ip != nil {
			if err := rfc2865.FramedIPAddress_Add(packet, ip); err != nil {
				return nil, err
			}
		}
	}
	if info.SessionID != "" {
		rfc2866.AcctSessionID_AddString(packet, info.SessionID)
	}
	if info.CallingStation != "" {
		rfc2865.CallingStationID_AddString(packet, info.CallingStation)
	}
	return packet.Encode()
}

func removeMatchingSession(client *routeros.Client, printCmd, removeCmd, username string) error {
	reply, err := core.SafeRun(client, printCmd)
	if err != nil {
		return err
	}
	if reply == nil {
		return fmt.Errorf("nil reply from router")
	}

	var targetID string
	usernameLower := strings.ToLower(strings.TrimSpace(username))

	for _, re := range reply.Re {
		routerUser := strings.ToLower(strings.TrimSpace(re.Map["user"]))
		routerName := strings.ToLower(strings.TrimSpace(re.Map["name"]))

		// Fuzzy search for common username keys if standard fields are missing
		if routerUser == "" && routerName == "" {
			for k, v := range re.Map {
				if strings.ToLower(k) == "name" || strings.ToLower(k) == "user" {
					if strings.ToLower(strings.TrimSpace(v)) == usernameLower {
						targetID = re.Map[".id"]
						break
					}
				}
			}
		}

		if targetID == "" {
			if routerUser == usernameLower || routerName == usernameLower {
				targetID = re.Map[".id"]
			}
		}

		if targetID != "" {
			break
		}
	}

	if targetID != "" {
		_, err := core.SafeRun(client, removeCmd, "=.id="+targetID)
		if err == nil {
			fmt.Printf("[radius] Disconnect SUCCESS for %s (%s)\n", username, targetID)
			return nil
		}
		return err
	}

	return fmt.Errorf("no matching active session")
}
