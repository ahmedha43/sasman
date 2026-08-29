package radsec

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/wxccs/radius/v2/crypto"
	"github.com/wxccs/radius/v2/packet"
	"github.com/wxccs/radius/v2/types"

	"mikrotik-manager/pkg/pki"
	"mikrotik-manager/pkg/tunnel"
)

type CentralRadSecServer struct {
	mu          sync.RWMutex
	agentsByNAS map[string]*CentralAgentConn
	agentsByCN  map[string]*CentralAgentConn
	authHandler func(subdomain string, req tunnel.GlobalAuthRequestPayload) tunnel.GlobalAuthResponsePayload
	getAgentSub func(cn string, nasIP string) string
	listener    net.Listener
	isShutdown  bool
}

type CentralAgentConn struct {
	Conn        *tls.Conn
	CommonName  string
	RemoteAddr  string
	NASIP       string
	ConnectedAt time.Time
	mu          sync.Mutex
}

func NewCentralRadSecServer(
	authHandler func(subdomain string, req tunnel.GlobalAuthRequestPayload) tunnel.GlobalAuthResponsePayload,
	getAgentSub func(cn string, nasIP string) string,
) *CentralRadSecServer {
	return &CentralRadSecServer{
		agentsByNAS: make(map[string]*CentralAgentConn),
		agentsByCN:  make(map[string]*CentralAgentConn),
		authHandler: authHandler,
		getAgentSub: getAgentSub,
	}
}

func (s *CentralRadSecServer) Start(port int) error {
	if err := pki.InitPKI(); err != nil {
		return fmt.Errorf("init central PKI: %w", err)
	}

	tlsConfig, err := pki.GetServerTLSConfig(func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(verifiedChains) == 0 || len(verifiedChains[0]) == 0 {
			return fmt.Errorf("no verified client certificate")
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("get server TLS config: %w", err)
	}

	bindAddr := fmt.Sprintf("0.0.0.0:%d", port)
	l, err := tls.Listen("tcp", bindAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", bindAddr, err)
	}
	s.listener = l
	log.Printf("[radsec-central] 🛡️ Central RadSec Server (RFC 6614 mTLS) listening on %s...", bindAddr)

	go s.acceptLoop()
	return nil
}

func (s *CentralRadSecServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.isShutdown {
				return
			}
			log.Printf("[radsec-central] Accept error: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *CentralRadSecServer) handleConnection(conn net.Conn) {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		_ = conn.Close()
		return
	}

	if err := tlsConn.Handshake(); err != nil {
		log.Printf("[radsec-central] ❌ Handshake failed from %s: %v", conn.RemoteAddr(), err)
		_ = conn.Close()
		return
	}

	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		log.Printf("[radsec-central] ❌ No client certificate from %s", conn.RemoteAddr())
		_ = conn.Close()
		return
	}

	clientCert := state.PeerCertificates[0]
	cn := clientCert.Subject.CommonName
	remoteAddr := conn.RemoteAddr().String()

	agent := &CentralAgentConn{
		Conn:        tlsConn,
		CommonName:  cn,
		RemoteAddr:  remoteAddr,
		ConnectedAt: time.Now(),
	}

	s.mu.Lock()
	s.agentsByCN[cn] = agent
	s.mu.Unlock()

	log.Printf("[radsec-central] 🟢 Registered active client: CN=%s, Remote=%s", cn, remoteAddr)

	defer func() {
		_ = tlsConn.Close()
		s.mu.Lock()
		delete(s.agentsByCN, cn)
		if agent.NASIP != "" {
			delete(s.agentsByNAS, agent.NASIP)
		}
		s.mu.Unlock()
		log.Printf("[radsec-central] 🔴 Client disconnected: CN=%s", cn)
	}()

	for {
		packetBytes, err := readFramedPacket(tlsConn)
		if err != nil {
			if err != io.EOF {
				log.Printf("[radsec-central] Read packet error for CN=%s: %v", cn, err)
			}
			return
		}

		log.Printf("[radsec-central] 📦 Received packet from CN=%s (len=%d, code=%d)", cn, len(packetBytes), packetBytes[0])

		pkt := &packet.Packet{}
		if err := pkt.Unmarshal(packetBytes, []byte("radsec")); err != nil {
			log.Printf("[radsec-central] RADIUS parse error for CN=%s: %v", cn, err)
			continue
		}

		s.processPacket(agent, pkt)
	}
}

func (s *CentralRadSecServer) processPacket(agent *CentralAgentConn, p *packet.Packet) {
	nasIP := ""
	for _, attr := range p.Attributes {
		if attr.Type == types.AttrNASIPAddress {
			nasIP = net.IP(attr.Value).String()
		} else if attr.Type == types.AttrNASIdentifier && nasIP == "" {
			nasIP = string(attr.Value)
		}
	}
	if nasIP != "" && agent.NASIP != nasIP {
		agent.NASIP = nasIP
		s.mu.Lock()
		s.agentsByNAS[nasIP] = agent
		s.mu.Unlock()
	}

	switch p.Code {
	case types.AccessRequest:
		s.handleAccessRequest(agent, p)
	case types.AccountingRequest:
		s.handleAccountingRequest(agent, p)
	default:
		log.Printf("[radsec-central] Unsupported RADIUS packet code: %d from %s", p.Code, agent.CommonName)
	}
}

func (s *CentralRadSecServer) handleAccessRequest(agent *CentralAgentConn, p *packet.Packet) {
	username := ""
	userPassword := ""
	callingStation := ""
	var userPasswordRaw []byte

	for _, attr := range p.Attributes {
		switch attr.Type {
		case types.AttrUserName:
			username = string(attr.Value)
		case types.AttrUserPassword:
			userPasswordRaw = attr.Value
		case types.AttrCallingStationID:
			callingStation = string(attr.Value)
		}
	}

	if len(userPasswordRaw) > 0 {
		decrypted, err := crypto.DecryptUserPassword(userPasswordRaw, p.Authenticator, []byte("radsec"))
		if err == nil {
			userPassword = string(bytes.TrimRight(decrypted, "\x00"))
		} else {
			userPassword = string(bytes.TrimRight(userPasswordRaw, "\x00"))
		}
	}

	nasIP := agent.NASIP
	if nasIP == "" {
		nasIP = agent.RemoteAddr
	}

	targetSubdomain := ""
	if s.getAgentSub != nil {
		targetSubdomain = s.getAgentSub(agent.CommonName, nasIP)
	}

	reqPayload := tunnel.GlobalAuthRequestPayload{
		RequestID:   fmt.Sprintf("radsec-%d", time.Now().UnixNano()),
		Username:    username,
		Password:    userPassword,
		UserMAC:     callingStation,
		UserIP:      "",
		NasIP:       nasIP,
	}

	var authResp tunnel.GlobalAuthResponsePayload
	if s.authHandler != nil {
		authResp = s.authHandler(targetSubdomain, reqPayload)
	} else {
		authResp = tunnel.GlobalAuthResponsePayload{
			Allow:        false,
			RejectReason: "No authentication handler configured",
		}
	}

	replyCode := types.AccessReject
	if authResp.Allow {
		replyCode = types.AccessAccept
	}

	reply := &packet.Packet{
		Code:          replyCode,
		Identifier:    p.Identifier,
		Authenticator: p.Authenticator,
	}

	if authResp.Allow {
		rateLimit := authResp.RateLimit
		if rateLimit == "" {
			rateLimit = "10M/10M"
		}
		// Mikrotik-Rate-Limit (Vendor: 14988, Subtype: 8)
		vsaData := make([]byte, 6+len(rateLimit))
		binary.BigEndian.PutUint32(vsaData[0:4], 14988)
		vsaData[4] = 8
		vsaData[5] = byte(2 + len(rateLimit))
		copy(vsaData[6:], []byte(rateLimit))
		reply.Attributes = append(reply.Attributes, packet.Attribute{
			Type:  types.AttrVendorSpecific,
			Value: vsaData,
		})

		timeout := authResp.SessionTimeout
		if timeout <= 0 {
			timeout = 86400
		}
		reply.Attributes = append(reply.Attributes, packet.NewInteger(types.AttrSessionTimeout, uint32(timeout)))

		if authResp.ReplyMessage != "" {
			reply.Attributes = append(reply.Attributes, packet.Attribute{
				Type:  types.AttrReplyMessage,
				Value: []byte(authResp.ReplyMessage),
			})
		}
	} else {
		reason := authResp.RejectReason
		if reason == "" {
			reason = "Authentication rejected"
		}
		reply.Attributes = append(reply.Attributes, packet.Attribute{
			Type:  types.AttrReplyMessage,
			Value: []byte(reason),
		})
	}

	replyWire, err := reply.Marshal([]byte("radsec"))
	if err != nil {
		log.Printf("[radsec-central] Failed to marshal Access-Reply for %s: %v", username, err)
		return
	}

	agent.mu.Lock()
	err = writeFramedPacket(agent.Conn, replyWire)
	agent.mu.Unlock()

	if err != nil {
		log.Printf("[radsec-central] Failed to send reply to %s: %v", agent.CommonName, err)
	} else {
		log.Printf("[radsec-central] ✅ Access-%s sent for [%s] via [%s]", map[bool]string{true: "Accept", false: "Reject"}[authResp.Allow], username, agent.CommonName)
	}
}

func (s *CentralRadSecServer) handleAccountingRequest(agent *CentralAgentConn, p *packet.Packet) {
	reply := &packet.Packet{
		Code:          types.AccountingResponse,
		Identifier:    p.Identifier,
		Authenticator: p.Authenticator,
	}

	replyWire, err := reply.Marshal([]byte("radsec"))
	if err != nil {
		log.Printf("[radsec-central] Failed to marshal Accounting-Response: %v", err)
		return
	}

	agent.mu.Lock()
	_ = writeFramedPacket(agent.Conn, replyWire)
	agent.mu.Unlock()
}

func (s *CentralRadSecServer) DisconnectUser(nasIP string, username string, sessionID string) error {
	s.mu.RLock()
	agent := s.agentsByNAS[nasIP]
	if agent == nil {
		for _, a := range s.agentsByCN {
			agent = a
			break
		}
	}
	s.mu.RUnlock()

	if agent == nil {
		return fmt.Errorf("no active RadSec connection for NAS [%s]", nasIP)
	}

	req := &packet.Packet{
		Code:       types.DisconnectRequest,
		Identifier: byte(time.Now().UnixNano() & 0xFF),
	}
	req.Attributes = append(req.Attributes, packet.Attribute{
		Type:  types.AttrUserName,
		Value: []byte(username),
	})
	if sessionID != "" {
		req.Attributes = append(req.Attributes, packet.Attribute{
			Type:  types.AttrAcctSessionID,
			Value: []byte(sessionID),
		})
	}

	reqWire, err := req.Marshal([]byte("radsec"))
	if err != nil {
		return fmt.Errorf("marshal Disconnect-Request: %w", err)
	}

	agent.mu.Lock()
	err = writeFramedPacket(agent.Conn, reqWire)
	agent.mu.Unlock()

	if err != nil {
		return fmt.Errorf("send reverse disconnect: %w", err)
	}

	log.Printf("[radsec-central] 📤 Sent Reverse Disconnect to [%s] for user [%s]", agent.CommonName, username)
	return nil
}

func readFramedPacket(r io.Reader) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint16(header[2:4])
	if length < 20 || length > 4096 {
		return nil, fmt.Errorf("invalid packet length: %d", length)
	}

	packetBytes := make([]byte, length)
	copy(packetBytes[:4], header)

	if _, err := io.ReadFull(r, packetBytes[4:]); err != nil {
		return nil, err
	}

	return packetBytes, nil
}

func writeFramedPacket(w io.Writer, p []byte) error {
	_, err := w.Write(p)
	return err
}
