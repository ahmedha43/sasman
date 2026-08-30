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
	"strings"
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
	acctHandler func(subdomain string, req tunnel.GlobalAcctPayload)
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
	acctHandler func(subdomain string, req tunnel.GlobalAcctPayload),
	getAgentSub func(cn string, nasIP string) string,
) *CentralRadSecServer {
	return &CentralRadSecServer{
		agentsByNAS: make(map[string]*CentralAgentConn),
		agentsByCN:  make(map[string]*CentralAgentConn),
		authHandler: authHandler,
		acctHandler: acctHandler,
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
	case types.DisconnectACK:
		log.Printf("[radsec-central] ✅ Disconnect-ACK received from [%s] (Session terminated successfully on MikroTik!)", agent.CommonName)
	case types.DisconnectNAK:
		log.Printf("[radsec-central] ⚠️ Disconnect-NAK received from [%s] (Session not found or already closed on MikroTik)", agent.CommonName)
	default:
		log.Printf("[radsec-central] Received RADIUS packet code: %d from %s", p.Code, agent.CommonName)
	}
}

func (s *CentralRadSecServer) handleAccessRequest(agent *CentralAgentConn, p *packet.Packet) {
	username := ""
	userPassword := ""
	callingStation := ""
	var userPasswordRaw []byte
	var chapPassword []byte
	var chapChallenge []byte
	var msc2Resp []byte
	var msc2Challenge []byte
	var reqServiceType uint32
	var reqFramedProtocol uint32

	for _, attr := range p.Attributes {
		switch attr.Type {
		case types.AttrUserName:
			username = string(attr.Value)
		case types.AttrUserPassword:
			userPasswordRaw = attr.Value
		case types.AttrCHAPPassword:
			chapPassword = attr.Value
		case types.AttrCHAPChallenge:
			chapChallenge = attr.Value
		case types.AttrCallingStationID:
			callingStation = string(attr.Value)
		case types.AttrServiceType:
			if len(attr.Value) == 4 {
				reqServiceType = binary.BigEndian.Uint32(attr.Value)
			}
		case types.AttrFramedProtocol:
			if len(attr.Value) == 4 {
				reqFramedProtocol = binary.BigEndian.Uint32(attr.Value)
			}
		case types.AttrVendorSpecific:
			if len(attr.Value) >= 6 {
				vID := binary.BigEndian.Uint32(attr.Value[0:4])
				if vID == 311 { // Microsoft
					subType := attr.Value[4]
					subLen := int(attr.Value[5])
					if len(attr.Value) >= 6+subLen-2 {
						subVal := attr.Value[6 : 6+subLen-2]
						if subType == 25 { // MS-CHAP2-Response
							msc2Resp = subVal
						} else if subType == 11 { // MS-CHAP-Challenge
							msc2Challenge = subVal
						}
					}
				}
			}
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

	log.Printf("[radsec-central] 🔑 Incoming Access-Request: User=%s, CN=%s, TargetSubdomain=%s (PAP=%t, CHAP=%t, MSCHAPv2=%t)",
		username, agent.CommonName, targetSubdomain, len(userPasswordRaw) > 0, len(chapPassword) > 0, len(msc2Resp) >= 50)

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

	dbPass := authResp.Password
	if dbPass == "" {
		dbPass = userPassword
	}

	// Verify CHAP if requested
	if authResp.Allow && len(chapPassword) > 0 {
		challenge := chapChallenge
		if len(challenge) == 0 {
			challenge = p.Authenticator[:]
		}
		if dbPass == "" || !crypto.VerifyCHAPResponse(chapPassword, dbPass, challenge) {
			authResp.Allow = false
			authResp.RejectReason = "كلمة المرور غير صحيحة (CHAP)"
		}
	}

	// Verify MS-CHAPv2 if requested
	var msChap2Success []byte
	var sendKeyEnc, recvKeyEnc []byte
	if authResp.Allow && len(msc2Resp) >= 50 {
		if dbPass == "" {
			authResp.Allow = false
			authResp.RejectReason = "كلمة المرور غير متوفرة للتحقق من MS-CHAPv2"
		} else {
			ident := msc2Resp[0]
			var authCh, peerCh [16]byte
			if len(msc2Challenge) >= 16 {
				copy(authCh[:], msc2Challenge[:16])
			} else {
				copy(authCh[:], p.Authenticator[:16])
			}
			copy(peerCh[:], msc2Resp[2:18])
			peerResponse := msc2Resp[26:50]
			ntResp := crypto.GenerateNTResponse(authCh, peerCh, username, dbPass)
			if !bytes.Equal(ntResp[:], peerResponse) {
				authResp.Allow = false
				authResp.RejectReason = "كلمة المرور غير صحيحة (MS-CHAPv2)"
			} else {
				authRespStr := crypto.GenerateAuthenticatorResponse(authCh, peerCh, ntResp, username, dbPass)
				msChap2Success = append([]byte{ident}, []byte(authRespStr)...)

				sendKey, recvKey, errMPPE := crypto.DeriveMPPEKeysFromPassword(dbPass, ntResp, 16)
				if errMPPE == nil {
					sendKeyEnc, _ = crypto.EncryptMPPEKey(sendKey, p.Authenticator, []byte("radsec"))
					recvKeyEnc, _ = crypto.EncryptMPPEKey(recvKey, p.Authenticator, []byte("radsec"))
				}
			}
		}
	}

	replyCode := types.AccessReject
	if authResp.Allow {
		replyCode = types.AccessAccept
	}

	resultText := "Access-Accept ✅"
	if replyCode == types.AccessReject {
		resultText = "Access-Reject ❌"
	}
	log.Printf("[radsec-central] 🏁 Auth Result for User=%s (Tenant: %s): %s (Reason: %s)",
		username, targetSubdomain, resultText, authResp.RejectReason)

	reply := &packet.Packet{
		Code:          replyCode,
		Identifier:    p.Identifier,
		Authenticator: p.Authenticator,
	}

	if authResp.Allow {
		isPPP := (reqFramedProtocol == 1 || reqServiceType == 2 || len(msc2Resp) > 0)

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

		// ONLY send Mikrotik-Group if explicitly configured and non-empty (NEVER hardcode "full"!)
		if authResp.MikrotikGroup != "" {
			groupData := make([]byte, 6+len(authResp.MikrotikGroup))
			binary.BigEndian.PutUint32(groupData[0:4], 14988)
			groupData[4] = 3
			groupData[5] = byte(2 + len(authResp.MikrotikGroup))
			copy(groupData[6:], []byte(authResp.MikrotikGroup))
			reply.Attributes = append(reply.Attributes, packet.Attribute{
				Type:  types.AttrVendorSpecific,
				Value: groupData,
			})
		}

		// Framed-Pool (for PPP)
		if authResp.FramedPool != "" && isPPP {
			reply.Attributes = append(reply.Attributes, packet.Attribute{
				Type:  88, // Framed-Pool
				Value: []byte(authResp.FramedPool),
			})
		}

		// Service-Type and Framed-Protocol
		if isPPP {
			stBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(stBuf, 2) // Framed-User
			reply.Attributes = append(reply.Attributes, packet.Attribute{
				Type:  types.AttrServiceType,
				Value: stBuf,
			})

			fpBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(fpBuf, 1) // PPP
			reply.Attributes = append(reply.Attributes, packet.Attribute{
				Type:  types.AttrFramedProtocol,
				Value: fpBuf,
			})
		} else if reqServiceType != 0 {
			stBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(stBuf, reqServiceType)
			reply.Attributes = append(reply.Attributes, packet.Attribute{
				Type:  types.AttrServiceType,
				Value: stBuf,
			})
		} else {
			stBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(stBuf, 1) // Login-User (Hotspot default)
			reply.Attributes = append(reply.Attributes, packet.Attribute{
				Type:  types.AttrServiceType,
				Value: stBuf,
			})
		}

		// Acct-Interim-Interval (300s = 5 minutes)
		interimBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(interimBuf, 300)
		reply.Attributes = append(reply.Attributes, packet.Attribute{
			Type:  types.AttrAcctInterimInterval,
			Value: interimBuf,
		})

		// Attach MS-CHAP2-Success & MPPE keys if applicable
		if len(msChap2Success) > 0 {
			vsaMS := make([]byte, 6+len(msChap2Success))
			binary.BigEndian.PutUint32(vsaMS[0:4], 311)
			vsaMS[4] = 26 // MS-CHAP2-Success
			vsaMS[5] = byte(2 + len(msChap2Success))
			copy(vsaMS[6:], msChap2Success)
			reply.Attributes = append(reply.Attributes, packet.Attribute{
				Type:  types.AttrVendorSpecific,
				Value: vsaMS,
			})

			if len(sendKeyEnc) > 0 && len(recvKeyEnc) > 0 {
				vsaSend := make([]byte, 6+len(sendKeyEnc))
				binary.BigEndian.PutUint32(vsaSend[0:4], 311)
				vsaSend[4] = 16 // MS-MPPE-Send-Key
				vsaSend[5] = byte(2 + len(sendKeyEnc))
				copy(vsaSend[6:], sendKeyEnc)
				reply.Attributes = append(reply.Attributes, packet.Attribute{
					Type:  types.AttrVendorSpecific,
					Value: vsaSend,
				})

				vsaRecv := make([]byte, 6+len(recvKeyEnc))
				binary.BigEndian.PutUint32(vsaRecv[0:4], 311)
				vsaRecv[4] = 17 // MS-MPPE-Recv-Key
				vsaRecv[5] = byte(2 + len(recvKeyEnc))
				copy(vsaRecv[6:], recvKeyEnc)
				reply.Attributes = append(reply.Attributes, packet.Attribute{
					Type:  types.AttrVendorSpecific,
					Value: vsaRecv,
				})

				policyBuf := make([]byte, 10)
				binary.BigEndian.PutUint32(policyBuf[0:4], 311)
				policyBuf[4] = 7 // MS-MPPE-Encryption-Policy
				policyBuf[5] = 6
				binary.BigEndian.PutUint32(policyBuf[6:], 1) // Encryption Required
				reply.Attributes = append(reply.Attributes, packet.Attribute{
					Type:  types.AttrVendorSpecific,
					Value: policyBuf,
				})

				typeBuf := make([]byte, 10)
				binary.BigEndian.PutUint32(typeBuf[0:4], 311)
				typeBuf[4] = 8 // MS-MPPE-Encryption-Types
				typeBuf[5] = 6
				binary.BigEndian.PutUint32(typeBuf[6:], 6) // RC4-40 or RC4-128
				reply.Attributes = append(reply.Attributes, packet.Attribute{
					Type:  types.AttrVendorSpecific,
					Value: typeBuf,
				})
			}
		}

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

	// Always append Message-Authenticator (RFC 2869/3579) so reply.Marshal signs the entire packet
	reply.Attributes = append(reply.Attributes, packet.Attribute{
		Type:  types.AttrMessageAuthenticator,
		Value: make([]byte, 16),
	})

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
	username := ""
	statusType := "Interim-Update"
	sessionID := ""
	userIP := ""
	userMAC := ""
	var inOctets, outOctets, inGiga, outGiga uint32
	var sessionTime uint32

	for _, attr := range p.Attributes {
		switch attr.Type {
		case types.AttrUserName:
			username = string(attr.Value)
		case types.AttrAcctStatusType:
			if len(attr.Value) == 4 {
				code := binary.BigEndian.Uint32(attr.Value)
				switch code {
				case 1:
					statusType = "Start"
				case 2:
					statusType = "Stop"
				case 3:
					statusType = "Interim-Update"
				}
			}
		case types.AttrAcctSessionID:
			sessionID = string(attr.Value)
		case types.AttrFramedIPAddress:
			userIP = net.IP(attr.Value).String()
		case types.AttrCallingStationID:
			userMAC = string(attr.Value)
		case types.AttrAcctInputOctets:
			if len(attr.Value) == 4 {
				inOctets = binary.BigEndian.Uint32(attr.Value)
			}
		case types.AttrAcctOutputOctets:
			if len(attr.Value) == 4 {
				outOctets = binary.BigEndian.Uint32(attr.Value)
			}
		case 52: // Acct-Input-Gigawords
			if len(attr.Value) == 4 {
				inGiga = binary.BigEndian.Uint32(attr.Value)
			}
		case 53: // Acct-Output-Gigawords
			if len(attr.Value) == 4 {
				outGiga = binary.BigEndian.Uint32(attr.Value)
			}
		case types.AttrAcctSessionTime:
			if len(attr.Value) == 4 {
				sessionTime = binary.BigEndian.Uint32(attr.Value)
			}
		}
	}

	bytesIn := int64(inOctets) + (int64(inGiga) << 32)
	bytesOut := int64(outOctets) + (int64(outGiga) << 32)

	nasIP := agent.NASIP
	if nasIP == "" {
		nasIP = agent.RemoteAddr
	}

	targetSubdomain := ""
	if s.getAgentSub != nil {
		targetSubdomain = s.getAgentSub(agent.CommonName, nasIP)
	}

	if s.acctHandler != nil && username != "" {
		s.acctHandler(targetSubdomain, tunnel.GlobalAcctPayload{
			SessionID:      sessionID,
			Username:       username,
			StatusType:     statusType,
			UserMAC:        userMAC,
			UserIP:         userIP,
			NasIP:          nasIP,
			BytesIn:        bytesIn,
			BytesOut:       bytesOut,
			SessionTimeSec: int(sessionTime),
		})
	}

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

func (s *CentralRadSecServer) DisconnectUser(subdomainOrNAS string, username string, sessionID string, framedIP string) error {
	s.mu.RLock()
	var targetAgent *CentralAgentConn

	// 1. Try exact CN match (agent-{subdomain}-SASMAN)
	cnKey := fmt.Sprintf("agent-%s-SASMAN", subdomainOrNAS)
	if a, ok := s.agentsByCN[cnKey]; ok && a.Conn != nil {
		targetAgent = a
	}

	// 2. Try direct CN match
	if targetAgent == nil {
		if a, ok := s.agentsByCN[subdomainOrNAS]; ok && a.Conn != nil {
			targetAgent = a
		}
	}

	// 3. Try NAS IP match
	if targetAgent == nil {
		if a, ok := s.agentsByNAS[subdomainOrNAS]; ok && a.Conn != nil {
			targetAgent = a
		}
	}

	// 4. Substring CN match
	if targetAgent == nil {
		for cn, a := range s.agentsByCN {
			if strings.Contains(strings.ToLower(cn), strings.ToLower(subdomainOrNAS)) && a.Conn != nil {
				targetAgent = a
				break
			}
		}
	}
	s.mu.RUnlock()

	if targetAgent == nil {
		return fmt.Errorf("no active RadSec mTLS connection found for tenant/NAS [%s]", subdomainOrNAS)
	}

	req := &packet.Packet{
		Code:       types.DisconnectRequest,
		Identifier: byte(time.Now().UnixNano() & 0xFF),
	}
	if req.Identifier == 0 {
		req.Identifier = 1
	}

	if username != "" {
		req.Attributes = append(req.Attributes, packet.Attribute{
			Type:  types.AttrUserName,
			Value: []byte(username),
		})
	}
	if sessionID != "" {
		req.Attributes = append(req.Attributes, packet.Attribute{
			Type:  types.AttrAcctSessionID,
			Value: []byte(sessionID),
		})
	}
	if framedIP != "" {
		if ip := net.ParseIP(framedIP).To4(); ip != nil {
			req.Attributes = append(req.Attributes, packet.Attribute{
				Type:  types.AttrFramedIPAddress,
				Value: ip,
			})
		}
	}

	reqWire, err := req.Marshal([]byte("radsec"))
	if err != nil {
		return fmt.Errorf("marshal Disconnect-Request: %w", err)
	}

	// 1. Send over RadSec TLS framed connection
	targetAgent.mu.Lock()
	_ = targetAgent.Conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	err = writeFramedPacket(targetAgent.Conn, reqWire)
	targetAgent.mu.Unlock()

	// 2. Also try direct UDP 3799 if remote IP is reachable
	go func() {
		host, _, splitErr := net.SplitHostPort(targetAgent.RemoteAddr)
		if splitErr == nil && host != "" {
			udpAddr := net.JoinHostPort(host, "3799")
			conn, dErr := net.DialTimeout("udp", udpAddr, 2*time.Second)
			if dErr == nil {
				defer conn.Close()
				_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				_, _ = conn.Write(reqWire)
			}
		}
	}()

	if err != nil {
		return fmt.Errorf("send reverse disconnect over TLS: %w", err)
	}

	log.Printf("[radsec-central] 📤 Sent Reverse Disconnect to [%s] (Remote: %s) for user [%s] (Session: %s)", targetAgent.CommonName, targetAgent.RemoteAddr, username, sessionID)
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
	if conn, ok := w.(net.Conn); ok {
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	}
	_, err := w.Write(p)
	return err
}
