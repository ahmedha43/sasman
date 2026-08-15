package broadcast

import (
	"os"
	"testing"
)

func TestBroadcastStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "broadcast_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	InitStore(tempDir)

	msg := BroadcastMessage{
		ID:                "test-bc-1",
		Title:             "تنبيه هام",
		Message:           "هذا إشعار تجريبي لاختبار النظام",
		DisplayType:       "banner",
		TargetType:        "agents",
		TargetProfiles:    "ALL",
		Frequency:         "once",
		SplashDurationSec: 10,
	}

	StoreActiveBroadcast(msg)

	active := GetActiveBroadcasts()
	if len(active) != 1 {
		t.Fatalf("expected 1 active broadcast, got %d", len(active))
	}

	if active[0].ID != "test-bc-1" {
		t.Errorf("expected ID 'test-bc-1', got '%s'", active[0].ID)
	}
	if active[0].Title != "تنبيه هام" {
		t.Errorf("expected Title 'تنبيه هام', got '%s'", active[0].Title)
	}

	// Test Dismiss
	DismissBroadcast("test-bc-1")
	activeAfter := GetActiveBroadcasts()
	if len(activeAfter) != 0 {
		t.Errorf("expected 0 active broadcasts after dismiss, got %d", len(activeAfter))
	}
}
