package ai

import (
	"path/filepath"
	"testing"

	"mikrotik-manager/server/internal/storage"
)

func TestRouterOSToolDefinitions(t *testing.T) {
	tools := GetRouterOSToolDefinitions()
	if len(tools) == 0 {
		t.Fatalf("expected tools to be defined, got 0")
	}

	foundAudit := false
	foundPlan := false
	for _, tool := range tools {
		if tool.Function.Name == "mikrotik_audit_router" {
			foundAudit = true
		}
		if tool.Function.Name == "mikrotik_generate_plan" {
			foundPlan = true
		}
	}

	if !foundAudit {
		t.Errorf("expected mikrotik_audit_router tool to be present")
	}
	if !foundPlan {
		t.Errorf("expected mikrotik_generate_plan tool to be present")
	}
}

func TestAISettingsStorage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-ai.db")

	repo, err := storage.NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to init sqlite repo: %v", err)
	}
	defer repo.Close()

	if err := repo.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	settings, err := repo.GetAISettings()
	if err != nil {
		t.Fatalf("failed to get default ai settings: %v", err)
	}
	if settings.Provider != "deepseek" {
		t.Errorf("expected default provider to be deepseek, got %s", settings.Provider)
	}

	// Update settings
	settings.Provider = "gemini"
	settings.APIKey = "test-key-12345"
	settings.Model = "gemini-2.0-flash"
	settings.Enabled = true

	if err := repo.SaveAISettings(settings); err != nil {
		t.Fatalf("failed to save ai settings: %v", err)
	}

	loaded, err := repo.GetAISettings()
	if err != nil {
		t.Fatalf("failed to load updated ai settings: %v", err)
	}
	if loaded.Provider != "gemini" || loaded.APIKey != "test-key-12345" || loaded.Model != "gemini-2.0-flash" {
		t.Errorf("loaded settings mismatch: %+v", loaded)
	}
}

func TestSecurityAuditLogStorage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-ai-audit.db")

	repo, err := storage.NewSQLiteRepository(dbPath)
	if err != nil {
		t.Fatalf("failed to init sqlite repo: %v", err)
	}
	defer repo.Close()

	if err := repo.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	err = repo.SaveAIAuditLog(storage.AIAuditLog{
		Subdomain:    "agent-baghdad",
		AuditType:    "security",
		Score:        85,
		FindingsJSON: `[{"title":"Open DNS","severity":"critical"}]`,
		RawSummary:   "تم فحص الراوتر بنجاح مع اكتشاف ثغرة DNS مفتوحة.",
	})
	if err != nil {
		t.Fatalf("failed to save audit log: %v", err)
	}

	logs, err := repo.GetAIAuditLogs("agent-baghdad", 10)
	if err != nil {
		t.Fatalf("failed to fetch audit logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(logs))
	}
	if logs[0].Score != 85 || logs[0].Subdomain != "agent-baghdad" {
		t.Errorf("unexpected log data: %+v", logs[0])
	}
}
