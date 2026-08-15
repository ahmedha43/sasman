package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"mikrotik-manager/pkg/relay"
	"mikrotik-manager/pkg/relay/multiplexer"
	"mikrotik-manager/pkg/relay/prober"
	"mikrotik-manager/pkg/relay/sniproxy"
)

// CloudSignalingClient interface for sending control/signaling messages to Cloud
type CloudSignalingClient interface {
	SendRelayControl(msg relay.RelayControlMessage) error
	GetCloudGatewayURL() string
}

// EngineConfig holds configuration parameters for the Relay Agent Engine
type EngineConfig struct {
	AgentID            string
	Subdomain          string
	ListenAddr         string // e.g. "0.0.0.0:18443"
	CloudGatewayURL    string
	EnableEgress       bool // Whether this agent allows other agents to route through it
	EnableInterception bool // Whether this agent intercepts local MikroTik traffic
}

// Engine is the unified SASMAN Service Relay agent coordinator
type Engine struct {
	cfg          EngineConfig
	mu           sync.RWMutex
	services     map[string]relay.ServiceDefinition
	routingTable relay.AgentRoutingTable
	peerSessions map[string]*multiplexer.Session // TargetAgentSubdomain -> Multiplex Session
	listener     *sniproxy.InterceptorListener
	prober       *prober.Prober
	signaler     CloudSignalingClient
	running      int32
	stopCh       chan struct{}
}

func NewEngine(cfg EngineConfig, signaler CloudSignalingClient) *Engine {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "0.0.0.0:18443"
	}

	eng := &Engine{
		cfg:          cfg,
		services:     make(map[string]relay.ServiceDefinition),
		peerSessions: make(map[string]*multiplexer.Session),
		signaler:     signaler,
		stopCh:       make(chan struct{}),
	}

	eng.prober = prober.NewProber(cfg.AgentID, cfg.Subdomain, eng.onTelemetry)
	eng.listener = sniproxy.NewInterceptorListener(cfg.ListenAddr, eng, eng)

	return eng
}

// Start boots the Relay Engine, prober, and local SNI interceptor
func (e *Engine) Start() error {
	if !atomic.CompareAndSwapInt32(&e.running, 0, 1) {
		return nil
	}

	log.Printf("[Relay Engine] Starting SASMAN Service Relay (Agent: %s, Subdomain: %s)", e.cfg.AgentID, e.cfg.Subdomain)

	if e.cfg.EnableInterception {
		if err := e.listener.Start(); err != nil {
			log.Printf("[Relay Engine] Warning: Failed to start interceptor listener: %v", err)
		}
	}

	e.prober.Start()
	return nil
}

// Stop cleanly terminates all active relay streams and listeners
func (e *Engine) Stop() {
	if atomic.CompareAndSwapInt32(&e.running, 1, 0) {
		close(e.stopCh)
		e.prober.Stop()
		e.listener.Stop()

		e.mu.Lock()
		for _, sess := range e.peerSessions {
			sess.Close()
		}
		e.peerSessions = make(map[string]*multiplexer.Session)
		e.mu.Unlock()
	}
}

// OnCatalogSync is invoked when the Cloud pushes a new Service Catalog
func (e *Engine) OnCatalogSync(catalog []relay.ServiceDefinition) {
	e.mu.Lock()
	e.services = make(map[string]relay.ServiceDefinition)
	for _, s := range catalog {
		if s.Enabled {
			e.services[s.ID] = s
		}
	}
	e.mu.Unlock()

	e.prober.UpdateServices(catalog)
	log.Printf("[Relay Engine] Updated catalog with %d services", len(catalog))
}

// OnRouteTableSync is invoked when Cloud computes optimal egress paths
func (e *Engine) OnRouteTableSync(table relay.AgentRoutingTable) {
	e.mu.Lock()
	e.routingTable = table
	e.mu.Unlock()
	log.Printf("[Relay Engine] Received new routing table version %d with %d routes", table.Version, len(table.Routes))
}

func (e *Engine) onTelemetry(telemetry relay.ServiceTelemetry) {
	if e.signaler == nil {
		return
	}

	payload, _ := json.Marshal(telemetry)
	_ = e.signaler.SendRelayControl(relay.RelayControlMessage{
		Type:    relay.MsgTelemetryPush,
		AgentID: e.cfg.AgentID,
		Payload: payload,
	})
}

// ResolveRoute matches an incoming connection to a configured service and egress agent
func (e *Engine) ResolveRoute(sni string, dstAddr string) (*relay.EgressRoute, *relay.ServiceDefinition, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, svc := range e.services {
		if !svc.Enabled {
			continue
		}

		matched := false
		for _, domain := range svc.Domains {
			if sniproxy.MatchDomain(sni, domain) {
				matched = true
				break
			}
		}

		if matched {
			if route, ok := e.routingTable.Routes[svc.ID]; ok {
				// Don't relay if we ourselves are the primary egress
				if route.PrimaryAgent == e.cfg.Subdomain || route.PrimaryAgent == e.cfg.AgentID {
					return nil, nil, fmt.Errorf("local agent is primary egress, routing directly")
				}
				return &route, &svc, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("no matching service relay route for %s", sni)
}

// DialEgressStream acquires or opens a multiplexed stream to the target egress agent
func (e *Engine) DialEgressStream(ctx context.Context, route *relay.EgressRoute, svc *relay.ServiceDefinition, targetHost string, rawPrefix []byte) (io.ReadWriteCloser, error) {
	targetAgent := route.PrimaryAgent
	if targetAgent == "" {
		return nil, fmt.Errorf("no primary egress agent assigned for service %s", svc.ID)
	}

	sess, err := e.getOrCreatePeerSession(ctx, targetAgent, route)
	if err != nil && route.BackupAgent != "" {
		log.Printf("[Relay Engine] Primary agent %s failed (%v), attempting failover to backup agent %s", targetAgent, err, route.BackupAgent)
		targetAgent = route.BackupAgent
		sess, err = e.getOrCreatePeerSession(ctx, targetAgent, route)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to egress agent %s: %w", targetAgent, err)
	}

	stream, err := sess.OpenStream(multiplexer.StreamMeta{
		ServiceID:  svc.ID,
		TargetHost: targetHost,
		SNI:        targetHost,
	})
	if err != nil {
		return nil, fmt.Errorf("open multiplex sub-stream: %w", err)
	}

	// Write the peeked ClientHello prefix
	if len(rawPrefix) > 0 {
		if _, err := stream.Write(rawPrefix); err != nil {
			stream.Close()
			return nil, fmt.Errorf("write prefix to stream: %w", err)
		}
	}

	return stream, nil
}

func (e *Engine) getOrCreatePeerSession(ctx context.Context, targetAgent string, route *relay.EgressRoute) (*multiplexer.Session, error) {
	e.mu.Lock()
	sess, ok := e.peerSessions[targetAgent]
	e.mu.Unlock()

	if ok && sess != nil {
		return sess, nil
	}

	// Connect to peer (Direct P2P or Cloud Relay Fallback)
	conn, err := e.dialPeer(ctx, targetAgent, route)
	if err != nil {
		return nil, err
	}

	sess = multiplexer.NewSession(conn, false)

	e.mu.Lock()
	e.peerSessions[targetAgent] = sess
	e.mu.Unlock()

	return sess, nil
}

func (e *Engine) dialPeer(ctx context.Context, targetAgent string, route *relay.EgressRoute) (net.Conn, error) {
	// If direct address is provided and reachable, try direct connection
	if route.DirectAddr != "" {
		dialer := net.Dialer{Timeout: 3 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", route.DirectAddr)
		if err == nil {
			log.Printf("[Relay Engine] Established direct P2P connection to %s (%s)", targetAgent, route.DirectAddr)
			return conn, nil
		}
		log.Printf("[Relay Engine] Direct P2P to %s failed (%v), falling back to signaling tunnel", route.DirectAddr, err)
	}

	// Fallback to Cloud Gateway Relay
	if e.cfg.CloudGatewayURL != "" {
		log.Printf("[Relay Engine] Connecting to egress %s via Cloud Relay Gateway", targetAgent)
		dialer := net.Dialer{Timeout: 5 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", e.cfg.CloudGatewayURL)
		if err != nil {
			return nil, fmt.Errorf("cloud relay connect: %w", err)
		}
		return conn, nil
	}

	return nil, fmt.Errorf("unable to reach egress agent %s", targetAgent)
}

// RegisterIncomingPeerSession is called when another agent connects to us as an Egress Node
func (e *Engine) RegisterIncomingPeerSession(peerID string, rawConn io.ReadWriteCloser) {
	sess := multiplexer.NewSession(rawConn, true)

	e.mu.Lock()
	e.peerSessions[peerID] = sess
	e.mu.Unlock()

	log.Printf("[Relay Engine] Egress: Registered incoming peer session from %s", peerID)

	go e.handleEgressSession(sess)
}

func (e *Engine) handleEgressSession(sess *multiplexer.Session) {
	defer sess.Close()

	for {
		stream, err := sess.AcceptStream()
		if err != nil {
			return
		}

		go e.handleEgressStream(stream)
	}
}

func (e *Engine) handleEgressStream(stream *multiplexer.Stream) {
	defer stream.Close()

	meta := stream.Meta()
	targetHost := meta.TargetHost
	if targetHost == "" {
		return
	}

	// Outbound dial from this agent to the actual service on the local ISP network
	dialer := net.Dialer{Timeout: 5 * time.Second}
	upstreamConn, err := dialer.Dial("tcp", targetHost)
	if err != nil {
		log.Printf("[Relay Engine] Egress dial to %s failed: %v", targetHost, err)
		return
	}
	defer upstreamConn.Close()

	// Bidirectional pipe
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(upstreamConn, stream)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(stream, upstreamConn)
	}()

	wg.Wait()
}
