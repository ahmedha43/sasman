package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mikrotik-manager/pkg/relay"
	relayagent "mikrotik-manager/pkg/relay/agent"

	"github.com/gorilla/websocket"
)

var (
	publicDNSList = []string{
		"1.1.1.1:53",
		"8.8.8.8:53",
		"1.0.0.1:53",
		"8.8.4.4:53",
		"9.9.9.9:53",
	}
	knownHosts = map[string]string{
		"sas-man.net": "167.86.73.203",
	}
)

// AgentClientConfig configures the resilient tunnel agent
type AgentClientConfig struct {
	Subdomain        string
	Token            string
	Version          string
	Arch             string
	DataDir          string
	GatewayURL       string
	LocalPort        string
	EnableRelay      bool
	RelayListenAddr  string
	OnSyncConfig     func(conn *websocket.Conn, writeMu *sync.Mutex)
	OnBackupRequest  func(conn *websocket.Conn, msg TunnelMessage, writeMu *sync.Mutex)
	OnLocalHTTP      func(req HttpRequestPayload, localPort string) HttpResponsePayload
	OnMikroTikSync   func(services []relay.ServiceDefinition)
	RouterAddress    string
}

// ResilientAgentClient manages persistent connection to SASMAN Central Gateway with exponential backoff
type ResilientAgentClient struct {
	lastPongAt atomic.Int64
	running    atomic.Int32
	cfg        AgentClientConfig
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewResilientAgentClient(cfg AgentClientConfig) *ResilientAgentClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &ResilientAgentClient{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *ResilientAgentClient) Start() {
	if !c.running.CompareAndSwap(0, 1) {
		return
	}

	go c.lifecycleLoop()
}

func (c *ResilientAgentClient) Stop() {
	if c.running.CompareAndSwap(1, 0) {
		c.cancel()
	}
}

func (c *ResilientAgentClient) lifecycleLoop() {
	backoff := 2 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("[Tunnel Client] Stopped.")
			return
		default:
		}

		err := c.connectAndServe()
		if err != nil {
			log.Printf("[Tunnel Client] Connection ended: %v. Reconnecting in %v...", err, backoff)
		}

		// Calculate jitter: ±20%
		jitter := time.Duration(float64(backoff) * (0.8 + 0.4*rand.Float64()))
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(jitter):
		}

		backoff = time.Duration(float64(backoff) * 1.5)
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func createResilientDialer(targetURL string, timeout time.Duration) *websocket.Dialer {
	u, err := url.Parse(targetURL)
	var hostname string
	if err == nil {
		hostname = u.Hostname()
	}

	netDialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 15 * time.Second,
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 2 * time.Second}
				// 1. Try system resolver first
				conn, err := d.DialContext(ctx, network, address)
				if err == nil {
					return conn, nil
				}
				// 2. Fallback to public DNS servers
				for _, dns := range publicDNSList {
					conn, err := d.DialContext(ctx, "udp", dns)
					if err == nil {
						return conn, nil
					}
				}
				return nil, err
			},
		},
	}

	var tlsConfig *tls.Config
	if hostname != "" {
		tlsConfig = &tls.Config{
			ServerName: hostname,
		}
	}

	return &websocket.Dialer{
		HandshakeTimeout: timeout,
		TLSClientConfig:  tlsConfig,
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
				if u != nil && (u.Scheme == "wss" || u.Scheme == "https") {
					port = "443"
				} else {
					port = "80"
				}
				addr = net.JoinHostPort(host, port)
			}

			// 1. Dial using resilient resolver
			conn, err := netDialer.DialContext(ctx, network, addr)
			if err == nil {
				return conn, nil
			}

			// 2. Direct IP fallback if DNS lookup timed out (e.g. sas-man.net -> 167.86.73.203)
			if fallbackIP, exists := knownHosts[strings.ToLower(host)]; exists {
				directAddr := net.JoinHostPort(fallbackIP, port)
				directConn, dErr := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, directAddr)
				if dErr == nil {
					return directConn, nil
				}
			}

			return nil, err
		},
	}
}

func (c *ResilientAgentClient) connectAndServe() error {
	gatewayURL := strings.TrimSpace(c.cfg.GatewayURL)
	if gatewayURL == "" {
		gatewayURL = "ws://127.0.0.1:8080/ws"
	}
	if !strings.HasPrefix(gatewayURL, "ws://") && !strings.HasPrefix(gatewayURL, "wss://") {
		gatewayURL = "ws://" + gatewayURL
	}

	dialer := createResilientDialer(gatewayURL, 10*time.Second)

	conn, _, err := dialer.Dial(gatewayURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	var writeMu sync.Mutex
	ver := c.cfg.Version
	if ver == "" {
		ver = "5.1.0"
	}
	arch := c.cfg.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}

	// Send registration message with version & arch
	regPayload, _ := json.Marshal(map[string]string{
		"subdomain": c.cfg.Subdomain,
		"token":     c.cfg.Token,
		"version":   ver,
		"arch":      arch,
	})
	if err := conn.WriteJSON(TunnelMessage{
		Type:    "register",
		Payload: regPayload,
	}); err != nil {
		return err
	}

	log.Printf("[Tunnel Client] Successfully connected and registered: %s (Subdomain: %s, Ver: %s, Arch: %s)", gatewayURL, c.cfg.Subdomain, ver, arch)
	c.lastPongAt.Store(time.Now().Unix())

	dataDir := c.cfg.DataDir
	if dataDir == "" {
		dataDir = os.Getenv("SASMAN_DATA_DIR")
		if dataDir == "" {
			dataDir = "data"
		}
	}
	_ = os.MkdirAll(dataDir, 0755)
	healthPath := filepath.Join(dataDir, "health_ok.json")
	_ = os.WriteFile(healthPath, []byte(fmt.Sprintf(`{"status":"ok","subdomain":"%s","version":"%s","connected_at":"%s"}`, c.cfg.Subdomain, ver, time.Now().Format(time.RFC3339))), 0644)

	var tcpConnsMap sync.Map
	defer func() {
		tcpConnsMap.Range(func(key, value any) bool {
			if conn, ok := value.(net.Conn); ok && conn != nil {
				conn.Close()
			}
			return true
		})
	}()

	// Initialize SASMAN Relay Engine if enabled
	signaler := &wsRelaySignaler{conn: conn, writeMu: &writeMu, gatewayURL: gatewayURL}
	relayListen := c.cfg.RelayListenAddr
	if relayListen == "" {
		relayListen = "0.0.0.0:18443"
	}

	relayEng := relayagent.NewEngine(relayagent.EngineConfig{
		AgentID:            c.cfg.Subdomain,
		Subdomain:          c.cfg.Subdomain,
		ListenAddr:         relayListen,
		CloudGatewayURL:    gatewayURL,
		EnableEgress:       true,
		EnableInterception: true,
	}, signaler)
	_ = relayEng.Start()
	defer relayEng.Stop()

	// Heartbeat watcher with dead connection detection
	stopHeartbeat := make(chan struct{})
	defer close(stopHeartbeat)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopHeartbeat:
				return
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				writeMu.Lock()
				err := conn.WriteJSON(TunnelMessage{Type: "ping"})
				writeMu.Unlock()
				if err != nil {
					log.Printf("[Tunnel Client] Ping write failed: %v", err)
					_ = conn.Close()
					return
				}

				lastPong := c.lastPongAt.Load()
				if time.Now().Unix()-lastPong > 25 {
					log.Printf("[Tunnel Client] ⚠️ Heartbeat timeout: No pong/message received for 25s! Reconnecting...")
					_ = conn.Close()
					return
				}
			}
		}
	}()

	for {
		var msg TunnelMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}

		c.lastPongAt.Store(time.Now().Unix())

		switch msg.Type {
		case "registered":
			if c.cfg.OnSyncConfig != nil {
				go c.cfg.OnSyncConfig(conn, &writeMu)
			}

		case "cmd_ota_upgrade":
			log.Printf("[Tunnel Client] 🚀 Received OTA upgrade command from server!")
			triggerPath := filepath.Join(dataDir, "ota_trigger.json")
			_ = os.WriteFile(triggerPath, msg.Payload, 0644)

		case "http_request":
			var reqPayload HttpRequestPayload
			if err := json.Unmarshal(msg.Payload, &reqPayload); err == nil && c.cfg.OnLocalHTTP != nil {
				go func(reqID string, rp HttpRequestPayload) {
					respPayload := c.cfg.OnLocalHTTP(rp, c.cfg.LocalPort)
					respPayloadBytes, _ := json.Marshal(respPayload)
					writeMu.Lock()
					_ = conn.WriteJSON(TunnelMessage{
						Type:      "http_response",
						RequestID: reqID,
						Payload:   respPayloadBytes,
					})
					writeMu.Unlock()
				}(msg.RequestID, reqPayload)
			}

		case "catalog_sync":
			var catalog []relay.ServiceDefinition
			if err := json.Unmarshal(msg.Payload, &catalog); err == nil {
				relayEng.OnCatalogSync(catalog)
				if c.cfg.OnMikroTikSync != nil {
					go c.cfg.OnMikroTikSync(catalog)
				}
			}

		case "route_table_push":
			var table relay.AgentRoutingTable
			if err := json.Unmarshal(msg.Payload, &table); err == nil {
				relayEng.OnRouteTableSync(table)
			}

		case "backup_request":
			if c.cfg.OnBackupRequest != nil {
				go c.cfg.OnBackupRequest(conn, msg, &writeMu)
			}
		}
	}
}

type wsRelaySignaler struct {
	conn       *websocket.Conn
	writeMu    *sync.Mutex
	gatewayURL string
}

func (s *wsRelaySignaler) SendRelayControl(msg relay.RelayControlMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteJSON(TunnelMessage{
		Type:    string(msg.Type),
		Payload: msg.Payload,
	})
}

func (s *wsRelaySignaler) GetCloudGatewayURL() string {
	return s.gatewayURL
}
