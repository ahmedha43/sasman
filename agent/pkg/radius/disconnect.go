package radius

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"mikrotik-manager/agent/pkg/core"

	"github.com/gofiber/fiber/v2"
	"github.com/wxccs/radius/v2/packet"
	"github.com/wxccs/radius/v2/types"
)

const (
	expirationSweepInterval = 45 * time.Second
	coaPort                 = 3799
	coaTimeout              = 3 * time.Second
)

func StartExpirationSweeper() {
	ensureAccountingIndexes()
	go func() {
		ticker := time.NewTicker(expirationSweepInterval)
		defer ticker.Stop()
		for {
			runExpirationSweep()
			<-ticker.C
		}
	}()
}

func StartPruningSweeper() {
	go func() {
		// Initial prune after 1 minute of startup
		time.Sleep(1 * time.Minute)
		runPruning()
		// Then every 24 hours
		for {
			time.Sleep(24 * time.Hour)
			runPruning()
		}
	}()
}

func runPruning() {
	if DB == nil {
		return
	}
	log.Println("[prune] Running daily database pruning...")

	// Prune radacct older than 7 days
	if res, err := DB.Exec("DELETE FROM radacct WHERE acctstarttime < datetime('now', '-7 days')"); err == nil {
		rows, _ := res.RowsAffected()
		log.Printf("[prune] Deleted %d old radacct records (>7 days)", rows)
	} else {
		log.Printf("[prune] Failed to prune radacct: %v", err)
	}

	// Prune radpostauth older than 2 days
	if res, err := DB.Exec("DELETE FROM radpostauth WHERE authdate < datetime('now', '-2 days')"); err == nil {
		rows, _ := res.RowsAffected()
		log.Printf("[prune] Deleted %d old radpostauth records (>2 days)", rows)
	} else {
		log.Printf("[prune] Failed to prune radpostauth: %v", err)
	}

	// Compress the SQLite database and reclaim physical disk space
	log.Println("[prune] Compressing database (VACUUM)...")
	if _, err := DB.Exec("VACUUM"); err != nil {
		log.Printf("[prune] Failed to VACUUM database: %v", err)
	} else {
		log.Println("[prune] Database VACUUM completed successfully")
	}

	log.Println("[prune] Database pruning completed")
}

func ManualPruneHandler(c *fiber.Ctx) error {
	go runPruning()
	return c.JSON(fiber.Map{"message": "جارٍ تقليم قاعدة البيانات في الخلفية..."})
}

func runExpirationSweep() {
	if DB == nil {
		return
	}
	rows, err := DB.Query(`
        SELECT username, expiration_unix FROM radius_user_meta
        WHERE expiration_unix IS NOT NULL AND expiration_unix > 0
          AND expiration_unix <= ?`, time.Now().Unix())
	if err != nil {
		return
	}
	expired := make([]string, 0)
	for rows.Next() {
		var name string
		var exp int64
		if err := rows.Scan(&name, &exp); err == nil {
			expired = append(expired, name)
		}
	}
	rows.Close()
	if len(expired) == 0 {
		return
	}
	sessions := loadSessionsCached()
	for _, user := range expired {
		info, ok := sessions[strings.ToLower(user)]
		if !ok || !info.Online {
			continue
		}

		// If user has an expired redirect pool or profile configured, check if they are already on it
		expiredPool, expiredProfile := lookupExpiredRedirect(user, nil)
		if expiredPool != "" || expiredProfile != "" {
			startedUnix, err := parseStartedAtUnix(info.StartedAt)
			if err == nil {
				// Fetch user's expiration_unix again to ensure precision
				var expUnix int64
				errExp := DB.QueryRow("SELECT expiration_unix FROM radius_user_meta WHERE username = ?", user).Scan(&expUnix)
				if errExp == nil && expUnix > 0 {
					if startedUnix > expUnix {
						// The session started after their expiration time.
						// This means they have already re-authenticated and are connected under the limited/expired profile.
						// We do not disconnect them again.
						continue
					}
				}
			}
		}

		// Use the specific original username stored in session info if available
		targetName := info.Username
		if targetName == "" {
			targetName = user
		}

		if err := DisconnectUserSession(targetName, info); err != nil {
			log.Printf("[radius] disconnect failed for %s: %v", targetName, err)
		}
	}
}

func DisconnectUserSession(username string, info SessionInfo) error {
	coaErr := sendCoADisconnect(username, info)
	routerErr := disconnectViaRouterOS(username, info)
	if coaErr == nil || routerErr == nil {
		return nil
	}
	return fmt.Errorf("coa: %v; router: %v", coaErr, routerErr)
}

func sendCoADisconnect(username string, info SessionInfo) error {
	if info.NASIP == "" {
		return fmt.Errorf("no NAS IP")
	}

	secret, err := lookupNASSecret(info.NASIP)
	if err != nil {
		secret = "radsec" // Fallback secret for RadSec TLS
	}

	// 1. Check if NAS is connected via RadSec (Reverse Disconnect over mTLS)
	if agent := GetRadSecAgentByNAS(info.NASIP); agent != nil {
		radiusLogger.Printf("[coa] 🔄 Using Reverse Disconnect via RadSec agent [%s] for NAS [%s]", agent.CommonName, info.NASIP)
		return SendReverseDisconnect(agent, username, info, secret)
	}

	if err != nil {
		return err
	}
	wire, err := buildDisconnectPacket(username, info, secret)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(info.NASIP, fmt.Sprintf("%d", coaPort))
	conn, err := net.DialTimeout("udp", addr, coaTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(coaTimeout))

	if _, err := conn.Write(wire); err != nil {
		return err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	if n < 20 {
		return fmt.Errorf("short reply")
	}
	reply := buf[:n]
	var response packet.Packet
	if err := response.Unmarshal(reply, []byte(secret)); err != nil {
		return fmt.Errorf("invalid disconnect response: %w", err)
	}
	if response.Code != types.DisconnectACK {
		return fmt.Errorf("disconnect NAK code=%d", response.Code)
	}
	return nil
}

func lookupNASSecret(nasIP string) (string, error) {
	var secret string
	if err := DB.QueryRow("SELECT secret FROM nas WHERE nasname=? OR profile_nas_ip=? LIMIT 1", nasIP, nasIP).Scan(&secret); err == nil && secret != "" {
		return secret, nil
	}
	return "", fmt.Errorf("NAS secret not found for %s", nasIP)
}

func disconnectViaRouterOS(username string, info SessionInfo) error {
	client, err := core.GetSharedClient()
	if err != nil {
		return err
	}
	// Shared connection

	var errs []string
	if err := removeMatchingSession(client, "/ppp/active/print", "/ppp/active/remove", username); err != nil {
		errs = append(errs, "ppp: "+err.Error())
	} else {
		return nil
	}
	if err := removeMatchingSession(client, "/ip/hotspot/active/print", "/ip/hotspot/active/remove", username); err != nil {
		errs = append(errs, "hotspot: "+err.Error())
	} else {
		return nil
	}
	if err := removeMatchingSession(client, "/user/active/print", "/user/active/remove", username); err != nil {
		errs = append(errs, "login: "+err.Error())
	} else {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

func parseStartedAtUnix(startedAt string) (int64, error) {
	if startedAt == "" {
		return 0, fmt.Errorf("empty started_at")
	}
	layouts := []string{"2006-01-02 15:04", "2006-01-02 15:04:05", "2006-01-02T15:04:05Z", time.RFC3339}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, startedAt, baghdadLocation); err == nil {
			return t.Unix(), nil
		}
	}
	return 0, fmt.Errorf("failed to parse started_at: %s", startedAt)
}
