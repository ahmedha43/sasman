package main

import (
	"log"
	"os"

	"mikrotik-manager/agent/pkg/supervisor"
)

func main() {
	appDir := os.Getenv("SASMAN_APP_DIR")
	if appDir == "" {
		appDir = "/app"
	}

	cfg := supervisor.Config{
		AppDir:          appDir,
		AgentBinary:     os.Getenv("SASMAN_AGENT_BIN"),
		DataDir:         os.Getenv("SASMAN_DATA_DIR"),
		ReleasesDir:     os.Getenv("SASMAN_RELEASES_DIR"),
		PublicKeyHex:    os.Getenv("SASMAN_OTA_PUBKEY"),
	}

	log.Printf("[Supervisor] Starting SASMAN Supervisor Daemon in %s...", appDir)
	sup := supervisor.NewSupervisor(cfg)

	if err := sup.Start(); err != nil {
		log.Fatalf("[Supervisor] Fatal error: %v", err)
	}
}
