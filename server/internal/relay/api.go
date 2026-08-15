package relay

import (
	"encoding/json"
	"time"

	"mikrotik-manager/pkg/relay"

	"github.com/gofiber/fiber/v2"
)

// APIHandler handles REST and Control Plane API requests for Service Relay
type APIHandler struct {
	catalog   *CatalogManager
	telemetry *TelemetryHub
	router    *RouterEngine
}

func NewAPIHandler(catalog *CatalogManager, telemetry *TelemetryHub, router *RouterEngine) *APIHandler {
	return &APIHandler{
		catalog:   catalog,
		telemetry: telemetry,
		router:    router,
	}
}

func (h *APIHandler) RegisterRoutes(router fiber.Router) {
	group := router.Group("/api/relay")

	group.Get("/services", h.handleListServices)
	group.Post("/services", h.handleSaveService)
	group.Delete("/services/:id", h.handleDeleteService)

	group.Get("/telemetry", h.handleGetTelemetry)
	group.Get("/routes", h.handleGetRoutes)
	group.Post("/recalculate", h.handleRecalculateRoutes)
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

	return c.JSON(fiber.Map{
		"success": true,
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
