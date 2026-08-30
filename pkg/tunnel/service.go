package tunnel

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"mikrotik-manager/pkg/broadcast"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type AgentSession struct {
	ID          string          `json:"id"`
	Subdomain   string          `json:"subdomain"`
	Token       string          `json:"token"`
	WinboxPort  int             `json:"winbox_port"`
	Version     string          `json:"version"`
	Arch        string          `json:"arch"`
	Connected   bool            `json:"connected"`
	LastSeen    time.Time       `json:"last_seen"`
	Conn        *websocket.Conn `json:"-"`
	SyncData    map[string]interface{} `json:"sync_data,omitempty"`
	writeMu     sync.Mutex
	tcpListener net.Listener
	tcpConns    map[string]net.Conn
	tcpMu       sync.RWMutex
}

func (s *AgentSession) WriteJSON(v any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.Conn == nil {
		return fmt.Errorf("agent offline")
	}
	return s.Conn.WriteJSON(v)
}

func (s *AgentSession) CloseTCP() {
	s.tcpMu.Lock()
	defer s.tcpMu.Unlock()

	if s.tcpListener != nil {
		_ = s.tcpListener.Close()
		s.tcpListener = nil
	}

	for id, conn := range s.tcpConns {
		if conn != nil {
			_ = conn.Close()
		}
		delete(s.tcpConns, id)
	}
}

type TunnelMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type HttpRequestPayload struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"` // Base64 encoded
}

type HttpResponsePayload struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"` // Base64 encoded
}

type TcpConnectPayload struct {
	ConnID string `json:"conn_id"`
}

type TcpDataPayload struct {
	ConnID string `json:"conn_id"`
	Data   string `json:"data"` // Base64 encoded TCP chunk
}

type TcpClosePayload struct {
	ConnID string `json:"conn_id"`
}

type BackupRequestPayload struct {
	RequestID string `json:"request_id"`
}

type BackupChunkPayload struct {
	RequestID string `json:"request_id"`
	Index     int    `json:"index"`
	Data      string `json:"data"`
}

type BackupCompletePayload struct {
	RequestID string `json:"request_id"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
}

type BackupErrorPayload struct {
	RequestID string `json:"request_id"`
	Error     string `json:"error"`
}

type BackupChunkMsg struct {
	Chunk    *BackupChunkPayload
	Done     bool
	Filename string
	Size     int64
	Error    string
}

type SyncConfigPayload struct {
	InstallationID string                 `json:"installation_id"`
	LastEvent      string                 `json:"last_event"`
	UpdatedAt      string                 `json:"updated_at"`
	OwnerName      string                 `json:"owner_name,omitempty"`
	OwnerPhone     string                 `json:"owner_phone,omitempty"`
	App            map[string]interface{} `json:"app"`
	Container      map[string]interface{} `json:"container"`
	Mikrotik       map[string]interface{} `json:"mikrotik"`
	License        map[string]interface{} `json:"license"`
	RemoteAccess   map[string]interface{} `json:"remote_access"`
	Credentials    map[string]interface{} `json:"credentials,omitempty"`
}

type GlobalAuthRequestPayload struct {
	RequestID        string `json:"request_id"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	CHAPPassword     string `json:"chap_password,omitempty"`
	CHAPChallenge    string `json:"chap_challenge,omitempty"`
	UserMAC          string `json:"user_mac"`
	UserIP           string `json:"user_ip"`
	NasIP            string `json:"nas_ip"`
	VisitedSubdomain string `json:"visited_subdomain"`
}

type GlobalAuthResponsePayload struct {
	RequestID      string `json:"request_id"`
	Allow          bool   `json:"allow"`
	RateLimit      string `json:"rate_limit,omitempty"`
	SessionTimeout int    `json:"session_timeout,omitempty"`
	IdleTimeout    int    `json:"idle_timeout,omitempty"`
	ReplyMessage   string `json:"reply_message,omitempty"`
	RejectReason   string `json:"reject_reason,omitempty"`
	AccountType    string `json:"account_type,omitempty"` // 'voucher' | 'roaming_user'
	Password       string `json:"password,omitempty"`
}

type GlobalAcctPayload struct {
	SessionID        string `json:"session_id"`
	Username         string `json:"username"`
	StatusType       string `json:"status_type"` // 'Start', 'Interim-Update', 'Stop'
	UserMAC          string `json:"user_mac"`
	UserIP           string `json:"user_ip"`
	NasIP            string `json:"nas_ip"`
	VisitedSubdomain string `json:"visited_subdomain"`
	BytesIn          int64  `json:"bytes_in"`
	BytesOut         int64  `json:"bytes_out"`
	SessionTimeSec   int    `json:"session_time_sec"`
	TerminateCause   string `json:"terminate_cause,omitempty"`
}

type Service struct {
	mu                   sync.RWMutex
	sessions             map[string]*AgentSession
	pendingRequests      map[string]chan *HttpResponsePayload
	pendingBackups       map[string]chan *BackupChunkMsg
	assetCache           *AssetCache
	db                   *sql.DB
	OnRelayMessage       func(session *AgentSession, msg TunnelMessage)
	OnBroadcastLog       func(log broadcast.BroadcastLogPayload)
	OnAgentRegistered    func(subdomain string)
	OnSyncConfigReceived func(subdomain string, payload SyncConfigPayload)
	OnGlobalAuthRequest  func(subdomain string, req GlobalAuthRequestPayload) GlobalAuthResponsePayload
	OnGlobalAcctUpdate   func(subdomain string, payload GlobalAcctPayload)
}

func NewService(db *sql.DB) *Service {
	return &Service{
		sessions:        make(map[string]*AgentSession),
		pendingRequests: make(map[string]chan *HttpResponsePayload),
		pendingBackups:  make(map[string]chan *BackupChunkMsg),
		assetCache:      nil, // Cache completely disabled - direct stream from agent container
		db:              db,
	}
}

func (s *Service) BroadcastToAgents(msg TunnelMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sess := range s.sessions {
		if sess.Connected && sess.Conn != nil {
			_ = sess.WriteJSON(msg)
		}
	}
}


func (s *Service) RestoreSessions() {
	if s.db == nil {
		return
	}

	rows, err := s.db.Query(`SELECT subdomain, token, winbox_port FROM subdomains WHERE status = 'active' AND token != '' AND winbox_port > 0`)
	if err != nil {
		log.Printf("[Tunnel] Failed to restore sessions: %v", err)
		return
	}
	defer rows.Close()

	restored := 0
	for rows.Next() {
		var subdomain, token string
		var winboxPort int
		if err := rows.Scan(&subdomain, &token, &winboxPort); err != nil {
			continue
		}

		s.mu.RLock()
		exists := false
		for _, sess := range s.sessions {
			if strings.EqualFold(sess.Subdomain, subdomain) {
				exists = true
				break
			}
		}
		s.mu.RUnlock()

		if exists {
			continue
		}

		id := fmt.Sprintf("agent-%d", time.Now().UnixNano())
		session := &AgentSession{
			ID:         id,
			Subdomain:  subdomain,
			Token:      token,
			WinboxPort: winboxPort,
			Connected:  false,
			LastSeen:   time.Now().UTC(),
			tcpConns:   make(map[string]net.Conn),
		}

		s.mu.Lock()
		s.sessions[id] = session
		s.mu.Unlock()

		go s.startWinboxTCPListener(session)
		restored++
	}

	if restored > 0 {
		log.Printf("[Tunnel] Restored %d session(s) from database", restored)
	}
}

func generateDefaultSubdomain() string {
	return fmt.Sprintf("sasman-%d", time.Now().Unix())
}

func generateDefaultToken() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 16)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return "tok-" + string(b)
}

func (s *Service) allocateFreeWinboxPort() int {
	usedPorts := make(map[int]bool)
	for _, sess := range s.sessions {
		if sess.WinboxPort > 0 {
			usedPorts[sess.WinboxPort] = true
		}
	}

	startPort := 10001
	endPort := 60000

	for port := startPort; port <= endPort; port++ {
		if !usedPorts[port] {
			l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err == nil {
				l.Close()
				return port
			}
		}
	}
	return 0
}

func (s *Service) RegisterAgent(subdomain, token string) *AgentSession {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(subdomain) == "" {
		subdomain = generateDefaultSubdomain()
	}
	if strings.TrimSpace(token) == "" {
		token = generateDefaultToken()
	}

	// Deduplicate: If an agent with the same subdomain exists, reuse it and update token
	for _, sess := range s.sessions {
		if strings.EqualFold(sess.Subdomain, subdomain) {
			if token != "" {
				sess.Token = token
			}
			_ = s.SaveSession(sess, "customer-default", fmt.Sprintf("license-%s", sess.ID))
			return sess
		}
	}

	winboxPort := s.allocateFreeWinboxPort()

	id := fmt.Sprintf("agent-%d", time.Now().UnixNano())
	session := &AgentSession{
		ID:         id,
		Subdomain:  subdomain,
		Token:      token,
		WinboxPort: winboxPort,
		Connected:  false,
		LastSeen:   time.Now().UTC(),
		tcpConns:   make(map[string]net.Conn),
	}
	s.sessions[id] = session

	if winboxPort > 0 {
		go s.startWinboxTCPListener(session)
	}

	_ = s.SaveSession(session, "customer-default", fmt.Sprintf("license-%s", id))

	return session
}

func (s *Service) SaveSession(session *AgentSession, customerID, licenseID string) error {
	if s.db == nil {
		return nil
	}

	if strings.TrimSpace(customerID) == "" {
		customerID = "customer-default"
	}
	if strings.TrimSpace(licenseID) == "" {
		licenseID = fmt.Sprintf("license-%s", session.ID)
	}

	_, err := s.db.Exec(`INSERT INTO subdomains (id, customer_id, license_id, subdomain, zone_name, status, token, winbox_port, assigned_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'active', ?, ?, datetime('now'), datetime('now'), datetime('now'))
		ON CONFLICT(subdomain) DO UPDATE SET
			token = excluded.token,
			winbox_port = excluded.winbox_port,
			status = 'active',
			updated_at = datetime('now')`,
		session.ID, customerID, licenseID, session.Subdomain, "sas-man.net", session.Token, session.WinboxPort)
	return err
}

func (s *Service) startWinboxTCPListener(session *AgentSession) {
	addr := fmt.Sprintf(":%d", session.WinboxPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[Tunnel] Failed to start Winbox TCP Listener on %s: %v", addr, err)
		return
	}

	session.tcpMu.Lock()
	session.tcpListener = listener
	session.tcpMu.Unlock()

	log.Printf("[Tunnel] Winbox TCP Listener running for subdomain %s on port %d", session.Subdomain, session.WinboxPort)

	for {
		conn, err := listener.Accept()
		if err != nil {
			break
		}

		if tcpConn, ok := conn.(*net.TCPConn); ok {
			_ = tcpConn.SetNoDelay(true)
			_ = tcpConn.SetKeepAlive(true)
			_ = tcpConn.SetKeepAlivePeriod(15 * time.Second)
		}

		if !session.Connected || session.Conn == nil {
			targetIP := resolveTunnelIPForSubdomain(session.Subdomain)
			if targetIP != "" {
				go handleDirectWinboxProxy(conn, targetIP, session.Subdomain)
				continue
			}
			conn.Close()
			continue
		}

		connID := fmt.Sprintf("tcp-%d-%d", time.Now().UnixNano(), rand.Intn(100000))

		session.tcpMu.Lock()
		session.tcpConns[connID] = conn
		session.tcpMu.Unlock()

		// Send tcp_connect to Agent
		payloadBytes, _ := json.Marshal(TcpConnectPayload{ConnID: connID})
		msg := TunnelMessage{
			Type:    "tcp_connect",
			Payload: payloadBytes,
		}
		if err := session.WriteJSON(msg); err != nil {
			log.Printf("[Tunnel] Failed to send tcp_connect to agent: %v", err)
			conn.Close()
			session.tcpMu.Lock()
			delete(session.tcpConns, connID)
			session.tcpMu.Unlock()
			continue
		}

		// Read from Winbox Client TCP connection and send to Agent via WebSocket
		go func(c net.Conn, id string) {
			defer func() {
				c.Close()
				session.tcpMu.Lock()
				delete(session.tcpConns, id)
				session.tcpMu.Unlock()

				closePayload, _ := json.Marshal(TcpClosePayload{ConnID: id})
				_ = session.WriteJSON(TunnelMessage{
					Type:    "tcp_close",
					Payload: closePayload,
				})
			}()

			buf := make([]byte, 32*1024)
			for {
				n, err := c.Read(buf)
				if n > 0 {
					dataB64 := base64.StdEncoding.EncodeToString(buf[:n])
					dataPayload, _ := json.Marshal(TcpDataPayload{
						ConnID: id,
						Data:   dataB64,
					})
					if writeErr := session.WriteJSON(TunnelMessage{
						Type:    "tcp_data",
						Payload: dataPayload,
					}); writeErr != nil {
						break
					}
				}
				if err != nil {
					break
				}
			}
		}(conn, connID)
	}
}

func (s *Service) ListAgents() []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessMap := make(map[string]*AgentSession)
	for _, session := range s.sessions {
		subKey := strings.ToLower(strings.TrimSpace(session.Subdomain))
		if subKey == "" {
			continue
		}
		existing, found := sessMap[subKey]
		if !found {
			sessMap[subKey] = session
		} else {
			if session.Connected && !existing.Connected {
				sessMap[subKey] = session
			} else if session.LastSeen.After(existing.LastSeen) && (!existing.Connected || session.Connected) {
				sessMap[subKey] = session
			}
		}
	}

	out := make([]map[string]any, 0, len(sessMap))
	for _, session := range sessMap {
		out = append(out, map[string]any{
			"id":          session.ID,
			"subdomain":   session.Subdomain,
			"token":       session.Token,
			"winbox_port": session.WinboxPort,
			"version":     session.Version,
			"arch":        session.Arch,
			"connected":   session.Connected,
			"last_seen":   session.LastSeen.UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		subI, _ := out[i]["subdomain"].(string)
		subJ, _ := out[j]["subdomain"].(string)
		return subI < subJ
	})
	return out
}

func (s *Service) GetAgentBySubdomain(subdomain string) *AgentSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		if strings.EqualFold(session.Subdomain, subdomain) && session.Connected {
			return session
		}
	}
	for _, session := range s.sessions {
		if strings.EqualFold(session.Subdomain, subdomain) {
			return session
		}
	}
	return nil
}

func (s *Service) IsAgentConnected(subdomain string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		if strings.EqualFold(session.Subdomain, subdomain) {
			return session.Connected && session.Conn != nil
		}
	}
	return false
}

func (s *Service) ListOnlineAgents() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []string
	seen := make(map[string]bool)
	for _, session := range s.sessions {
		if session.Connected && session.Conn != nil && session.Subdomain != "" {
			subKey := strings.ToLower(session.Subdomain)
			if !seen[subKey] {
				result = append(result, session.Subdomain)
				seen[subKey] = true
			}
		}
	}
	return result
}

func (s *Service) RemoveAgent(subdomain string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for id, session := range s.sessions {
		if strings.EqualFold(session.Subdomain, subdomain) {
			session.CloseTCP()
			if session.Conn != nil {
				_ = session.Conn.Close()
			}
			delete(s.sessions, id)
			found = true
		}
	}
	return found
}

func (s *Service) RotateToken(subdomain string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session := range s.sessions {
		if session.Subdomain == subdomain {
			session.Token = generateDefaultToken()
			return session.Token, true
		}
	}
	return "", false
}

func (s *Service) ForwardRequestToAgent(c *fiber.Ctx, subdomain string) error {
	originalPath := strings.Clone(c.OriginalURL())
	method := strings.Clone(c.Method())

	agent := s.GetAgentBySubdomain(subdomain)
	if agent == nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("SASMAN tunnel agent is offline")
	}

	bodyBytes := c.Body()
	bodyBase64 := base64.StdEncoding.EncodeToString(bodyBytes)

	headers := make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})

	// Strip compression headers so agent sends clean uncompressed payloads over tunnel
	delete(headers, "Accept-Encoding")
	delete(headers, "accept-encoding")

	reqPayload := HttpRequestPayload{
		Method:  method,
		Path:    originalPath,
		Headers: headers,
		Body:    bodyBase64,
	}

	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("marshal request: " + err.Error())
	}

	reqID := fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), rand.Intn(100000))
	respChan := make(chan *HttpResponsePayload, 1)

	s.mu.Lock()
	s.pendingRequests[reqID] = respChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pendingRequests, reqID)
		s.mu.Unlock()
	}()

	msg := TunnelMessage{
		Type:      "http_request",
		RequestID: reqID,
		Payload:   payloadBytes,
	}

	if err := agent.WriteJSON(msg); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("write to agent failed: " + err.Error())
	}

	select {
	case resp := <-respChan:
		respBody, err := base64.StdEncoding.DecodeString(resp.Body)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("decode response body: " + err.Error())
		}

		for k, v := range resp.Headers {
			lk := strings.ToLower(k)
			if lk == "connection" || lk == "content-length" || lk == "transfer-encoding" || lk == "cache-control" || lk == "expires" || lk == "pragma" || lk == "content-encoding" {
				continue
			}
			c.Set(k, v)
		}

		// Strictly prevent any browser/proxy caching for forwarded agent web UI
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")
		c.Set("Pragma", "no-cache")
		c.Set("Expires", "0")

		return c.Status(resp.Status).Send(respBody)

	case <-time.After(20 * time.Second):
		return c.Status(fiber.StatusGatewayTimeout).SendString("gateway timeout waiting for agent response")
	}
}

// ClearAssetCache purges the in-memory static assets cache
func (s *Service) ClearAssetCache() int {
	return 0
}

// GetAssetCacheStats returns stats about the current static asset cache
func (s *Service) GetAssetCacheStats() map[string]interface{} {
	return map[string]interface{}{"enabled": false, "cached_files": 0, "total_bytes": 0}
}

// SendAgentHTTPRequest sends an internal HTTP request to a connected agent and returns the decoded response
func (s *Service) SendAgentHTTPRequest(subdomain string, method, path string, body []byte, headers map[string]string) (*HttpResponsePayload, []byte, error) {
	agent := s.GetAgentBySubdomain(subdomain)
	if agent == nil {
		return nil, nil, fmt.Errorf("الوكيل غير متصل حالياً (Agent offline)")
	}

	bodyBase64 := ""
	if len(body) > 0 {
		bodyBase64 = base64.StdEncoding.EncodeToString(body)
	}

	if headers == nil {
		headers = make(map[string]string)
	}
	if headers["Content-Type"] == "" && headers["content-type"] == "" {
		headers["Content-Type"] = "application/json"
	}

	reqPayload := HttpRequestPayload{
		Method:  method,
		Path:    path,
		Headers: headers,
		Body:    bodyBase64,
	}

	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request: %w", err)
	}

	reqID := fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), rand.Intn(100000))
	respChan := make(chan *HttpResponsePayload, 1)

	s.mu.Lock()
	s.pendingRequests[reqID] = respChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pendingRequests, reqID)
		s.mu.Unlock()
	}()

	msg := TunnelMessage{
		Type:      "http_request",
		RequestID: reqID,
		Payload:   payloadBytes,
	}

	if err := agent.WriteJSON(msg); err != nil {
		return nil, nil, fmt.Errorf("write to agent: %w", err)
	}

	select {
	case resp := <-respChan:
		if resp == nil {
			return nil, nil, fmt.Errorf("empty response received from agent")
		}
		respBody, _ := base64.StdEncoding.DecodeString(resp.Body)
		return resp, respBody, nil
	case <-time.After(30 * time.Second):
		return nil, nil, fmt.Errorf("مهلة الاستجابة انتهت من الراوتر (Request timeout)")
	}
}

func (s *Service) RequestBackup(subdomain string) ([]byte, string, error) {
	agent := s.GetAgentBySubdomain(subdomain)
	if agent == nil {
		return nil, "", fmt.Errorf("agent offline")
	}

	reqID := fmt.Sprintf("backup-%d-%d", time.Now().UnixNano(), rand.Intn(100000))
	respChan := make(chan *BackupChunkMsg, 256)

	s.mu.Lock()
	s.pendingBackups[reqID] = respChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pendingBackups, reqID)
		s.mu.Unlock()
		close(respChan)
	}()

	payloadBytes, _ := json.Marshal(BackupRequestPayload{RequestID: reqID})
	msg := TunnelMessage{
		Type:    "backup_request",
		Payload: payloadBytes,
	}

	if err := agent.WriteJSON(msg); err != nil {
		return nil, "", fmt.Errorf("write to agent failed: %v", err)
	}

	var buf bytes.Buffer
	var filename string

	for {
		select {
		case resp, ok := <-respChan:
			if !ok {
				return nil, "", fmt.Errorf("backup channel closed unexpectedly")
			}
			if resp.Error != "" {
				return nil, "", fmt.Errorf("backup failed: %s", resp.Error)
			}
			if resp.Done {
				filename = resp.Filename
				return buf.Bytes(), filename, nil
			}
			if resp.Chunk != nil {
				data, err := base64.StdEncoding.DecodeString(resp.Chunk.Data)
				if err != nil {
					return nil, "", fmt.Errorf("decode chunk failed: %v", err)
				}
				buf.Write(data)
			}
		case <-time.After(120 * time.Second):
			return nil, "", fmt.Errorf("backup timeout waiting for agent response")
		}
	}
}

func (s *Service) WebSocketHandler(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}

func (s *Service) WebSocketUpgrade(c *fiber.Ctx) error {
	return websocket.New(func(conn *websocket.Conn) {
		var boundSession *AgentSession
		defer func() {
			conn.Close()
			if boundSession != nil {
				boundSession.writeMu.Lock()
				if boundSession.Conn == conn {
					boundSession.Conn = nil
					boundSession.Connected = false
				}
				boundSession.writeMu.Unlock()
			}
		}()

		for {
			var msg TunnelMessage
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}

			if msg.Type == "register" {
				var reg struct {
					Subdomain string `json:"subdomain"`
					Token     string `json:"token"`
					Version   string `json:"version"`
					Arch      string `json:"arch"`
				}
				_ = json.Unmarshal(msg.Payload, &reg)

				regSub := strings.TrimSpace(reg.Subdomain)
				if regSub == "" {
					regSub = generateDefaultSubdomain()
				}

				var session *AgentSession
				s.mu.Lock()
				for _, sess := range s.sessions {
					if strings.EqualFold(sess.Subdomain, regSub) {
						session = sess
						break
					}
				}
				s.mu.Unlock()

				if session == nil {
					session = s.RegisterAgent(regSub, reg.Token)
				}

				boundSession = session
				session.writeMu.Lock()
				if session.Conn != nil && session.Conn != conn {
					_ = session.Conn.Close()
				}
				session.Conn = conn
				session.Connected = true
				if reg.Token != "" {
					session.Token = reg.Token
				}
				if reg.Version != "" {
					session.Version = reg.Version
				}
				if reg.Arch != "" {
					session.Arch = reg.Arch
				}
				session.LastSeen = time.Now().UTC()
				session.writeMu.Unlock()

				registeredPayload, _ := json.Marshal(map[string]any{
					"agent_id":    session.ID,
					"subdomain":   session.Subdomain,
					"token":       session.Token,
					"winbox_port": session.WinboxPort,
				})

				resp := TunnelMessage{
					Type:    "registered",
					Payload: registeredPayload,
				}
				_ = session.WriteJSON(resp)

				if s.OnAgentRegistered != nil {
					go s.OnAgentRegistered(session.Subdomain)
				}

			} else if msg.Type == "http_response" {
				var respPayload HttpResponsePayload
				if err := json.Unmarshal(msg.Payload, &respPayload); err == nil {
					s.mu.RLock()
					ch, ok := s.pendingRequests[msg.RequestID]
					s.mu.RUnlock()
					if ok {
						select {
						case ch <- &respPayload:
						default:
						}
					}
				}
			} else if msg.Type == "tcp_data" {
				var dataPayload TcpDataPayload
				if err := json.Unmarshal(msg.Payload, &dataPayload); err == nil && boundSession != nil {
					boundSession.tcpMu.RLock()
					tcpConn, ok := boundSession.tcpConns[dataPayload.ConnID]
					boundSession.tcpMu.RUnlock()

					if ok && tcpConn != nil {
						raw, err := base64.StdEncoding.DecodeString(dataPayload.Data)
						if err == nil {
							_, _ = tcpConn.Write(raw)
						}
					}
				}
			} else if msg.Type == "tcp_close" {
				var closePayload TcpClosePayload
				if err := json.Unmarshal(msg.Payload, &closePayload); err == nil && boundSession != nil {
					boundSession.tcpMu.Lock()
					if tcpConn, ok := boundSession.tcpConns[closePayload.ConnID]; ok {
						_ = tcpConn.Close()
						delete(boundSession.tcpConns, closePayload.ConnID)
					}
					boundSession.tcpMu.Unlock()
				}
		} else if msg.Type == "ping" {
			if boundSession != nil {
				boundSession.writeMu.Lock()
				boundSession.LastSeen = time.Now().UTC()
				_ = conn.WriteJSON(TunnelMessage{Type: "pong"})
				boundSession.writeMu.Unlock()
			}
		} else if msg.Type == "backup_chunk" {
			var chunkPayload BackupChunkPayload
			if err := json.Unmarshal(msg.Payload, &chunkPayload); err == nil {
				s.mu.RLock()
				ch, ok := s.pendingBackups[chunkPayload.RequestID]
				s.mu.RUnlock()
				if ok {
					select {
					case ch <- &BackupChunkMsg{Chunk: &chunkPayload}:
					default:
					}
				}
			}
		} else if msg.Type == "backup_complete" {
			var completePayload BackupCompletePayload
			if err := json.Unmarshal(msg.Payload, &completePayload); err == nil {
				s.mu.RLock()
				ch, ok := s.pendingBackups[completePayload.RequestID]
				s.mu.RUnlock()
				if ok {
					select {
					case ch <- &BackupChunkMsg{Done: true, Filename: completePayload.Filename, Size: completePayload.Size}:
					default:
					}
				}
			}
		} else if msg.Type == "backup_error" {
			var errPayload BackupErrorPayload
			if err := json.Unmarshal(msg.Payload, &errPayload); err == nil {
				s.mu.RLock()
				ch, ok := s.pendingBackups[errPayload.RequestID]
				s.mu.RUnlock()
				if ok {
					select {
					case ch <- &BackupChunkMsg{Error: errPayload.Error}:
					default:
					}
				}
			}
		} else if msg.Type == "sync_config" {
			var syncPayload SyncConfigPayload
			if err := json.Unmarshal(msg.Payload, &syncPayload); err == nil && boundSession != nil {
				boundSession.writeMu.Lock()
				boundSession.SyncData = map[string]interface{}{
					"installation_id": syncPayload.InstallationID,
					"last_event":      syncPayload.LastEvent,
					"updated_at":      syncPayload.UpdatedAt,
					"owner_name":      syncPayload.OwnerName,
					"owner_phone":     syncPayload.OwnerPhone,
					"app":             syncPayload.App,
					"container":       syncPayload.Container,
					"mikrotik":        syncPayload.Mikrotik,
					"license":         syncPayload.License,
					"remote_access":   syncPayload.RemoteAccess,
					"credentials":     syncPayload.Credentials,
				}
				boundSession.writeMu.Unlock()
				if s.OnSyncConfigReceived != nil {
					go s.OnSyncConfigReceived(boundSession.Subdomain, syncPayload)
				}
			}
		} else if msg.Type == "broadcast_log" {
			var bLog broadcast.BroadcastLogPayload
			if err := json.Unmarshal(msg.Payload, &bLog); err == nil && s.OnBroadcastLog != nil {
				if bLog.AgentID == "" && boundSession != nil {
					bLog.AgentID = boundSession.Subdomain
				}
				go s.OnBroadcastLog(bLog)
			}
		} else if msg.Type == "global_auth_request" {
			var authReq GlobalAuthRequestPayload
			if err := json.Unmarshal(msg.Payload, &authReq); err == nil && boundSession != nil {
				go func(sub string, r GlobalAuthRequestPayload, sess *AgentSession) {
					var resp GlobalAuthResponsePayload
					if s.OnGlobalAuthRequest != nil {
						resp = s.OnGlobalAuthRequest(sub, r)
					} else {
						resp = GlobalAuthResponsePayload{
							RequestID:    r.RequestID,
							Allow:        false,
							RejectReason: "Global Auth Broker is not configured on Central Server",
						}
					}
					respBytes, _ := json.Marshal(resp)
					sess.writeMu.Lock()
					_ = sess.WriteJSON(TunnelMessage{
						Type:      "global_auth_response",
						RequestID: r.RequestID,
						Payload:   respBytes,
					})
					sess.writeMu.Unlock()
				}(boundSession.Subdomain, authReq, boundSession)
			}
		} else if msg.Type == "global_acct_update" {
			var acctPayload GlobalAcctPayload
			if err := json.Unmarshal(msg.Payload, &acctPayload); err == nil && boundSession != nil {
				if s.OnGlobalAcctUpdate != nil {
					go s.OnGlobalAcctUpdate(boundSession.Subdomain, acctPayload)
				}
			}
		} else if strings.HasPrefix(msg.Type, "relay_") || msg.Type == "telemetry_push" || msg.Type == "p2p_offer" || msg.Type == "p2p_answer" || msg.Type == "p2p_candidate" {
			if s.OnRelayMessage != nil && boundSession != nil {
				s.OnRelayMessage(boundSession, msg)
			}
		}
		}
	})(c)
}


func ExtractSubdomainForHost(host string, centralDomain string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return ""
	}
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}
	centralDomain = strings.TrimSpace(strings.ToLower(centralDomain))
	if strings.Contains(centralDomain, ":") {
		centralDomain = strings.Split(centralDomain, ":")[0]
	}

	if centralDomain == "" {
		parts := strings.Split(host, ".")
		if len(parts) < 3 {
			return ""
		}
		return parts[0]
	}

	if host == centralDomain {
		return ""
	}

	if strings.HasSuffix(host, "."+centralDomain) {
		return strings.TrimSuffix(host, "."+centralDomain)
	}

	return ""
}

func (s *Service) ProxyHTTP(w http.ResponseWriter, r *http.Request, subdomain string) {
	if s.GetAgentBySubdomain(subdomain) == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("tunnel offline"))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("tunnel proxy placeholder for " + subdomain))
}

// SendTunnelMessage sends an arbitrary JSON message to an active agent session
func (s *Service) SendTunnelMessage(subdomain string, msgType string, payload any) error {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := TunnelMessage{
		Type:    msgType,
		Payload: rawPayload,
	}

	s.mu.RLock()
	var agent *AgentSession
	for _, sess := range s.sessions {
		if sess.Subdomain == subdomain && sess.Connected && sess.Conn != nil {
			agent = sess
			break
		}
	}
	s.mu.RUnlock()

	if agent == nil {
		return fmt.Errorf("agent %s is offline", subdomain)
	}

	return agent.WriteJSON(msg)
}

func handleDirectWinboxProxy(clientConn net.Conn, targetIP string, subdomain string) {
	defer clientConn.Close()

	targetAddr := fmt.Sprintf("%s:8291", targetIP)
	routerConn, err := net.DialTimeout("tcp", targetAddr, 6*time.Second)
	if err != nil {
		log.Printf("[Tunnel-Winbox] ❌ Failed to connect to MikroTik at %s for [%s]: %v", targetAddr, subdomain, err)
		return
	}
	defer routerConn.Close()

	if tcpConn, ok := routerConn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(15 * time.Second)
	}

	log.Printf("[Tunnel-Winbox] 🚀 Active Winbox session opened for [%s] -> %s", subdomain, targetAddr)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(routerConn, clientConn)
		_ = routerConn.Close()
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, routerConn)
		_ = clientConn.Close()
	}()

	wg.Wait()
	log.Printf("[Tunnel-Winbox] 🏁 Winbox session closed for [%s]", subdomain)
}

func resolveTunnelIPForSubdomain(subdomain string) string {
	sub := strings.ToLower(strings.TrimSpace(subdomain))
	cnTarget := fmt.Sprintf("agent-%s-SASMAN", sub)

	statusPaths := []string{
		"/app/data/openvpn-status.log",
		"/root/sasman-central/server/data/openvpn-status.log",
		"data/openvpn-status.log",
		"/etc/openvpn/openvpn-status.log",
		"/run/openvpn/server.status",
		"/tmp/openvpn-status.log",
	}

	for _, path := range statusPaths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		isRoutingTable := false

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "ROUTING TABLE" {
				isRoutingTable = true
				continue
			}
			if line == "GLOBAL STATS" || line == "END" {
				isRoutingTable = false
			}

			if isRoutingTable {
				parts := strings.Split(line, ",")
				if len(parts) >= 2 {
					ip := strings.TrimSpace(parts[0])
					cn := strings.TrimSpace(parts[1])
					if strings.EqualFold(cn, cnTarget) {
						return ip
					}
				}
			}
		}
	}

	// Dynamic probe active Winbox listeners on the /30 net30 range
	candidates := []string{"10.250.0.6", "10.250.0.10", "10.250.0.14", "10.250.0.18", "10.250.0.2"}
	for _, ip := range candidates {
		conn, err := net.DialTimeout("tcp", ip+":8291", 400*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return ip
		}
	}

	return "10.250.0.6"
}
