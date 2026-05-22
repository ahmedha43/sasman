package radius

import (
	"log"
	"mikrotik-manager/pkg/core"
	"strings"
	"time"
)

// StartReconciliationWorker runs a background task that ensures
// the RADIUS database 'online' status matches the actual state on MikroTik.
func StartReconciliationWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			reconcileSessions()
		}
	}()
}

func reconcileSessions() {
	client, err := core.GetSharedClient()
	if err != nil {
		// Silently fail if router is not connected
		return
	}
	// No defer client.Close() here, as it's a shared persistent connection

	// 1. Fetch active users from MikroTik (PPPoE + Hotspot)
	activeUsers := make(map[string]bool)

	// PPPoE Active
	pppRes, err := core.SafeRun(client, "/ppp/active/print")
	if err != nil {
		log.Printf("[reconciler] Error fetching PPP active users: %v. Aborting reconciliation to prevent false-offline state.", err)
		return
	}
	if pppRes != nil {
		for _, re := range pppRes.Re {
			if user, ok := re.Map["name"]; ok {
				activeUsers[strings.ToLower(user)] = true
			}
		}
	}

	// Hotspot Active
	hsRes, err := core.SafeRun(client, "/ip/hotspot/active/print")
	if err != nil {
		log.Printf("[reconciler] Error fetching Hotspot active users: %v. Aborting reconciliation.", err)
		return
	}
	if hsRes != nil {
		for _, re := range hsRes.Re {
			if user, ok := re.Map["user"]; ok {
				activeUsers[strings.ToLower(user)] = true
			}
		}
	}

	// 2. Fetch 'Online' sessions from RADIUS database (where stop time is null)
	// We only care about sessions that have been open for at least 2 minutes to avoid race conditions
	// with the actual login process.
	rows, err := DB.Query(`
		SELECT acctsessionid, username 
		FROM radacct 
		WHERE acctstoptime IS NULL 
		AND acctstarttime < datetime('now', '-2 minutes')
	`)
	if err != nil {
		log.Printf("[reconciler] DB query error: %v", err)
		return
	}
	defer rows.Close()

	var staleSessions []string
	for rows.Next() {
		var sid, username string
		if err := rows.Scan(&sid, &username); err != nil {
			continue
		}

		uKey := strings.ToLower(strings.TrimSpace(username))
		if !activeUsers[uKey] {
			staleSessions = append(staleSessions, sid)
			log.Printf("[reconciler] Detected stale session for user [%s] (ID: %s). Closing it.", username, sid)
		}
	}

	// 3. Mark stale sessions as closed
	if len(staleSessions) > 0 {
		for _, sid := range staleSessions {
			_, err := DB.Exec(`
				UPDATE radacct 
				SET acctstoptime = CURRENT_TIMESTAMP,
				    acctterminatecause = 'Lost-Carrier'
				WHERE acctsessionid = ? AND acctstoptime IS NULL`, sid)
			if err != nil {
				log.Printf("[reconciler] Failed to close session %s: %v", sid, err)
			}
		}
		// Clear cache if any session was updated
		InvalidateSessionCache()
	}
}
