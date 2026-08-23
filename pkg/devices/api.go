package devices

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RegisterAPIRoutes registers all network device API endpoints on the provided Fiber router group
func RegisterAPIRoutes(router fiber.Router) {
	group := router.Group("/devices")

	group.Get("/summary", handleGetSummary)
	group.Get("/vendors", handleListVendors)
	group.Get("/types", handleListTypes)
	group.Post("/test-connection", handleTestConnection)
	group.Post("/discover", handleDiscover)

	group.Get("/", handleListDevices)
	group.Post("/", handleAddDevice)
	group.Get("/alerts", handleListAlerts)

	group.Get("/:id", handleGetDeviceDetail)
	group.Put("/:id", handleUpdateDevice)
	group.Delete("/:id", handleDeleteDevice)
	group.Post("/:id/poll", handleTriggerPoll)

	group.Get("/:id/interfaces", handleGetInterfaces)
	group.Get("/:id/metrics", handleGetMetrics)
	group.Get("/:id/clients", handleGetClients)
	group.Get("/:id/events", handleGetEvents)
}

func handleGetSummary(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}
	stats, err := GlobalService.GetSummaryStats()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(stats)
}

func handleListVendors(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}
	vendors, err := GlobalService.repo.ListVendors()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(vendors)
}

func handleListTypes(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}
	types, err := GlobalService.repo.ListDeviceTypes()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(types)
}

func handleTestConnection(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}

	var req struct {
		VendorSlug string `json:"vendor_slug"`
		IP         string `json:"ip"`
		Port       int    `json:"port"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		AuthType   string `json:"auth_type"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request format"})
	}

	driver, err := GetDriver(req.VendorSlug)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	port := req.Port
	if port <= 0 {
		port = 8728
	}

	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()

	err = driver.TestConnection(ctx, TargetConfig{
		IP:       req.IP,
		Port:     port,
		Username: req.Username,
		Password: req.Password,
		AuthType: req.AuthType,
		Timeout:  6 * time.Second,
	})

	if err != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": false,
			"message": "فشل الاتصال بالجهاز: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "تم الاتصال بنجاح وتوثيق بيانات الاعتماد!",
	})
}

func handleDiscover(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}

	var req struct {
		VendorSlug string `json:"vendor_slug"`
		IP         string `json:"ip"`
		Port       int    `json:"port"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		AuthType   string `json:"auth_type"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request format"})
	}

	port := req.Port
	if port <= 0 {
		port = 8728
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := GlobalService.DiscoverAndProbe(ctx, req.VendorSlug, TargetConfig{
		IP:       req.IP,
		Port:     port,
		Username: req.Username,
		Password: req.Password,
		AuthType: req.AuthType,
		Timeout:  8 * time.Second,
	})

	if err != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": false,
			"message": "فشل فحص واكتشاف الجهاز: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"discovery": result,
	})
}

func handleListDevices(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}

	typeFilter := c.Query("type")
	vendorFilter := c.Query("vendor")
	statusFilter := c.Query("status")

	list, err := GlobalService.repo.ListDevices(typeFilter, vendorFilter, statusFilter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(list)
}

func handleAddDevice(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}

	var req AddDeviceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid JSON body"})
	}

	if req.IP == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "IP address is required"})
	}
	if req.VendorSlug == "" {
		req.VendorSlug = "mikrotik"
	}
	if req.TypeSlug == "" {
		req.TypeSlug = "switch"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dev, err := GlobalService.AddDevice(ctx, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"device":  dev,
	})
}

func handleGetDeviceDetail(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid device ID"})
	}

	detail, err := GlobalService.GetDeviceDetail(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Device not found"})
	}

	return c.JSON(detail)
}

func handleUpdateDevice(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid device ID"})
	}

	existing, err := GlobalService.repo.GetDevice(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Device not found"})
	}

	var req struct {
		Name            string `json:"name"`
		IP              string `json:"ip"`
		Port            int    `json:"port"`
		PollIntervalSec int    `json:"poll_interval_sec"`
		IsMonitored     bool   `json:"is_monitored"`
		Location        string `json:"location"`
		Notes           string `json:"notes"`
		Username        string `json:"username,omitempty"`
		Password        string `json:"password,omitempty"`
		AuthType        string `json:"auth_type,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request format"})
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.IP != "" {
		existing.IP = req.IP
	}
	if req.Port > 0 {
		existing.Port = req.Port
	}
	if req.PollIntervalSec > 0 {
		existing.PollIntervalSec = req.PollIntervalSec
	}
	existing.IsMonitored = req.IsMonitored
	existing.Location = req.Location
	existing.Notes = req.Notes

	if err := GlobalService.repo.UpdateDevice(existing); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if req.Username != "" && req.Password != "" {
		_ = GlobalService.repo.UpdateDeviceCredentials(id, req.Username, req.Password, req.AuthType)
	}

	_ = GlobalService.repo.LogEvent(id, "config_change", "info", "Device configuration updated", "")

	return c.JSON(fiber.Map{
		"success": true,
		"device":  existing,
	})
}

func handleDeleteDevice(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid device ID"})
	}

	if err := GlobalService.repo.DeleteDevice(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Device deleted successfully",
	})
}

func handleTriggerPoll(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid device ID"})
	}

	err = GlobalService.monitor.PollDevice(id)
	if err != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	detail, _ := GlobalService.GetDeviceDetail(id)
	return c.JSON(fiber.Map{
		"success": true,
		"detail":  detail,
	})
}

func handleGetInterfaces(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	ifaces, err := GlobalService.repo.ListDeviceInterfaces(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ifaces)
}

func handleGetMetrics(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	metrics, err := GlobalService.repo.GetMetricsHistory(id, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(metrics)
}

func handleGetClients(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	clients, err := GlobalService.repo.ListWirelessClients(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(clients)
}

func handleGetEvents(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit", "30"))
	events, err := GlobalService.repo.ListEvents(id, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(events)
}

func handleListAlerts(c *fiber.Ctx) error {
	if GlobalService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Devices service not initialized"})
	}
	activeOnly := c.Query("active") != "false"
	alerts, err := GlobalService.repo.ListAlerts(activeOnly)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(alerts)
}
