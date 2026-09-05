package cloudtenant

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PruningReport struct {
	ExecutedAt         time.Time        `json:"executed_at"`
	TenantsProcessed   int              `json:"tenants_processed"`
	RadacctDeleted     int64            `json:"radacct_deleted"`
	RadpostauthDeleted int64            `json:"radpostauth_deleted"`
	CentralPruned      map[string]int64 `json:"central_pruned"`
	DurationMs         int64            `json:"duration_ms"`
	Errors             []string         `json:"errors,omitempty"`
}

var (
	lastPruningReport *PruningReport
	lastPruningMu     sync.RWMutex
	isPruningRunning  bool
	pruningRunningMu  sync.Mutex
)

// GetLastPruningReport returns the most recent pruning execution metrics
func GetLastPruningReport() *PruningReport {
	lastPruningMu.RLock()
	defer lastPruningMu.RUnlock()
	return lastPruningReport
}

// StartPruningSweeper runs the background sweeper every 24 hours
func (m *Manager) StartPruningSweeper() {
	go func() {
		// Wait 2 minutes after server startup before running the initial prune
		time.Sleep(2 * time.Minute)
		log.Println("[cloud-prune] 🧹 Initial cloud pruning check starting...")
		if _, err := m.RunPruning(); err != nil {
			log.Printf("[cloud-prune] ⚠️ Initial prune completed with warnings: %v", err)
		}

		// Periodic daily ticker (runs every 24 hours)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			log.Println("[cloud-prune] 🧹 Running daily scheduled cloud pruning...")
			if _, err := m.RunPruning(); err != nil {
				log.Printf("[cloud-prune] ⚠️ Scheduled prune completed with warnings: %v", err)
			}
		}
	}()
}

// RunPruning executes the pruning process across all tenant databases and central data
func (m *Manager) RunPruning() (*PruningReport, error) {
	pruningRunningMu.Lock()
	if isPruningRunning {
		pruningRunningMu.Unlock()
		return nil, fmt.Errorf("pruning job is already running")
	}
	isPruningRunning = true
	pruningRunningMu.Unlock()

	defer func() {
		pruningRunningMu.Lock()
		isPruningRunning = false
		pruningRunningMu.Unlock()
	}()

	start := time.Now()
	report := &PruningReport{
		ExecutedAt:    start,
		CentralPruned: make(map[string]int64),
		Errors:        make([]string, 0),
	}

	// 1. Collect all tenant subdomains to prune
	subdomainSet := make(map[string]bool)

	// A) From repository database
	if m.repo != nil {
		if subs, err := m.repo.ListAllSubdomains(); err == nil {
			for _, s := range subs {
				s = strings.ToLower(strings.TrimSpace(s))
				if s != "" {
					subdomainSet[s] = true
				}
			}
		}
	}

	// B) From disk directories under baseDir (catches any subdomains present on filesystem)
	if m.pool != nil && m.pool.baseDir != "" {
		if entries, err := os.ReadDir(m.pool.baseDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					name := strings.ToLower(strings.TrimSpace(entry.Name()))
					dbFile := filepath.Join(m.pool.baseDir, name, "radius.db")
					if _, errStat := os.Stat(dbFile); errStat == nil {
						subdomainSet[name] = true
					}
				}
			}
		}
	}

	log.Printf("[cloud-prune] Found %d cloud tenant databases to prune", len(subdomainSet))

	// 2. Prune each tenant database
	for sub := range subdomainSet {
		db, err := m.pool.Get(sub)
		if err != nil || db == nil {
			report.Errors = append(report.Errors, fmt.Sprintf("tenant [%s] open failed: %v", sub, err))
			continue
		}

		// Prune closed sessions older than 7 days, or abandoned open sessions older than 14 days
		if res, err := db.Exec(`
			DELETE FROM radacct 
			WHERE acctstarttime < datetime('now', '-7 days')
			  AND (acctstoptime IS NOT NULL OR acctstarttime < datetime('now', '-14 days'))
		`); err == nil {
			if n, _ := res.RowsAffected(); n > 0 {
				report.RadacctDeleted += n
			}
		} else {
			report.Errors = append(report.Errors, fmt.Sprintf("tenant [%s] radacct prune error: %v", sub, err))
		}

		// Prune authentication log attempts older than 3 days
		if res, err := db.Exec(`
			DELETE FROM radpostauth 
			WHERE authdate < datetime('now', '-3 days')
		`); err == nil {
			if n, _ := res.RowsAffected(); n > 0 {
				report.RadpostauthDeleted += n
			}
		} else {
			report.Errors = append(report.Errors, fmt.Sprintf("tenant [%s] radpostauth prune error: %v", sub, err))
		}

		// Compress and reclaim disk space
		_, _ = db.Exec("VACUUM")
		report.TenantsProcessed++
	}

	// 3. Prune central database records
	if m.repo != nil {
		centralCounts, err := m.repo.PruneCentralData()
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("central data prune error: %v", err))
		} else {
			report.CentralPruned = centralCounts
		}
	}

	report.DurationMs = time.Since(start).Milliseconds()

	// Update last report cache
	lastPruningMu.Lock()
	lastPruningReport = report
	lastPruningMu.Unlock()

	log.Printf("[cloud-prune] ✅ Pruning completed in %dms: %d tenants processed, %d radacct deleted, %d radpostauth deleted",
		report.DurationMs, report.TenantsProcessed, report.RadacctDeleted, report.RadpostauthDeleted)

	return report, nil
}
