package radius

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	sessionCacheTTL    = 5 * time.Second
	sessionStaleWindow = 10 * time.Minute
)

type SessionInfo struct {
	Status         string `json:"status"`
	Online         bool   `json:"online"`
	Stale          bool   `json:"stale"`
	IP             string `json:"ip"`
	DownloadBytes  int64  `json:"download_bytes"`
	UploadBytes    int64  `json:"upload_bytes"`
	Download       string `json:"download"`
	Upload         string `json:"upload"`
	SessionID      string `json:"session_id"`
	NASIP          string `json:"nas_ip"`
	CallingStation string `json:"calling_station"`
	StartedAt      string `json:"started_at"`
	LastUpdateAt   string `json:"last_update_at"`
	StoppedAt      string `json:"stopped_at"`
	SessionSeconds int64  `json:"session_seconds"`
	Username       string `json:"username"` // Preserve original casing
}

var (
	sessionCacheMu      sync.Mutex
	sessionCacheData    map[string]SessionInfo
	sessionCacheFetched time.Time
)

func ensureAccountingIndexes() {
	stmts := []string{
		"CREATE INDEX IF NOT EXISTS acct_open_session ON radacct(username) WHERE acctstoptime IS NULL",
	}
	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			_, _ = DB.Exec("CREATE INDEX IF NOT EXISTS acct_user_stop ON radacct(username, acctstoptime)")
		}
	}
}

func InvalidateSessionCache() {
	sessionCacheMu.Lock()
	sessionCacheData = nil
	sessionCacheFetched = time.Time{}
	sessionCacheMu.Unlock()
}

func GetSessionForUser(username string) SessionInfo {
	sessions := loadSessionsCached()
	key := strings.ToLower(strings.TrimSpace(username))
	if info, ok := sessions[key]; ok {
		return info
	}
	return SessionInfo{Status: "offline"}
}

func loadSessionsCached() map[string]SessionInfo {
	sessionCacheMu.Lock()
	defer sessionCacheMu.Unlock()

	if sessionCacheData != nil && time.Since(sessionCacheFetched) < sessionCacheTTL {
		return sessionCacheData
	}

	data, err := LoadSessionsFromDB()
	if err != nil {
		if sessionCacheData != nil {
			return sessionCacheData
		}
		return map[string]SessionInfo{}
	}

	sessionCacheData = data
	sessionCacheFetched = time.Now()
	return data
}

func LoadSessionsFromDB() (map[string]SessionInfo, error) {
	result := make(map[string]SessionInfo)

	openRows, err := DB.Query(`
        SELECT username, COALESCE(framedipaddress,''), COALESCE(acctinputoctets,0), COALESCE(acctoutputoctets,0),
               COALESCE(acctsessionid,''), COALESCE(nasipaddress,''), COALESCE(callingstationid,''),
               acctstarttime, acctupdatetime, COALESCE(acctsessiontime,0)
        FROM radacct
        WHERE acctstoptime IS NULL
        ORDER BY acctstarttime DESC`)
	if err != nil {
		return nil, err
	}
	for openRows.Next() {
		info := scanSessionRow(openRows)
		if info.username == "" {
			continue
		}
		// Use trimmed lower case for mapping keys but keep original in the object
		uKey := strings.ToLower(strings.TrimSpace(info.username))
		if _, exists := result[uKey]; exists {
			continue
		}
		info.session.Username = info.username
		info.session.Status = classifySessionStatus(true, info.session.LastUpdateAt)
		info.session.Online = info.session.Status == "online"
		info.session.Stale = info.session.Status == "stale"
		result[uKey] = info.session
	}
	openRows.Close()

	lastRows, err := DB.Query(`
        SELECT username, COALESCE(framedipaddress,''), COALESCE(acctinputoctets,0), COALESCE(acctoutputoctets,0),
               COALESCE(acctsessionid,''), COALESCE(nasipaddress,''), COALESCE(callingstationid,''),
               acctstarttime, acctupdatetime, COALESCE(acctsessiontime,0), acctstoptime
        FROM radacct
        WHERE acctstoptime IS NOT NULL 
          AND acctstarttime > datetime('now', '-2 days')
        ORDER BY acctstarttime DESC LIMIT 3000`)
	if err != nil {
		return result, nil
	}
	defer lastRows.Close()
	for lastRows.Next() {
		info := scanSessionRowWithStop(lastRows)
		if info.username == "" {
			continue
		}
		uKey := strings.ToLower(strings.TrimSpace(info.username))
		if _, exists := result[uKey]; exists {
			continue
		}
		info.session.Username = info.username
		info.session.Status = "offline"
		result[uKey] = info.session
	}

	// 3. Merge LMDB Active Sessions (High Performance Store)
	lmdbSessions, _ := GetLMDBSessions()
	for username, rawData := range lmdbSessions {
		uKey := strings.ToLower(strings.TrimSpace(username))

		parts := strings.Split(rawData, "|")
		sid := parts[0]
		ip := ""
		cli := ""
		if len(parts) > 1 {
			ip = parts[1]
		}
		if len(parts) > 2 {
			cli = parts[2]
		}

		if sess, exists := result[uKey]; exists {
			// Always update with latest LMDB info if it's more "live"
			sess.Status = "online"
			sess.Online = true
			sess.SessionID = sid
			if ip != "" {
				sess.IP = ip
			}
			if cli != "" {
				sess.CallingStation = cli
			}
			result[uKey] = sess
		} else {
			// Not found in SQLite at all, create a minimal online session
			result[uKey] = SessionInfo{
				Username:       username,
				Status:         "online",
				Online:         true,
				SessionID:      sid,
				IP:             ip,
				CallingStation: cli,
			}
		}
	}

	// 4. Merge LMDB Bandwidth (written by C module on every Interim-Update)
	lmdbBandwidth, _ := GetLMDBBandwidth()
	for uKey, bw := range lmdbBandwidth {
		sess, exists := result[uKey]
		if !exists {
			continue
		}
		if bw.InputOctets > 0 || bw.OutputOctets > 0 {
			sess.DownloadBytes = bw.InputOctets
			sess.UploadBytes   = bw.OutputOctets
			sess.Download      = humanBytes(bw.InputOctets)
			sess.Upload        = humanBytes(bw.OutputOctets)
		}
		if bw.StartTime > 0 {
			sess.StartedAt = time.Unix(bw.StartTime, 0).In(baghdadLocation).Format("2006-01-02 15:04")
			liveElapsed := time.Now().Unix() - bw.StartTime
			if liveElapsed > sess.SessionSeconds {
				sess.SessionSeconds = liveElapsed
			}
		} else if bw.SessionSeconds > 0 && sess.SessionSeconds == 0 {
			sess.SessionSeconds = bw.SessionSeconds
		}
		if bw.SessionID != "" {
			sess.SessionID = bw.SessionID
		}
		if bw.IP != "" {
			sess.IP = bw.IP
		}
		if bw.CallingStation != "" {
			sess.CallingStation = bw.CallingStation
		}
		result[uKey] = sess
	}

	return result, nil
}

type sessionScan struct {
	username string
	session  SessionInfo
}

func scanSessionRow(rows *sql.Rows) sessionScan {
	var (
		username, ip, sid, nasIP, calling string
		inputOctets, outputOctets, sess   int64
		startAt, updateAt                 sql.NullString
	)
	if err := rows.Scan(&username, &ip, &inputOctets, &outputOctets, &sid, &nasIP, &calling, &startAt, &updateAt, &sess); err != nil {
		return sessionScan{}
	}

	startedStr := normalizeAcctTime(startAt)
	// Calculate live elapsed seconds if the session is currently active
	if startAt.Valid && startAt.String != "" {
		layouts := []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05Z", time.RFC3339, "2006-01-02 15:04"}
		for _, l := range layouts {
			if t, err := time.ParseInLocation(l, startAt.String, baghdadLocation); err == nil {
				elapsed := int64(time.Since(t).Seconds())
				if elapsed > sess {
					sess = elapsed
				}
				break
			}
		}
	}

	info := SessionInfo{
		IP:             ip,
		DownloadBytes:  inputOctets,
		UploadBytes:    outputOctets,
		Download:       humanBytes(inputOctets),
		Upload:         humanBytes(outputOctets),
		SessionID:      sid,
		NASIP:          nasIP,
		CallingStation: calling,
		StartedAt:      startedStr,
		LastUpdateAt:   normalizeAcctTime(updateAt),
		SessionSeconds: sess,
	}
	return sessionScan{username: username, session: info}
}

func scanSessionRowWithStop(rows *sql.Rows) sessionScan {
	var (
		username, ip, sid, nasIP, calling   string
		inputOctets, outputOctets, sess     int64
		startAt, updateAt, stopAt           sql.NullString
	)
	if err := rows.Scan(&username, &ip, &inputOctets, &outputOctets, &sid, &nasIP, &calling, &startAt, &updateAt, &sess, &stopAt); err != nil {
		return sessionScan{}
	}
	info := SessionInfo{
		IP:             ip,
		DownloadBytes:  inputOctets,
		UploadBytes:    outputOctets,
		Download:       humanBytes(inputOctets),
		Upload:         humanBytes(outputOctets),
		SessionID:      sid,
		NASIP:          nasIP,
		CallingStation: calling,
		StartedAt:      normalizeAcctTime(startAt),
		LastUpdateAt:   normalizeAcctTime(updateAt),
		StoppedAt:      normalizeAcctTime(stopAt),
		SessionSeconds: sess,
	}
	return sessionScan{username: username, session: info}
}

func humanBytes(b int64) string {
	if b <= 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffix := []string{"KB", "MB", "GB", "TB", "PB"}[exp]
	return fmt.Sprintf("%.2f %s", float64(b)/float64(div), suffix)
}

func normalizeAcctTime(v sql.NullString) string {
	if !v.Valid || v.String == "" {
		return ""
	}
	layouts := []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05Z", time.RFC3339}
	for _, l := range layouts {
		if t, err := time.Parse(l, v.String); err == nil {
			return t.In(baghdadLocation).Format("2006-01-02 15:04")
		}
	}
	return v.String
}

func classifySessionStatus(open bool, lastUpdate string) string {
	if !open {
		return "offline"
	}
	if lastUpdate == "" {
		return "online"
	}
	t, err := time.Parse("2006-01-02 15:04", lastUpdate)
	if err != nil {
		return "online"
	}
	if time.Since(t.In(baghdadLocation)) > sessionStaleWindow {
		return "stale"
	}
	return "online"
}

// ListActiveSessionsHandler returns all currently active sessions
func ListActiveSessionsHandler(c *fiber.Ctx) error {
	sessions, err := LoadSessionsFromDB()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	activeList := make([]SessionInfo, 0)
	for _, s := range sessions {
		if s.Online {
			activeList = append(activeList, s)
		}
	}
	return c.JSON(fiber.Map{"sessions": activeList, "total": len(activeList)})
}
