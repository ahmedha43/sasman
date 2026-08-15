package backup

import (
	"log"
	"mikrotik-manager/pkg/tunnel"
	"os"
	"path/filepath"
	"time"
)

type Scheduler struct {
	svc       *tunnel.Service
	backupDir string
}

func NewScheduler(svc *tunnel.Service) *Scheduler {
	EnsureBackupDir()
	return &Scheduler{svc: svc, backupDir: GetBackupDir()}
}

func (s *Scheduler) Start() {
	go s.run()
}

func (s *Scheduler) run() {
	for {
		now := time.Now()
		next := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
		wait := time.Until(next)

		log.Printf("[Backup] Next scheduled backup in %v at %v", wait, next)
		time.Sleep(wait)

		s.RunDailyBackup()
	}
}

func (s *Scheduler) RunDailyBackup() {
	log.Println("[Backup] Running daily backup for all agents...")

	agents := s.svc.ListAgents()
	for _, agent := range agents {
		subdomain := agent["subdomain"].(string)
		if subdomain == "" {
			continue
		}

		log.Printf("[Backup] Requesting backup for agent: %s", subdomain)
		data, filename, err := s.svc.RequestBackup(subdomain)
		if err != nil {
			log.Printf("[Backup] Failed for %s: %v", subdomain, err)
			continue
		}

		backupPath := filepath.Join(s.backupDir, subdomain+".db")
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			log.Printf("[Backup] Failed to save %s: %v", subdomain, err)
			continue
		}

		log.Printf("[Backup] Saved backup for %s (%d bytes, filename: %s)", subdomain, len(data), filename)
	}
}
