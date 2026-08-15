package relay

import (
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"mikrotik-manager/pkg/relay"
)

// RouterEngine calculates optimal egress paths and manages automatic failover
type RouterEngine struct {
	catalog   *CatalogManager
	telemetry *TelemetryHub
	mu        sync.RWMutex
	lastTable relay.AgentRoutingTable
	version   int64
	onUpdate  func(table relay.AgentRoutingTable)
	running   int32
	stopCh    chan struct{}
}

func NewRouterEngine(catalog *CatalogManager, telemetry *TelemetryHub, onUpdate func(relay.AgentRoutingTable)) *RouterEngine {
	return &RouterEngine{
		catalog:   catalog,
		telemetry: telemetry,
		onUpdate:  onUpdate,
		stopCh:    make(chan struct{}),
	}
}

func (re *RouterEngine) Start() {
	if !atomic.CompareAndSwapInt32(&re.running, 0, 1) {
		return
	}

	go re.evalLoop()
}

func (re *RouterEngine) Stop() {
	if atomic.CompareAndSwapInt32(&re.running, 1, 0) {
		close(re.stopCh)
	}
}

func (re *RouterEngine) evalLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-re.stopCh:
			return
		case <-ticker.C:
			table := re.ComputeGlobalRoutingTable()
			if re.hasTableChanged(table) {
				re.mu.Lock()
				re.lastTable = table
				re.mu.Unlock()

				log.Printf("[Relay Router] Computed new global routing table (v%d) with %d routes", table.Version, len(table.Routes))
				if re.onUpdate != nil {
					re.onUpdate(table)
				}
			}
		}
	}
}

// ComputeGlobalRoutingTable evaluates all services and selects best primary/backup egress agents
func (re *RouterEngine) ComputeGlobalRoutingTable() relay.AgentRoutingTable {
	services := re.catalog.GetAllServices()
	routes := make(map[string]relay.EgressRoute)

	for _, svc := range services {
		if !svc.Enabled {
			continue
		}

		allProviders := re.telemetry.GetServiceProviders(svc.ID)
		if len(allProviders) == 0 {
			continue
		}

		// Filter by AllowedProviders if specified
		var providers []AgentServiceStatus
		if len(svc.AllowedProviders) > 0 {
			allowedMap := make(map[string]bool)
			for _, ap := range svc.AllowedProviders {
				allowedMap[ap] = true
			}
			for _, p := range allProviders {
				if allowedMap[p.Subdomain] || allowedMap[p.AgentID] {
					providers = append(providers, p)
				}
			}
		} else {
			providers = allProviders
		}

		if len(providers) == 0 {
			continue
		}

		// Calculate penalty scores and sort
		type scoredCandidate struct {
			provider AgentServiceStatus
			score    float64
		}

		var candidates []scoredCandidate
		for _, p := range providers {
			// Score formula: lower is better
			score := (p.Probe.LatencyMs * 1.0) +
				(p.Probe.PacketLoss * 5.0) +
				(p.NodeMetrics.CPUPercent * 0.2) +
				(float64(p.NodeMetrics.ActiveRelays) * 0.5)

			candidates = append(candidates, scoredCandidate{
				provider: p,
				score:    score,
			})
		}

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].score < candidates[j].score
		})

		primary := candidates[0].provider.Subdomain
		primaryScore := candidates[0].score
		primaryLatency := candidates[0].provider.Probe.LatencyMs

		backup := ""
		if len(candidates) > 1 {
			backup = candidates[1].provider.Subdomain
		}

		routes[svc.ID] = relay.EgressRoute{
			ServiceID:    svc.ID,
			PrimaryAgent: primary,
			BackupAgent:  backup,
			Score:        primaryScore,
			LatencyMs:    primaryLatency,
			UseCloudFall: true,
			UpdatedAt:    time.Now().UTC(),
		}
	}

	newVersion := atomic.AddInt64(&re.version, 1)
	return relay.AgentRoutingTable{
		Version:   newVersion,
		Timestamp: time.Now().UTC(),
		Routes:    routes,
	}
}

// ComputeRoutingTableForAgent filters the global routing table for services accessible by this specific agent and its group
func (re *RouterEngine) ComputeRoutingTableForAgent(agentSubdomain, agentGroup string) relay.AgentRoutingTable {
	global := re.ComputeGlobalRoutingTable()
	filteredRoutes := make(map[string]relay.EgressRoute)

	for svcID, route := range global.Routes {
		svc, ok := re.catalog.GetServiceByID(svcID)
		if !ok || !svc.Enabled {
			continue
		}

		if svc.TargetScope == "all" || (len(svc.AllowedConsumers) == 0 && len(svc.TargetGroups) == 0) {
			filteredRoutes[svcID] = route
			continue
		}

		// Check if agent is in allowed consumers
		isAllowed := false
		for _, allowed := range svc.AllowedConsumers {
			if allowed == agentSubdomain {
				isAllowed = true
				break
			}
		}

		// Check if agent's group is in target groups
		if !isAllowed && agentGroup != "" {
			for _, g := range svc.TargetGroups {
				if g == agentGroup {
					isAllowed = true
					break
				}
			}
		}

		if isAllowed {
			filteredRoutes[svcID] = route
		}
	}

	return relay.AgentRoutingTable{
		Version:   global.Version,
		Timestamp: global.Timestamp,
		Routes:    filteredRoutes,
	}
}

func (re *RouterEngine) hasTableChanged(newTable relay.AgentRoutingTable) bool {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if len(newTable.Routes) != len(re.lastTable.Routes) {
		return true
	}

	for k, newRoute := range newTable.Routes {
		oldRoute, ok := re.lastTable.Routes[k]
		if !ok {
			return true
		}
		if newRoute.PrimaryAgent != oldRoute.PrimaryAgent || newRoute.BackupAgent != oldRoute.BackupAgent {
			return true
		}
	}

	return false
}

func (re *RouterEngine) GetCurrentRoutingTable() relay.AgentRoutingTable {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.lastTable
}
