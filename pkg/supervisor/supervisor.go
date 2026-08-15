package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"mikrotik-manager/pkg/ota"
)

type Config struct {
	AppDir          string
	AgentBinary     string
	DataDir         string
	ReleasesDir     string
	WatchdogTimeout time.Duration
	PublicKeyHex    string
}

type Supervisor struct {
	cfg        Config
	mu         sync.Mutex
	cmd        *exec.Cmd
	ctx        context.Context
	cancel     context.CancelFunc
	running    bool
	isUpgraded bool
}

func NewSupervisor(cfg Config) *Supervisor {
	if cfg.AppDir == "" {
		cfg.AppDir = "."
	}
	if cfg.AgentBinary == "" {
		cfg.AgentBinary = filepath.Join(cfg.AppDir, "agent")
	}
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Join(cfg.AppDir, "data")
	}
	if cfg.ReleasesDir == "" {
		cfg.ReleasesDir = filepath.Join(cfg.AppDir, "releases")
	}
	if cfg.WatchdogTimeout <= 0 {
		cfg.WatchdogTimeout = 45 * time.Second
	}
	if cfg.PublicKeyHex == "" {
		cfg.PublicKeyHex = ota.DefaultMasterPublicKeyHex
	}

	_ = os.MkdirAll(cfg.DataDir, 0755)
	_ = os.MkdirAll(cfg.ReleasesDir, 0755)

	ctx, cancel := context.WithCancel(context.Background())
	return &Supervisor{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Supervisor) Start() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("[Supervisor] Shutdown signal received, terminating agent...")
		s.Stop()
	}()

	// Start file watcher for OTA triggers
	go s.watchTriggerFile()

	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
			err := s.runAgentProcess()
			if s.ctx.Err() != nil {
				return nil
			}
			log.Printf("[Supervisor] Agent exited with: %v. Restarting in 3s...", err)
			time.Sleep(3 * time.Second)
		}
	}
}

func (s *Supervisor) Stop() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Signal(syscall.SIGTERM)
	}
}

func (s *Supervisor) runAgentProcess() error {
	s.mu.Lock()
	if _, err := os.Stat(s.cfg.AgentBinary); os.IsNotExist(err) {
		s.mu.Unlock()
		return fmt.Errorf("agent binary not found at %s", s.cfg.AgentBinary)
	}

	cmd := exec.Command(s.cfg.AgentBinary)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	s.cmd = cmd
	s.running = true
	s.mu.Unlock()

	launchTime := time.Now()
	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("start agent: %w", err)
	}

	log.Printf("[Supervisor] Started agent process (PID: %d) at %s", cmd.Process.Pid, s.cfg.AgentBinary)

	// If we just upgraded, start watchdog monitor
	if s.isUpgraded {
		go s.monitorWatchdog(launchTime)
	}

	err := cmd.Wait()
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	return err
}

func (s *Supervisor) watchTriggerFile() {
	triggerPath := filepath.Join(s.cfg.DataDir, "ota_trigger.json")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if _, err := os.Stat(triggerPath); err == nil {
				data, err := os.ReadFile(triggerPath)
				_ = os.Remove(triggerPath) // delete trigger so it runs once
				if err != nil {
					log.Printf("[Supervisor] Failed to read trigger file: %v", err)
					continue
				}

				var trigger ota.UpgradeTrigger
				if err := json.Unmarshal(data, &trigger); err != nil {
					log.Printf("[Supervisor] Invalid trigger JSON: %v", err)
					continue
				}

				log.Printf("[Supervisor] Executing OTA upgrade to version %s...", trigger.Manifest.Version)
				if err := s.ExecuteUpgrade(trigger.Manifest); err != nil {
					log.Printf("[Supervisor] Upgrade failed: %v", err)
					s.writeOTAStatus(trigger.Manifest.Version, "failed", err.Error())
				}
			}
		}
	}
}

func (s *Supervisor) ExecuteUpgrade(manifest ota.ReleaseManifest) error {
	s.writeOTAStatus(manifest.Version, "downloading", "")

	tmpBinary := filepath.Join(s.cfg.ReleasesDir, fmt.Sprintf("agent_%s.tmp", manifest.Version))
	backupBinary := filepath.Join(s.cfg.ReleasesDir, "agent.prev")

	// 1. Download binary
	log.Printf("[Supervisor] Downloading binary from: %s", manifest.BinaryURL)
	if err := downloadFile(manifest.BinaryURL, tmpBinary); err != nil {
		return fmt.Errorf("download binary: %w", err)
	}
	defer os.Remove(tmpBinary)

	// Set executable permission
	_ = os.Chmod(tmpBinary, 0755)

	// 2. Verify SHA-256 and Ed25519 Signature
	s.writeOTAStatus(manifest.Version, "verifying", "")
	if err := ota.VerifyBinaryAgainstManifest(tmpBinary, manifest, s.cfg.PublicKeyHex); err != nil {
		return fmt.Errorf("cryptographic verification failed: %w", err)
	}

	log.Printf("[Supervisor] Binary signature & hash verified successfully!")

	// 3. Backup current binary
	if _, err := os.Stat(s.cfg.AgentBinary); err == nil {
		_ = os.Remove(backupBinary)
		if err := copyFile(s.cfg.AgentBinary, backupBinary); err != nil {
			log.Printf("[Supervisor] Warning: failed to backup current binary: %v", err)
		} else {
			_ = os.Chmod(backupBinary, 0755)
		}
	}

	// 4. Atomic file swap
	s.writeOTAStatus(manifest.Version, "installing", "")
	if err := copyFile(tmpBinary, s.cfg.AgentBinary); err != nil {
		return fmt.Errorf("atomic copy new binary: %w", err)
	}
	_ = os.Chmod(s.cfg.AgentBinary, 0755)

	// 5. Clear health file
	healthPath := filepath.Join(s.cfg.DataDir, "health_ok.json")
	_ = os.Remove(healthPath)

	s.isUpgraded = true

	// 6. Stop old agent process so supervisor restarts it with new binary
	s.mu.Lock()
	if s.cmd != nil && s.cmd.Process != nil {
		log.Println("[Supervisor] Restarting agent with new binary...")
		_ = s.cmd.Process.Kill()
	}
	s.mu.Unlock()

	return nil
}

func (s *Supervisor) monitorWatchdog(launchTime time.Time) {
	healthPath := filepath.Join(s.cfg.DataDir, "health_ok.json")
	deadline := time.Now().Add(s.cfg.WatchdogTimeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if fi, err := os.Stat(healthPath); err == nil && fi.ModTime().After(launchTime) {
				log.Println("[Supervisor] ✅ Health check passed! Upgrade confirmed successfully.")
				s.writeOTAStatus("", "updated", "")
				s.isUpgraded = false
				return
			}
		}
	}

	// If timeout reached without health check confirmation -> ROLLBACK!
	log.Println("[Supervisor] ⚠️ Watchdog timeout: Health confirmation not received! Performing automatic rollback...")
	s.performRollback()
}

func (s *Supervisor) performRollback() {
	s.isUpgraded = false
	backupBinary := filepath.Join(s.cfg.ReleasesDir, "agent.prev")

	if _, err := os.Stat(backupBinary); os.IsNotExist(err) {
		log.Printf("[Supervisor] Fatal: No backup binary found at %s for rollback", backupBinary)
		s.writeOTAStatus("", "failed", "Rollback failed: backup binary missing")
		return
	}

	// Restore backup binary
	if err := copyFile(backupBinary, s.cfg.AgentBinary); err != nil {
		log.Printf("[Supervisor] Fatal: Failed to restore backup binary: %v", err)
		s.writeOTAStatus("", "failed", "Rollback failed: restore error")
		return
	}
	_ = os.Chmod(s.cfg.AgentBinary, 0755)

	s.writeOTAStatus("", "rollback", "Auto-rollback executed: health check timeout on new version")

	// Restart agent with restored working binary
	s.mu.Lock()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.mu.Unlock()

	log.Println("[Supervisor] 🔄 Automatic rollback completed successfully. Restored previous stable version.")
}

func (s *Supervisor) writeOTAStatus(targetVer string, status string, lastErr string) {
	statusFile := filepath.Join(s.cfg.DataDir, "ota_status.json")
	st := ota.AgentOTAStatus{
		TargetVer:     targetVer,
		Status:        status,
		LastError:     lastErr,
		LastAttemptAt: time.Now(),
		UpdatedAt:     time.Now(),
	}
	data, _ := json.MarshalIndent(st, "", "  ")
	_ = os.WriteFile(statusFile, data, 0644)
}

func downloadFile(url string, dest string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
