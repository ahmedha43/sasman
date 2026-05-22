//go:build windows

package radius

import (
	"log"

	"layeh.com/radius/rfc2866"
)

// Stub implementations for Windows development environment.
// The real LMDB implementation lives in lmdb_sync.go (Linux-only, CGO required).
// These are no-ops that allow `go vet` and `go build` to pass on Windows.

func InitLMDB() {
	log.Println("[lmdb] LMDB is disabled on Windows (CGO/Linux only). Running in stub mode.")
}

func QueueUserSync(username string) {
	log.Printf("[lmdb-stub] QueueUserSync skipped for [%s] on Windows", username)
}

func SyncAllUsers() {
	log.Println("[lmdb-stub] SyncAllUsers skipped on Windows")
}

func SyncUsersInGroup(groupName string) {
	log.Printf("[lmdb-stub] SyncUsersInGroup skipped for group [%s] on Windows", groupName)
}

func GetLMDBSessions() (map[string]string, error) {
	return map[string]string{}, nil
}

func clearLMDBSessions() error {
	return nil
}

// LMDBBandwidth stub — mirrors the Linux struct for Windows compilation.
type LMDBBandwidth struct {
	InputOctets    int64
	OutputOctets   int64
	StartTime      int64
	SessionSeconds int64
	SessionID      string
	IP             string
	CallingStation string
}


func GetLMDBBandwidth() (map[string]LMDBBandwidth, error) {
	return map[string]LMDBBandwidth{}, nil
}

func fetchUserFromLMDB(username string) (string, error) {
	return "", nil
}

func saveAccountingToLMDB(username string, status rfc2866.AcctStatusType, sid, ip, cli string, in, out uint64, secs int64) {
}
