package ota

import "time"

// ReleaseManifest defines a signed software release for SASMAN Agents
type ReleaseManifest struct {
	Version          string    `json:"version"`            // e.g. "5.1.0"
	Channel          string    `json:"channel"`            // e.g. "stable", "beta"
	TargetArch       string    `json:"target_arch"`        // e.g. "linux_arm64", "linux_arm", "linux_amd64"
	BinaryURL        string    `json:"binary_url"`         // e.g. "/api/ota/bin/linux_arm64/5.1.0"
	Sha256           string    `json:"sha256"`             // SHA-256 hash of binary
	SignatureEd25519 string    `json:"signature_ed25519"`  // Ed25519 cryptographic signature of sha256
	MinAgentVersion  string    `json:"min_agent_version"`  // e.g. "5.0.0"
	ReleaseNotes     string    `json:"release_notes"`      // Changelog / description
	CreatedAt        time.Time `json:"created_at"`
}

// UpgradeTrigger is the payload sent from Server to Agent or written locally for Supervisor
type UpgradeTrigger struct {
	Manifest     ReleaseManifest `json:"manifest"`
	TriggerID    string          `json:"trigger_id"`
	RequestedAt  time.Time       `json:"requested_at"`
	RolloutGroup string          `json:"rollout_group,omitempty"`
}

// AgentOTAStatus represents the current state of an Agent's OTA lifecycle
type AgentOTAStatus struct {
	Subdomain     string    `json:"subdomain"`
	CurrentVer    string    `json:"current_version"`
	TargetVer     string    `json:"target_version"`
	Arch          string    `json:"arch"`
	Status        string    `json:"status"` // "idle", "pending", "downloading", "verifying", "updated", "failed", "rollback"
	LastError     string    `json:"last_error,omitempty"`
	LastAttemptAt time.Time `json:"last_attempt_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CanaryRolloutConfig defines parameters for a staged rolling rollout
type CanaryRolloutConfig struct {
	ReleaseVersion   string   `json:"release_version"`
	TargetScope      string   `json:"target_scope"` // "all", "group", "selected"
	TargetGroup      string   `json:"target_group,omitempty"`
	TargetAgents     []string `json:"target_agents,omitempty"`
	Steps            []int    `json:"steps"` // e.g. [1, 5, 25, 100, 0] (0 means all remaining)
	IntervalSeconds  int      `json:"interval_seconds"`
	MaxFailureRate   float64  `json:"max_failure_rate"` // e.g. 0.05 (5%)
	AutoStopOnErrors bool     `json:"auto_stop_on_errors"`
}
