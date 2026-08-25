package relay

import (
	"encoding/json"
	"time"

	"mikrotik-manager/pkg/relay"

	"github.com/gofiber/fiber/v2"
)

// APIHandler handles REST and Control Plane API requests for Service Relay
type APIHandler struct {
	catalog              *CatalogManager
	telemetry            *TelemetryHub
	router               *RouterEngine
	broadcastCatalogSync func(catalog []relay.ServiceDefinition)
	broadcastProbeReq    func(serviceID string)
	getAgentList         func() []string
}

func NewAPIHandler(catalog *CatalogManager, telemetry *TelemetryHub, router *RouterEngine) *APIHandler {
	return &APIHandler{
		catalog:   catalog,
		telemetry: telemetry,
		router:    router,
	}
}

func (h *APIHandler) SetBroadcaster(syncCatalog func([]relay.ServiceDefinition), probeReq func(string), agentList func() []string) {
	h.broadcastCatalogSync = syncCatalog
	h.broadcastProbeReq = probeReq
	h.getAgentList = agentList
}

func (h *APIHandler) RegisterRoutes(router fiber.Router) {
	group := router.Group("/api/relay")

	group.Get("/services", h.handleListServices)
	group.Post("/services", h.handleSaveService)
	group.Delete("/services/:id", h.handleDeleteService)

	group.Get("/services/:id/status", h.handleGetServiceStatus)
	group.Post("/services/:id/probe", h.handleProbeService)

	group.Get("/telemetry", h.handleGetTelemetry)
	group.Get("/nodes", h.handleListNodes)
	group.Post("/nodes/:id/role", h.handleSetNodeRole)
	group.Post("/nodes/:id/test-ip", h.handleTestNodeIP)

	group.Get("/routes", h.handleGetRoutes)
	group.Post("/recalculate", h.handleRecalculateRoutes)
	group.Post("/sync-all", h.handleForceSyncAll)
	group.Post("/kill-switch", h.handleToggleKillSwitch)
	group.Post("/strategy", h.handleSetStrategy)
}

func (h *APIHandler) handleListServices(c *fiber.Ctx) error {
	services := h.catalog.GetAllServices()
	return c.JSON(fiber.Map{
		"success":  true,
		"services": services,
	})
}

func (h *APIHandler) handleSaveService(c *fiber.Ctx) error {
	var svc relay.ServiceDefinition
	if err := c.BodyParser(&svc); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	if svc.ID == "" || svc.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Service ID and Name are required",
		})
	}

	if err := h.catalog.SaveService(svc); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to save service: " + err.Error(),
		})
	}

	// Trigger route recalculation
	_ = h.router.ComputeGlobalRoutingTable()

	// Broadcast updated catalog to all connected agents for instant MikroTik sync
	if h.broadcastCatalogSync != nil {
		h.broadcastCatalogSync(h.catalog.GetAllServices())
	}

	return c.JSON(fiber.Map{
		"success": true,
		"service": svc,
	})
}

func (h *APIHandler) handleDeleteService(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Service ID is required",
		})
	}

	if err := h.catalog.DeleteService(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to delete service: " + err.Error(),
		})
	}

	// Recalculate routes and broadcast updated catalog to agents
	_ = h.router.ComputeGlobalRoutingTable()
	if h.broadcastCatalogSync != nil {
		h.broadcastCatalogSync(h.catalog.GetAllServices())
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

func (h *APIHandler) handleGetServiceStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Service ID is required"})
	}

	svc, err := h.catalog.GetService(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Service not found"})
	}

	allTel := h.telemetry.GetAllTelemetry()
	type AgentStatusItem struct {
		Subdomain    string  `json:"subdomain"`
		Available    bool    `json:"available"`
		LatencyMs    float64 `json:"latency_ms"`
		PacketLoss   float64 `json:"packet_loss"`
		ErrorMessage string  `json:"error_message,omitempty"`
		LastChecked  string  `json:"last_checked"`
		LastSeen     string  `json:"last_seen"`
		Status       string  `json:"status"` // "available", "unavailable", "pending", "offline"
	}

	var results []AgentStatusItem
	var agentNames []string
	if h.getAgentList != nil {
		agentNames = h.getAgentList()
	} else {
		for k := range allTel {
			agentNames = append(agentNames, k)
		}
	}

	availableCount := 0
	for _, sub := range agentNames {
		tel, ok := allTel[sub]
		if !ok {
			results = append(results, AgentStatusItem{
				Subdomain: sub,
				Status:    "offline",
			})
			continue
		}

		lastSeenStr := tel.Timestamp.Format(time.RFC3339)
		if probe, exists := tel.Services[id]; exists {
			st := "unavailable"
			if probe.Available {
				st = "available"
				availableCount++
			}
			results = append(results, AgentStatusItem{
				Subdomain:    sub,
				Available:    probe.Available,
				LatencyMs:    probe.LatencyMs,
				PacketLoss:   probe.PacketLoss,
				ErrorMessage: probe.ErrorMessage,
				LastChecked:  probe.LastChecked.Format(time.RFC3339),
				LastSeen:     lastSeenStr,
				Status:       st,
			})
		} else {
			results = append(results, AgentStatusItem{
				Subdomain: sub,
				LastSeen:  lastSeenStr,
				Status:    "pending",
			})
		}
	}

	return c.JSON(fiber.Map{
		"success":         true,
		"service":         svc,
		"agents":          results,
		"total":           len(results),
		"available_count": availableCount,
	})
}

func (h *APIHandler) handleProbeService(c *fiber.Ctx) error {
	id := c.Params("id")
	if h.broadcastProbeReq != nil {
		h.broadcastProbeReq(id)
	}
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Probe request broadcasted to all active agents",
	})
}

func (h *APIHandler) handleGetTelemetry(c *fiber.Ctx) error {
	telemetry := h.telemetry.GetAllTelemetry()
	return c.JSON(fiber.Map{
		"success":   true,
		"telemetry": telemetry,
	})
}

func (h *APIHandler) handleGetRoutes(c *fiber.Ctx) error {
	table := h.router.GetCurrentRoutingTable()
	return c.JSON(fiber.Map{
		"success": true,
		"routes":  table,
	})
}

func (h *APIHandler) handleRecalculateRoutes(c *fiber.Ctx) error {
	table := h.router.ComputeGlobalRoutingTable()
	return c.JSON(fiber.Map{
		"success": true,
		"routes":  table,
	})
}

func (h *APIHandler) handleListNodes(c *fiber.Ctx) error {
	telemetries := h.telemetry.GetAllTelemetry()
	type NodeInfo struct {
		Subdomain    string                 `json:"subdomain"`
		AgentID      string                 `json:"agent_id"`
		PublicIP     string                 `json:"public_ip"`
		ISPName      string                 `json:"isp_name"`
		ASN          string                 `json:"asn"`
		CountryCode  string                 `json:"country_code"`
		AssignedRole relay.NodeRole         `json:"assigned_role"`
		CPUPercent   float64                `json:"cpu_percent"`
		MemoryMB     float64                `json:"memory_mb"`
		ActiveRelays int                    `json:"active_relays"`
		Services     map[string]relay.HealthProbe `json:"services"`
		LastSeen     time.Time              `json:"last_seen"`
	}

	var nodes []NodeInfo
	for sub, tel := range telemetries {
		role := h.telemetry.GetAgentRole(sub)
		nodes = append(nodes, NodeInfo{
			Subdomain:    sub,
			AgentID:      tel.AgentID,
			PublicIP:     tel.PublicIP,
			ISPName:      tel.ISPName,
			ASN:          tel.ASN,
			CountryCode:  tel.CountryCode,
			AssignedRole: role,
			CPUPercent:   tel.NodeMetrics.CPUPercent,
			MemoryMB:     tel.NodeMetrics.MemoryMB,
			ActiveRelays: tel.NodeMetrics.ActiveRelays,
			Services:     tel.Services,
			LastSeen:     tel.Timestamp,
		})
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"nodes":         nodes,
		"global_bypass": h.router.IsGlobalBypass(),
		"strategy":      h.router.GetStrategy(),
	})
}

func (h *APIHandler) handleSetNodeRole(c *fiber.Ctx) error {
	agentID := c.Params("id")
	var req struct {
		Role string `json:"role"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": err.Error()})
	}

	role := relay.NodeRole(req.Role)
	if role != relay.NodeRoleExit && role != relay.NodeRoleConsumer && role != relay.NodeRoleHybrid {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Invalid role"})
	}

	h.telemetry.SetAgentRole(agentID, role)
	_ = h.router.ComputeGlobalRoutingTable()

	return c.JSON(fiber.Map{
		"success": true,
		"agent":   agentID,
		"role":    role,
	})
}

func (h *APIHandler) handleTestNodeIP(c *fiber.Ctx) error {
	agentID := c.Params("id")
	tel, ok := h.telemetry.GetAgentTelemetry(agentID)
	
	result := relay.EgressProbeResult{
		AgentID:     agentID,
		Subdomain:   agentID,
		EgressAgent: "direct/relay",
		PublicIP:    tel.PublicIP,
		Org:         tel.ISPName + " (" + tel.ASN + ")",
		Country:     tel.CountryCode,
		IsIraqiIP:   tel.CountryCode == "IQ",
		CheckedAt:   time.Now().UTC(),
	}
	if !ok || result.PublicIP == "" {
		result.PublicIP = "Unknown"
		result.Country = "IQ"
		result.IsIraqiIP = true
		result.Org = "Local Domestic Egress"
	}

	return c.JSON(fiber.Map{
		"success": true,
		"result":  result,
	})
}

func (h *APIHandler) handleForceSyncAll(c *fiber.Ctx) error {
	if h.broadcastCatalogSync != nil {
		h.broadcastCatalogSync(h.catalog.GetAllServices())
	}
	_ = h.router.ComputeGlobalRoutingTable()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Force sync broadcasted to all connected MikroTik agents",
	})
}

func (h *APIHandler) handleToggleKillSwitch(c *fiber.Ctx) error {
	var req struct {
		Bypass bool `json:"bypass"`
	}
	if err := c.BodyParser(&req); err != nil {
		h.router.SetGlobalBypass(!h.router.IsGlobalBypass())
	} else {
		h.router.SetGlobalBypass(req.Bypass)
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"global_bypass": h.router.IsGlobalBypass(),
	})
}

func (h *APIHandler) handleSetStrategy(c *fiber.Ctx) error {
	var req struct {
		Strategy string `json:"strategy"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": err.Error()})
	}

	h.router.SetStrategy(req.Strategy)
	return c.JSON(fiber.Map{
		"success":  true,
		"strategy": h.router.GetStrategy(),
	})
}

// IngestTelemetryMessage is called when WebSocket receives MsgTelemetryPush
func (h *APIHandler) IngestTelemetryMessage(agentKey string, payload []byte) {
	var tel relay.ServiceTelemetry
	if err := json.Unmarshal(payload, &tel); err == nil {
		if tel.Subdomain == "" {
			tel.Subdomain = agentKey
		}
		if tel.Timestamp.IsZero() {
			tel.Timestamp = time.Now().UTC()
		}
		h.telemetry.IngestTelemetry(tel)
	}
}
