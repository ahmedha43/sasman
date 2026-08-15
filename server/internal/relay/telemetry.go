package relay

import (
	"sync"
	"time"

	"mikrotik-manager/pkg/relay"
)

// TelemetryHub aggregates and stores real-time telemetry from all connected SASMAN Agents
type TelemetryHub struct {
	mu           sync.RWMutex
	reports      map[string]relay.ServiceTelemetry // Key: Subdomain or AgentID
	lastReportAt map[string]time.Time
}

func NewTelemetryHub() *TelemetryHub {
	hub := &TelemetryHub{
		reports:      make(map[string]relay.ServiceTelemetry),
		lastReportAt: make(map[string]time.Time),
	}
	go hub.staleCleanerLoop()
	return hub
}

// IngestTelemetry saves a new telemetry report from an agent
func (th *TelemetryHub) IngestTelemetry(t relay.ServiceTelemetry) {
	th.mu.Lock()
	defer th.mu.Unlock()

	key := t.Subdomain
	if key == "" {
		key = t.AgentID
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

// GetServiceProviders returns all agents that currently report a service as available
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

		if probe, ok := tel.Services[serviceID]; ok && probe.Available {
			providers = append(providers, AgentServiceStatus{
				AgentID:     tel.AgentID,
				Subdomain:   agentKey,
				Probe:       probe,
				NodeMetrics: tel.NodeMetrics,
				LastSeen:    lastSeen,
			})
		}
	}

	return providers
}

type AgentServiceStatus struct {
	AgentID     string            `json:"agent_id"`
	Subdomain   string            `json:"subdomain"`
	Probe       relay.HealthProbe `json:"probe"`
	NodeMetrics relay.NodeMetrics `json:"node_metrics"`
	LastSeen    time.Time         `json:"last_seen"`
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
