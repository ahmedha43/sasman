package ota

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"mikrotik-manager/pkg/ota"
	"mikrotik-manager/pkg/tunnel"
	"mikrotik-manager/server/internal/storage"
)

type Manager struct {
	repo       *storage.SQLiteRepository
	tunnel     *tunnel.Service
	privKey    ed25519.PrivateKey
	pubKeyHex  string
	mu         sync.Mutex
	rollouts   map[string]*CanaryRolloutSession
}

type CanaryRolloutSession struct {
	ID        string
	Config    ota.CanaryRolloutConfig
	Manifest  ota.ReleaseManifest
	Status    string // "running", "paused", "completed", "stopped"
	CurrentStep int
	TotalCount  int
	SuccessCount int
	FailedCount  int
	RollbackCount int
	PendingCount int
	StartedAt time.Time
	Cancel    context.CancelFunc
}

func NewManager(repo *storage.SQLiteRepository, tunnelSvc *tunnel.Service) *Manager {
	// Check or generate signing key
	privHex := os.Getenv("SASMAN_OTA_PRIVKEY")
	var privKey ed25519.PrivateKey
	var pubKeyHex string

	if privHex != "" {
		bytes, err := hex.DecodeString(privHex)
		if err == nil && len(bytes) == ed25519.PrivateKeySize {
			privKey = ed25519.PrivateKey(bytes)
			pubKeyHex = hex.EncodeToString(privKey.Public().(ed25519.PublicKey))
		}
	}

	if privKey == nil {
		pub, priv, err := ota.GenerateKeyPair()
		if err == nil {
			privKey = priv
			pubKeyHex = hex.EncodeToString(pub)
		} else {
			pubKeyHex = ota.DefaultMasterPublicKeyHex
		}
	}

	return &Manager{
		repo:      repo,
		tunnel:    tunnelSvc,
		privKey:   privKey,
		pubKeyHex: pubKeyHex,
		rollouts:  make(map[string]*CanaryRolloutSession),
	}
}

func (m *Manager) GetPublicKeyHex() string {
	return m.pubKeyHex
}

func (m *Manager) PublishRelease(version, channel, targetArch, releaseNotes string, binaryData []byte) (*ota.ReleaseManifest, error) {
	if version == "" || targetArch == "" || len(binaryData) == 0 {
		return nil, fmt.Errorf("version, target_arch, and binary data are required")
	}

	shaHex := ota.CalculateSHA256(binaryData)
	var sigHex string
	if m.privKey != nil {
		var err error
		sigHex, err = ota.SignSHA256(m.privKey, shaHex)
		if err != nil {
			return nil, fmt.Errorf("sign release sha256: %w", err)
		}
	}

	manifest := ota.ReleaseManifest{
		Version:          version,
		Channel:          channel,
		TargetArch:       targetArch,
		Sha256:           shaHex,
		SignatureEd25519: sigHex,
		MinAgentVersion:  "5.0.0",
		ReleaseNotes:     releaseNotes,
		CreatedAt:        time.Now().UTC(),
	}

	if err := m.repo.SaveRelease(manifest, binaryData); err != nil {
		return nil, fmt.Errorf("save release to db: %w", err)
	}

	log.Printf("[OTA Manager] Successfully published release %s (%s) with SHA256 %s", version, targetArch, shaHex)
	return &manifest, nil
}

func (m *Manager) TriggerSingleUpgrade(subdomain string, version string, targetArch string) error {
	manifest, _, err := m.repo.GetRelease(version, targetArch)
	if err != nil {
		return fmt.Errorf("release %s (%s) not found: %w", version, targetArch, err)
	}

	manifestCopy := *manifest
	if !strings.HasPrefix(manifestCopy.BinaryURL, "http://") && !strings.HasPrefix(manifestCopy.BinaryURL, "https://") {
		centralHost := os.Getenv("SASMAN_CENTRAL_DOMAIN")
		if centralHost == "" {
			centralHost = "sas-man.net"
		}
		manifestCopy.BinaryURL = fmt.Sprintf("https://%s/%s", centralHost, strings.TrimPrefix(manifestCopy.BinaryURL, "/"))
	}

	trigger := ota.UpgradeTrigger{
		Manifest:    manifestCopy,
		TriggerID:   fmt.Sprintf("trig_%d", time.Now().UnixNano()),
		RequestedAt: time.Now().UTC(),
	}

	_ = m.repo.UpdateAgentOTAStatus(ota.AgentOTAStatus{
		Subdomain:     subdomain,
		TargetVer:     version,
		Arch:          targetArch,
		Status:        "pending",
		LastAttemptAt: time.Now().UTC(),
	})

	// Dispatch message over persistent WebSocket tunnel
	return m.tunnel.SendTunnelMessage(subdomain, "cmd_ota_upgrade", trigger)
}

func (m *Manager) StartCanaryRollout(cfg ota.CanaryRolloutConfig) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rolloutID := fmt.Sprintf("rollout_%d", time.Now().Unix())
	ctx, cancel := context.WithCancel(context.Background())

	session := &CanaryRolloutSession{
		ID:        rolloutID,
		Config:    cfg,
		Status:    "running",
		StartedAt: time.Now().UTC(),
		Cancel:    cancel,
	}

	m.rollouts[rolloutID] = session
	go m.runRolloutPipeline(ctx, session)

	return rolloutID, nil
}

func (m *Manager) runRolloutPipeline(ctx context.Context, s *CanaryRolloutSession) {
	log.Printf("[Canary Rollout %s] Starting rollout of version %s...", s.ID, s.Config.ReleaseVersion)

	subdomains, err := m.repo.ListAllSubdomains()
	if err != nil {
		s.Status = "stopped"
		log.Printf("[Canary Rollout %s] Failed to get subdomains: %v", s.ID, err)
		return
	}

	// Filter targets
	var targets []string
	for _, sub := range subdomains {
		if s.Config.TargetScope == "group" && s.Config.TargetGroup != "" {
			if grp := m.repo.GetSubdomainGroup(sub); grp != s.Config.TargetGroup {
				continue
			}
		}
		targets = append(targets, sub)
	}

	s.TotalCount = len(targets)
	s.PendingCount = len(targets)

	steps := s.Config.Steps
	if len(steps) == 0 {
		steps = []int{1, 5, 25, 100, 0}
	}

	currentIndex := 0
	interval := time.Duration(s.Config.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 15 * time.Second
	}

	for stepIdx, batchSize := range steps {
		select {
		case <-ctx.Done():
			s.Status = "stopped"
			return
		default:
		}

		s.CurrentStep = stepIdx + 1
		count := batchSize
		if count == 0 || (currentIndex+count) > len(targets) {
			count = len(targets) - currentIndex
		}
		if count <= 0 {
			break
		}

		batch := targets[currentIndex : currentIndex+count]
		currentIndex += count

		log.Printf("[Canary Rollout %s] Dispatching Step %d (%d agents)...", s.ID, s.CurrentStep, len(batch))

		for _, sub := range batch {
			// Get agent arch
			arch, _ := m.repo.GetSubdomainArch(sub)
			if arch == "" {
				arch = "linux_arm64"
			}
			_ = m.TriggerSingleUpgrade(sub, s.Config.ReleaseVersion, arch)
		}

		// Wait interval and observe health
		time.Sleep(interval)

		// Check failure rate
		statuses, _ := m.repo.GetAgentOTAStatuses()
		failed := 0
		success := 0
		for _, sub := range targets[:currentIndex] {
			st := statuses[sub]
			if st.Status == "failed" || st.Status == "rollback" {
				failed++
			} else if st.Status == "updated" {
				success++
			}
		}

		s.SuccessCount = success
		s.FailedCount = failed
		s.PendingCount = s.TotalCount - success - failed

		if currentIndex > 0 && s.Config.AutoStopOnErrors {
			failureRate := float64(failed) / float64(currentIndex)
			if failureRate > s.Config.MaxFailureRate && s.Config.MaxFailureRate > 0 {
				log.Printf("[Canary Rollout %s] ⚠️ Rollout HALTED! Failure rate %.1f%% exceeded threshold %.1f%%", s.ID, failureRate*100, s.Config.MaxFailureRate*100)
				s.Status = "paused"
				return
			}
		}
	}

	s.Status = "completed"
	log.Printf("[Canary Rollout %s] 🎉 Rollout pipeline finished successfully!", s.ID)
}

func (m *Manager) GetRolloutSessions() []*CanaryRolloutSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []*CanaryRolloutSession
	for _, s := range m.rollouts {
		list = append(list, s)
	}
	return list
}
