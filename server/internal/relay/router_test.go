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

func TestAgentCustomEgressOverrides(t *testing.T) {
	catalog, err := NewCatalogManager(nil)
	if err != nil {
		t.Fatalf("Failed to init catalog: %v", err)
	}

	telemetry := NewTelemetryHub()
	router := NewRouterEngine(catalog, telemetry, nil)

	// Save two services: geoip and speedtest
	_ = catalog.SaveService(relay.ServiceDefinition{
		ID:      "geoip_identity",
		Name:    "GeoIP Identity",
		Domains: []string{"ipinfo.io", "ifconfig.co"},
		Enabled: true,
	})
	_ = catalog.SaveService(relay.ServiceDefinition{
		ID:      "speedtest",
		Name:    "Speedtest Benchmark",
		Domains: []string{"speedtest.net", "fast.com"},
		Enabled: true,
	})

	// Add telemetry for an Iraqi exit node: agent-iq-earthlink
	telemetry.IngestTelemetry(relay.ServiceTelemetry{
		AgentID:   "agent-iq-earthlink",
		Subdomain: "agent-iq-earthlink",
		Services: map[string]relay.HealthProbe{
			"geoip_identity": {Available: true, LatencyMs: 15, PacketLoss: 0, LastChecked: time.Now()},
			"speedtest":      {Available: true, LatencyMs: 20, PacketLoss: 0, LastChecked: time.Now()},
		},
	})

	// Case 1: Agent without override -> Gets Auto Iraqi Exit (agent-iq-earthlink)
	tableAuto := router.ComputeRoutingTableForAgent("agent-starlink-1", "default")
	if tableAuto.Routes["geoip_identity"].PrimaryAgent != "agent-iq-earthlink" {
		t.Errorf("Expected auto route to agent-iq-earthlink, got %s", tableAuto.Routes["geoip_identity"].PrimaryAgent)
	}

	// Case 2: Agent with VPS override on GeoIP only -> GeoIP routes via VPS, Speedtest stays auto
	router.SetAgentOverride("agent-starlink-2", "geoip_identity", "vps")
	tableVPS := router.ComputeRoutingTableForAgent("agent-starlink-2", "default")
	if tableVPS.Routes["geoip_identity"].PrimaryAgent != "vps" {
		t.Errorf("Expected GeoIP primary agent to be 'vps', got %s", tableVPS.Routes["geoip_identity"].PrimaryAgent)
	}
	if tableVPS.Routes["speedtest"].PrimaryAgent != "agent-iq-earthlink" {
		t.Errorf("Expected Speedtest to stay auto agent-iq-earthlink, got %s", tableVPS.Routes["speedtest"].PrimaryAgent)
	}

	// Case 3: Agent with Direct bypass on Speedtest -> Speedtest is omitted from table
	router.SetAgentOverride("agent-starlink-2", "speedtest", "direct")
	tableDirect := router.ComputeRoutingTableForAgent("agent-starlink-2", "default")
	if _, exists := tableDirect.Routes["speedtest"]; exists {
		t.Errorf("Expected speedtest route to be omitted when override is 'direct'")
	}

	// Case 4: Clear overrides -> returns to auto
	router.SetAgentOverrides("agent-starlink-2", nil)
	tableCleared := router.ComputeRoutingTableForAgent("agent-starlink-2", "default")
	if tableCleared.Routes["speedtest"].PrimaryAgent != "agent-iq-earthlink" {
		t.Errorf("Expected cleared speedtest to be auto, got %s", tableCleared.Routes["speedtest"].PrimaryAgent)
	}
}
