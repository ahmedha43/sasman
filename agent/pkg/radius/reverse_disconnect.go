package radius

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/wxccs/radius/v2/packet"
	"github.com/wxccs/radius/v2/types"
)

// RadSecAgent represents an active mTLS RadSec agent connection
type RadSecAgent struct {
	ID          int64
	CommonName  string
	NASName     string
	RemoteAddr  string
	Conn        net.Conn
	ConnectedAt time.Time
	LastPing    time.Time
	writeMu     sync.Mutex

	// In-flight reply wait channels for reverse requests (keyed by packet Identifier)
	pendingReplies sync.Map // map[byte]chan *packet.Packet
}

// RadSecAgentInfo represents agent status for API reporting
type RadSecAgentInfo struct {
	CommonName  string    `json:"common_name"`
	NASName     string    `json:"nas_name"`
	RemoteAddr  string    `json:"remote_addr"`
	ConnectedAt time.Time `json:"connected_at"`
	LastPing    time.Time `json:"last_ping"`
	UptimeSec   int64     `json:"uptime_sec"`
	Status      string    `json:"status"`
}

var (
	// Registry of active RadSec agent connections (keyed by CommonName)
	agentRegistry sync.Map
	// NAS Name / IP to CommonName lookup
	nasToAgentMap sync.Map
)

// RegisterRadSecAgent registers an active RadSec agent connection
func RegisterRadSecAgent(agent *RadSecAgent) {
	agentRegistry.Store(agent.CommonName, agent)
	if agent.NASName != "" {
		nasToAgentMap.Store(agent.NASName, agent.CommonName)
	}
	radiusLogger.Printf("[radsec] 🟢 Registered active agent: CN=%s, NAS=%s, Remote=%s", agent.CommonName, agent.NASName, agent.RemoteAddr)
}

// UnregisterRadSecAgent unregisters an agent connection
func UnregisterRadSecAgent(commonName string) {
	if val, ok := agentRegistry.LoadAndDelete(commonName); ok {
		agent := val.(*RadSecAgent)
		if agent.NASName != "" {
			nasToAgentMap.Delete(agent.NASName)
		}
		_ = agent.Conn.Close()
		radiusLogger.Printf("[radsec] 🔴 Unregistered agent: CN=%s", commonName)
	}
}

// GetRadSecAgentByCN retrieves an agent by its CommonName
func GetRadSecAgentByCN(cn string) *RadSecAgent {
	if val, ok := agentRegistry.Load(cn); ok {
		return val.(*RadSecAgent)
	}
	return nil
}

// GetRadSecAgentByNAS retrieves an agent by NAS Identifier or IP
func GetRadSecAgentByNAS(nasNameOrIP string) *RadSecAgent {
	if cnVal, ok := nasToAgentMap.Load(nasNameOrIP); ok {
		if cn, ok := cnVal.(string); ok {
			return GetRadSecAgentByCN(cn)
		}
	}
	// Direct CN match fallback
	return GetRadSecAgentByCN(nasNameOrIP)
}

// ListRadSecAgents returns active agent statuses for UI/monitoring
func ListRadSecAgents() []RadSecAgentInfo {
	var list []RadSecAgentInfo
	now := time.Now()

	agentRegistry.Range(func(key, value interface{}) bool {
		agent := value.(*RadSecAgent)
		list = append(list, RadSecAgentInfo{
			CommonName:  agent.CommonName,
			NASName:     agent.NASName,
			RemoteAddr:  agent.RemoteAddr,
			ConnectedAt: agent.ConnectedAt,
			LastPing:    agent.LastPing,
			UptimeSec:   int64(now.Sub(agent.ConnectedAt).Seconds()),
			Status:      "online",
		})
		return true
	})
	return list
}

// SendReverseDisconnect sends a Disconnect-Request packet through the active RadSec TLS connection
func SendReverseDisconnect(agent *RadSecAgent, username string, info SessionInfo, secret string) error {
	if agent == nil || agent.Conn == nil {
		return fmt.Errorf("agent is offline or nil")
	}

	if secret == "" {
		secret = "radsec" // RFC 6614 standard secret for TLS
	}

	wire, err := buildDisconnectPacket(username, info, secret)
	if err != nil {
		return fmt.Errorf("failed to build disconnect packet: %w", err)
	}

	pkt := &packet.Packet{}
	if err := pkt.Unmarshal(wire, []byte(secret)); err != nil {
		return fmt.Errorf("failed to unmarshal disconnect packet: %w", err)
	}

	ident := pkt.Identifier
	replyChan := make(chan *packet.Packet, 1)
	agent.pendingReplies.Store(ident, replyChan)
	defer agent.pendingReplies.Delete(ident)

	// Send framed over TLS (RFC 6614 / RFC 6613 framing: Length in header)
	agent.writeMu.Lock()
	_ = agent.Conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	_, writeErr := agent.Conn.Write(wire)
	agent.writeMu.Unlock()

	if writeErr != nil {
		return fmt.Errorf("failed to write reverse disconnect packet: %w", writeErr)
	}

	radiusLogger.Printf("[radsec-reverse] 📤 Sent Reverse Disconnect to [%s] for user [%s] (ID=%d)", agent.CommonName, username, ident)

	// Wait for reply with timeout
	select {
	case reply := <-replyChan:
		if reply.Code == types.DisconnectACK {
			radiusLogger.Printf("[radsec-reverse] ✅ Received Disconnect-ACK from [%s] for user [%s]", agent.CommonName, username)
			return nil
		}
		return fmt.Errorf("received disconnect NAK code=%d", reply.Code)
	case <-time.After(4 * time.Second):
		return fmt.Errorf("timeout waiting for reverse Disconnect-ACK from agent %s", agent.CommonName)
	}
}

// HandleRadSecInboundPacket processes incoming packets from a RadSec TLS stream
func (agent *RadSecAgent) HandleInboundPacket(raw []byte, secret []byte) (*packet.Packet, error) {
	if len(raw) < 20 {
		return nil, fmt.Errorf("packet too short")
	}

	code := types.Code(raw[0])
	ident := raw[1]

	// 1. Check if this is a reply to an in-flight Reverse Disconnect/CoA request
	if code == types.DisconnectACK || code == types.DisconnectNAK || code == types.CoAACK || code == types.CoANAK {
		if chVal, ok := agent.pendingReplies.Load(ident); ok {
			replyPkt := &packet.Packet{}
			if err := replyPkt.Unmarshal(raw, secret); err == nil {
				if ch, ok := chVal.(chan *packet.Packet); ok {
					select {
					case ch <- replyPkt:
					default:
					}
				}
			}
			return nil, nil // No server response needed for ACK/NAK
		}
	}

	// 2. Otherwise, treat as inbound RADIUS Request (Access-Request or Accounting-Request)
	pkt := &packet.Packet{}
	if err := pkt.Unmarshal(raw, secret); err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	agent.LastPing = time.Now()

	req := &serverRequestAdapter{
		pkt:        pkt,
		secret:     secret,
		remoteAddr: agent.Conn.RemoteAddr(),
	}

	switch pkt.Code {
	case types.AccessRequest:
		return handleAuthRequest(context.Background(), req.toServerRequest())
	case types.AccountingRequest:
		return handleAcctRequest(context.Background(), req.toServerRequest())
	default:
		return nil, fmt.Errorf("unsupported RadSec packet code: %v", pkt.Code)
	}
}

// ReadFramedPacket reads a single Length-framed RADIUS packet from a stream connection
func ReadFramedPacket(r io.Reader, deadline time.Duration, conn net.Conn) ([]byte, error) {
	if deadline > 0 && conn != nil {
		_ = conn.SetReadDeadline(time.Now().Add(deadline))
	}

	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint16(header[2:4])
	if length < 20 || length > 4096 {
		return nil, fmt.Errorf("invalid RADIUS packet length: %d", length)
	}

	out := make([]byte, length)
	copy(out[0:4], header[:])
	if _, err := io.ReadFull(r, out[4:]); err != nil {
		return nil, err
	}
	return out, nil
}
