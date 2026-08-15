package backup

import (
	"os"
	"path/filepath"
)

func GetBackupDir() string {
	return filepath.Join("data", "agent_backups")
}

func EnsureBackupDir() {
	os.MkdirAll(GetBackupDir(), 0755)
}

func GetBackupPath(subdomain string) string {
	return filepath.Join(GetBackupDir(), subdomain+".db")
}
