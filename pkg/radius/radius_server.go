package radius

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"layeh.com/radius"
	"layeh.com/radius/rfc2759"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
	"layeh.com/radius/rfc2869"
	"layeh.com/radius/rfc3079"
	"layeh.com/radius/vendors/microsoft"

	"mikrotik-manager/pkg/tunnel"
)

var radiusLogger *log.Logger

// SecretSourceFunc is a functional adapter for radius.SecretSource
type SecretSourceFunc func(ctx context.Context, addr net.Addr) ([]byte, error)

func (f SecretSourceFunc) RADIUSSecret(ctx context.Context, addr net.Addr) ([]byte, error) {
	return f(ctx, addr)
}

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

// StartRadiusServer initializes and starts the Go RADIUS server
func StartRadiusServer() {
	// Initialize Logger
	debugEnabled := os.Getenv("DEBUG_RADIUS") == "1" || os.Getenv("DEBUG_RADIUS") == "true" || os.Getenv("DEBUG") == "1"
	logFile, err := os.OpenFile("data/radius.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		if debugEnabled {
			// Log to both standard output (which supervisor captures) and the log file
			radiusLogger = log.New(io.MultiWriter(os.Stdout, logFile), "", log.LstdFlags)
		} else {
			// Log ONLY to the log file (hides request logs from terminal/stdout)
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

	// Load Profile Redirect cache from DB into memory (bottleneck-free)
	UpdateProfileRedirectsCache()

	server := radius.PacketServer{
		Addr:         ":1812",
		Handler:      radius.HandlerFunc(handleRadiusPacket),
		SecretSource: SecretSourceFunc(getNASSecret),
	}

	log.Printf("[radius] Starting Go RADIUS server on :1812 (Auth) and :1813 (Acct)...")

	// Start Auth server
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("[radius] Auth server failed: %v", err)
		}
	}()

	// Start Acct server
	go func() {
		acctServer := radius.PacketServer{
			Addr:         ":1813",
			Handler:      radius.HandlerFunc(handleRadiusPacket),
			SecretSource: SecretSourceFunc(getNASSecret),
		}
		if err := acctServer.ListenAndServe(); err != nil {
			log.Fatalf("[radius] Acct server failed: %v", err)
		}
	}()
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

func getNASSecret(ctx context.Context, remote net.Addr) ([]byte, error) {
	nasMu.RLock()
	defer nasMu.RUnlock()

	host, _, _ := net.SplitHostPort(remote.String())
	host = strings.TrimSpace(host)
	debugEnabled := os.Getenv("DEBUG_RADIUS") == "1" || os.Getenv("DEBUG_RADIUS") == "true" || os.Getenv("DEBUG") == "1"

	if secret, ok := nasSecrets[host]; ok {
		if debugEnabled {
			fp := sha256.Sum256([]byte(secret))
			radiusLogger.Printf("[radius] [DEBUG] NAS [%s] matched secret (len=%d, sha256_prefix=%x)", host, len(secret), fp[:4])
		} else {
			radiusLogger.Printf("[radius] Request from NAS IP: %s", host)
		}
		return []byte(secret), nil
	}

	// Wildcard support: If 0.0.0.0 exists in nasSecrets, use its secret for everyone
	if secret, ok := nasSecrets["0.0.0.0"]; ok {
		if debugEnabled {
			fp := sha256.Sum256([]byte(secret))
			radiusLogger.Printf("[radius] [DEBUG] NAS [%s] matched wildcard 0.0.0.0 secret (len=%d, sha256_prefix=%x)", host, len(secret), fp[:4])
		} else {
			radiusLogger.Printf("[radius] Request from NAS IP: %s (wildcard)", host)
		}
		return []byte(secret), nil
	}

	var known []string
	for k := range nasSecrets {
		known = append(known, k)
	}
	radiusLogger.Printf("[radius] ❌ Unknown NAS: %s (registered NAS in DB: %v)", host, known)
	return nil, fmt.Errorf("unknown NAS: %s", host)
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
	case 1:
		return "User-Name"
	case 2:
		return "User-Password"
	case 3:
		return "CHAP-Password"
	case 4:
		return "NAS-IP-Address"
	case 5:
		return "NAS-Port"
	case 6:
		return "Service-Type"
	case 7:
		return "Framed-Protocol"
	case 8:
		return "Framed-IP-Address"
	case 9:
		return "Framed-IP-Netmask"
	case 24:
		return "State"
	case 26:
		return "Vendor-Specific (VSA)"
	case 27:
		return "Session-Timeout"
	case 28:
		return "Idle-Timeout"
	case 30:
		return "Called-Station-Id"
	case 31:
		return "Calling-Station-Id"
	case 32:
		return "NAS-Identifier"
	case 40:
		return "Acct-Status-Type"
	case 41:
		return "Acct-Delay-Time"
	case 42:
		return "Acct-Input-Octets"
	case 43:
		return "Acct-Output-Octets"
	case 44:
		return "Acct-Session-Id"
	case 45:
		return "Acct-Authentic"
	case 46:
		return "Acct-Session-Time"
	case 49:
		return "Acct-Terminate-Cause"
	case 60:
		return "CHAP-Challenge"
	case 61:
		return "NAS-Port-Type"
	case 80:
		return "Message-Authenticator"
	case 85:
		return "Acct-Interim-Interval"
	case 88:
		return "Framed-Pool"
	default:
		return "Unknown-Attr"
	}
}

func logPacketIn(r *radius.Request) {
	if !isRadiusDebug() {
		return
	}
	radiusLogger.Printf("============================== [📥 RADIUS INCOMING PACKET] ==============================")
	radiusLogger.Printf("  🔹 Code: %v | ID: %d | From: %v | To: %v", r.Code, r.Identifier, r.RemoteAddr, r.LocalAddr)
	radiusLogger.Printf("  🔹 Request Authenticator: %x", r.Authenticator)
	radiusLogger.Printf("  🔹 Attributes List (%d total):", len(r.Attributes))
	for i, avp := range r.Attributes {
		typeCode := byte(avp.Type)
		name := getAttributeName(typeCode)
		attr := avp.Attribute
		valStr := string(attr)
		isPrintable := true
		for _, b := range attr {
			if b < 32 || b > 126 {
				isPrintable = false
				break
			}
		}
		if isPrintable && len(valStr) > 0 {
			radiusLogger.Printf("     [%02d] %-22s #%d: %q (len=%d, hex=%x)", typeCode, name, i+1, valStr, len(attr), attr)
		} else {
			radiusLogger.Printf("     [%02d] %-22s #%d: [binary len=%d, hex=%x]", typeCode, name, i+1, len(attr), attr)
		}
	}
	radiusLogger.Printf("-----------------------------------------------------------------------------------------")
}

func logPacketOut(response *radius.Packet, remote net.Addr, start time.Time, writeErr error) {
	if !isRadiusDebug() {
		return
	}
	dur := time.Since(start)
	statusStr := "SUCCESS ✅"
	if writeErr != nil {
		statusStr = fmt.Sprintf("FAILED ❌ (%v)", writeErr)
	}
	radiusLogger.Printf("============================== [📤 RADIUS OUTGOING PACKET] =============================")
	radiusLogger.Printf("  🔸 Code: %v | ID: %d | To: %v | Status: %s | Time: %v", response.Code, response.Identifier, remote, statusStr, dur)
	radiusLogger.Printf("  🔸 Response Authenticator (pre-encode): %x", response.Authenticator)
	radiusLogger.Printf("  🔸 Attributes List (%d total):", len(response.Attributes))
	for i, avp := range response.Attributes {
		typeCode := byte(avp.Type)
		name := getAttributeName(typeCode)
		attr := avp.Attribute
		valStr := string(attr)
		isPrintable := true
		for _, b := range attr {
			if b < 32 || b > 126 {
				isPrintable = false
				break
			}
		}
		if isPrintable && len(valStr) > 0 {
			radiusLogger.Printf("     [%02d] %-22s #%d: %q (len=%d, hex=%x)", typeCode, name, i+1, valStr, len(attr), attr)
		} else {
			radiusLogger.Printf("     [%02d] %-22s #%d: [binary len=%d, hex=%x]", typeCode, name, i+1, len(attr), attr)
		}
	}
	radiusLogger.Printf("=========================================================================================")
}

func handleRadiusPacket(w radius.ResponseWriter, r *radius.Request) {
	logPacketIn(r)
	switch r.Code {
	case radius.CodeAccessRequest:
		username := rfc2865.UserName_GetString(r.Packet)
		nasIP, _, _ := net.SplitHostPort(r.RemoteAddr.String())
		radiusLogger.Printf("[radius] 🔑 طلب مصادقة جديد: يوزر [%s] | من NAS: %s", username, nasIP)
		handleAuthRequest(w, r)
	case radius.CodeAccountingRequest:
		handleAcctRequest(w, r)
	default:
		radiusLogger.Printf("[radius] Received unknown packet code: %v", r.Code)
	}
}

func handleAuthRequest(w radius.ResponseWriter, r *radius.Request) {
	startAuthTime := time.Now()
	username := rfc2865.UserName_GetString(r.Packet)
	debugEnabled := isRadiusDebug()

	if debugEnabled {
		radiusLogger.Printf("[radius] [DEBUG] handleAuthRequest: processing user [%s] from remote %v", username, r.RemoteAddr)
	}

	// 0. Check Scheduled Internet Shutdown
	if IsShutdownActiveForUser(username) {
		radiusLogger.Printf("[radius] ❌ رفض الاتصال: يوزر [%s] | السبب: جدول قطع الخدمة نشط حالياً", username)
		writeAccessReject(w, r, username, "internet_shutdown")
		return
	}

	// 1. Check Global Bypass (Blind Accept)
	if isBypassEnabled() {
		radiusLogger.Printf("[radius] ✅ تجاوز عام نشط: تم قبول اتصال [%s] تلقائياً", username)
		w.Write(r.Response(radius.CodeAccessAccept))
		return
	}

	// 2. Check if username specifies a cross-agent domain (e.g. user@ahmed.sas-man.net or user@ahmed)
	if strings.Contains(username, "@") || strings.Contains(username, "/") || strings.Contains(username, "\\") {
		if handleGlobalHotspotAuth(w, r, username) {
			return
		}
	}

	// 3. Fetch User Data from LMDB (with SQLite fallback for local users)
	data, err := getLMDBUserData(username)
	if err != nil || data == "" {
		if DB != nil {
			var dbPass string
			errDB := DB.QueryRow("SELECT value FROM radcheck WHERE username = ? AND attribute = 'Cleartext-Password'", username).Scan(&dbPass)
			if errDB == nil && dbPass != "" {
				var lines []string
				lines = append(lines, dbPass)
				// Fetch other attributes from radreply
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
		// Not found locally -> Try Central Server Global HotSpot / Voucher authentication before rejecting!
		if handleGlobalHotspotAuth(w, r, username) {
			return
		}
		if debugEnabled {
			radiusLogger.Printf("[radius] [DEBUG] User [%s] not found in LMDB/SQLite or query failed: %v", username, err)
		}
		writeAccessReject(w, r, username, "user not found")
		return
	}

	if debugEnabled {
		radiusLogger.Printf("[radius] [DEBUG] Retrieved raw LMDB data for user [%s]: %q", username, data)
	}

	lines := strings.Split(data, "\n")
	if len(lines) < 1 {
		if debugEnabled {
			radiusLogger.Printf("[radius] [DEBUG] Invalid user data format in LMDB for user [%s]", username)
		}
		writeAccessReject(w, r, username, "invalid user data")
		return
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

	// 3. Determine Auth Protocol & Verify Password
	var authSuccess bool
	var mppeKeys []byte
	authProtocol := "PAP"

	if password := rfc2865.UserPassword_GetString(r.Packet); password != "" {
		// PAP
		authSuccess = (password == dbPassword)
		if debugEnabled {
			radiusLogger.Printf("[radius] [DEBUG] PAP auth check for [%s]: received=%q (len=%d), dbPassword=%q (len=%d), Match=%t",
				username, password, len(password), dbPassword, len(dbPassword), authSuccess)
			if !authSuccess {
				radiusLogger.Printf("[radius] [DEBUG] PAP mismatch hex: received=%x, dbPassword=%x", []byte(password), []byte(dbPassword))
			}
		}
	} else if chapPass := rfc2865.CHAPPassword_Get(r.Packet); len(chapPass) > 0 {
		// CHAP
		authProtocol = "CHAP"
		if len(chapPass) == 17 {
			chapIdent := chapPass[0]
			chapHash := chapPass[1:]

			challenge := rfc2865.CHAPChallenge_Get(r.Packet)
			if len(challenge) == 0 {
				challenge = r.Packet.Authenticator[:]
			}

			// Calculate MD5 hash: MD5(chapIdent + dbPassword + challenge)
			h := md5.New()
			h.Write([]byte{chapIdent})
			h.Write([]byte(dbPassword))
			h.Write(challenge)
			expectedHash := h.Sum(nil)

			authSuccess = bytes.Equal(expectedHash, chapHash)

			if debugEnabled {
				radiusLogger.Printf("[radius] [DEBUG] CHAP check for [%s]: ident=%d, challenge=%x, receivedHash=%x, expectedHash=%x, Match=%t",
					username, chapIdent, challenge, chapHash, expectedHash, authSuccess)
			}
		} else {
			if debugEnabled {
				radiusLogger.Printf("[radius] [DEBUG] CHAP check for [%s]: invalid CHAPPassword length: %d (expected 17)", username, len(chapPass))
			}
		}
	} else if msc2Resp := microsoft.MSCHAP2Response_Get(r.Packet); msc2Resp != nil {
		// MS-CHAPv2
		authProtocol = "MS-CHAPv2"
		challenge := microsoft.MSCHAPChallenge_Get(r.Packet)
		if len(challenge) == 16 && len(msc2Resp) == 50 {
			peerChallenge := msc2Resp[2:18]
			peerResponse := msc2Resp[26:50]

			ntResponse, err := rfc2759.GenerateNTResponse(challenge, peerChallenge, []byte(username), []byte(dbPassword))
			if err == nil && bytes.Equal(ntResponse, peerResponse) {
				authSuccess = true
				// Generate MPPE keys
				ntHash := rfc2759.NTPasswordHash([]byte(dbPassword))
				ntHashHash := rfc2759.NTPasswordHash(ntHash)
				masterKey := rfc3079.GetMasterKey(ntHashHash, ntResponse)
				sendKey, _ := rfc3079.GetAsymmetricStartKey(masterKey, 16, true)
				recvKey, _ := rfc3079.GetAsymmetricStartKey(masterKey, 16, false)
				mppeKeys = append(sendKey, recvKey...)
			}
			if debugEnabled {
				radiusLogger.Printf("[radius] [DEBUG] MS-CHAPv2 check for [%s]: challenge=%x, peerChallenge=%x, peerResponse=%x, calculated NTResponse=%x, error=%v, Match=%t",
					username, challenge, peerChallenge, peerResponse, ntResponse, err, authSuccess)
				if !authSuccess {
					radiusLogger.Printf("[radius] [DEBUG] MS-CHAPv2 fail details: dbPassword=%q (len=%d), dbPasswordHex=%x", dbPassword, len(dbPassword), []byte(dbPassword))
				}
			}
		} else {
			if debugEnabled {
				radiusLogger.Printf("[radius] [DEBUG] MS-CHAPv2 invalid structure for [%s]: challenge length=%d (expected 16), msc2Resp length=%d (expected 50)",
					username, len(challenge), len(msc2Resp))
			}
		}
	} else {
		if debugEnabled {
			radiusLogger.Printf("[radius] [DEBUG] Unknown or unsupported auth protocol for user [%s]. Packet attributes: %v", username, r.Packet)
		}
	}

	if !authSuccess {
		if debugEnabled {
			radiusLogger.Printf("[radius] [DEBUG] Authentication failed for user [%s] using protocol %s", username, authProtocol)
		}
		writeAccessReject(w, r, username, "invalid credentials")
		return
	}

	// 4. Check Enabled Status & Expiration
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

	// Always read expired pool/profile FRESH from memory cache to avoid stale LMDB data or SQLite bottlenecks
	expiredPool, expiredProfile := lookupExpiredRedirect(username, attributes)

	// Also check LMDB attributes as fallback (in case DB lookup fails)
	if expiredPool == "" {
		expiredPool = strings.TrimSpace(attributes["Expired-Pool"])
	}
	if expiredProfile == "" {
		expiredProfile = strings.TrimSpace(attributes["Expired-Profile"])
	}

	if isExpiredOrDisabled {
		radiusLogger.Printf("[radius] ⚠️ المشترك [%s] منتهي/معطل | Expired-Pool=[%s] Expired-Profile=[%s]", username, expiredPool, expiredProfile)
		if expiredPool == "" && expiredProfile == "" {
			reason := "user expired"
			if isDisabled {
				reason = "user disabled"
			}
			writeAccessReject(w, r, username, reason)
			return
		}
	}

	// 5. Check NAS-IP-Address binding
	if val, ok := attributes["NAS-IP-Address"]; ok && val != "ALL" && val != "" {
		nasIP, _, _ := net.SplitHostPort(r.RemoteAddr.String())
		if val != nasIP {
			writeAccessReject(w, r, username, "NAS-IP-Address mismatch")
			return
		}
	}

	// 6. Check Simultaneous-Use (only for active users)
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

	// 7. Success - Build Accept Packet
	response := r.Response(radius.CodeAccessAccept)

	// Add User-Name back (important for some NAS)
	rfc2865.UserName_Add(response, []byte(username))

	// Match protocol requirements: Add Framed-Protocol: PPP only for PPP sessions
	reqServiceType := rfc2865.ServiceType_Get(r.Packet)
	reqFramedProtocol := rfc2865.FramedProtocol_Get(r.Packet)
	isPPP := (reqFramedProtocol == rfc2865.FramedProtocol_Value_PPP || reqServiceType == rfc2865.ServiceType_Value_FramedUser)
	if isPPP {
		rfc2865.ServiceType_Add(response, rfc2865.ServiceType_Value_FramedUser)
		rfc2865.FramedProtocol_Add(response, rfc2865.FramedProtocol_Value_PPP)
	} else if reqServiceType != 0 {
		rfc2865.ServiceType_Add(response, reqServiceType)
	} else {
		rfc2865.ServiceType_Add(response, rfc2865.ServiceType_Value_LoginUser)
	}

	// Optional: Set Interim Interval to 5 minutes
	rfc2869.AcctInterimInterval_Add(response, 300)

	// Add attributes to response
	if isExpiredOrDisabled {
		if expiredPool != "" && isPPP {
			addReplyAttribute(response, "Framed-Pool", expiredPool)
		}
		if expiredProfile != "" {
			addReplyAttribute(response, "Mikrotik-Group", expiredProfile)
		}
	} else {
		// Add standard attributes from LMDB for active subscribers
		for k, v := range attributes {
			if k == "Enabled" || k == "Expiration" || k == "Simultaneous-Use" || k == "NAS-IP-Address" || k == "Expired-Pool" || k == "Expired-Profile" || k == "User-Group" {
				continue
			}
			// Do NOT send Framed-Pool or Framed-IP-Netmask to HotSpot (non-PPP) users
			if (k == "Framed-Pool" || k == "Framed-IP-Netmask") && !isPPP {
				continue
			}
			addReplyAttribute(response, k, v)
		}
	}

	// Add MS-CHAPv2 success and MPPE keys if applicable
	if msc2Resp := microsoft.MSCHAP2Response_Get(r.Packet); msc2Resp != nil && authSuccess {
		peerResponse := msc2Resp[26:50]
		challenge := microsoft.MSCHAPChallenge_Get(r.Packet)
		authResp, _ := rfc2759.GenerateAuthenticatorResponse(challenge, msc2Resp[2:18], peerResponse, []byte(username), []byte(dbPassword))
		microsoft.MSCHAP2Success_Add(response, append([]byte{msc2Resp[0]}, []byte(authResp)...))

		if len(mppeKeys) == 32 {
			sendKeyRaw := mppeKeys[0:16]
			recvKeyRaw := mppeKeys[16:32]

			microsoft.MSMPPESendKey_Add(response, sendKeyRaw)
			microsoft.MSMPPERecvKey_Add(response, recvKeyRaw)
			microsoft.MSMPPEEncryptionPolicy_Add(response, microsoft.MSMPPEEncryptionPolicy_Value_EncryptionRequired)
			microsoft.MSMPPEEncryptionTypes_Add(response, microsoft.MSMPPEEncryptionTypes_Value_RC4128bitAllowed)
		}
	}

	if isExpiredOrDisabled {
		reason := "منتهي الاشتراك"
		if isDisabled {
			reason = "حساب معطل"
		}
		radiusLogger.Printf("[radius] ⚠️ تحويل: قبول اتصال [%s] بالباقة المحدودة (%s) | Pool=%s, Profile=%s | NAS: %v", username, reason, expiredPool, expiredProfile, r.RemoteAddr)
	} else {
		radiusLogger.Printf("[radius] ✅ مصادقة ناجحة: تم قبول اتصال [%s] بنجاح | البروتوكول: %s | NAS: %v", username, authProtocol, r.RemoteAddr)
	}

	// Sign Message-Authenticator if the request contained Message-Authenticator (RFC 2869 Requirement)
	if reqMA := rfc2869.MessageAuthenticator_Get(r.Packet); reqMA != nil {
		if err := signMessageAuthenticator(response); err != nil {
			radiusLogger.Printf("[radius] ❌ خطأ في توقيع Message-Authenticator: %v", err)
		}
	}

	writeErr := w.Write(response)
	logPacketOut(response, r.RemoteAddr, startAuthTime, writeErr)
}

func handleGlobalHotspotAuth(w radius.ResponseWriter, r *radius.Request, username string) bool {
	startAuthTime := time.Now()
	password := rfc2865.UserPassword_GetString(r.Packet)
	callingStation := rfc2865.CallingStationID_GetString(r.Packet)
	framedIP := rfc2865.FramedIPAddress_Get(r.Packet)
	nasIP, _, _ := net.SplitHostPort(r.RemoteAddr.String())

	var framedIPStr string
	if framedIP != nil {
		framedIPStr = framedIP.String()
	}

	gReq := tunnel.GlobalAuthRequestPayload{
		Username:      username,
		Password:      password,
		UserMAC:       callingStation,
		UserIP:        framedIPStr,
		NasIP:         nasIP,
	}

	radiusLogger.Printf("[radius] 🌐 توجيه طلب المصادقة للسيرفر المركزي: يوزر [%s] | MAC: %s", username, callingStation)

	resp, err := tunnel.RequestGlobalAuth(gReq, 4*time.Second)
	if err != nil {
		radiusLogger.Printf("[radius] ⚠️ تعذر الوصول للسيرفر المركزي للمستخدم [%s]: %v", username, err)
		return false
	}

	if !resp.Allow {
		radiusLogger.Printf("[radius] ❌ رفض المصادقة المركزية للمستخدم [%s]: %s", username, resp.RejectReason)
		writeAccessReject(w, r, username, resp.RejectReason)
		return true
	}

	radiusLogger.Printf("[radius] ✅ تم قبول المصادقة المركزية للمستخدم [%s] بنجاح! نوع الحساب: %s | السرعة: %s", username, resp.AccountType, resp.RateLimit)

	reply := r.Response(radius.CodeAccessAccept)
	if resp.RateLimit != "" {
		addReplyAttribute(reply, "Mikrotik-Rate-Limit", resp.RateLimit)
	}
	if resp.SessionTimeout > 0 {
		rfc2865.SessionTimeout_Add(reply, rfc2865.SessionTimeout(resp.SessionTimeout))
	}
	if resp.IdleTimeout > 0 {
		rfc2865.IdleTimeout_Add(reply, rfc2865.IdleTimeout(resp.IdleTimeout))
	}
	if resp.ReplyMessage != "" {
		rfc2865.ReplyMessage_Add(reply, []byte(resp.ReplyMessage))
	} else {
		rfc2865.ReplyMessage_Add(reply, []byte("SASMAN Global HotSpot Welcome"))
	}

	// Sign Message-Authenticator if request contained it
	if reqMA := rfc2869.MessageAuthenticator_Get(r.Packet); reqMA != nil {
		if err := signMessageAuthenticator(reply); err != nil {
			radiusLogger.Printf("[radius] ❌ خطأ في توقيع Message-Authenticator المركزي: %v", err)
		}
	}

	writeErr := w.Write(reply)
	logPacketOut(reply, r.RemoteAddr, startAuthTime, writeErr)
	return true
}

func writeAccessReject(w radius.ResponseWriter, r *radius.Request, username, reason string) {
	response := r.Response(radius.CodeAccessReject)
	if username != "" {
		rfc2865.UserName_Add(response, []byte(username))
	}
	if reason != "" {
		rfc2865.ReplyMessage_AddString(response, reason)
	}
	radiusLogger.Printf("[radius] ❌ رفض الاتصال: يوزر [%s] | السبب: %s | NAS: %v", username, translateRejectReason(reason), r.RemoteAddr)

	// Sign Message-Authenticator if request contained it
	if reqMA := rfc2869.MessageAuthenticator_Get(r.Packet); reqMA != nil {
		if err := signMessageAuthenticator(response); err != nil {
			radiusLogger.Printf("[radius] ❌ خطأ في توقيع Message-Authenticator للرفض: %v", err)
		}
	}

	writeErr := w.Write(response)
	logPacketOut(response, r.RemoteAddr, time.Now(), writeErr)
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

func handleAcctRequest(w radius.ResponseWriter, r *radius.Request) {
	username := rfc2865.UserName_GetString(r.Packet)
	statusType := rfc2866.AcctStatusType_Get(r.Packet)

	// Acknowledge immediately
	w.Write(r.Response(radius.CodeAccountingResponse))

	if username == "" {
		return
	}

	sid := rfc2866.AcctSessionID_GetString(r.Packet)
	ip := rfc2865.FramedIPAddress_Get(r.Packet).String()
	cli := rfc2865.CallingStationID_GetString(r.Packet)

	inOct := uint64(rfc2866.AcctInputOctets_Get(r.Packet))
	outOct := uint64(rfc2866.AcctOutputOctets_Get(r.Packet))

	// Handle Gigawords (for 64-bit counters)
	if inGW := r.Packet.Get(rfc2869.AcctInputGigawords_Type); inGW != nil {
		inOct += uint64(rfc2869.AcctInputGigawords_Get(r.Packet)) << 32
	}
	if outGW := r.Packet.Get(rfc2869.AcctOutputGigawords_Type); outGW != nil {
		outOct += uint64(rfc2869.AcctOutputGigawords_Get(r.Packet)) << 32
	}

	sessionSecs := int64(rfc2866.AcctSessionTime_Get(r.Packet))
	nasIP, _, _ := net.SplitHostPort(r.RemoteAddr.String())
	if nasAttr := rfc2865.NASIPAddress_Get(r.Packet); nasAttr != nil {
		nasIP = nasAttr.String()
	}
	termCause := rfc2866.AcctTerminateCause_Get(r.Packet)

	statusStr := ""
	switch statusType {
	case rfc2866.AcctStatusType_Value_Start:
		statusStr = "بدء اتصال 🟢"
	case rfc2866.AcctStatusType_Value_Stop:
		statusStr = "قطع اتصال 🔴"
	case rfc2866.AcctStatusType_Value_InterimUpdate:
		statusStr = "تحديث دوري 🔄"
	}
	
	if statusStr != "" && statusType != rfc2866.AcctStatusType_Value_InterimUpdate {
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
		case rfc2866.AcctStatusType_Value_Start:
			statusTypeStr = "Start"
		case rfc2866.AcctStatusType_Value_Stop:
			statusTypeStr = "Stop"
		case rfc2866.AcctStatusType_Value_InterimUpdate:
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

	// 4. Invalidate Session Cache so dashboard and user list reflect changes instantly
	InvalidateSessionCache()
}

func recordSQLiteAccounting(username string, status rfc2866.AcctStatusType, sid, ip, cli, nasIP string, in, out uint64, secs int64, termCause rfc2866.AcctTerminateCause) {
	if DB == nil || username == "" {
		return
	}
	switch status {
	case rfc2866.AcctStatusType_Value_Start:
		// Close previous open sessions for this user/sid
		_, _ = DB.Exec(`UPDATE radacct SET acctstoptime = CURRENT_TIMESTAMP, acctterminatecause = 'Stale-Replaced' WHERE username = ? AND acctstoptime IS NULL`, username)
		_, _ = DB.Exec(`INSERT INTO radacct (username, acctsessionid, nasipaddress, callingstationid, framedipaddress, acctstarttime, acctupdatetime, acctsessiontime, acctinputoctets, acctoutputoctets)
			VALUES (?, ?, ?, ?, ?, datetime('now', 'localtime'), datetime('now', 'localtime'), 0, 0, 0)`,
			username, sid, nasIP, cli, ip)
	case rfc2866.AcctStatusType_Value_InterimUpdate:
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
	case rfc2866.AcctStatusType_Value_Stop:
		causeStr := termCause.String()
		if causeStr == "" {
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

// getLMDBUserData is a wrapper around LMDB lookups
// This will be implemented in lmdb_sync.go (Linux) and lmdb_sync_stub.go (Windows)
func getLMDBUserData(username string) (string, error) {
	// We'll add this function to lmdb_sync.go
	return fetchUserFromLMDB(username)
}

// updateLMDBAccounting is a wrapper around LMDB writes
func updateLMDBAccounting(username string, status rfc2866.AcctStatusType, sid, ip, cli string, in, out uint64, secs int64) {
	// We'll add this function to lmdb_sync.go
	saveAccountingToLMDB(username, status, sid, ip, cli, in, out, secs)
}

// VerifyLocalUser checks local LMDB / SQLite user credentials for cross-agent validation
func VerifyLocalUser(username, password string) (bool, string, string, error) {
	data, err := getLMDBUserData(username)
	if err != nil {
		if DB == nil {
			return false, "", "User not found", fmt.Errorf("user not found")
		}
		var dbPass string
		err = DB.QueryRow("SELECT value FROM radcheck WHERE username = ? AND attribute = 'Cleartext-Password'", username).Scan(&dbPass)
		if err != nil {
			return false, "", "المستخدم غير مسجل لدى هذا الوكيل", fmt.Errorf("user not found")
		}
		if password != "" && password != dbPass {
			return false, "", "كلمة المرور غير صحيحة", nil
		}
		return true, "10M/10M", "OK", nil
	}

	lines := strings.Split(data, "\n")
	if len(lines) < 1 {
		return false, "", "بيانات المستخدم معطوبة", fmt.Errorf("invalid user data")
	}
	dbPassword := lines[0]
	if password != "" && password != dbPassword {
		return false, "", "كلمة المرور غير صحيحة", nil
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
	return true, rateLimit, "OK", nil
}

// addReplyAttribute adds an attribute to the response packet.
func addReplyAttribute(p *radius.Packet, name, value string) {
	radiusLogger.Printf("[radius] Adding Reply Attr: %s = %s", name, value)

	nameLower := strings.ToLower(name)

	// Handle MikroTik specific attributes by name
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

		val := []byte(value)
		vsa := make([]byte, 4+2+len(val))
		vsa[0], vsa[1], vsa[2], vsa[3] = 0, 0, 0x3a, 0x98 // Vendor 14988
		vsa[4] = subType
		vsa[5] = byte(len(val) + 2)
		copy(vsa[6:], val)
		p.Add(26, vsa)
		return
	}

	// Standard attributes
	switch name {
	case "Framed-IP-Address":
		if ip := net.ParseIP(value); ip != nil {
			rfc2865.FramedIPAddress_Add(p, ip)
		} else {
			radiusLogger.Printf("[radius] ERROR: Invalid IP address for Framed-IP-Address: %s", value)
		}
	case "Framed-Pool":
		p.Add(88, []byte(value))
	case "Session-Timeout":
		if v, err := strconv.Atoi(value); err == nil {
			rfc2865.SessionTimeout_Add(p, rfc2865.SessionTimeout(v))
		}
	default:
		radiusLogger.Printf("[radius] WARNING: Skipping unknown standard attribute %s", name)
	}
}

func signMessageAuthenticator(p *radius.Packet) error {
	rfc2869.MessageAuthenticator_Set(p, make([]byte, md5.Size))

	wire, err := p.MarshalBinary()
	if err != nil {
		return err
	}

	mac := hmac.New(md5.New, p.Secret)
	mac.Write(wire)
	return rfc2869.MessageAuthenticator_Set(p, mac.Sum(nil))
}

// UpdateProfileRedirectsCache loads all expired redirect pool and profile mappings
// from SQLite into the thread-safe in-memory cache. This must be called at startup
// and whenever a profile is created/updated.
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

