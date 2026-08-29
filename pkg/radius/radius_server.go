package radius

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wxccs/radius/v2/crypto"
	"github.com/wxccs/radius/v2/packet"
	"github.com/wxccs/radius/v2/server"
	"github.com/wxccs/radius/v2/types"
	"github.com/wxccs/radius/v2/vendors/microsoft"

	"mikrotik-manager/pkg/tunnel"
)

var radiusLogger = log.Default()

const MikroTikVendorID uint32 = 14988

type ExpiredRedirect struct {
	ExpiredPool    string
	ExpiredProfile string
}

var (
	nasSecrets            = make(map[string]string)
	nasMu                 sync.RWMutex
	profileRedirectsCache = make(map[string]ExpiredRedirect)
	profileRedirectsMu    sync.RWMutex
)

// StartRadiusServer initializes and starts the Go RADIUS server using github.com/wxccs/radius/v2
func StartRadiusServer() {
	// Initialize Logger
	debugEnabled := os.Getenv("DEBUG_RADIUS") == "1" || os.Getenv("DEBUG_RADIUS") == "true" || os.Getenv("DEBUG") == "1"
	logFile, err := os.OpenFile("data/radius.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		if debugEnabled {
			// Log to both standard output and the log file
			radiusLogger = log.New(io.MultiWriter(os.Stdout, logFile), "", log.LstdFlags)
		} else {
			// Log ONLY to the log file
			radiusLogger = log.New(logFile, "", log.LstdFlags)
		}
	} else {
		log.Printf("[radius] WARNING: Could not open data/radius.log: %v", err)
		if debugEnabled {
			radiusLogger = log.Default()
		} else {
			radiusLogger = log.New(io.Discard, "", log.LstdFlags)
		}
	}

	// Load NAS secrets from DB
	UpdateNASSecrets()

	// Load Profile Redirect cache from DB into memory
	UpdateProfileRedirectsCache()

	authPort := 1812
	acctPort := 1813

	network := "udp4"
	bindIP := net.IPv4(0, 0, 0, 0)

	secretLookup := func(remoteIP net.IP) ([]byte, bool) {
		return getNASSecretByIP(remoteIP)
	}

	// 1. Auth Server Handler
	authHandler := server.HandlerFunc(func(ctx context.Context, req *server.Request) (*packet.Packet, error) {
		logPacketIn(req)
		return handleAuthRequest(ctx, req)
	})

	// 2. Acct Server Handler
	acctHandler := server.HandlerFunc(func(ctx context.Context, req *server.Request) (*packet.Packet, error) {
		logPacketIn(req)
		return handleAcctRequest(ctx, req)
	})

	log.Printf("[radius] Starting Go RADIUS server (wxccs/radius/v2) on :%d (Auth) and :%d (Acct) [%s]...", authPort, acctPort, network)

	// Start Auth Server
	go func() {
		authSrv, err := server.NewUDPServer(
			network,
			&net.UDPAddr{IP: bindIP, Port: authPort},
			authHandler,
			secretLookup,
		)
		if err != nil {
			log.Fatalf("[radius] Auth server initialization failed: %v", err)
		}
		if err := authSrv.Serve(context.Background()); err != nil {
			log.Fatalf("[radius] Auth server stopped: %v", err)
		}
	}()

	// Start Acct Server
	go func() {
		acctSrv, err := server.NewUDPServer(
			network,
			&net.UDPAddr{IP: bindIP, Port: acctPort},
			acctHandler,
			secretLookup,
		)
		if err != nil {
			log.Fatalf("[radius] Acct server initialization failed: %v", err)
		}
		if err := acctSrv.Serve(context.Background()); err != nil {
			log.Fatalf("[radius] Acct server stopped: %v", err)
		}
	}()

	// Start RadSec Server (TLS/mTLS on port 2083)
	StartRadSecServer()
}

// UpdateNASSecrets reloads NAS secrets from the database
func UpdateNASSecrets() {
	if DB == nil {
		return
	}
	// Auto-trim any accidental whitespace in the database
	_, _ = DB.Exec("UPDATE nas SET nasname = TRIM(nasname), secret = TRIM(secret)")

	rows, err := DB.Query("SELECT nasname, secret FROM nas")
	if err != nil {
		log.Printf("[radius] Failed to load NAS secrets: %v", err)
		return
	}
	defer rows.Close()

	nasMu.Lock()
	defer nasMu.Unlock()

	// Clear and reload
	newSecrets := make(map[string]string)
	for rows.Next() {
		var ip, secret string
		if err := rows.Scan(&ip, &secret); err == nil {
			ip = strings.TrimSpace(ip)
			secret = strings.TrimSpace(secret)
			if ip != "" && secret != "" {
				newSecrets[ip] = secret
			}
		}
	}
	nasSecrets = newSecrets
	log.Printf("[radius] Loaded %d NAS secrets", len(nasSecrets))
}

func getNASSecretByIP(remoteIP net.IP) ([]byte, bool) {
	nasMu.RLock()
	defer nasMu.RUnlock()

	host := remoteIP.String()
	debugEnabled := isRadiusDebug()

	if secret, ok := nasSecrets[host]; ok {
		if debugEnabled {
			fp := sha256.Sum256([]byte(secret))
			radiusLogger.Printf("[radius] [DEBUG] NAS [%s] matched secret (len=%d, sha256_prefix=%x)", host, len(secret), fp[:4])
		}
		return []byte(secret), true
	}

	// Localhost fallback
	if host == "127.0.0.1" || host == "::1" {
		if secret, ok := nasSecrets["127.0.0.1"]; ok {
			return []byte(secret), true
		}
	}

	// Wildcard 0.0.0.0 fallback
	if secret, ok := nasSecrets["0.0.0.0"]; ok {
		if debugEnabled {
			fp := sha256.Sum256([]byte(secret))
			radiusLogger.Printf("[radius] [DEBUG] NAS [%s] matched wildcard 0.0.0.0 secret (len=%d, sha256_prefix=%x)", host, len(secret), fp[:4])
		}
		return []byte(secret), true
	}

	var known []string
	for k := range nasSecrets {
		known = append(known, k)
	}
	radiusLogger.Printf("[radius] ❌ Unknown NAS: %s (registered NAS in DB: %v)", host, known)
	return nil, false
}

func isRadiusDebug() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("DEBUG_RADIUS")))
	if v == "1" || v == "true" || v == "yes" || v == "all" || v == "debug" {
		return true
	}
	d := strings.ToLower(strings.TrimSpace(os.Getenv("DEBUG")))
	return d == "1" || d == "true" || d == "all"
}

func getAttributeName(typeCode byte) string {
	switch typeCode {
	case types.AttrUserName:
		return "User-Name"
	case types.AttrUserPassword:
		return "User-Password"
	case types.AttrCHAPPassword:
		return "CHAP-Password"
	case types.AttrNASIPAddress:
		return "NAS-IP-Address"
	case types.AttrNASPort:
		return "NAS-Port"
	case types.AttrServiceType:
		return "Service-Type"
	case types.AttrFramedProtocol:
		return "Framed-Protocol"
	case types.AttrFramedIPAddress:
		return "Framed-IP-Address"
	case types.AttrFramedIPNetmask:
		return "Framed-IP-Netmask"
	case types.AttrState:
		return "State"
	case types.AttrVendorSpecific:
		return "Vendor-Specific (VSA)"
	case types.AttrSessionTimeout:
		return "Session-Timeout"
	case types.AttrIdleTimeout:
		return "Idle-Timeout"
	case types.AttrCalledStationID:
		return "Called-Station-Id"
	case types.AttrCallingStationID:
		return "Calling-Station-Id"
	case types.AttrNASIdentifier:
		return "NAS-Identifier"
	case types.AttrAcctStatusType:
		return "Acct-Status-Type"
	case types.AttrAcctDelayTime:
		return "Acct-Delay-Time"
	case types.AttrAcctInputOctets:
		return "Acct-Input-Octets"
	case types.AttrAcctOutputOctets:
		return "Acct-Output-Octets"
	case types.AttrAcctSessionID:
		return "Acct-Session-Id"
	case types.AttrAcctAuthentic:
		return "Acct-Authentic"
	case types.AttrAcctSessionTime:
		return "Acct-Session-Time"
	case types.AttrAcctTerminateCause:
		return "Acct-Terminate-Cause"
	case types.AttrCHAPChallenge:
		return "CHAP-Challenge"
	case types.AttrNASPortType:
		return "NAS-Port-Type"
	case types.AttrMessageAuthenticator:
		return "Message-Authenticator"
	case types.AttrAcctInterimInterval:
		return "Acct-Interim-Interval"
	case 88:
		return "Framed-Pool"
	default:
		return "Unknown-Attr"
	}
}

func logPacketIn(req *server.Request) {
	if !isRadiusDebug() {
		return
	}
	radiusLogger.Printf("============================== [📥 RADIUS INCOMING PACKET] ==============================")
	radiusLogger.Printf("  🔹 Code: %v | ID: %d | From: %v", req.Code, req.Identifier, req.RemoteAddr)
	radiusLogger.Printf("  🔹 Request Authenticator: %x", req.Authenticator)
	radiusLogger.Printf("  🔹 Attributes List (%d total):", len(req.Attributes))
	for i, avp := range req.Attributes {
		name := getAttributeName(avp.Type)
		attr := avp.Value
		valStr := string(attr)
		isPrintable := true
		for _, b := range attr {
			if b < 32 || b > 126 {
				isPrintable = false
				break
			}
		}
		if isPrintable && len(valStr) > 0 {
			radiusLogger.Printf("     [%02d] %-22s #%d: %q (len=%d, hex=%x)", avp.Type, name, i+1, valStr, len(attr), attr)
		} else {
			radiusLogger.Printf("     [%02d] %-22s #%d: [binary len=%d, hex=%x]", avp.Type, name, i+1, len(attr), attr)
		}
	}
	radiusLogger.Printf("-----------------------------------------------------------------------------------------")
}

func logPacketOut(response *packet.Packet, remote net.Addr, start time.Time) {
	if !isRadiusDebug() {
		return
	}
	dur := time.Since(start)
	radiusLogger.Printf("============================== [📤 RADIUS OUTGOING PACKET] =============================")
	radiusLogger.Printf("  🔸 Code: %v | ID: %d | To: %v | Time: %v", response.Code, response.Identifier, remote, dur)
	radiusLogger.Printf("  🔸 Attributes List (%d total):", len(response.Attributes))
	for i, avp := range response.Attributes {
		name := getAttributeName(avp.Type)
		attr := avp.Value
		valStr := string(attr)
		isPrintable := true
		for _, b := range attr {
			if b < 32 || b > 126 {
				isPrintable = false
				break
			}
		}
		if isPrintable && len(valStr) > 0 {
			radiusLogger.Printf("     [%02d] %-22s #%d: %q (len=%d, hex=%x)", avp.Type, name, i+1, valStr, len(attr), attr)
		} else {
			radiusLogger.Printf("     [%02d] %-22s #%d: [binary len=%d, hex=%x]", avp.Type, name, i+1, len(attr), attr)
		}
	}
	radiusLogger.Printf("=========================================================================================")
}

// Helpers to extract attributes from packet
func getAttrString(p *packet.Packet, attrType byte) string {
	for _, a := range p.Attributes {
		if a.Type == attrType {
			return string(a.Value)
		}
	}
	return ""
}

func getAttrBytes(p *packet.Packet, attrType byte) []byte {
	for _, a := range p.Attributes {
		if a.Type == attrType {
			return a.Value
		}
	}
	return nil
}

func getAttrInteger(p *packet.Packet, attrType byte) uint32 {
	for _, a := range p.Attributes {
		if a.Type == attrType && len(a.Value) == 4 {
			return binary.BigEndian.Uint32(a.Value)
		}
	}
	return 0
}

func getAttrIP(p *packet.Packet, attrType byte) net.IP {
	for _, a := range p.Attributes {
		if a.Type == attrType {
			return net.IP(a.Value)
		}
	}
	return nil
}

func hasAttribute(p *packet.Packet, attrType byte) bool {
	for _, a := range p.Attributes {
		if a.Type == attrType {
			return true
		}
	}
	return false
}

// VSA MikroTik Builder Helpers
func NewMikrotikString(vsaType byte, val string) packet.Attribute {
	payload := make([]byte, 2+len(val))
	payload[0] = vsaType
	payload[1] = byte(2 + len(val))
	copy(payload[2:], val)
	return packet.NewVendorSpecific(MikroTikVendorID, payload)
}

func NewMikrotikInteger(vsaType byte, val uint32) packet.Attribute {
	payload := make([]byte, 6)
	payload[0] = vsaType
	payload[1] = 6
	binary.BigEndian.PutUint32(payload[2:], val)
	return packet.NewVendorSpecific(MikroTikVendorID, payload)
}

func handleAuthRequest(ctx context.Context, req *server.Request) (*packet.Packet, error) {
	startAuthTime := time.Now()
	username := getAttrString(req.Packet, types.AttrUserName)
	debugEnabled := isRadiusDebug()

	if debugEnabled {
		nasIP, _, _ := net.SplitHostPort(req.RemoteAddr.String())
		radiusLogger.Printf("[radius] 🔑 طلب مصادقة جديد: يوزر [%s] | من NAS: %s", username, nasIP)
	}

	// 0. Check Scheduled Internet Shutdown
	if IsShutdownActiveForUser(username) {
		radiusLogger.Printf("[radius] ❌ رفض الاتصال: يوزر [%s] | السبب: جدول قطع الخدمة نشط حالياً", username)
		return buildAccessReject(req, username, "internet_shutdown", startAuthTime), nil
	}

	// 1. Check Global Bypass (Blind Accept)
	if isBypassEnabled() {
		radiusLogger.Printf("[radius] ✅ تجاوز عام نشط: تم قبول اتصال [%s] تلقائياً", username)
		resp := &packet.Packet{
			Code:          types.AccessAccept,
			Identifier:    req.Identifier,
			Authenticator: req.Authenticator,
		}
		if hasAttribute(req.Packet, types.AttrMessageAuthenticator) {
			resp.Attributes = append(resp.Attributes, packet.NewOctets(types.AttrMessageAuthenticator, make([]byte, 16)))
		}
		logPacketOut(resp, req.RemoteAddr, startAuthTime)
		return resp, nil
	}

	// 2. Check cross-agent roaming / Central HotSpot domain
	if strings.Contains(username, "@") || strings.Contains(username, "/") || strings.Contains(username, "\\") {
		if resp, handled := handleGlobalHotspotAuth(req, username, startAuthTime); handled {
			return resp, nil
		}
	}

	// 3. Fetch User Data from LMDB (with SQLite fallback)
	data, err := getLMDBUserData(username)
	if err != nil || data == "" {
		if DB != nil {
			var dbPass string
			errDB := DB.QueryRow("SELECT value FROM radcheck WHERE username = ? AND attribute = 'Cleartext-Password'", username).Scan(&dbPass)
			if errDB == nil && dbPass != "" {
				var lines []string
				lines = append(lines, dbPass)
				rows, errRows := DB.Query("SELECT attribute, value FROM radreply WHERE username = ?", username)
				if errRows == nil && rows != nil {
					for rows.Next() {
						var attr, val string
						if rows.Scan(&attr, &val) == nil {
							lines = append(lines, fmt.Sprintf("%s=%s", attr, val))
						}
					}
					rows.Close()
				}
				data = strings.Join(lines, "\n")
				err = nil
			}
		}
	}

	if err != nil || data == "" {
		// Not found locally -> Try Central Server Global HotSpot / Voucher authentication
		if resp, handled := handleGlobalHotspotAuth(req, username, startAuthTime); handled {
			return resp, nil
		}
		if debugEnabled {
			radiusLogger.Printf("[radius] [DEBUG] User [%s] not found in LMDB/SQLite or query failed: %v", username, err)
		}
		return buildAccessReject(req, username, "user not found", startAuthTime), nil
	}

	lines := strings.Split(data, "\n")
	if len(lines) < 1 {
		if debugEnabled {
			radiusLogger.Printf("[radius] [DEBUG] Invalid user data format in LMDB for user [%s]", username)
		}
		return buildAccessReject(req, username, "invalid user data", startAuthTime), nil
	}

	dbPassword := lines[0]
	attributes := make(map[string]string)
	for _, line := range lines[1:] {
		if idx := strings.Index(line, "="); idx != -1 {
			k := strings.TrimSpace(line[:idx])
			v := strings.TrimSpace(line[idx+1:])
			attributes[k] = v
		}
	}

	// 4. Verify Authentication Method
	var authSuccess bool
	var mppeKeys []byte
	var msChap2Success []byte
	authProtocol := "PAP"

	userPasswordRaw := getAttrBytes(req.Packet, types.AttrUserPassword)
	chapPassword := getAttrBytes(req.Packet, types.AttrCHAPPassword)

	// Check for Microsoft MS-CHAPv2 VSA
	var msc2Resp []byte
	var msc2Challenge []byte
	for _, a := range req.Attributes {
		if a.Type == types.AttrVendorSpecific {
			vID, payload, errVSA := a.VendorSpecific()
			if errVSA == nil && vID == microsoft.VendorID && len(payload) >= 2 {
				subType := payload[0]
				subVal := payload[2:] // Skip Vendor-Type and Vendor-Length
				if subType == microsoft.VendorTypeMSCHAP2Response {
					msc2Resp = subVal
				} else if subType == 11 { // MS-CHAP-Challenge
					msc2Challenge = subVal
				}
			}
		}
	}

	if len(userPasswordRaw) > 0 {
		// PAP (password is encrypted with Request Authenticator & Secret)
		decryptedPass, decErr := crypto.DecryptUserPassword(userPasswordRaw, req.Authenticator, req.Secret)
		if decErr == nil {
			authSuccess = (string(decryptedPass) == dbPassword)
			if debugEnabled {
				radiusLogger.Printf("[radius] [DEBUG] PAP auth check for [%s]: received=%q, dbPassword=%q, Match=%t",
					username, string(decryptedPass), dbPassword, authSuccess)
			}
		}
	} else if len(chapPassword) > 0 {
		// CHAP
		authProtocol = "CHAP"
		challenge := getAttrBytes(req.Packet, types.AttrCHAPChallenge)
		if len(challenge) == 0 {
			challenge = req.Authenticator[:]
		}

		authSuccess = crypto.VerifyCHAPResponse(chapPassword, dbPassword, challenge)
		if debugEnabled {
			radiusLogger.Printf("[radius] [DEBUG] CHAP check for [%s]: challenge=%x, receivedPassword=%x, Match=%t",
				username, challenge, chapPassword, authSuccess)
		}
	} else if len(msc2Resp) >= 49 {
		// MS-CHAPv2
		authProtocol = "MS-CHAPv2"
		challenge := msc2Challenge
		if len(challenge) == 0 {
			challenge = req.Authenticator[:]
		}
		if len(challenge) >= 16 && len(msc2Resp) >= 50 {
			ident := msc2Resp[0]
			var authCh, peerCh [16]byte
			copy(authCh[:], challenge[:16])
			copy(peerCh[:], msc2Resp[2:18])
			peerResponse := msc2Resp[26:50]

			// Verify MS-CHAPv2 using wxccs crypto
			ntResponse := crypto.GenerateNTResponse(authCh, peerCh, username, dbPassword)
			if bytes.Equal(ntResponse[:], peerResponse) {
				authSuccess = true
				authRespStr := crypto.GenerateAuthenticatorResponse(authCh, peerCh, ntResponse, username, dbPassword)
				msChap2Success = append([]byte{ident}, []byte(authRespStr)...)

				// Derive MPPE keys
				sendKey, recvKey, errMPPE := crypto.DeriveMPPEKeysFromPassword(dbPassword, ntResponse, 16)
				if errMPPE == nil {
					mppeKeys = append(sendKey, recvKey...)
				}
			}
		}
	}

	if !authSuccess {
		if debugEnabled {
			radiusLogger.Printf("[radius] [DEBUG] Authentication failed for user [%s] using protocol %s", username, authProtocol)
		}
		return buildAccessReject(req, username, "invalid credentials", startAuthTime), nil
	}

	// 5. Check Enabled Status & Expiration
	isDisabled := false
	if val, ok := attributes["Enabled"]; ok && val == "0" {
		isDisabled = true
	}

	isExpired := false
	if val, ok := attributes["Expiration"]; ok {
		exp, _ := strconv.ParseInt(val, 10, 64)
		if exp > 0 && time.Now().Unix() >= exp {
			isExpired = true
		}
	}

	isExpiredOrDisabled := isDisabled || isExpired
	expiredPool, expiredProfile := lookupExpiredRedirect(username, attributes)

	if isExpiredOrDisabled {
		radiusLogger.Printf("[radius] ⚠️ المشترك [%s] منتهي/معطل | Expired-Pool=[%s] Expired-Profile=[%s]", username, expiredPool, expiredProfile)
		if expiredPool == "" && expiredProfile == "" {
			reason := "user expired"
			if isDisabled {
				reason = "user disabled"
			}
			return buildAccessReject(req, username, reason, startAuthTime), nil
		}
	}

	// 6. Check NAS-IP-Address binding
	if val, ok := attributes["NAS-IP-Address"]; ok && val != "ALL" && val != "" {
		nasIP, _, _ := net.SplitHostPort(req.RemoteAddr.String())
		if val != nasIP {
			return buildAccessReject(req, username, "NAS-IP-Address mismatch", startAuthTime), nil
		}
	}

	// 7. Check Simultaneous-Use
	if !isExpiredOrDisabled {
		if val, ok := attributes["Simultaneous-Use"]; ok {
			limit, _ := strconv.Atoi(val)
			if limit > 0 {
				sessions, _ := GetLMDBSessions()
				if countActiveSessions(sessions, username) >= limit {
					radiusLogger.Printf("[radius] ⚠️ تنبيه: المشترك [%s] تجاوز حد الاتصال المتزامن (%d)، لكن سيتم السماح له لتجنب انقطاع الخدمة", username, limit)
				}
			}
		}
	}

	// 8. Build Access-Accept Packet
	response := &packet.Packet{
		Code:          types.AccessAccept,
		Identifier:    req.Identifier,
		Authenticator: req.Authenticator,
	}

	// Standard User-Name
	response.Attributes = append(response.Attributes, packet.NewString(types.AttrUserName, username))

	// Determine Service-Type & Framed-Protocol
	reqServiceType := getAttrInteger(req.Packet, types.AttrServiceType)
	reqFramedProtocol := getAttrInteger(req.Packet, types.AttrFramedProtocol)
	isPPP := (reqFramedProtocol == 1 || reqServiceType == 2) // 1 = PPP, 2 = Framed-User

	if isPPP {
		response.Attributes = append(response.Attributes, packet.NewInteger(types.AttrServiceType, 2))  // Framed-User
		response.Attributes = append(response.Attributes, packet.NewInteger(types.AttrFramedProtocol, 1)) // PPP
	} else if reqServiceType != 0 {
		response.Attributes = append(response.Attributes, packet.NewInteger(types.AttrServiceType, reqServiceType))
	} else {
		response.Attributes = append(response.Attributes, packet.NewInteger(types.AttrServiceType, 1)) // Login-User
	}

	// Acct-Interim-Interval (5 minutes = 300s)
	response.Attributes = append(response.Attributes, packet.NewInteger(types.AttrAcctInterimInterval, 300))

	// Add Reply Attributes
	if isExpiredOrDisabled {
		if expiredPool != "" && isPPP {
			response.Attributes = append(response.Attributes, packet.NewString(88, expiredPool)) // Framed-Pool
		}
		if expiredProfile != "" {
			response.Attributes = append(response.Attributes, NewMikrotikString(3, expiredProfile)) // Mikrotik-Group
		}
	} else {
		for k, v := range attributes {
			if k == "Enabled" || k == "Expiration" || k == "Simultaneous-Use" || k == "NAS-IP-Address" || k == "Expired-Pool" || k == "Expired-Profile" || k == "User-Group" {
				continue
			}
			if (k == "Framed-Pool" || k == "Framed-IP-Netmask") && !isPPP {
				continue
			}
			addReplyAttributeToPacket(response, k, v)
		}
	}

	// Add MS-CHAPv2 success and MPPE keys if applicable
	if len(msChap2Success) > 0 {
		response.Attributes = append(response.Attributes, packet.NewVendorSpecific(microsoft.VendorID, append([]byte{microsoft.VendorTypeMSCHAP2Success, byte(len(msChap2Success) + 2)}, msChap2Success...)))
		if len(mppeKeys) == 32 {
			sendKeyEnc, _ := crypto.EncryptMPPEKey(mppeKeys[0:16], req.Authenticator, req.Secret)
			recvKeyEnc, _ := crypto.EncryptMPPEKey(mppeKeys[16:32], req.Authenticator, req.Secret)

			response.Attributes = append(response.Attributes, packet.NewVendorSpecific(microsoft.VendorID, append([]byte{16, byte(len(sendKeyEnc) + 2)}, sendKeyEnc...))) // MS-MPPE-Send-Key
			response.Attributes = append(response.Attributes, packet.NewVendorSpecific(microsoft.VendorID, append([]byte{17, byte(len(recvKeyEnc) + 2)}, recvKeyEnc...))) // MS-MPPE-Recv-Key
			
			// Encryption Policy & Types
			policyBuf := make([]byte, 6)
			policyBuf[0] = 7 // MS-MPPE-Encryption-Policy
			policyBuf[1] = 6
			binary.BigEndian.PutUint32(policyBuf[2:], 1) // Encryption Required
			response.Attributes = append(response.Attributes, packet.NewVendorSpecific(microsoft.VendorID, policyBuf))

			typeBuf := make([]byte, 6)
			typeBuf[0] = 8 // MS-MPPE-Encryption-Types
			typeBuf[1] = 6
			binary.BigEndian.PutUint32(typeBuf[2:], 4) // RC4-128bit
			response.Attributes = append(response.Attributes, packet.NewVendorSpecific(microsoft.VendorID, typeBuf))
		}
	}

	// If request contained Message-Authenticator, append empty one so Marshal calculates HMAC-MD5 (RFC 2869/3579)
	if hasAttribute(req.Packet, types.AttrMessageAuthenticator) {
		response.Attributes = append(response.Attributes, packet.NewOctets(types.AttrMessageAuthenticator, make([]byte, 16)))
	}

	if isExpiredOrDisabled {
		reason := "منتهي الاشتراك"
		if isDisabled {
			reason = "حساب معطل"
		}
		radiusLogger.Printf("[radius] ⚠️ تحويل: قبول اتصال [%s] بالباقة المحدودة (%s) | Pool=%s, Profile=%s | NAS: %v", username, reason, expiredPool, expiredProfile, req.RemoteAddr)
	} else {
		radiusLogger.Printf("[radius] ✅ مصادقة ناجحة: تم قبول اتصال [%s] بنجاح | البروتوكول: %s | NAS: %v", username, authProtocol, req.RemoteAddr)
	}

	logPacketOut(response, req.RemoteAddr, startAuthTime)
	return response, nil
}

func handleGlobalHotspotAuth(req *server.Request, username string, startAuthTime time.Time) (*packet.Packet, bool) {
	passwordRaw := getAttrBytes(req.Packet, types.AttrUserPassword)
	password, _ := crypto.DecryptUserPassword(passwordRaw, req.Authenticator, req.Secret)
	callingStation := getAttrString(req.Packet, types.AttrCallingStationID)
	framedIP := getAttrIP(req.Packet, types.AttrFramedIPAddress)
	nasIP, _, _ := net.SplitHostPort(req.RemoteAddr.String())

	var framedIPStr string
	if framedIP != nil {
		framedIPStr = framedIP.String()
	}

	gReq := tunnel.GlobalAuthRequestPayload{
		Username:      username,
		Password:      string(password),
		UserMAC:       callingStation,
		UserIP:        framedIPStr,
		NasIP:         nasIP,
	}

	radiusLogger.Printf("[radius] 🌐 توجيه طلب المصادقة للسيرفر المركزي: يوزر [%s] | MAC: %s", username, callingStation)

	resp, err := tunnel.RequestGlobalAuth(gReq, 4*time.Second)
	if err != nil {
		radiusLogger.Printf("[radius] ⚠️ تعذر الوصول للسيرفر المركزي للمستخدم [%s]: %v", username, err)
		return nil, false
	}

	if !resp.Allow {
		radiusLogger.Printf("[radius] ❌ رفض المصادقة المركزية للمستخدم [%s]: %s", username, resp.RejectReason)
		return buildAccessReject(req, username, resp.RejectReason, startAuthTime), true
	}

	radiusLogger.Printf("[radius] ✅ تم قبول المصادقة المركزية للمستخدم [%s] بنجاح! نوع الحساب: %s | السرعة: %s", username, resp.AccountType, resp.RateLimit)

	reply := &packet.Packet{
		Code:          types.AccessAccept,
		Identifier:    req.Identifier,
		Authenticator: req.Authenticator,
	}

	reply.Attributes = append(reply.Attributes, packet.NewString(types.AttrUserName, username))
	reply.Attributes = append(reply.Attributes, packet.NewInteger(types.AttrServiceType, 1)) // Login-User

	if resp.RateLimit != "" {
		reply.Attributes = append(reply.Attributes, NewMikrotikString(8, resp.RateLimit))
	}
	if resp.SessionTimeout > 0 {
		reply.Attributes = append(reply.Attributes, packet.NewInteger(types.AttrSessionTimeout, uint32(resp.SessionTimeout)))
	}
	if resp.IdleTimeout > 0 {
		reply.Attributes = append(reply.Attributes, packet.NewInteger(types.AttrIdleTimeout, uint32(resp.IdleTimeout)))
	}
	if resp.ReplyMessage != "" {
		reply.Attributes = append(reply.Attributes, packet.NewString(types.AttrReplyMessage, resp.ReplyMessage))
	} else {
		reply.Attributes = append(reply.Attributes, packet.NewString(types.AttrReplyMessage, "SASMAN Global HotSpot Welcome"))
	}

	if hasAttribute(req.Packet, types.AttrMessageAuthenticator) {
		reply.Attributes = append(reply.Attributes, packet.NewOctets(types.AttrMessageAuthenticator, make([]byte, 16)))
	}

	logPacketOut(reply, req.RemoteAddr, startAuthTime)
	return reply, true
}

func buildAccessReject(req *server.Request, username, reason string, startAuthTime time.Time) *packet.Packet {
	response := &packet.Packet{
		Code:          types.AccessReject,
		Identifier:    req.Identifier,
		Authenticator: req.Authenticator,
	}
	if username != "" {
		response.Attributes = append(response.Attributes, packet.NewString(types.AttrUserName, username))
	}
	if reason != "" {
		response.Attributes = append(response.Attributes, packet.NewString(types.AttrReplyMessage, reason))
	}
	if hasAttribute(req.Packet, types.AttrMessageAuthenticator) {
		response.Attributes = append(response.Attributes, packet.NewOctets(types.AttrMessageAuthenticator, make([]byte, 16)))
	}

	radiusLogger.Printf("[radius] ❌ رفض الاتصال: يوزر [%s] | السبب: %s | NAS: %v", username, translateRejectReason(reason), req.RemoteAddr)
	logPacketOut(response, req.RemoteAddr, startAuthTime)
	return response
}

func translateRejectReason(reason string) string {
	switch reason {
	case "invalid credentials":
		return "كلمة المرور غير صحيحة"
	case "user disabled":
		return "الحساب معطل من قبل الإدارة"
	case "user expired":
		return "الاشتراك منتهي الصلاحية"
	case "user not found":
		return "المستخدم غير موجود في النظام"
	case "NAS-IP-Address mismatch":
		return "عدم تطابق عنوان IP الخاص بالـ NAS"
	case "invalid user data":
		return "بيانات المستخدم غير صالحة"
	case "internet_shutdown":
		return "تم قطع الإنترنت مؤقتاً لجدول قطع الخدمة المجدول"
	default:
		return reason
	}
}

func countActiveSessions(sessions map[string]string, username string) int {
	count := 0
	for u := range sessions {
		if strings.EqualFold(u, username) {
			count++
		}
	}
	return count
}

func handleAcctRequest(ctx context.Context, req *server.Request) (*packet.Packet, error) {
	startAcctTime := time.Now()
	username := getAttrString(req.Packet, types.AttrUserName)
	statusType := getAttrInteger(req.Packet, types.AttrAcctStatusType)

	// Create Accounting-Response packet
	reply := &packet.Packet{
		Code:          types.AccountingResponse,
		Identifier:    req.Identifier,
		Authenticator: req.Authenticator,
	}

	if username == "" {
		logPacketOut(reply, req.RemoteAddr, startAcctTime)
		return reply, nil
	}

	sid := getAttrString(req.Packet, types.AttrAcctSessionID)
	framedIP := getAttrIP(req.Packet, types.AttrFramedIPAddress)
	ip := ""
	if framedIP != nil {
		ip = framedIP.String()
	}
	cli := getAttrString(req.Packet, types.AttrCallingStationID)

	inOct := uint64(getAttrInteger(req.Packet, types.AttrAcctInputOctets))
	outOct := uint64(getAttrInteger(req.Packet, types.AttrAcctOutputOctets))

	// Handle Gigawords (Attributes 52 & 53)
	inGW := uint64(getAttrInteger(req.Packet, 52))
	outGW := uint64(getAttrInteger(req.Packet, 53))
	inOct += inGW << 32
	outOct += outGW << 32

	sessionSecs := int64(getAttrInteger(req.Packet, types.AttrAcctSessionTime))
	nasIP, _, _ := net.SplitHostPort(req.RemoteAddr.String())
	if nasAttr := getAttrIP(req.Packet, types.AttrNASIPAddress); nasAttr != nil {
		nasIP = nasAttr.String()
	}
	termCause := getAttrInteger(req.Packet, types.AttrAcctTerminateCause)

	statusStr := ""
	switch statusType {
	case 1:
		statusStr = "بدء اتصال 🟢"
	case 2:
		statusStr = "قطع اتصال 🔴"
	case 3:
		statusStr = "تحديث دوري 🔄"
	}

	if statusStr != "" && statusType != 3 {
		radiusLogger.Printf("[radius] 📊 محاسبة: يوزر [%s] | الحالة: %s | الجلسة: %s | IP: %s | MAC: %s", username, statusStr, sid, ip, cli)
	}

	// 1. Update High-Performance LMDB Store
	updateLMDBAccounting(username, statusType, sid, ip, cli, inOct, outOct, sessionSecs)

	// 2. Persist Accounting Record into SQLite radacct
	go recordSQLiteAccounting(username, statusType, sid, ip, cli, nasIP, inOct, outOct, sessionSecs, termCause)

	// 3. Forward to Central Server for Global Roaming / Voucher Sessions
	go func() {
		statusTypeStr := ""
		switch statusType {
		case 1:
			statusTypeStr = "Start"
		case 2:
			statusTypeStr = "Stop"
		case 3:
			statusTypeStr = "Interim-Update"
		}
		if statusTypeStr != "" {
			_ = tunnel.SendGlobalAcct(tunnel.GlobalAcctPayload{
				SessionID:      sid,
				Username:       username,
				StatusType:     statusTypeStr,
				UserMAC:        cli,
				UserIP:         ip,
				NasIP:          nasIP,
				BytesIn:        int64(inOct),
				BytesOut:       int64(outOct),
				SessionTimeSec: int(sessionSecs),
			})
		}
	}()

	// 4. Invalidate Session Cache
	InvalidateSessionCache()

	logPacketOut(reply, req.RemoteAddr, startAcctTime)
	return reply, nil
}

func recordSQLiteAccounting(username string, status uint32, sid, ip, cli, nasIP string, in, out uint64, secs int64, termCause uint32) {
	if DB == nil || username == "" {
		return
	}
	switch status {
	case 1: // Start
		_, _ = DB.Exec(`UPDATE radacct SET acctstoptime = CURRENT_TIMESTAMP, acctterminatecause = 'Stale-Replaced' WHERE username = ? AND acctstoptime IS NULL`, username)
		_, _ = DB.Exec(`INSERT INTO radacct (username, acctsessionid, nasipaddress, callingstationid, framedipaddress, acctstarttime, acctupdatetime, acctsessiontime, acctinputoctets, acctoutputoctets)
			VALUES (?, ?, ?, ?, ?, datetime('now', 'localtime'), datetime('now', 'localtime'), 0, 0, 0)`,
			username, sid, nasIP, cli, ip)
	case 3: // Interim-Update
		res, err := DB.Exec(`UPDATE radacct SET acctupdatetime = datetime('now', 'localtime'), acctsessiontime = ?, acctinputoctets = ?, acctoutputoctets = ?, framedipaddress = CASE WHEN ? != '' THEN ? ELSE framedipaddress END
			WHERE acctsessionid = ? AND acctstoptime IS NULL`,
			secs, in, out, ip, ip, sid)
		if err == nil {
			if n, _ := res.RowsAffected(); n == 0 {
				_, _ = DB.Exec(`UPDATE radacct SET acctupdatetime = datetime('now', 'localtime'), acctsessiontime = ?, acctinputoctets = ?, acctoutputoctets = ?, framedipaddress = CASE WHEN ? != '' THEN ? ELSE framedipaddress END
					WHERE username = ? AND acctstoptime IS NULL`,
					secs, in, out, ip, ip, username)
			}
		}
	case 2: // Stop
		causeStr := fmt.Sprintf("Cause-%d", termCause)
		if termCause == 0 {
			causeStr = "User-Request"
		}
		res, err := DB.Exec(`UPDATE radacct SET acctstoptime = datetime('now', 'localtime'), acctupdatetime = datetime('now', 'localtime'), acctsessiontime = ?, acctinputoctets = ?, acctoutputoctets = ?, acctterminatecause = ?
			WHERE acctsessionid = ? AND acctstoptime IS NULL`,
			secs, in, out, causeStr, sid)
		if err == nil {
			if n, _ := res.RowsAffected(); n == 0 {
				_, _ = DB.Exec(`UPDATE radacct SET acctstoptime = datetime('now', 'localtime'), acctupdatetime = datetime('now', 'localtime'), acctsessiontime = ?, acctinputoctets = ?, acctoutputoctets = ?, acctterminatecause = ?
					WHERE username = ? AND acctstoptime IS NULL`,
					secs, in, out, causeStr, username)
			}
		}
	}
}

type InternalAcctPayload struct {
	Username       string `json:"username"`
	StatusType     string `json:"status_type"` // Start, Stop, Interim-Update
	SessionID      string `json:"session_id"`
	UserIP         string `json:"user_ip"`
	UserMAC        string `json:"user_mac"`
	NasIP          string `json:"nas_ip"`
	BytesIn        int64  `json:"bytes_in"`
	BytesOut       int64  `json:"bytes_out"`
	SessionTimeSec int64  `json:"session_time_sec"`
	TerminateCause uint32 `json:"terminate_cause"`
}

func RecordAccountingPayload(p InternalAcctPayload) {
	var statusCode uint32
	switch p.StatusType {
	case "Start":
		statusCode = 1
	case "Stop":
		statusCode = 2
	case "Interim-Update":
		statusCode = 3
	default:
		statusCode = 3
	}

	recordSQLiteAccounting(p.Username, statusCode, p.SessionID, p.UserIP, p.UserMAC, p.NasIP, uint64(p.BytesIn), uint64(p.BytesOut), p.SessionTimeSec, p.TerminateCause)
	updateLMDBAccounting(p.Username, statusCode, p.SessionID, p.UserIP, p.UserMAC, uint64(p.BytesIn), uint64(p.BytesOut), p.SessionTimeSec)
	InvalidateSessionCache()
}

func getLMDBUserData(username string) (string, error) {
	return fetchUserFromLMDB(username)
}

func updateLMDBAccounting(username string, status uint32, sid, ip, cli string, in, out uint64, secs int64) {
	saveAccountingToLMDB(username, status, sid, ip, cli, in, out, secs)
}

func VerifyLocalUser(username, password string) (bool, string, string, string, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimRight(strings.TrimSpace(password), "\x00")
	data, err := getLMDBUserData(username)
	if err != nil || strings.TrimSpace(data) == "" {
		if DB == nil {
			return false, "", "", "User not found", fmt.Errorf("user not found")
		}
		var dbPass string
		err = DB.QueryRow("SELECT value FROM radcheck WHERE username = ? AND attribute = 'Cleartext-Password'", username).Scan(&dbPass)
		if err != nil {
			return false, "", "", "المستخدم غير مسجل لدى هذا الوكيل", fmt.Errorf("user not found")
		}
		dbPass = strings.TrimRight(strings.TrimSpace(dbPass), "\x00")
		if password != "" && password != dbPass {
			return false, "", "", "كلمة المرور غير صحيحة", nil
		}
		return true, "10M/10M", dbPass, "OK", nil
	}

	lines := strings.Split(data, "\n")
	if len(lines) < 1 {
		return false, "", "", "بيانات المستخدم معطوبة", fmt.Errorf("invalid user data")
	}
	dbPassword := strings.TrimRight(strings.TrimSpace(lines[0]), "\x00\r\n")
	if password != "" && password != dbPassword {
		return false, "", "", "كلمة المرور غير صحيحة", nil
	}

	rateLimit := "10M/10M"
	for _, line := range lines[1:] {
		if idx := strings.Index(line, "="); idx != -1 {
			k := strings.ToLower(strings.TrimSpace(line[:idx]))
			v := strings.TrimSpace(line[idx+1:])
			if k == "mikrotik-rate-limit" {
				rateLimit = v
			}
		}
	}
	return true, rateLimit, dbPassword, "OK", nil
}

func addReplyAttributeToPacket(p *packet.Packet, name, value string) {
	nameLower := strings.ToLower(name)

	// MikroTik VSAs
	if strings.HasPrefix(nameLower, "mikrotik-") {
		var subType byte
		switch nameLower {
		case "mikrotik-recv-limit":
			subType = 1
		case "mikrotik-xmit-limit":
			subType = 2
		case "mikrotik-group":
			subType = 3
		case "mikrotik-rate-limit":
			subType = 8
		default:
			radiusLogger.Printf("[radius] WARNING: Unsupported Mikrotik attribute %s", name)
			return
		}
		p.Attributes = append(p.Attributes, NewMikrotikString(subType, value))
		return
	}

	// Standard attributes
	switch name {
	case "Framed-IP-Address":
		if ip := net.ParseIP(value); ip != nil {
			p.Attributes = append(p.Attributes, packet.NewIPAddr(types.AttrFramedIPAddress, ip))
		}
	case "Framed-Pool":
		p.Attributes = append(p.Attributes, packet.NewString(88, value))
	case "Session-Timeout":
		if v, err := strconv.Atoi(value); err == nil {
			p.Attributes = append(p.Attributes, packet.NewInteger(types.AttrSessionTimeout, uint32(v)))
		}
	default:
		radiusLogger.Printf("[radius] WARNING: Skipping unknown standard attribute %s", name)
	}
}

func UpdateProfileRedirectsCache() {
	if DB == nil {
		return
	}
	rows, err := DB.Query("SELECT groupname, COALESCE(expired_pool, ''), COALESCE(expired_profile, '') FROM radius_profile_meta")
	if err != nil {
		log.Printf("[radius] Error loading profile redirects cache: %v", err)
		return
	}
	defer rows.Close()

	profileRedirectsMu.Lock()
	defer profileRedirectsMu.Unlock()

	// Clear old cache
	profileRedirectsCache = make(map[string]ExpiredRedirect)
	count := 0
	for rows.Next() {
		var group, pool, profile string
		if err := rows.Scan(&group, &pool, &profile); err == nil {
			profileRedirectsCache[group] = ExpiredRedirect{
				ExpiredPool:    pool,
				ExpiredProfile: profile,
			}
			count++
		}
	}
	log.Printf("[radius] Loaded %d profile redirect caches into memory successfully", count)
}

// lookupExpiredRedirect fetches expired_pool and expired_profile in sub-microseconds
// from the thread-safe in-memory cache. It retrieves the user's group from the attributes map
// (saved during LMDB sync as User-Group) and does a fast map lookup, having zero database overhead.
func lookupExpiredRedirect(username string, attrs map[string]string) (expiredPool, expiredProfile string) {
	groupName := ""
	if attrs != nil {
		groupName = strings.TrimSpace(attrs["User-Group"])
	}

	// Fallback to SQLite query only if LMDB attributes are missing User-Group
	if groupName == "" && DB != nil {
		_ = DB.QueryRow(
			"SELECT groupname FROM radusergroup WHERE username=? ORDER BY priority LIMIT 1",
			username,
		).Scan(&groupName)
	}

	if groupName == "" {
		return "", ""
	}

	profileRedirectsMu.RLock()
	defer profileRedirectsMu.RUnlock()

	if redirect, exists := profileRedirectsCache[groupName]; exists {
		return redirect.ExpiredPool, redirect.ExpiredProfile
	}
	return "", ""
}

