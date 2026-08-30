package cloudtenant

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type tenantConn struct {
	db       *sql.DB
	lastUsed time.Time
}

type TenantDBPool struct {
	mu      sync.RWMutex
	conns   map[string]*tenantConn
	baseDir string
}

func NewTenantDBPool(baseDir string) *TenantDBPool {
	if baseDir == "" {
		if _, err := os.Stat("/app/data"); err == nil {
			baseDir = "/app/data/tenants"
		} else {
			baseDir = "data/tenants"
		}
	}
	_ = os.MkdirAll(baseDir, 0755)

	pool := &TenantDBPool{
		conns:   make(map[string]*tenantConn),
		baseDir: baseDir,
	}

	// Periodic idle cleanup worker
	go pool.startIdleCleanupWorker(10*time.Minute, 30*time.Minute)

	return pool
}

func (p *TenantDBPool) GetTenantDir(subdomain string) string {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	return filepath.Join(p.baseDir, subdomain)
}

func (p *TenantDBPool) GetTenantDBPath(subdomain string) string {
	return filepath.Join(p.GetTenantDir(subdomain), "radius.db")
}

func (p *TenantDBPool) Get(subdomain string) (*sql.DB, error) {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	if subdomain == "" {
		return nil, fmt.Errorf("empty subdomain")
	}

	p.mu.RLock()
	c, exists := p.conns[subdomain]
	if exists && c.db != nil {
		if err := c.db.Ping(); err == nil {
			c.lastUsed = time.Now()
			p.mu.RUnlock()
			return c.db, nil
		}
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if c, exists := p.conns[subdomain]; exists && c.db != nil {
		if err := c.db.Ping(); err == nil {
			c.lastUsed = time.Now()
			return c.db, nil
		}
		_ = c.db.Close()
		delete(p.conns, subdomain)
	}

	dbPath := p.GetTenantDBPath(subdomain)
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tenant sqlite database: %w", err)
	}

	// Performance Pragmas for multi-tenant SQLite
	_, _ = db.Exec("PRAGMA journal_mode=WAL")
	_, _ = db.Exec("PRAGMA synchronous=NORMAL")
	_, _ = db.Exec("PRAGMA busy_timeout=5000")
	_, _ = db.Exec("PRAGMA cache_size=-16000") // 16MB per active tenant
	_, _ = db.Exec("PRAGMA foreign_keys=ON")

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping tenant sqlite database: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(0)

	// Ensure Schema is initialized
	if err := EnsureTenantSchema(db); err != nil {
		log.Printf("[cloudtenant] Warning during schema init for [%s]: %v", subdomain, err)
	}

	p.conns[subdomain] = &tenantConn{
		db:       db,
		lastUsed: time.Now(),
	}

	return db, nil
}

func (p *TenantDBPool) Close(subdomain string) {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	p.mu.Lock()
	defer p.mu.Unlock()

	if c, exists := p.conns[subdomain]; exists {
		if c.db != nil {
			_ = c.db.Close()
		}
		delete(p.conns, subdomain)
	}
}

func (p *TenantDBPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for sub, c := range p.conns {
		if c.db != nil {
			_ = c.db.Close()
		}
		delete(p.conns, sub)
	}
}

func (p *TenantDBPool) startIdleCleanupWorker(interval, maxIdle time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		now := time.Now()
		p.mu.Lock()
		for sub, c := range p.conns {
			if now.Sub(c.lastUsed) > maxIdle {
				log.Printf("[cloudtenant] Closing idle connection for tenant [%s]", sub)
				if c.db != nil {
					_ = c.db.Close()
				}
				delete(p.conns, sub)
			}
		}
		p.mu.Unlock()
	}
}
