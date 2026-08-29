package radius

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type ShutdownConfig struct {
	Enabled       bool     `json:"enabled"`
	Dates         []string `json:"dates"`          // e.g. ["2026-06-02", "2026-06-06"]
	StartTime     string   `json:"start_time"`     // e.g. "05:59:55"
	EndTime       string   `json:"end_time"`       // e.g. "07:30:00"
	ExcludedUsers []string `json:"excluded_users"` // Usernames to exclude
}

// Global state to track previous blackout state
var lastShutdownState bool = false

func IsShutdownActive() bool {
	config, err := LoadShutdownConfigFromDB()
	if err != nil || !config.Enabled {
		return false
	}

	now := time.Now().In(baghdadLocation)
	todayStr := now.Format("2006-01-02")

	// Check if today is one of the blackout dates
	dateMatched := false
	for _, d := range config.Dates {
		if strings.TrimSpace(d) == todayStr {
			dateMatched = true
			break
		}
	}

	if !dateMatched {
		return false
	}

	// Parse start and end times for today
	startTime, err := parseTimeToday(config.StartTime, now)
	if err != nil {
		return false
	}

	endTime, err := parseTimeToday(config.EndTime, now)
	if err != nil {
		return false
	}

	// Active if now is between start and end
	return now.After(startTime) && now.Before(endTime)
}

func IsShutdownActiveForUser(username string) bool {
	if !IsShutdownActive() {
		return false
	}

	config, err := LoadShutdownConfigFromDB()
	if err != nil {
		return true // Fallback to safe default
	}

	for _, u := range config.ExcludedUsers {
		if strings.EqualFold(strings.TrimSpace(u), strings.TrimSpace(username)) {
			return false // Excluded!
		}
	}

	return true
}

func parseTimeToday(timeStr string, now time.Time) (time.Time, error) {
	timeStr = strings.TrimSpace(timeStr)
	var parsedTime time.Time
	var err error

	// Try format "15:04:05"
	parsedTime, err = time.ParseInLocation("15:04:05", timeStr, baghdadLocation)
	if err != nil {
		// Try format "15:04"
		parsedTime, err = time.ParseInLocation("15:04", timeStr, baghdadLocation)
		if err != nil {
			return time.Time{}, err
		}
	}

	// Combine today's date with the parsed time
	result := time.Date(now.Year(), now.Month(), now.Day(), parsedTime.Hour(), parsedTime.Minute(), parsedTime.Second(), 0, baghdadLocation)
	return result, nil
}

func LoadShutdownConfigFromDB() (ShutdownConfig, error) {
	var config ShutdownConfig
	var val string
	err := DB.QueryRow("SELECT value FROM radius_system_settings WHERE `key`='internet_shutdown_config' LIMIT 1").Scan(&val)
	if err != nil {
		// Return disabled default if missing
		return ShutdownConfig{Enabled: false, Dates: []string{}, StartTime: "06:00:00", EndTime: "07:30:00"}, nil
	}

	err = json.Unmarshal([]byte(val), &config)
	if err != nil {
		return ShutdownConfig{Enabled: false, Dates: []string{}, StartTime: "06:00:00", EndTime: "07:30:00"}, err
	}

	return config, nil
}

func SaveShutdownConfigInDB(config ShutdownConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`INSERT INTO radius_system_settings (key, value, updated_at) 
	                 VALUES ('internet_shutdown_config', ?, CURRENT_TIMESTAMP)
	                 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, string(data))
	return err
}

func DisconnectAllActiveUsers() {
	log.Println("[shutdown] Scheduled shutdown triggered! Kicking active users...")
	sessions, err := LoadSessionsFromDB()
	if err != nil {
		log.Printf("[shutdown] Failed to load active sessions to disconnect: %v", err)
		return
	}

	config, _ := LoadShutdownConfigFromDB()

	count := 0
	for _, info := range sessions {
		if info.Online {
			targetName := info.Username
			if targetName == "" {
				continue
			}

			// Check if user is excluded from shutdown
			isExcluded := false
			for _, u := range config.ExcludedUsers {
				if strings.EqualFold(strings.TrimSpace(u), strings.TrimSpace(targetName)) {
					isExcluded = true
					break
				}
			}

			if isExcluded {
				log.Printf("[shutdown] Skipping active user [%s] (EXCLUDED from shutdown)...", targetName)
				continue
			}

			log.Printf("[shutdown] Kicking active user [%s] due to scheduled shutdown...", targetName)
			if err := DisconnectUserSession(targetName, info); err != nil {
				log.Printf("[shutdown] Failed to disconnect %s: %v", targetName, err)
			} else {
				count++
			}
		}
	}
	log.Printf("[shutdown] Completed kicking active users. Total kicked: %d", count)
}

func StartShutdownMonitor() {
	// Periodic check every 5 seconds
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Set initial state
		lastShutdownState = IsShutdownActive()
		if lastShutdownState {
			log.Println("[shutdown] Started during an active shutdown period.")
		}

		for range ticker.C {
			currentState := IsShutdownActive()
			if currentState && !lastShutdownState {
				log.Println("[shutdown] Transitioned into shutdown period! Disconnecting active users...")
				DisconnectAllActiveUsers()
			}
			lastShutdownState = currentState
		}
	}()
}

// API Handlers
func GetShutdownConfig(c *fiber.Ctx) error {
	config, err := LoadShutdownConfigFromDB()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to load shutdown config: " + err.Error()})
	}
	return c.JSON(config)
}

func SaveShutdownConfig(c *fiber.Ctx) error {
	var req ShutdownConfig
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	req.StartTime = strings.TrimSpace(req.StartTime)
	req.EndTime = strings.TrimSpace(req.EndTime)

	// Clean up dates
	var cleanDates []string
	for _, d := range req.Dates {
		d = strings.TrimSpace(d)
		if d != "" {
			cleanDates = append(cleanDates, d)
		}
	}
	req.Dates = cleanDates

	// Clean up excluded users
	var cleanExcluded []string
	for _, u := range req.ExcludedUsers {
		u = strings.TrimSpace(u)
		if u != "" {
			cleanExcluded = append(cleanExcluded, u)
		}
	}
	req.ExcludedUsers = cleanExcluded

	if req.StartTime == "" || req.EndTime == "" {
		return c.Status(400).JSON(fiber.Map{"error": "وقت البدء ووقت الانتهاء مطلوبان"})
	}

	// Simple validation to ensure start/end can be parsed
	now := time.Now().In(baghdadLocation)
	if _, err := parseTimeToday(req.StartTime, now); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "صيغة وقت البدء غير صالحة، يرجى كتابتها بالصيغة HH:MM:SS أو HH:MM"})
	}
	if _, err := parseTimeToday(req.EndTime, now); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "صيغة وقت الانتهاء غير صالحة، يرجى كتابتها بالصيغة HH:MM:SS أو HH:MM"})
	}

	if err := SaveShutdownConfigInDB(req); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save shutdown config: " + err.Error()})
	}

	// Update last state immediately so we don't double trigger if saved during active state
	lastShutdownState = IsShutdownActive()

	log.Printf("[shutdown] Updated shutdown configuration: Enabled=%v, Dates=%v, ExcludedUsers=%v, Start=%s, End=%s", req.Enabled, req.Dates, req.ExcludedUsers, req.StartTime, req.EndTime)
	return c.JSON(fiber.Map{"message": "تم حفظ إعدادات جدول قطع الخدمة المجدول بنجاح"})
}
