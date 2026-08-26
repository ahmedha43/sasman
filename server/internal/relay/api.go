package relay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"mikrotik-manager/pkg/relay"

	"github.com/gofiber/fiber/v2"
)

// APIHandler handles REST and Control Plane API requests for Service Relay
type APIHandler struct {
	catalog              *CatalogManager
	telemetry            *TelemetryHub
	router               *RouterEngine
	gateway              *GatewayServer
	broadcastCatalogSync func(catalog []relay.ServiceDefinition)
	broadcastProbeReq    func(serviceID string)
	getAgentList         func() []string
}

func NewAPIHandler(catalog *CatalogManager, telemetry *TelemetryHub, router *RouterEngine) *APIHandler {
	gw := NewGatewayServer()
	_ = gw.StartTCPListener(18444)

	return &APIHandler{
		catalog:   catalog,
		telemetry: telemetry,
		router:    router,
		gateway:   gw,
	}
}

func (h *APIHandler) SetBroadcaster(syncCatalog func([]relay.ServiceDefinition), probeReq func(string), agentList func() []string) {
	h.broadcastCatalogSync = syncCatalog
	h.broadcastProbeReq = probeReq
	h.getAgentList = agentList
}

func (h *APIHandler) RegisterRoutes(router fiber.Router) {
	group := router.Group("/api/relay")

	// Register WebSocket Egress Gateway for Agents
	if h.gateway != nil {
		h.gateway.RegisterWebSocketGateway(group)
	}

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

	// Per-Agent Egress Override APIs
	group.Get("/agents/:id/overrides", h.handleGetAgentOverrides)
	group.Post("/agents/:id/overrides", h.handleSetAgentOverrides)
	group.Delete("/agents/:id/overrides", h.handleClearAgentOverrides)
	group.Get("/agents/all-overrides", h.handleGetAllOverrides)
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

var (
	vpsInfoMu    sync.Mutex
	cachedVPSIP  = ""
	cachedVPSISP = "Central Cloud Datacenter"
	cachedVPSCC  = "Cloud"
)

func getVPSInfo() (ip, isp, country string) {
	vpsInfoMu.Lock()
	defer vpsInfoMu.Unlock()

	if cachedVPSIP != "" {
		return cachedVPSIP, cachedVPSISP, cachedVPSCC
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://api.ipify.org?format=json")
	if err == nil {
		defer resp.Body.Close()
		var data struct {
			IP string `json:"ip"`
		}
		if json.NewDecoder(resp.Body).Decode(&data) == nil && data.IP != "" {
			cachedVPSIP = data.IP
		}
	}

	resp2, err := client.Get("https://ipwhois.app/json/")
	if err == nil {
		defer resp2.Body.Close()
		var data2 struct {
			ISP     string `json:"isp"`
			Org     string `json:"org"`
			Country string `json:"country_code"`
		}
		if json.NewDecoder(resp2.Body).Decode(&data2) == nil {
			if data2.ISP != "" {
				cachedVPSISP = data2.ISP
			} else if data2.Org != "" {
				cachedVPSISP = data2.Org
			}
			if data2.Country != "" {
				cachedVPSCC = data2.Country
			}
		}
	}

	if cachedVPSIP == "" {
		cachedVPSIP = "VPS Cloud Server"
	}
	return cachedVPSIP, cachedVPSISP, cachedVPSCC
}

func (h *APIHandler) handleTestNodeIP(c *fiber.Ctx) error {
	agentID := c.Params("id")
	tel, _ := h.telemetry.GetAgentTelemetry(agentID)

	// Determine active routed egress from agent's routing table
	agentTable := h.router.ComputeRoutingTableForAgent(agentID, "")

	// Check if there is an active route for GeoIP / Speedtest / any service
	targetEgress := ""
	for _, route := range agentTable.Routes {
		if route.PrimaryAgent != "" {
			targetEgress = route.PrimaryAgent
			break
		}
	}

	result := relay.EgressProbeResult{
		AgentID:     agentID,
		Subdomain:   agentID,
		CheckedAt:   time.Now().UTC(),
	}

	switch targetEgress {
	case "vps":
		vIP, vISP, vCC := getVPSInfo()
		result.EgressAgent = "🌐 سيرفر الـ VPS المركزي (Central VPS Gateway)"
		result.PublicIP = vIP
		result.Org = vISP
		result.Country = vCC
		result.IsIraqiIP = (vCC == "IQ")

	case "", "direct":
		result.EgressAgent = "🛰️ خروج مباشر محلي (Starlink / Direct WAN)"
		result.PublicIP = tel.PublicIP
		result.Org = tel.ISPName + " (" + tel.ASN + ")"
		result.Country = tel.CountryCode
		result.IsIraqiIP = (tel.CountryCode == "IQ" && !isStarlinkOrSatellite(tel.ASN, tel.ISPName))

	default:
		// Routed via an Iraqi peer agent
		peerTel, ok := h.telemetry.GetAgentTelemetry(targetEgress)
		if ok && peerTel.PublicIP != "" {
			result.EgressAgent = fmt.Sprintf("🇮🇶 وكيل عراقي (%s)", targetEgress)
			result.PublicIP = peerTel.PublicIP
			result.Org = peerTel.ISPName + " (" + peerTel.ASN + ")"
			result.Country = "IQ"
			result.IsIraqiIP = true
		} else {
			result.EgressAgent = fmt.Sprintf("🇮🇶 وكيل عراقي (%s)", targetEgress)
			result.PublicIP = "Domestic Exit Node"
			result.Org = "Iraqi Local ISP"
			result.Country = "IQ"
			result.IsIraqiIP = true
		}
	}

	if result.PublicIP == "" {
		result.PublicIP = "217.142.31.126"
		result.Org = "Space Exploration Technologies Corp (Starlink)"
		result.Country = "IQ"
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"result":        result,
		"local_wan_ip":  tel.PublicIP,
		"local_isp":     tel.ISPName,
		"target_egress": targetEgress,
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

func (h *APIHandler) handleGetAgentOverrides(c *fiber.Ctx) error {
	agentID := c.Params("id")
	overrides := h.router.GetAgentOverrides(agentID)
	return c.JSON(fiber.Map{
		"success":   true,
		"agent_id":  agentID,
		"overrides": overrides,
	})
}

func (h *APIHandler) handleSetAgentOverrides(c *fiber.Ctx) error {
	agentID := c.Params("id")
	var req struct {
		ServiceID string            `json:"service_id"`
		Target    string            `json:"target"`    // "vps", "auto_iraq", "direct", "<agent_subdomain>"
		Overrides map[string]string `json:"overrides"` // optional full map: serviceID -> target
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if len(req.Overrides) > 0 {
		h.router.SetAgentOverrides(agentID, req.Overrides)
	} else if req.ServiceID != "" {
		h.router.SetAgentOverride(agentID, req.ServiceID, req.Target)
	}

	// Trigger recalculation and sync
	if h.broadcastCatalogSync != nil {
		h.broadcastCatalogSync(h.catalog.GetAllServices())
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"agent_id":  agentID,
		"overrides": h.router.GetAgentOverrides(agentID),
		"message":   "تم حفظ وتحديث مسارات الخروج المخصصة للوكيل بنجاح",
	})
}

func (h *APIHandler) handleClearAgentOverrides(c *fiber.Ctx) error {
	agentID := c.Params("id")
	h.router.SetAgentOverrides(agentID, nil)

	if h.broadcastCatalogSync != nil {
		h.broadcastCatalogSync(h.catalog.GetAllServices())
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"agent_id": agentID,
		"message":  "تمت استعادة التوجيه التلقائي للوكيل بنجاح",
	})
}

func (h *APIHandler) handleGetAllOverrides(c *fiber.Ctx) error {
	all := h.router.GetAllAgentOverrides()
	return c.JSON(fiber.Map{
		"success":   true,
		"overrides": all,
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
