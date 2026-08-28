package radius

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"mikrotik-manager/pkg/core"

	"github.com/go-routeros/routeros/v3"
	"github.com/wxccs/radius/v2/packet"
	"github.com/wxccs/radius/v2/types"
)

func buildDisconnectPacket(username string, info SessionInfo, secret string) ([]byte, error) {
	p := &packet.Packet{
		Code:       types.DisconnectRequest,
		Identifier: byte(rand.Intn(256)),
	}
	if p.Identifier == 0 {
		p.Identifier = byte(time.Now().UnixNano() & 0xFF)
	}

	if username != "" {
		p.Attributes = append(p.Attributes, packet.NewString(types.AttrUserName, username))
	}
	if info.NASIP != "" {
		if ip := net.ParseIP(info.NASIP).To4(); ip != nil {
			p.Attributes = append(p.Attributes, packet.NewIPAddr(types.AttrNASIPAddress, ip))
		}
	}
	if info.IP != "" {
		if ip := net.ParseIP(info.IP).To4(); ip != nil {
			p.Attributes = append(p.Attributes, packet.NewIPAddr(types.AttrFramedIPAddress, ip))
		}
	}
	if info.SessionID != "" {
		p.Attributes = append(p.Attributes, packet.NewString(types.AttrAcctSessionID, info.SessionID))
	}
	if info.CallingStation != "" {
		p.Attributes = append(p.Attributes, packet.NewString(types.AttrCallingStationID, info.CallingStation))
	}
	return p.Marshal([]byte(secret))
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
