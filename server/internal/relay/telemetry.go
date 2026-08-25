package relay

import (
	"strings"
	"sync"
	"time"

	"mikrotik-manager/pkg/relay"
)

// TelemetryHub aggregates and stores real-time telemetry from all connected SASMAN Agents
type TelemetryHub struct {
	mu           sync.RWMutex
	reports      map[string]relay.ServiceTelemetry // Key: Subdomain or AgentID
	lastReportAt map[string]time.Time
	agentRoles   map[string]relay.NodeRole // Key: Subdomain or AgentID
}

func NewTelemetryHub() *TelemetryHub {
	hub := &TelemetryHub{
		reports:      make(map[string]relay.ServiceTelemetry),
		lastReportAt: make(map[string]time.Time),
		agentRoles:   make(map[string]relay.NodeRole),
	}
	go hub.staleCleanerLoop()
	return hub
}

// SetAgentRole assigns an operational role to an agent
func (th *TelemetryHub) SetAgentRole(agentKey string, role relay.NodeRole) {
	th.mu.Lock()
	defer th.mu.Unlock()
	th.agentRoles[agentKey] = role
	if tel, exists := th.reports[agentKey]; exists {
		tel.AssignedRole = role
		th.reports[agentKey] = tel
	}
}

func isStarlinkOrSatellite(asn, isp string) bool {
	asnUpper := strings.ToUpper(strings.TrimSpace(asn))
	ispLower := strings.ToLower(strings.TrimSpace(isp))
	return asnUpper == "AS14593" || strings.Contains(ispLower, "space") || strings.Contains(ispLower, "starlink")
}

// GetAgentRole returns the assigned or auto-detected role of an agent
func (th *TelemetryHub) GetAgentRole(agentKey string) relay.NodeRole {
	th.mu.RLock()
	defer th.mu.RUnlock()
	if role, exists := th.agentRoles[agentKey]; exists && role != "" {
		return role
	}

	// Auto-detect role from telemetry
	if tel, exists := th.reports[agentKey]; exists {
		// 1. Starlink / SpaceX is ALWAYS a Consumer Node (even if GeoIP is Iraq)
		if isStarlinkOrSatellite(tel.ASN, tel.ISPName) {
			return relay.NodeRoleConsumer
		}

		// 2. Domestic Iraqi Line (Cinemana reachable or Domestic Iraqi ISP)
		if tel.Services["cinemana"].Available || (tel.CountryCode == "IQ" && tel.PublicIP != "") {
			return relay.NodeRoleExit
		}
		if tel.CountryCode != "IQ" && tel.CountryCode != "" {
			return relay.NodeRoleConsumer
		}
	}
	return relay.NodeRoleHybrid
}

// IngestTelemetry saves a new telemetry report from an agent
func (th *TelemetryHub) IngestTelemetry(t relay.ServiceTelemetry) {
	th.mu.Lock()
	defer th.mu.Unlock()

	key := t.Subdomain
	if key == "" {
		key = t.AgentID
	}

	if role, exists := th.agentRoles[key]; exists && role != "" {
		t.AssignedRole = role
	} else if isStarlinkOrSatellite(t.ASN, t.ISPName) {
		t.AssignedRole = relay.NodeRoleConsumer
	} else if t.Services["cinemana"].Available || (t.CountryCode == "IQ" && t.PublicIP != "") {
		t.AssignedRole = relay.NodeRoleExit
	} else if t.CountryCode != "IQ" && t.CountryCode != "" {
		t.AssignedRole = relay.NodeRoleConsumer
	} else {
		t.AssignedRole = relay.NodeRoleHybrid
	}

	th.reports[key] = t
	th.lastReportAt[key] = time.Now().UTC()
}

// GetAgentTelemetry returns the latest telemetry for a specific agent
func (th *TelemetryHub) GetAgentTelemetry(agentKey string) (relay.ServiceTelemetry, bool) {
	th.mu.RLock()
	defer th.mu.RUnlock()

	t, ok := th.reports[agentKey]
	return t, ok
}

// GetAllTelemetry returns the map of all active agent telemetry reports
func (th *TelemetryHub) GetAllTelemetry() map[string]relay.ServiceTelemetry {
	th.mu.RLock()
	defer th.mu.RUnlock()

	out := make(map[string]relay.ServiceTelemetry, len(th.reports))
	for k, v := range th.reports {
		out[k] = v
	}
	return out
}

// GetServiceProviders returns all agents eligible as exit nodes that currently report a service as available
func (th *TelemetryHub) GetServiceProviders(serviceID string) []AgentServiceStatus {
	th.mu.RLock()
	defer th.mu.RUnlock()

	var providers []AgentServiceStatus
	now := time.Now().UTC()

	for agentKey, tel := range th.reports {
		lastSeen := th.lastReportAt[agentKey]
		if now.Sub(lastSeen) > 45*time.Second {
			continue // Skip stale/offline agents
		}

		// Only EXIT_NODE and HYBRID agents can serve as Egress Exit Providers
		role := tel.AssignedRole
		if r, ok := th.agentRoles[agentKey]; ok && r != "" {
			role = r
		}
		if role == relay.NodeRoleConsumer {
			continue // Consumer nodes (Starlink) cannot act as egress exit nodes
		}

		if probe, ok := tel.Services[serviceID]; ok && probe.Available {
			providers = append(providers, AgentServiceStatus{
				AgentID:      tel.AgentID,
				Subdomain:    agentKey,
				AssignedRole: role,
				PublicIP:     tel.PublicIP,
				ISPName:      tel.ISPName,
				ASN:          tel.ASN,
				CountryCode:  tel.CountryCode,
				Probe:        probe,
				NodeMetrics:  tel.NodeMetrics,
				LastSeen:     lastSeen,
			})
		}
	}

	return providers
}

type AgentServiceStatus struct {
	AgentID      string            `json:"agent_id"`
	Subdomain    string            `json:"subdomain"`
	AssignedRole relay.NodeRole    `json:"assigned_role"`
	PublicIP     string            `json:"public_ip,omitempty"`
	ISPName      string            `json:"isp_name,omitempty"`
	ASN          string            `json:"asn,omitempty"`
	CountryCode  string            `json:"country_code,omitempty"`
	Probe        relay.HealthProbe `json:"probe"`
	NodeMetrics  relay.NodeMetrics `json:"node_metrics"`
	LastSeen     time.Time         `json:"last_seen"`
}

func (th *TelemetryHub) staleCleanerLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		th.mu.Lock()
		now := time.Now().UTC()
		for k, t := range th.lastReportAt {
			if now.Sub(t) > 2*time.Minute {
				delete(th.reports, k)
				delete(th.lastReportAt, k)
			}
		}
		th.mu.Unlock()
	}
}
