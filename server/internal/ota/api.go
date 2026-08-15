package ota

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"mikrotik-manager/pkg/ota"
	"mikrotik-manager/server/internal/storage"

	"github.com/gofiber/fiber/v2"
)

type APIHandler struct {
	manager *Manager
	repo    *storage.SQLiteRepository
}

func NewAPIHandler(manager *Manager, repo *storage.SQLiteRepository) *APIHandler {
	return &APIHandler{
		manager: manager,
		repo:    repo,
	}
}

func (h *APIHandler) RegisterRoutes(router fiber.Router) {
	group := router.Group("/api/ota")

	// Public endpoints for Agents to download manifests & binaries
	group.Get("/manifest/:arch/:version", h.handleGetManifest)
	group.Get("/bin/:arch/:version", h.handleDownloadBinary)
	group.Post("/report-status", h.handleReportStatus)

	// Admin-managed endpoints
	group.Get("/releases", h.handleListReleases)
	group.Post("/releases", h.handlePublishRelease)
	group.Delete("/releases/:version/:arch", h.handleDeleteRelease)
	group.Delete("/releases/:version", h.handleDeleteRelease)
	group.Post("/pull-docker", h.handlePullDockerRelease)
	group.Get("/status", h.handleGetStatuses)
	group.Post("/trigger/:subdomain", h.handleTriggerSingle)
	group.Post("/rollout", h.handleStartRollout)
	group.Get("/rollouts", h.handleGetRollouts)
}

func (h *APIHandler) handleGetManifest(c *fiber.Ctx) error {
	arch := c.Params("arch")
	version := c.Params("version")

	manifest, _, err := h.repo.GetRelease(version, arch)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("Release %s for %s not found", version, arch),
		})
	}

	return c.JSON(manifest)
}

func (h *APIHandler) handleDownloadBinary(c *fiber.Ctx) error {
	arch := c.Params("arch")
	version := c.Params("version")

	manifest, binaryData, err := h.repo.GetRelease(version, arch)
	if err != nil || len(binaryData) == 0 {
		return c.Status(fiber.StatusNotFound).SendString("Binary not found")
	}

	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=sasman-agent-%s-%s", arch, version))
	c.Set("X-Checksum-SHA256", manifest.Sha256)
	c.Set("X-Signature-Ed25519", manifest.SignatureEd25519)

	return c.Send(binaryData)
}

func (h *APIHandler) handlePublishRelease(c *fiber.Ctx) error {
	version := c.FormValue("version")
	channel := c.FormValue("channel")
	targetArch := c.FormValue("target_arch")
	releaseNotes := c.FormValue("release_notes")

	if channel == "" {
		channel = "stable"
	}
	if targetArch == "" {
		targetArch = "linux_arm64"
	}

	file, err := c.FormFile("binary")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Binary file is required",
		})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer f.Close()

	binaryData, err := io.ReadAll(f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	manifest, err := h.manager.PublishRelease(version, channel, targetArch, releaseNotes, binaryData)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"manifest": manifest,
	})
}

func (h *APIHandler) handleListReleases(c *fiber.Ctx) error {
	releases, err := h.repo.ListReleases()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"success":  true,
		"releases": releases,
	})
}

func (h *APIHandler) handleGetStatuses(c *fiber.Ctx) error {
	statuses, err := h.repo.GetAgentOTAStatuses()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"success":  true,
		"statuses": statuses,
	})
}

func (h *APIHandler) handleTriggerSingle(c *fiber.Ctx) error {
	subdomain := c.Params("subdomain")
	var req struct {
		Version    string `json:"version"`
		TargetArch string `json:"target_arch"`
	}
	if err := c.BodyParser(&req); err != nil || req.Version == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "version is required"})
	}
	if req.TargetArch == "" {
		arch, _ := h.repo.GetSubdomainArch(subdomain)
		req.TargetArch = arch
	}

	if err := h.manager.TriggerSingleUpgrade(subdomain, req.Version, req.TargetArch); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Upgrade command sent to %s for version %s (%s)", subdomain, req.Version, req.TargetArch),
	})
}

func (h *APIHandler) handleStartRollout(c *fiber.Ctx) error {
	var cfg ota.CanaryRolloutConfig
	if err := c.BodyParser(&cfg); err != nil || cfg.ReleaseVersion == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "release_version is required"})
	}

	rolloutID, err := h.manager.StartCanaryRollout(cfg)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"rollout_id": rolloutID,
	})
}

func (h *APIHandler) handleGetRollouts(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"success":  true,
		"rollouts": h.manager.GetRolloutSessions(),
	})
}

func (h *APIHandler) handleReportStatus(c *fiber.Ctx) error {
	var st ota.AgentOTAStatus
	if err := c.BodyParser(&st); err != nil || st.Subdomain == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid status report"})
	}

	if err := h.repo.UpdateAgentOTAStatus(st); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *APIHandler) handlePullDockerRelease(c *fiber.Ctx) error {
	var req struct {
		Image        string `json:"image"`
		Version      string `json:"version"`
		Channel      string `json:"channel"`
		ReleaseNotes string `json:"release_notes"`
	}
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Image) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "docker image is required (e.g. ahmedkin99/sasman-manager:v5)",
		})
	}

	summary, err := h.manager.PullAndPublishFromDocker(c.Context(), req.Image, req.Version, req.Channel, req.ReleaseNotes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   err.Error(),
			"summary": summary,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Successfully processed %d architectures from Docker Hub!", summary.TotalPulled),
		"summary": summary,
	})
}

func (h *APIHandler) handleDeleteRelease(c *fiber.Ctx) error {
	version := c.Params("version")
	arch := c.Params("arch")

	if err := h.repo.DeleteRelease(version, arch); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Also delete matching .tar and binary files on disk
	filenames := []string{
		"sasman-arm64.tar", "sasman-armv7.tar", "sasman-amd64.tar",
		fmt.Sprintf("sasman-%s-%s.tar", arch, version),
		fmt.Sprintf("sasman-agent-%s-%s", arch, version),
	}
	for _, fn := range filenames {
		for _, dir := range []string{"data/releases", "data", "/app/data/releases", "/app/data"} {
			p := filepath.Join(dir, fn)
			_ = os.Remove(p)
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("تم حذف الإصدار %s بنجاح", version),
	})
}

