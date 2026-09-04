//go:build !windows

package radius

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PowerDNS/lmdb-go/lmdb"
)

var (
	lmdbEnv   *lmdb.Env
	syncQueue = make(chan string, 1000) // Buffer for 1000 sync tasks
)

func InitLMDB() {
	var err error
	lmdbEnv, err = lmdb.NewEnv()
	if err != nil {
		log.Fatalf("Failed to create LMDB env: %v", err)
	}

	dbDir := "/app/data/radius_db"
	_ = os.MkdirAll(dbDir, 0755)

	err = lmdbEnv.SetMaxDBs(10)
	if err != nil {
		log.Fatalf("Failed to set max DBs: %v", err)
	}

	err = lmdbEnv.SetMapSize(100 * 1024 * 1024) // 100MB
	if err != nil {
		log.Fatalf("Failed to set map size: %v", err)
	}

	// Important: Try standard flags first, fallback to lock cleanup and NoLock if kernel restricts mutexes
	err = lmdbEnv.Open(dbDir, 0, 0644)
	if err != nil {
		log.Printf("[lmdb] Standard Open failed (%v). Attempting lock cleanup and retry...", err)
		_ = os.Remove(dbDir + "/lock.mdb")
		_ = os.Remove(dbDir + "/data.mdb.lock")

		err = lmdbEnv.Open(dbDir, 0, 0644)
		if err != nil {
			log.Printf("[lmdb] Retrying with NoLock mode for single-process RouterOS/ARM64 kernel compatibility...")
			err = lmdbEnv.Open(dbDir, lmdb.NoLock, 0644)
			if err != nil {
				log.Fatalf("Failed to open LMDB after all fallbacks: %v", err)
			}
		}
	}

	log.Printf("LMDB Engine Initialized at %s", dbDir)

	// Clear stale sessions from previous container run.
	// Sessions in LMDB persist on disk (Docker volume), but become invalid after restart.
	// Clearing them prevents phantom "online" status and Simultaneous-Use false-rejections.
	if err := clearLMDBSessions(); err != nil {
		log.Printf("[lmdb] WARNING: Failed to clear stale sessions on startup: %v", err)
	} else {
		log.Println("[lmdb] Stale sessions cleared on startup.")
	}

	// Sync all users on startup to ensure consistency
	go func() {
		time.Sleep(2 * time.Second) // Wait for DB to settle
		SyncAllUsers()
	}()

	// Start Background Sync Worker
	go startSyncWorker()
}

// clearLMDBSessions drops all sessions AND bandwidth data on startup.
func clearLMDBSessions() error {
	return lmdbEnv.Update(func(txn *lmdb.Txn) error {
		for _, dbName := range []string{"sessions", "bandwidth"} {
			dbi, err := txn.OpenDBI(dbName, lmdb.Create)
			if err != nil {
				return err
			}
			if err := txn.Drop(dbi, false); err != nil {
				return err
			}
		}
		return nil
	})
}

func SyncUsersInGroup(groupName string) {
	log.Printf("[lmdb-sync] Syncing all users in group [%s]...", groupName)
	rows, err := DB.Query("SELECT username FROM radusergroup WHERE groupname=?", groupName)
	if err != nil {
		log.Printf("[lmdb-sync] Failed to fetch users for group sync: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err == nil {
			QueueUserSync(username)
			count++
		}
	}
	log.Printf("[lmdb-sync] Group sync completed for [%s]. Queued %d users.", groupName, count)
}
func SyncAllUsers() {
	log.Println("[lmdb-sync] Starting full synchronization...")
	rows, err := DB.Query("SELECT username FROM radcheck WHERE attribute='Cleartext-Password'")
	if err != nil {
		log.Printf("[lmdb-sync] Failed to fetch users for sync: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err == nil {
			QueueUserSync(username)
			count++
		}
	}
	log.Printf("[lmdb-sync] Full sync completed. Queued %d users.", count)
}

func startSyncWorker() {
	log.Println("[lmdb-worker] Sync worker started")
	for username := range syncQueue {
		retryCount := 0
		for {
			err := performSync(username)
			if err == nil {
				break // Success
			}

			retryCount++
			if retryCount > 5 {
				log.Printf("[lmdb-worker] CRITICAL: Failed to sync user [%s] after 5 retries: %v", username, err)
				break
			}
			time.Sleep(time.Duration(retryCount) * 500 * time.Millisecond)
		}
	}
}

// QueueUserSync pushes a username to the sync queue for background processing
func QueueUserSync(username string) {
	select {
	case syncQueue <- username:
	default:
		log.Printf("[lmdb-worker] WARNING: Sync queue full! Dropping sync for [%s]", username)
	}
}

func performSync(username string) error {
	// Fetch fresh data from SQLite
	var password string
	var exp int64
	var enabled int

	// 1. Get Password
	err := DB.QueryRow("SELECT value FROM radcheck WHERE username=? AND attribute='Cleartext-Password' LIMIT 1", username).Scan(&password)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// 2. Get Meta (Expiration and Enabled status)
	err = DB.QueryRow("SELECT COALESCE(expiration_unix, 0), COALESCE(enabled, 1) FROM radius_user_meta WHERE username=?", username).Scan(&exp, &enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			// If not in meta, we assume it's deleted from our management system
			return removeFromLMDB(username)
		}
		return err
	}

	// 3. Get User-specific Reply Attributes
	rows, err := DB.Query("SELECT attribute, value FROM radreply WHERE username=?", username)
	allAttrs := make(map[string]string)
	if err == nil {
		for rows.Next() {
			var a, v string
			rows.Scan(&a, &v)
			allAttrs[a] = v
		}
		rows.Close()
	}

	// 4. Get Group (Profile) Attributes
	var groupName string
	err = DB.QueryRow("SELECT groupname FROM radusergroup WHERE username=? ORDER BY priority LIMIT 1", username).Scan(&groupName)
	if err == nil && groupName != "" {
		// Group Reply
		gRows, gErr := DB.Query("SELECT attribute, value FROM radgroupreply WHERE groupname=?", groupName)
		if gErr == nil {
			for gRows.Next() {
				var a, v string
				gRows.Scan(&a, &v)
				if _, exists := allAttrs[a]; !exists {
					allAttrs[a] = v
				}
			}
			gRows.Close()
		}
		// Group Check (Simultaneous-Use, etc.)
		gcRows, gcErr := DB.Query("SELECT attribute, value FROM radgroupcheck WHERE groupname=?", groupName)
		if gcErr == nil {
			for gcRows.Next() {
				var a, v string
				gcRows.Scan(&a, &v)
				if _, exists := allAttrs[a]; !exists {
					allAttrs[a] = v
				}
			}
			gcRows.Close()
		}
	}

	// 5. Get User Check Attributes (other than Password)
	cRows, cErr := DB.Query("SELECT attribute, value FROM radcheck WHERE username=? AND attribute != 'Cleartext-Password'", username)
	if cErr == nil {
		for cRows.Next() {
			var a, v string
			cRows.Scan(&a, &v)
			allAttrs[a] = v
		}
		cRows.Close()
	}

	// 4.5 Fetch Expired Pool and Expired Profile
	var expiredPool, expiredProfile string
	if groupName != "" {
		_ = DB.QueryRow("SELECT COALESCE(expired_pool, ''), COALESCE(expired_profile, '') FROM radius_profile_meta WHERE groupname=?", groupName).Scan(&expiredPool, &expiredProfile)
	}

	// Format: password\nExpiration=val\nEnabled=val\nAttr1=val...
	lmdbData := password + "\n"
	lmdbData += fmt.Sprintf("Expiration=%d\n", exp)
	lmdbData += fmt.Sprintf("Enabled=%d\n", enabled)
	lmdbData += fmt.Sprintf("Expired-Pool=%s\n", expiredPool)
	lmdbData += fmt.Sprintf("Expired-Profile=%s\n", expiredProfile)
	lmdbData += fmt.Sprintf("User-Group=%s\n", groupName)
	for a, v := range allAttrs {
		lmdbData += a + "=" + v + "\n"
	}

	return lmdbEnv.Update(func(txn *lmdb.Txn) error {
		dbi, err := txn.OpenDBI("radcheck", lmdb.Create)
		if err != nil {
			return err
		}
		return txn.Put(dbi, []byte(username), []byte(lmdbData), 0)
	})
}

func removeFromLMDB(username string) error {
	return lmdbEnv.Update(func(txn *lmdb.Txn) error {
		dbi, err := txn.OpenDBI("radcheck", 0)
		if err != nil {
			if lmdb.IsNotFound(err) {
				return nil
			}
			return err
		}
		err = txn.Del(dbi, []byte(username), nil)
		if lmdb.IsNotFound(err) {
			return nil
		}
		return err
	})
}

// GetLMDBSessions returns a map of username -> sessionID for all active sessions in LMDB
func GetLMDBSessions() (map[string]string, error) {
	sessions := make(map[string]string)
	err := lmdbEnv.View(func(txn *lmdb.Txn) error {
		dbi, err := txn.OpenDBI("sessions", 0)
		if err != nil {
			if lmdb.IsNotFound(err) {
				return nil
			}
			return err
		}

		cursor, err := txn.OpenCursor(dbi)
		if err != nil {
			return err
		}
		defer cursor.Close()

		for {
			k, v, err := cursor.Get(nil, nil, lmdb.Next)
			if err != nil {
				if lmdb.IsNotFound(err) {
					break
				}
				return err
			}
			sessions[string(k)] = string(v)
		}
		return nil
	})
	return sessions, err
}

// LMDBBandwidth holds real-time session stats written by the C module on Interim-Update.
type LMDBBandwidth struct {
	InputOctets    int64
	OutputOctets   int64
	StartTime      int64 // Unix timestamp of session start
	SessionSeconds int64
	SessionID      string
	IP             string
	CallingStation string
}

// GetLMDBBandwidth reads the 'bandwidth' LMDB database.
// Format written by C module: inBytes|outBytes|startTime|sessionSecs|sessionId|ip|cli
func GetLMDBBandwidth() (map[string]LMDBBandwidth, error) {
	result := make(map[string]LMDBBandwidth)
	err := lmdbEnv.View(func(txn *lmdb.Txn) error {
		dbi, err := txn.OpenDBI("bandwidth", 0)
		if err != nil {
			if lmdb.IsNotFound(err) {
				return nil
			}
			return err
		}

		cursor, err := txn.OpenCursor(dbi)
		if err != nil {
			return err
		}
		defer cursor.Close()

		for {
			k, v, err := cursor.Get(nil, nil, lmdb.Next)
			if err != nil {
				if lmdb.IsNotFound(err) {
					break
				}
				return err
			}
			uKey := strings.ToLower(strings.TrimSpace(string(k)))
			parts := strings.Split(string(v), "|")
			bw := LMDBBandwidth{}
			if len(parts) >= 1 {
				bw.InputOctets, _ = strconv.ParseInt(parts[0], 10, 64)
			}
			if len(parts) >= 2 {
				bw.OutputOctets, _ = strconv.ParseInt(parts[1], 10, 64)
			}
			if len(parts) >= 3 {
				bw.StartTime, _ = strconv.ParseInt(parts[2], 10, 64)
			}
			if len(parts) >= 4 {
				bw.SessionSeconds, _ = strconv.ParseInt(parts[3], 10, 64)
			}
			if len(parts) >= 5 {
				bw.SessionID = parts[4]
			}
			if len(parts) >= 6 {
				bw.IP = parts[5]
			}
			if len(parts) >= 7 {
				bw.CallingStation = parts[6]
			}
			result[uKey] = bw
		}
		return nil
	})
	return result, err
}

func fetchUserFromLMDB(username string) (string, error) {
	var data []byte
	err := lmdbEnv.View(func(txn *lmdb.Txn) error {
		dbi, err := txn.OpenDBI("radcheck", 0)
		if err != nil {
			return err
		}
		val, err := txn.Get(dbi, []byte(username))
		if err != nil {
			return err
		}
		data = make([]byte, len(val))
		copy(data, val)
		return nil
	})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func saveAccountingToLMDB(username string, status uint32, sid, ip, cli string, in, out uint64, secs int64) {
	_ = lmdbEnv.Update(func(txn *lmdb.Txn) error {
		dbiSessions, _ := txn.OpenDBI("sessions", lmdb.Create)
		dbiBandwidth, _ := txn.OpenDBI("bandwidth", lmdb.Create)

		key := []byte(username)

		switch status {
		case 1: // Start
			// sessions: sid|ip|cli
			sessVal := fmt.Sprintf("%s|%s|%s", sid, ip, cli)
			_ = txn.Put(dbiSessions, key, []byte(sessVal), 0)

			// bandwidth: in|out|startTime|secs|sid|ip|cli
			bwVal := fmt.Sprintf("0|0|%d|0|%s|%s|%s", time.Now().Unix(), sid, ip, cli)
			_ = txn.Put(dbiBandwidth, key, []byte(bwVal), 0)

		case 3: // Interim-Update
			// Recover start time
			startTime := time.Now().Unix() - secs
			if existing, err := txn.Get(dbiBandwidth, key); err == nil {
				parts := strings.Split(string(existing), "|")
				if len(parts) >= 3 {
					if st, err := strconv.ParseInt(parts[2], 10, 64); err == nil {
						startTime = st
					}
				}
			}

			bwVal := fmt.Sprintf("%d|%d|%d|%d|%s|%s|%s", in, out, startTime, secs, sid, ip, cli)
			_ = txn.Put(dbiBandwidth, key, []byte(bwVal), 0)

			sessVal := fmt.Sprintf("%s|%s|%s", sid, ip, cli)
			_ = txn.Put(dbiSessions, key, []byte(sessVal), 0)

		case 2: // Stop
			_ = txn.Del(dbiSessions, key, nil)
			_ = txn.Del(dbiBandwidth, key, nil)
		}
		return nil
	})
}
