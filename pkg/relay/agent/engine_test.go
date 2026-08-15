package agent

import (
	"testing"
	"time"

	"mikrotik-manager/pkg/relay"
)

type mockSignaler struct {
	sent []relay.RelayControlMessage
}

func (m *mockSignaler) SendRelayControl(msg relay.RelayControlMessage) error {
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockSignaler) GetCloudGatewayURL() string {
	return "127.0.0.1:18080"
}

func TestRelayAgentEngineRouteResolution(t *testing.T) {
	signaler := &mockSignaler{}
	cfg := EngineConfig{
		AgentID:            "agent-001",
		Subdomain:          "agent-001",
		ListenAddr:         "127.0.0.1:19443",
		EnableInterception: false, // Don't bind port in unit test
	}

	engine := NewEngine(cfg, signaler)
	defer engine.Stop()

	// Sync catalog
	services := []relay.ServiceDefinition{
		{
			ID:       "cinemana",
			Name:     "Cinemana",
			Domains:  []string{"cinemana.shabakaty.cc", "*.shabakaty.cc"},
			Ports:    []int{443},
			Enabled:  true,
		},
		{
			ID:       "shabakaty_tv",
			Name:     "Shabakaty TV",
			Domains:  []string{"tv.shabakaty.com"},
			Ports:    []int{443},
			Enabled:  true,
		},
	}
	engine.OnCatalogSync(services)

	// Sync routing table
	table := relay.AgentRoutingTable{
		Version:   1,
		Timestamp: time.Now(),
		Routes: map[string]relay.EgressRoute{
			"cinemana": {
				ServiceID:    "cinemana",
				PrimaryAgent: "agent-002",
				BackupAgent:  "agent-004",
				LatencyMs:    18.5,
			},
			"shabakaty_tv": {
				ServiceID:    "shabakaty_tv",
				PrimaryAgent: "agent-001", // Local agent is provider
			},
		},
	}
	engine.OnRouteTableSync(table)

	// 1. Resolve cinemana.shabakaty.cc -> should route to agent-002
	route, svc, err := engine.ResolveRoute("cinemana.shabakaty.cc", "10.0.0.1:443")
	if err != nil {
		t.Fatalf("Failed to resolve route: %v", err)
	}
	if svc.ID != "cinemana" {
		t.Errorf("Expected service 'cinemana', got %s", svc.ID)
	}
	if route.PrimaryAgent != "agent-002" {
		t.Errorf("Expected primary agent 'agent-002', got %s", route.PrimaryAgent)
	}

	// 2. Resolve wildcard sub.shabakaty.cc -> should route to agent-002
	routeWild, _, errWild := engine.ResolveRoute("video.shabakaty.cc", "10.0.0.1:443")
	if errWild != nil {
		t.Fatalf("Failed to resolve wildcard route: %v", errWild)
	}
	if routeWild.PrimaryAgent != "agent-002" {
		t.Errorf("Expected primary agent 'agent-002', got %s", routeWild.PrimaryAgent)
	}

	// 3. Resolve shabakaty_tv -> local agent is primary, should NOT relay
	_, _, errLocal := engine.ResolveRoute("tv.shabakaty.com", "10.0.0.1:443")
	if errLocal == nil {
		t.Errorf("Expected error because local agent is primary egress, but got nil")
	}

	// 4. Resolve unrelated domain google.com -> no match
	_, _, errUnknown := engine.ResolveRoute("google.com", "10.0.0.1:443")
	if errUnknown == nil {
		t.Errorf("Expected error for non-service domain, but got nil")
	}
}
