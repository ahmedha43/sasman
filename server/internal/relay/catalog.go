package relay

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"mikrotik-manager/pkg/relay"
)

// CatalogManager handles persistence and memory caching of Service Definitions
type CatalogManager struct {
	db       *sql.DB
	mu       sync.RWMutex
	services map[string]relay.ServiceDefinition
}

func NewCatalogManager(db *sql.DB) (*CatalogManager, error) {
	cm := &CatalogManager{
		db:       db,
		services: make(map[string]relay.ServiceDefinition),
	}

	if err := cm.initSchema(); err != nil {
		return nil, fmt.Errorf("init relay schema: %w", err)
	}

	if err := cm.loadFromDB(); err != nil {
		log.Printf("[Relay Catalog] Warning loading from DB: %v", err)
	}

	if len(cm.services) == 0 {
		cm.seedDefaultServices()
	}

	return cm, nil
}

func (cm *CatalogManager) initSchema() error {
	if cm.db == nil {
		return nil
	}

	query := `
	CREATE TABLE IF NOT EXISTS relay_services (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		category TEXT NOT NULL DEFAULT 'Streaming',
		domains TEXT NOT NULL,
		ip_ranges TEXT NOT NULL DEFAULT '[]',
		ports TEXT NOT NULL DEFAULT '[443]',
		protocols TEXT NOT NULL DEFAULT '["tls"]',
		probe_config TEXT NOT NULL DEFAULT '{}',
		target_scope TEXT NOT NULL DEFAULT 'all',
		target_groups TEXT NOT NULL DEFAULT '[]',
		allowed_consumers TEXT NOT NULL DEFAULT '[]',
		allowed_providers TEXT NOT NULL DEFAULT '[]',
		max_bandwidth_mbps INTEGER NOT NULL DEFAULT 0,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	`
	if _, err := cm.db.Exec(query); err != nil {
		return err
	}

	// Safe column migration for existing sqlite db
	_, _ = cm.db.Exec("ALTER TABLE relay_services ADD COLUMN target_scope TEXT NOT NULL DEFAULT 'all'")
	_, _ = cm.db.Exec("ALTER TABLE relay_services ADD COLUMN target_groups TEXT NOT NULL DEFAULT '[]'")
	_, _ = cm.db.Exec("ALTER TABLE relay_services ADD COLUMN allowed_consumers TEXT NOT NULL DEFAULT '[]'")
	_, _ = cm.db.Exec("ALTER TABLE relay_services ADD COLUMN allowed_providers TEXT NOT NULL DEFAULT '[]'")
	_, _ = cm.db.Exec("ALTER TABLE relay_services ADD COLUMN max_bandwidth_mbps INTEGER NOT NULL DEFAULT 0")

	return nil
}

func (cm *CatalogManager) loadFromDB() error {
	if cm.db == nil {
		return nil
	}

	rows, err := cm.db.Query("SELECT id, name, category, domains, ip_ranges, ports, protocols, probe_config, target_scope, target_groups, allowed_consumers, allowed_providers, max_bandwidth_mbps, enabled, created_at, updated_at FROM relay_services")
	if err != nil {
		return err
	}
	defer rows.Close()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	for rows.Next() {
		var (
			id, name, category, domainsJSON, ipsJSON, portsJSON, protocolsJSON, probeJSON, scope, groupsJSON, consumersJSON, providersJSON, createdStr, updatedStr string
			maxBandwidth, enabledInt                                                                                                                               int
		)

		if err := rows.Scan(&id, &name, &category, &domainsJSON, &ipsJSON, &portsJSON, &protocolsJSON, &probeJSON, &scope, &groupsJSON, &consumersJSON, &providersJSON, &maxBandwidth, &enabledInt, &createdStr, &updatedStr); err != nil {
			continue
		}

		var (
			domains   []string
			ipRanges  []string
			ports     []int
			protocols []string
			probe     relay.ProbeConfig
			groups    []string
			consumers []string
			providers []string
		)

		_ = json.Unmarshal([]byte(domainsJSON), &domains)
		_ = json.Unmarshal([]byte(ipsJSON), &ipRanges)
		_ = json.Unmarshal([]byte(portsJSON), &ports)
		_ = json.Unmarshal([]byte(protocolsJSON), &protocols)
		_ = json.Unmarshal([]byte(probeJSON), &probe)
		_ = json.Unmarshal([]byte(groupsJSON), &groups)
		_ = json.Unmarshal([]byte(consumersJSON), &consumers)
		_ = json.Unmarshal([]byte(providersJSON), &providers)

		if scope == "" {
			scope = "all"
		}

		created, _ := time.Parse(time.RFC3339, createdStr)
		updated, _ := time.Parse(time.RFC3339, updatedStr)

		cm.services[id] = relay.ServiceDefinition{
			ID:               id,
			Name:             name,
			Category:         category,
			Domains:          domains,
			IPRanges:         ipRanges,
			Ports:            ports,
			Protocols:        protocols,
			ProbeConfig:      probe,
			TargetScope:      scope,
			TargetGroups:     groups,
			AllowedConsumers: consumers,
			AllowedProviders: providers,
			MaxBandwidthMbps: maxBandwidth,
			Enabled:          enabledInt == 1,
			CreatedAt:        created,
			UpdatedAt:        updated,
		}
	}

	return nil
}

func (cm *CatalogManager) seedDefaultServices() {
	defaults := []relay.ServiceDefinition{
		{
			ID:          "cinemana",
			Name:        "Shabakaty Cinemana",
			Category:    "Streaming",
			Domains:     []string{"cinemana.shabakaty.cc", "*.shabakaty.cc", "*.shabakaty.com"},
			Ports:       []int{80, 443},
			Protocols:   []string{"tcp", "tls"},
			TargetScope: "all",
			ProbeConfig: relay.ProbeConfig{
				Type:        "https",
				TargetURL:   "https://cinemana.shabakaty.cc",
				IntervalSec: 15,
				TimeoutSec:  3,
			},
			Enabled:   true,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		{
			ID:          "shabakaty_tv",
			Name:        "Shabakaty TV / Live",
			Category:    "IPTV",
			Domains:     []string{"tv.shabakaty.com", "*.shabakaty.tv"},
			Ports:       []int{80, 443, 8080},
			Protocols:   []string{"tcp", "tls"},
			TargetScope: "all",
			ProbeConfig: relay.ProbeConfig{
				Type:        "https",
				TargetURL:   "https://tv.shabakaty.com",
				IntervalSec: 15,
				TimeoutSec:  3,
			},
			Enabled:   true,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		{
			ID:          "earthlink_share",
			Name:        "EarthLink Share CDN",
			Category:    "CDN",
			Domains:     []string{"share.earthlink.iq", "*.earthlink.iq"},
			Ports:       []int{80, 443},
			Protocols:   []string{"tcp", "tls"},
			TargetScope: "all",
			ProbeConfig: relay.ProbeConfig{
				Type:        "https",
				TargetURL:   "https://share.earthlink.iq",
				IntervalSec: 15,
				TimeoutSec:  3,
			},
			Enabled:   true,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}

	for _, svc := range defaults {
		_ = cm.SaveService(svc)
	}
}

// SaveService inserts or updates a service in the database and memory
func (cm *CatalogManager) SaveService(svc relay.ServiceDefinition) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	svc.UpdatedAt = time.Now().UTC()
	if svc.CreatedAt.IsZero() {
		svc.CreatedAt = svc.UpdatedAt
	}
	if svc.TargetScope == "" {
		svc.TargetScope = "all"
	}

	domainsJSON, _ := json.Marshal(svc.Domains)
	ipsJSON, _ := json.Marshal(svc.IPRanges)
	portsJSON, _ := json.Marshal(svc.Ports)
	protosJSON, _ := json.Marshal(svc.Protocols)
	probeJSON, _ := json.Marshal(svc.ProbeConfig)
	groupsJSON, _ := json.Marshal(svc.TargetGroups)
	consumersJSON, _ := json.Marshal(svc.AllowedConsumers)
	providersJSON, _ := json.Marshal(svc.AllowedProviders)

	enabledInt := 0
	if svc.Enabled {
		enabledInt = 1
	}

	if cm.db != nil {
		query := `
		INSERT INTO relay_services (id, name, category, domains, ip_ranges, ports, protocols, probe_config, target_scope, target_groups, allowed_consumers, allowed_providers, max_bandwidth_mbps, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			category=excluded.category,
			domains=excluded.domains,
			ip_ranges=excluded.ip_ranges,
			ports=excluded.ports,
			protocols=excluded.protocols,
			probe_config=excluded.probe_config,
			target_scope=excluded.target_scope,
			target_groups=excluded.target_groups,
			allowed_consumers=excluded.allowed_consumers,
			allowed_providers=excluded.allowed_providers,
			max_bandwidth_mbps=excluded.max_bandwidth_mbps,
			enabled=excluded.enabled,
			updated_at=excluded.updated_at
		`
		_, err := cm.db.Exec(query, svc.ID, svc.Name, svc.Category, string(domainsJSON), string(ipsJSON), string(portsJSON), string(protosJSON), string(probeJSON), svc.TargetScope, string(groupsJSON), string(consumersJSON), string(providersJSON), svc.MaxBandwidthMbps, enabledInt, svc.CreatedAt.Format(time.RFC3339), svc.UpdatedAt.Format(time.RFC3339))
		if err != nil {
			return err
		}
	}

	cm.services[svc.ID] = svc
	return nil
}

// DeleteService removes a service from memory and database
func (cm *CatalogManager) DeleteService(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.db != nil {
		_, err := cm.db.Exec("DELETE FROM relay_services WHERE id = ?", id)
		if err != nil {
			return err
		}
	}

	delete(cm.services, id)
	return nil
}

// GetAllServices returns all services
func (cm *CatalogManager) GetAllServices() []relay.ServiceDefinition {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	out := make([]relay.ServiceDefinition, 0, len(cm.services))
	for _, s := range cm.services {
		out = append(out, s)
	}
	return out
}

// GetServicesForAgent filters services permitted for a specific consumer agent by subdomain and group
func (cm *CatalogManager) GetServicesForAgent(agentSubdomain, agentGroup string) []relay.ServiceDefinition {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var out []relay.ServiceDefinition
	for _, s := range cm.services {
		if !s.Enabled {
			continue
		}
		if s.TargetScope == "all" || (len(s.AllowedConsumers) == 0 && len(s.TargetGroups) == 0) {
			out = append(out, s)
			continue
		}

		// Check direct subdomain match
		allowed := false
		for _, consumer := range s.AllowedConsumers {
			if consumer == agentSubdomain {
				allowed = true
				break
			}
		}

		// Check group match
		if !allowed && agentGroup != "" {
			for _, g := range s.TargetGroups {
				if g == agentGroup {
					allowed = true
					break
				}
			}
		}

		if allowed {
			out = append(out, s)
		}
	}
	return out
}

// GetServiceByID returns a single service definition
func (cm *CatalogManager) GetServiceByID(id string) (relay.ServiceDefinition, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	s, ok := cm.services[id]
	return s, ok
}
