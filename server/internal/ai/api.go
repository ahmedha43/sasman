package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mikrotik-manager/server/internal/storage"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

type APIHandler struct {
	engine *Engine
	repo   *storage.SQLiteRepository
}

func NewAPIHandler(engine *Engine, repo *storage.SQLiteRepository) *APIHandler {
	return &APIHandler{
		engine: engine,
		repo:   repo,
	}
}

func (h *APIHandler) RegisterRoutes(app *fiber.App) {
	aiGroup := app.Group("/api/ai")

	aiGroup.Get("/status", h.handleStatus)
	aiGroup.Get("/settings", h.handleGetSettings)
	aiGroup.Post("/settings", h.handleSaveSettings)
	aiGroup.Post("/chat", h.handleChat)
	aiGroup.Post("/chat/stream", h.handleChatStream)
	aiGroup.Post("/audit/:subdomain", h.handleAudit)
	aiGroup.Get("/audit/logs", h.handleGetAuditLogs)
	aiGroup.Post("/apply/:subdomain", h.handleApplyPlan)
	aiGroup.Get("/memory/:subdomain", h.handleGetMemory)
	aiGroup.Delete("/memory/:subdomain", h.handleDeleteMemory)
	aiGroup.Patch("/memory/:subdomain/notes", h.handleUpdateMemoryNotes)
	aiGroup.Post("/memory/:subdomain/discover-topology", h.handleDiscoverTopology)
	aiGroup.Put("/memory/:subdomain/topology", h.handleSaveTopologyProfile)
}

func (h *APIHandler) handleStatus(c *fiber.Ctx) error {
	settings, err := h.repo.GetAISettings()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	hasKey := strings.TrimSpace(settings.APIKey) != ""
	mcpOnline := false
	mcpUrl := "http://127.0.0.1:8000"
	if h.engine.mcpBridge != nil {
		mcpOnline = h.engine.mcpBridge.IsAvailable()
		mcpUrl = h.engine.mcpBridge.baseURL
	}

	return c.JSON(fiber.Map{
		"enabled":            settings.Enabled,
		"provider":           settings.Provider,
		"model":              settings.Model,
		"has_api_key":        hasKey,
		"tools_count":        len(GetRouterOSToolDefinitions()),
		"mcp_sidecar_online": mcpOnline,
		"mcp_tools_count":    885,
		"mcp_url":            mcpUrl,
	})
}

func (h *APIHandler) handleGetSettings(c *fiber.Ctx) error {
	settings, err := h.repo.GetAISettings()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	maskedKey := ""
	if len(settings.APIKey) > 8 {
		maskedKey = settings.APIKey[:4] + "••••••••" + settings.APIKey[len(settings.APIKey)-4:]
	} else if len(settings.APIKey) > 0 {
		maskedKey = "••••••••"
	}

	return c.JSON(fiber.Map{
		"id":            settings.ID,
		"provider":      settings.Provider,
		"api_key":       maskedKey,
		"has_api_key":   settings.APIKey != "",
		"model":         settings.Model,
		"base_url":      settings.BaseURL,
		"system_prompt": settings.SystemPrompt,
		"temperature":   settings.Temperature,
		"enabled":       settings.Enabled,
		"updated_at":    settings.UpdatedAt,
	})
}

func (h *APIHandler) handleSaveSettings(c *fiber.Ctx) error {
	var req struct {
		Provider     string  `json:"provider"`
		APIKey       string  `json:"api_key"`
		Model        string  `json:"model"`
		BaseURL      string  `json:"base_url"`
		SystemPrompt string  `json:"system_prompt"`
		Temperature  float64 `json:"temperature"`
		Enabled      bool    `json:"enabled"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "تنسيق البيانات غير صحيح"})
	}

	current, _ := h.repo.GetAISettings()
	if req.APIKey == "" && current != nil {
		req.APIKey = current.APIKey
	}

	if req.Provider == "" {
		req.Provider = "deepseek"
	}
	if req.Model == "" {
		if req.Provider == "deepseek" {
			req.Model = "deepseek-chat"
		} else if req.Provider == "gemini" {
			req.Model = "gemini-2.0-flash"
		} else if req.Provider == "openai" {
			req.Model = "gpt-4o"
		}
	}
	if req.Temperature <= 0 {
		req.Temperature = 0.2
	}

	newSettings := &storage.AISettings{
		ID:           "default",
		Provider:     req.Provider,
		APIKey:       strings.TrimSpace(req.APIKey),
		Model:        req.Model,
		BaseURL:      req.BaseURL,
		SystemPrompt: req.SystemPrompt,
		Temperature:  req.Temperature,
		Enabled:      req.Enabled,
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	if err := h.repo.SaveAISettings(newSettings); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل حفظ الإعدادات: " + err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "تم حفظ إعدادات الذكاء الاصطناعي بنجاح",
	})
}

func (h *APIHandler) handleChat(c *fiber.Ctx) error {
	var req struct {
		Messages []ChatMessage `json:"messages"`
		Subdomain string       `json:"subdomain"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "تنسيق الطلب غير صحيح"})
	}

	if len(req.Messages) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "الرسائل فارغة"})
	}

	ctx := c.Context()
	replyMsg, plan, err := h.engine.Chat(ctx, req.Messages, req.Subdomain)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": replyMsg,
		"plan":    plan,
	})
}

func (h *APIHandler) handleChatStream(c *fiber.Ctx) error {
	var req struct {
		Messages  []ChatMessage `json:"messages"`
		Subdomain string        `json:"subdomain"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "تنسيق الطلب غير صحيح"})
	}

	if len(req.Messages) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "الرسائل فارغة"})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		sendEvent := func(ev StreamEvent) {
			data, err := json.Marshal(ev)
			if err == nil {
				_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
				_ = w.Flush()
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		_, _, _ = h.engine.ChatStream(ctx, req.Messages, req.Subdomain, sendEvent)
	}))

	return nil
}

func (h *APIHandler) handleAudit(c *fiber.Ctx) error {
	subdomain := c.Params("subdomain")
	if subdomain == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم النطاق مطلوب"})
	}

	ctx := c.Context()
	report, err := h.engine.RunSecurityAudit(ctx, subdomain)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"report":  report,
	})
}

func (h *APIHandler) handleGetAuditLogs(c *fiber.Ctx) error {
	subdomain := c.Query("subdomain")
	logs, err := h.repo.GetAIAuditLogs(subdomain, 20)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"logs":    logs,
	})
}

func (h *APIHandler) handleApplyPlan(c *fiber.Ctx) error {
	subdomain := c.Params("subdomain")
	if subdomain == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم النطاق مطلوب"})
	}

	var req struct {
		Commands []string `json:"commands"`
		Title    string   `json:"title"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "تنسيق الأوامر غير صحيح"})
	}

	if len(req.Commands) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "لا توجد أوامر للتطبيق"})
	}

	res, err := h.engine.ExecuteBatchCommands(subdomain, req.Commands)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل تطبيق الأوامر: " + err.Error()})
	}

	// Auto-record applied commands in agent persistent memory
	for _, cmd := range req.Commands {
		_ = h.repo.AppendAppliedCommand(subdomain, cmd, req.Title)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "تم تطبيق الخطة بنجاح على راوتر الوكيل وتسجيلها في الذاكرة",
		"results": res,
	})
}

func (h *APIHandler) handleGetMemory(c *fiber.Ctx) error {
	subdomain := c.Params("subdomain")
	if subdomain == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم النطاق مطلوب"})
	}

	mem, err := h.repo.GetAgentMemory(subdomain)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"memory":  mem,
	})
}

func (h *APIHandler) handleDeleteMemory(c *fiber.Ctx) error {
	subdomain := c.Params("subdomain")
	if subdomain == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم النطاق مطلوب"})
	}

	if err := h.repo.DeleteAgentMemory(subdomain); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("تم مسح ذاكرة الوكيل (%s) بنجاح", subdomain),
	})
}

func (h *APIHandler) handleUpdateMemoryNotes(c *fiber.Ctx) error {
	subdomain := c.Params("subdomain")
	if subdomain == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم النطاق مطلوب"})
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "تنسيق البيانات غير صحيح"})
	}

	if err := h.repo.UpdateAgentNotes(subdomain, req.Notes); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "تم تحديث ملاحظات الوكيل في الذاكرة بنجاح",
	})
}

func (h *APIHandler) handleDiscoverTopology(c *fiber.Ctx) error {
	subdomain := c.Params("subdomain")
	if subdomain == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم النطاق مطلوب"})
	}

	topo, err := h.engine.DiscoverNetworkTopology(subdomain)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل استكشاف هيكلة الشبكة: " + err.Error()})
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"message":  "تم استكشاف وتحديث هيكلة الشبكة وتوزيع الخطوط بالذاكرة بنجاح",
		"topology": topo,
	})
}

func (h *APIHandler) handleSaveTopologyProfile(c *fiber.Ctx) error {
	subdomain := c.Params("subdomain")
	if subdomain == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم النطاق مطلوب"})
	}

	var topo map[string]interface{}
	if err := c.BodyParser(&topo); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "تنسيق البيانات غير صحيح"})
	}

	if err := h.repo.UpdateAgentTopologyProfile(subdomain, topo); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "تم حفظ وتحديث مخطط هيكلة الشبكة بالذاكرة بنجاح",
	})
}
