package relay

import (
	"encoding/json"
	"time"
)

// ServiceDefinition defines a managed service (e.g. Cinemana, Shabakaty, etc.)
type ServiceDefinition struct {
	ID               string      `json:"id"`                // Unique service ID (e.g. "cinemana")
	Name             string      `json:"name"`              // Display name (e.g. "Shabakaty Cinemana")
	Category         string      `json:"category"`          // Category (e.g. "Streaming", "CDN", "Gaming")
	Domains          []string    `json:"domains"`           // Target domains (e.g. ["cinemana.shabakaty.cc", "*.shabakaty.cc"])
	IPRanges         []string    `json:"ip_ranges"`         // Target IP CIDRs (e.g. ["10.0.0.0/8"])
	Ports            []int       `json:"ports"`             // Ports (e.g. [80, 443])
	Protocols        []string    `json:"protocols"`         // Protocols (e.g. ["tcp", "tls", "quic"])
	ProbeConfig      ProbeConfig `json:"probe_config"`      // Health check configuration
	TargetScope      string      `json:"target_scope"`      // "all" (all agents) or "selected" (only allowed consumers/groups)
	TargetGroups     []string    `json:"target_groups"`     // Groups allowed to route through relay (e.g. ["VIP", "Baghdad"])
	AllowedConsumers []string    `json:"allowed_consumers"` // Subdomains/AgentIDs allowed to route through relay
	AllowedProviders []string    `json:"allowed_providers"` // Subdomains/AgentIDs allowed to act as Egress nodes (optional)
	MaxBandwidthMbps int         `json:"max_bandwidth_mbps"`// Max bandwidth per stream in Mbps (0 for unlimited)
	Enabled          bool        `json:"enabled"`           // Whether service relay is enabled
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

// ProbeConfig contains health check probing rules for a service
type ProbeConfig struct {
	Type         string        `json:"type"`          // "http", "https", "tcp_ping", "dns"
	TargetURL    string        `json:"target_url"`    // e.g. "https://cinemana.shabakaty.cc"
	ExpectedCode int           `json:"expected_code"` // Expected HTTP status (default 200)
	HostHeader   string        `json:"host_header"`   // Optional Host header override
	IntervalSec  int           `json:"interval_sec"`  // Probe interval in seconds (default 15s)
	TimeoutSec   int           `json:"timeout_sec"`   // Probe timeout in seconds (default 3s)
}

// HealthProbe represents the result of a single service probe on an agent
type HealthProbe struct {
	Available    bool      `json:"available"`
	LatencyMs    float64   `json:"latency_ms"`
	PacketLoss   float64   `json:"packet_loss"`
	LastChecked  time.Time `json:"last_checked"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// NodeMetrics represents the resource utilization of an Agent
type NodeMetrics struct {
	CPUPercent   float64 `json:"cpu_percent"`
	MemoryMB     float64 `json:"memory_mb"`
	ActiveRelays int     `json:"active_relays"`
	TxBytesRate  int64   `json:"tx_bytes_rate"` // Bytes/sec
	RxBytesRate  int64   `json:"rx_bytes_rate"` // Bytes/sec
}

// ServiceTelemetry is sent periodically from Agent to Cloud
type ServiceTelemetry struct {
	AgentID     string                 `json:"agent_id"`
	Subdomain   string                 `json:"subdomain"`
	Timestamp   time.Time              `json:"timestamp"`
	Services    map[string]HealthProbe `json:"services"` // ServiceID -> HealthProbe
	NodeMetrics NodeMetrics            `json:"node_metrics"`
}

// EgressRoute contains the assigned egress path for a service
type EgressRoute struct {
	ServiceID    string    `json:"service_id"`
	PrimaryAgent string    `json:"primary_agent"` // Subdomain/ID of primary egress
	BackupAgent  string    `json:"backup_agent"`  // Subdomain/ID of backup egress
	DirectAddr   string    `json:"direct_addr"`   // Public/P2P IP:Port of primary if known
	UseCloudFall bool      `json:"use_cloud_fall"`// Fallback to Cloud Relay if direct fails
	Score        float64   `json:"score"`
	LatencyMs    float64   `json:"latency_ms"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AgentRoutingTable is distributed by Cloud to Agents
type AgentRoutingTable struct {
	Version   int64                  `json:"version"`
	Timestamp time.Time              `json:"timestamp"`
	Routes    map[string]EgressRoute `json:"routes"` // ServiceID -> EgressRoute
}

// SignalingMsgType defines types of P2P / Relay control messages
type SignalingMsgType string

const (
	MsgCatalogSync    SignalingMsgType = "catalog_sync"
	MsgTelemetryPush  SignalingMsgType = "telemetry_push"
	MsgRouteTablePush SignalingMsgType = "route_table_push"
	MsgP2POffer       SignalingMsgType = "p2p_offer"
	MsgP2PAnswer      SignalingMsgType = "p2p_answer"
	MsgP2PCandidate   SignalingMsgType = "p2p_candidate"
	MsgRelayOpen      SignalingMsgType = "relay_open"
	MsgRelayData      SignalingMsgType = "relay_data"
	MsgRelayClose     SignalingMsgType = "relay_close"
)

// RelayControlMessage is the envelope for Control Plane messages over WebSocket
type RelayControlMessage struct {
	Type      SignalingMsgType `json:"type"`
	AgentID   string           `json:"agent_id,omitempty"`
	TargetID  string           `json:"target_id,omitempty"`
	SessionID string           `json:"session_id,omitempty"`
	Payload   json.RawMessage  `json:"payload,omitempty"`
}

// StreamOpenPayload is the header sent when initiating a relay stream to an egress agent
type StreamOpenPayload struct {
	StreamID   string `json:"stream_id"`
	ServiceID  string `json:"service_id"`
	TargetHost string `json:"target_host"` // e.g. "cinemana.shabakaty.cc:443"
	SNI        string `json:"sni"`
	ClientIP   string `json:"client_ip,omitempty"`
}
