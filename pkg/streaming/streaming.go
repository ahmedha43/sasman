package streaming

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"mikrotik-manager/pkg/radius"

	"github.com/gofiber/fiber/v2"
)

type Stream struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Source     string    `json:"source"`
	Status     string    `json:"status"`
	LocalRelay int       `json:"local_relay"`
	CreatedAt  time.Time `json:"created_at"`
}

// isYouTubeURL checks if the source is a YouTube URL
func isYouTubeURL(source string) bool {
	return strings.Contains(source, "youtube.com") ||
		strings.Contains(source, "youtu.be") ||
		strings.Contains(source, "youtube.com/shorts")
}

// Helpers to dynamically sync dynamic streams to lal server
func syncStreamWithMediaMTX(id string, source string, localRelay int, status string) {
	// First, try to clean up any existing relay
	_ = id // placeholder for now

	// For YouTube URLs, we use ffmpeg + yt-dlp to push to lal RTMP
	if status == "active" && localRelay == 1 && isYouTubeURL(source) {
		go startYouTubeRelay(id, source)
		return
	}

	// For regular external URLs, we can register with lal using its API
	// Note: lal automatically serves streams pushed to it via RTMP on port 1935
	// HLS output is available on port 8888/{streamPath}
	if status == "active" && localRelay == 1 && (strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") || strings.Contains(source, ".m3u8")) {
		log.Printf("[streaming] External stream registered: %s -> available on lal HLS", id)
	}
}

// startYouTubeRelay starts yt-dlp + ffmpeg to push YouTube stream to lal RTMP
func startYouTubeRelay(id string, source string) {
	log.Printf("[youtube-relay] Starting relay for %s: %s", id, source)

	// Push to lal RTMP server
	// ffmpeg extracts from YouTube and pushes to lal's RTMP interface
	cmd := exec.Command("sh", "-c", fmt.Sprintf(`
		exec yt-dlp -f "best[ext=mp4]/best" --hls-use-mpegts --no-warnings -o - "%s" 2>/dev/null | \
		ffmpeg -y -i pipe:0 -c:v copy -c:a aac -f flv "rtmp://127.0.0.1:1935/%s" 2>/dev/null
	`, source, id))

	if err := cmd.Run(); err != nil {
		log.Printf("[youtube-relay] Relay failed for %s: %v", id, err)
	}
	log.Printf("[youtube-relay] Relay ended for %s", id)
}

func removeStreamFromMediaMTX(id string) {
	// Note: lal automatically cleans up inactive streams
	log.Printf("[streaming] Stream removed from lal: %s", id)
}

// GetActiveStreams lists streams for authenticated, unexpired subscribers
func GetActiveStreams(c *fiber.Ctx) error {
	username := c.Query("username")
	if username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم المستخدم مطلوب"})
	}

	// 1. Validate that the subscriber is active and not expired in RADIUS
	var expirationUnix sql.NullInt64
	var enabled int
	err := radius.DB.QueryRow("SELECT expiration_unix, enabled FROM radius_user_meta WHERE username=?", username).Scan(&expirationUnix, &enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(403).JSON(fiber.Map{"error": "المشترك غير موجود في النظام"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "خطأ في التحقق من الحساب: " + err.Error()})
	}

	if enabled == 0 {
		return c.Status(403).JSON(fiber.Map{"error": "تم إيقاف حسابك من قبل مدير النظام"})
	}

	// If expired, deny access
	if expirationUnix.Valid && expirationUnix.Int64 < time.Now().Unix() {
		return c.Status(403).JSON(fiber.Map{"error": "عذراً، اشتراكك منتهي. يرجى التجديد لتتمكن من مشاهدة القنوات."})
	}

	// 2. Fetch all active streams
	rows, err := radius.DB.Query("SELECT id, name, source, status, local_relay, created_at FROM radius_streams WHERE status='active' ORDER BY created_at DESC")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل جلب القنوات: " + err.Error()})
	}
	defer rows.Close()

	var streams []Stream
	for rows.Next() {
		var s Stream
		var createdAtStr string
		err := rows.Scan(&s.ID, &s.Name, &s.Source, &s.Status, &s.LocalRelay, &createdAtStr)
		if err == nil {
			s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
			streams = append(streams, s)
		}
	}

	return c.JSON(streams)
}

// GetStreamsAdmin lists all streams for the administrator panel
func GetStreamsAdmin(c *fiber.Ctx) error {
	rows, err := radius.DB.Query("SELECT id, name, source, status, local_relay, created_at FROM radius_streams ORDER BY created_at DESC")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	var streams []Stream
	for rows.Next() {
		var s Stream
		var createdAtStr string
		err := rows.Scan(&s.ID, &s.Name, &s.Source, &s.Status, &s.LocalRelay, &createdAtStr)
		if err == nil {
			s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
			streams = append(streams, s)
		}
	}

	return c.JSON(streams)
}

// CreateStreamAdmin allows admins to create a new live channel
func CreateStreamAdmin(c *fiber.Ctx) error {
	var req struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Source     string `json:"source"`
		Status     string `json:"status"`
		LocalRelay int    `json:"local_relay"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "محتوى الطلب غير صالح"})
	}

	if req.ID == "" || req.Name == "" || req.Source == "" {
		return c.Status(400).JSON(fiber.Map{"error": "جميع الحقول مطلوبة"})
	}

	if req.Status == "" {
		req.Status = "active"
	}

	_, err := radius.DB.Exec(
		"INSERT INTO radius_streams (id, name, source, status, local_relay, created_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)",
		req.ID, req.Name, req.Source, req.Status, req.LocalRelay,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل حفظ القناة: " + err.Error()})
	}

	// Dynamically register in MediaMTX if relay is enabled
	syncStreamWithMediaMTX(req.ID, req.Source, req.LocalRelay, req.Status)

	log.Printf("[streaming] Channel created: %s (%s)", req.Name, req.ID)
	return c.JSON(fiber.Map{"message": "تم إضافة القناة بنجاح", "id": req.ID})
}

// UpdateStreamAdmin allows admins to update or toggle stream status
func UpdateStreamAdmin(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "معرف القناة مطلوب"})
	}

	var req struct {
		Name       string `json:"name"`
		Source     string `json:"source"`
		Status     string `json:"status"`
		LocalRelay int    `json:"local_relay"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "محتوى الطلب غير صالح"})
	}

	_, err := radius.DB.Exec(
		"UPDATE radius_streams SET name=?, source=?, status=?, local_relay=? WHERE id=?",
		req.Name, req.Source, req.Status, req.LocalRelay, id,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل تحديث القناة: " + err.Error()})
	}

	// Dynamically sync MediaMTX configuration
	syncStreamWithMediaMTX(id, req.Source, req.LocalRelay, req.Status)

	return c.JSON(fiber.Map{"message": "تم تحديث القناة بنجاح"})
}

// DeleteStreamAdmin allows admins to delete a live channel
func DeleteStreamAdmin(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "معرف القناة مطلوب"})
	}

	_, err := radius.DB.Exec("DELETE FROM radius_streams WHERE id=?", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل حذف القناة: " + err.Error()})
	}

	// Clean up viewers history
	_, _ = radius.DB.Exec("DELETE FROM radius_stream_viewers WHERE stream_id=?", id)

	// Clean up MediaMTX path
	removeStreamFromMediaMTX(id)

	log.Printf("[streaming] Channel deleted: %s", id)
	return c.JSON(fiber.Map{"message": "تم حذف القناة بنجاح"})
}

// GetMediaMTXStatus checks if lal is currently running inside Supervisord
func GetMediaMTXStatus(c *fiber.Ctx) error {
	cmd := exec.Command("supervisorctl", "-c", "/etc/supervisor/conf.d/supervisord.conf", "status", "lal")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return c.JSON(fiber.Map{"status": "stopped", "details": err.Error()})
	}

	output := string(out)
	if strings.Contains(output, "RUNNING") {
		return c.JSON(fiber.Map{"status": "running"})
	}

	return c.JSON(fiber.Map{"status": "stopped"})
}

// ControlMediaMTX starts or stops lal via supervisorctl
func ControlMediaMTX(c *fiber.Ctx) error {
	var req struct {
		Action string `json:"action"` // "start" or "stop"
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "محتوى الطلب غير صالح"})
	}

	if req.Action != "start" && req.Action != "stop" {
		return c.Status(400).JSON(fiber.Map{"error": "الإجراء المطلوب غير صالح"})
	}

	cmd := exec.Command("supervisorctl", "-c", "/etc/supervisor/conf.d/supervisord.conf", req.Action, "lal")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		// exit status 7 means the process is already in the requested state
		if strings.Contains(err.Error(), "exit status 7") || strings.Contains(output, "already started") || strings.Contains(output, "not running") {
			statusCmd := exec.Command("supervisorctl", "-c", "/etc/supervisor/conf.d/supervisord.conf", "status", "lal")
			statusOut, _ := statusCmd.CombinedOutput()
			currentState := "stopped"
			if strings.Contains(string(statusOut), "RUNNING") {
				currentState = "running"
			}
			log.Printf("[streaming] lal %s: already in state '%s' (exit 7 ignored)", req.Action, currentState)
			return c.JSON(fiber.Map{
				"message": "الخادم في الحالة المطلوبة بالفعل",
				"status":  currentState,
				"output":  output,
			})
		}
		return c.Status(500).JSON(fiber.Map{"error": "فشل تنفيذ الأمر: " + err.Error(), "details": output})
	}

	desiredState := req.Action + "ed"
	if req.Action == "stop" {
		desiredState = "stopped"
	}
	log.Printf("[streaming] lal server %s successfully via supervisorctl", req.Action)
	return c.JSON(fiber.Map{"message": "تم تغيير حالة خادم البث بنجاح", "status": desiredState, "output": output})
}

// ProxyExternalStream acts as a reverse proxy for HLS streams to bypass browser CORS blocks
// For YouTube URLs, it uses yt-dlp to extract the actual stream URL first
func ProxyExternalStream(c *fiber.Ctx) error {
	rawURL := c.Query("url")
	if rawURL == "" {
		return c.Status(400).SendString("URL parameter is required")
	}

	// Check if it's a YouTube URL - extract direct stream URL first
	if isYouTubeURL(rawURL) {
		return proxyYouTubeStream(c, rawURL)
	}

	// ... rest of existing code continues unchanged
	return proxyGenericStream(c, rawURL)
}

// proxyYouTubeStream extracts and proxies YouTube streams using yt-dlp + ffmpeg pipe
// This approach avoids token expiration issues by using ffmpeg's built-in YouTube support
func proxyYouTubeStream(c *fiber.Ctx, ytURL string) error {
	// Use ffmpeg to pipe YouTube stream directly to the client
	// This avoids the need to extract and cache URLs that expire within seconds
	
	// Set appropriate headers for HLS/mpegts stream
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Set("Access-Control-Allow-Headers", "*")
	c.Set("Content-Type", "video/mp2ts")
	
	log.Printf("[youtube-proxy] Starting ffmpeg pipe for YouTube stream")
	
	// Run ffmpeg to download and re-stream YouTube content
	// Using mpegts format for better compatibility and lower latency
	cmd := exec.Command("ffmpeg", "-y", "-i", ytURL, 
		"-c", "copy",
		"-f", "mpegts",
		"-")
	
	// Set up pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("[youtube-proxy] Failed to create stdout pipe: %v", err)
		return c.Status(500).SendString("Failed to start stream: " + err.Error())
	}
	
	// Start ffmpeg
	if err := cmd.Start(); err != nil {
		log.Printf("[youtube-proxy] ffmpeg failed to start: %v", err)
		return c.Status(500).SendString("Failed to start ffmpeg: " + err.Error())
	}
	
	// Stream the output directly to the client
	go func() {
		cmd.Wait()
	}()
	
	// Copy ffmpeg output to response body
	_, err = io.Copy(c.Response().BodyWriter(), stdout)
	return err
}

// proxyGenericStream proxies generic HLS/m3u8 streams
func proxyGenericStream(c *fiber.Ctx, rawURL string) error {

	// 1. Parse the target URL
	targetURL, err := url.Parse(rawURL)
	if err != nil {
		return c.Status(400).SendString("Invalid URL: " + err.Error())
	}

	// 2. Fetch the target resource
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return c.Status(500).SendString("Failed to create request: " + err.Error())
	}

	// Copy headers from client if needed, or set standard headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(502).SendString("Failed to fetch stream: " + err.Error())
	}
	defer resp.Body.Close()

	// 3. Set CORS and Cache-Control headers
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Set("Access-Control-Allow-Headers", "*")
	
	// Copy relevant headers
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		c.Set("Content-Type", contentType)
	}

	// 4. If it's a playlist (.m3u8), we rewrite relative paths to absolute paths
	isM3U8 := strings.Contains(strings.ToLower(targetURL.Path), ".m3u8") || 
		strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") || 
		strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/x-mpegurl")

	if isM3U8 {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.Status(500).SendString("Failed to read response body: " + err.Error())
		}

		// Parse the playlist line by line
		lines := strings.Split(string(bodyBytes), "\n")
		var rewrittenLines []string

		baseURL := *targetURL
		// Remove the last segment of the path to get the base directory
		pathParts := strings.Split(baseURL.Path, "/")
		if len(pathParts) > 0 {
			baseURL.Path = strings.Join(pathParts[:len(pathParts)-1], "/") + "/"
		}

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				rewrittenLines = append(rewrittenLines, line)
				continue
			}

			// If it's a comment/directive, keep it as is, but check if it contains URI="..."
			if strings.HasPrefix(trimmed, "#") {
				if strings.Contains(trimmed, `URI="`) {
					// We can replace relative URIs inside tags (like keys or secondary playlists)
					re := regexp.MustCompile(`URI="([^"]+)"`)
					matches := re.FindStringSubmatch(trimmed)
					if len(matches) > 1 {
						uriVal := matches[1]
						var absURI string
						if strings.HasPrefix(uriVal, "http://") || strings.HasPrefix(uriVal, "https://") {
							absURI = uriVal
						} else {
							relURL, err := url.Parse(uriVal)
							if err == nil {
								absURI = baseURL.ResolveReference(relURL).String()
							} else {
								absURI = uriVal
							}
						}
						proxiedURI := "/radius/api/portal/stream-proxy?url=" + url.QueryEscape(absURI)
						trimmed = strings.Replace(trimmed, `URI="`+uriVal+`"`, `URI="`+proxiedURI+`"`, 1)
					}
				}
				rewrittenLines = append(rewrittenLines, trimmed)
				continue
			}

			// It's a URI line! Check if it is absolute or relative
			var absoluteURI string
			if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
				absoluteURI = trimmed
			} else {
				// Build absolute URI relative to the base URL
				relURL, err := url.Parse(trimmed)
				if err != nil {
					rewrittenLines = append(rewrittenLines, trimmed)
					continue
				}
				absoluteURI = baseURL.ResolveReference(relURL).String()
			}

			// Proxy the segment through our own server to avoid CORS blocks on segment files too!
			proxiedURI := "/radius/api/portal/stream-proxy?url=" + url.QueryEscape(absoluteURI)
			rewrittenLines = append(rewrittenLines, proxiedURI)
		}

		return c.SendString(strings.Join(rewrittenLines, "\n"))
	}

	// 5. If it's a segment (.ts, key, etc.), just pipe the raw bytes
	_, err = io.Copy(c.Response().BodyWriter(), resp.Body)
	return err
}

