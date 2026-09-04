package streaming

import (
	"database/sql"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"mikrotik-manager/agent/pkg/radius"

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

// GetActiveStreams lists streams for authenticated, unexpired subscribers.
func GetActiveStreams(c *fiber.Ctx) error {
	username := c.Query("username")
	if username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم المستخدم مطلوب"})
	}

	var expirationUnix sql.NullInt64
	var enabled int
	err := radius.DB.QueryRow("SELECT COALESCE(expiration_unix, 0), COALESCE(enabled, 1) FROM radius_user_meta WHERE username=?", username).Scan(&expirationUnix, &enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(403).JSON(fiber.Map{"error": "المشترك غير موجود في النظام"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "خطأ في التحقق من الحساب: " + err.Error()})
	}

	if enabled == 0 {
		return c.Status(403).JSON(fiber.Map{"error": "تم إيقاف حسابك من قبل مدير النظام"})
	}

	if expirationUnix.Valid && expirationUnix.Int64 < time.Now().Unix() {
		return c.Status(403).JSON(fiber.Map{"error": "عذراً، اشتراكك منتهي. يرجى التجديد لتتمكن من مشاهدة القنوات."})
	}

	rows, err := radius.DB.Query("SELECT id, name, source, status, local_relay, created_at FROM radius_streams WHERE status='active' ORDER BY created_at DESC")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل جلب القنوات: " + err.Error()})
	}
	defer rows.Close()

	streams, err := scanStreams(rows)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل قراءة القنوات: " + err.Error()})
	}
	return c.JSON(streams)
}

// GetStreamsAdmin lists all streams for the administrator panel.
func GetStreamsAdmin(c *fiber.Ctx) error {
	rows, err := radius.DB.Query("SELECT id, name, source, status, local_relay, created_at FROM radius_streams ORDER BY created_at DESC")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	streams, err := scanStreams(rows)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(streams)
}

func scanStreams(rows *sql.Rows) ([]Stream, error) {
	var streams []Stream
	for rows.Next() {
		var s Stream
		var createdAtStr string
		if err := rows.Scan(&s.ID, &s.Name, &s.Source, &s.Status, &s.LocalRelay, &createdAtStr); err != nil {
			return nil, err
		}
		s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
		s.LocalRelay = 0
		streams = append(streams, s)
	}
	return streams, rows.Err()
}

// CreateStreamAdmin allows admins to create a source-based live channel.
func CreateStreamAdmin(c *fiber.Ctx) error {
	var req struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Source string `json:"source"`
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "محتوى الطلب غير صالح"})
	}

	req.ID = strings.TrimSpace(strings.ToLower(req.ID))
	req.Name = strings.TrimSpace(req.Name)
	req.Source = strings.TrimSpace(req.Source)
	if req.Status == "" {
		req.Status = "active"
	}

	if req.ID == "" || req.Name == "" || req.Source == "" {
		return c.Status(400).JSON(fiber.Map{"error": "جميع الحقول مطلوبة"})
	}
	if !isValidStreamSource(req.Source) {
		return c.Status(400).JSON(fiber.Map{"error": "مصدر البث يجب أن يكون رابط http أو https صالحاً"})
	}

	_, err := radius.DB.Exec(
		"INSERT INTO radius_streams (id, name, source, status, local_relay, created_at) VALUES (?, ?, ?, ?, 0, CURRENT_TIMESTAMP)",
		req.ID, req.Name, req.Source, req.Status,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل حفظ القناة: " + err.Error()})
	}

	log.Printf("[streaming] Channel created: %s (%s)", req.Name, req.ID)
	return c.JSON(fiber.Map{"message": "تم إضافة القناة بنجاح", "id": req.ID})
}

// UpdateStreamAdmin allows admins to update or toggle stream status.
func UpdateStreamAdmin(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "معرف القناة مطلوب"})
	}

	var req struct {
		Name   string `json:"name"`
		Source string `json:"source"`
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "محتوى الطلب غير صالح"})
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Source = strings.TrimSpace(req.Source)
	if req.Status == "" {
		req.Status = "active"
	}
	if req.Name == "" || req.Source == "" {
		return c.Status(400).JSON(fiber.Map{"error": "اسم القناة ومصدر البث مطلوبان"})
	}
	if !isValidStreamSource(req.Source) {
		return c.Status(400).JSON(fiber.Map{"error": "مصدر البث يجب أن يكون رابط http أو https صالحاً"})
	}

	_, err := radius.DB.Exec(
		"UPDATE radius_streams SET name=?, source=?, status=?, local_relay=0 WHERE id=?",
		req.Name, req.Source, req.Status, id,
	)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل تحديث القناة: " + err.Error()})
	}

	return c.JSON(fiber.Map{"message": "تم تحديث القناة بنجاح"})
}

// DeleteStreamAdmin allows admins to delete a live channel.
func DeleteStreamAdmin(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "معرف القناة مطلوب"})
	}

	_, err := radius.DB.Exec("DELETE FROM radius_streams WHERE id=?", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "فشل حذف القناة: " + err.Error()})
	}

	_, _ = radius.DB.Exec("DELETE FROM radius_stream_viewers WHERE stream_id=?", id)

	log.Printf("[streaming] Channel deleted: %s", id)
	return c.JSON(fiber.Map{"message": "تم حذف القناة بنجاح"})
}

func isValidStreamSource(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// ProxyExternalStream acts as a reverse proxy for HLS streams to bypass browser CORS blocks.
func ProxyExternalStream(c *fiber.Ctx) error {
	rawURL := c.Query("url")
	if rawURL == "" {
		return c.Status(400).SendString("URL parameter is required")
	}
	if !isValidStreamSource(rawURL) {
		return c.Status(400).SendString("Invalid stream URL")
	}

	return proxyGenericStream(c, rawURL)
}

func proxyGenericStream(c *fiber.Ctx, rawURL string) error {
	targetURL, err := url.Parse(rawURL)
	if err != nil {
		return c.Status(400).SendString("Invalid URL: " + err.Error())
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return c.Status(500).SendString("Failed to create request: " + err.Error())
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(502).SendString("Failed to fetch stream: " + err.Error())
	}
	defer resp.Body.Close()

	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	c.Set("Access-Control-Allow-Headers", "*")
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		c.Set("Content-Type", contentType)
	}

	isM3U8 := strings.Contains(strings.ToLower(targetURL.Path), ".m3u8") ||
		strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "mpegurl") ||
		strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/x-mpegurl")
	if !isM3U8 {
		_, err = io.Copy(c.Response().BodyWriter(), resp.Body)
		return err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(500).SendString("Failed to read response body: " + err.Error())
	}

	baseURL := *targetURL
	pathParts := strings.Split(baseURL.Path, "/")
	if len(pathParts) > 0 {
		baseURL.Path = strings.Join(pathParts[:len(pathParts)-1], "/") + "/"
	}

	lines := strings.Split(string(bodyBytes), "\n")
	rewrittenLines := make([]string, 0, len(lines))
	keyURIRe := regexp.MustCompile(`URI="([^"]+)"`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			rewrittenLines = append(rewrittenLines, line)
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			if strings.Contains(trimmed, `URI="`) {
				trimmed = keyURIRe.ReplaceAllStringFunc(trimmed, func(match string) string {
					matches := keyURIRe.FindStringSubmatch(match)
					if len(matches) < 2 {
						return match
					}
					return `URI="` + proxiedStreamURI(baseURL, matches[1]) + `"`
				})
			}
			rewrittenLines = append(rewrittenLines, trimmed)
			continue
		}

		rewrittenLines = append(rewrittenLines, proxiedStreamURI(baseURL, trimmed))
	}

	return c.SendString(strings.Join(rewrittenLines, "\n"))
}

func proxiedStreamURI(baseURL url.URL, uri string) string {
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return "/radius/api/portal/stream-proxy?url=" + url.QueryEscape(uri)
	}
	relURL, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	return "/radius/api/portal/stream-proxy?url=" + url.QueryEscape(baseURL.ResolveReference(relURL).String())
}
