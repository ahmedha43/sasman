package relay

import (
	"testing"
	"time"

	"mikrotik-manager/pkg/relay"
)

func TestRouterEngineScoringAndFailover(t *testing.T) {
	catalog, err := NewCatalogManager(nil)
	if err != nil {
		t.Fatalf("Failed to init catalog: %v", err)
	}

	telemetry := NewTelemetryHub()
	router := NewRouterEngine(catalog, telemetry, nil)

	// Simulate 3 Agents reporting Cinemana health:
	// Agent 001: 50ms, 0% loss
	// Agent 002: 18ms, 0% loss (Best)
	// Agent 004: 35ms, 0% loss (Second Best)
	telemetry.IngestTelemetry(relay.ServiceTelemetry{
		AgentID:   "agent-001",
		Subdomain: "agent-001",
		Services: map[string]relay.HealthProbe{
			"cinemana": {Available: true, LatencyMs: 50, PacketLoss: 0, LastChecked: time.Now()},
		},
		NodeMetrics: relay.NodeMetrics{CPUPercent: 10, ActiveRelays: 2},
	})

	telemetry.IngestTelemetry(relay.ServiceTelemetry{
		AgentID:   "agent-002",
		Subdomain: "agent-002",
		Services: map[string]relay.HealthProbe{
			"cinemana": {Available: true, LatencyMs: 18, PacketLoss: 0, LastChecked: time.Now()},
		},
		NodeMetrics: relay.NodeMetrics{CPUPercent: 15, ActiveRelays: 1},
	})

	telemetry.IngestTelemetry(relay.ServiceTelemetry{
		AgentID:   "agent-004",
		Subdomain: "agent-004",
		Services: map[string]relay.HealthProbe{
			"cinemana": {Available: true, LatencyMs: 35, PacketLoss: 0, LastChecked: time.Now()},
		},
		NodeMetrics: relay.NodeMetrics{CPUPercent: 8, ActiveRelays: 0},
	})

	table := router.ComputeGlobalRoutingTable()
	route, ok := table.Routes["cinemana"]
	if !ok {
		t.Fatalf("Expected route for cinemana")
	}

	if route.PrimaryAgent != "agent-002" {
		t.Errorf("Expected primary agent 'agent-002' (18ms), got %q", route.PrimaryAgent)
	}

	if route.BackupAgent != "agent-004" {
		t.Errorf("Expected backup agent 'agent-004' (35ms), got %q", route.BackupAgent)
	}

	// Test Group-based Routing Table
	_ = catalog.SaveService(relay.ServiceDefinition{
		ID:           "vip_stream",
		Name:         "VIP Stream",
		Domains:      []string{"vip.stream.local"},
		TargetScope:  "selected",
		TargetGroups: []string{"VIP"},
		Enabled:      true,
	})
	telemetry.IngestTelemetry(relay.ServiceTelemetry{
		AgentID:   "agent-002",
		Subdomain: "agent-002",
		Services: map[string]relay.HealthProbe{
			"vip_stream": {Available: true, LatencyMs: 20, PacketLoss: 0, LastChecked: time.Now()},
		},
	})

	// Non-VIP agent: should NOT get vip_stream
	normalTable := router.ComputeRoutingTableForAgent("agent-001", "normal")
	if _, hasVIP := normalTable.Routes["vip_stream"]; hasVIP {
		t.Errorf("Expected normal agent to NOT receive vip_stream route")
	}

	// VIP agent: SHOULD get vip_stream
	vipTable := router.ComputeRoutingTableForAgent("agent-003", "VIP")
	if _, hasVIP := vipTable.Routes["vip_stream"]; !hasVIP {
		t.Errorf("Expected VIP agent to receive vip_stream route")
	}

	// Now simulate failover: Agent 002 degrades (high packet loss / unavailable)
	telemetry.IngestTelemetry(relay.ServiceTelemetry{
		AgentID:   "agent-002",
		Subdomain: "agent-002",
		Services: map[string]relay.HealthProbe{
			"cinemana": {Available: false, LatencyMs: 999, PacketLoss: 100, LastChecked: time.Now()},
		},
		NodeMetrics: relay.NodeMetrics{CPUPercent: 90, ActiveRelays: 10},
	})

	tableAfterFailover := router.ComputeGlobalRoutingTable()
	routeAfter, ok := tableAfterFailover.Routes["cinemana"]
	if !ok {
		t.Fatalf("Expected route after failover")
	}

	if routeAfter.PrimaryAgent != "agent-004" {
		t.Errorf("Expected primary agent to switch to 'agent-004' (35ms), got %q", routeAfter.PrimaryAgent)
	}

	if routeAfter.BackupAgent != "agent-001" {
		t.Errorf("Expected backup agent to switch to 'agent-001' (50ms), got %q", routeAfter.BackupAgent)
	}
}
