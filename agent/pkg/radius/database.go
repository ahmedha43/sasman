package radius

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // CGO-free sqlite driver
)

var DB *sql.DB

func InitDB() {
	var err error

	dbPath := "data/sasman.db"
	if os.Getenv("SQLITE_DB_PATH") != "" {
		dbPath = os.Getenv("SQLITE_DB_PATH")
	} else if _, err := os.Stat("/app/data"); err == nil {
		dbPath = "/app/data/sasman.db"
	}

	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Core radius failure: Failed to open SQLite database: %v", err)
	}

	// Performance optimizations for SQLite on MikroTik (USB storage)
	_, _ = DB.Exec("PRAGMA journal_mode=WAL")
	_, _ = DB.Exec("PRAGMA synchronous=NORMAL")
	_, _ = DB.Exec("PRAGMA cache_size=-64000") // 64MB cache
	_, _ = DB.Exec("PRAGMA wal_autocheckpoint=1000")

	if err = DB.Ping(); err != nil {
		log.Fatalf("Core radius failure: Failed to ping SQLite: %v", err)
	}

	// Limit connection pool to save RAM and prevent DB locks
	DB.SetMaxOpenConns(10)
	DB.SetMaxIdleConns(2)
	DB.SetConnMaxLifetime(5 * time.Minute)

	log.Printf("SASMAN SQLite Engine Initialized at %s", dbPath)

	// Ensure all tables exist
	EnsureSchema()

	// Initialize LMDB for FreeRADIUS High-Speed AAA
	InitLMDB()

	StartSQLiteWALCheckpointWorker()
}

func reloadFreeRADIUS() {
	UpdateNASSecrets()
}

func StartSQLiteWALCheckpointWorker() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			checkpointSQLiteWAL(false)
		}
	}()
}

func checkpointSQLiteWAL(truncate bool) {
	if DB == nil {
		return
	}
	mode := "PASSIVE"
	if truncate {
		mode = "TRUNCATE"
	}
	if _, err := DB.Exec("PRAGMA wal_checkpoint(" + mode + ")"); err != nil {
		log.Printf("[sqlite] WAL checkpoint failed: %v", err)
	}
}
