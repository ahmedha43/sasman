package broadcast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

var (
	storeMu          sync.RWMutex
	activeBroadcasts = make(map[string]BroadcastMessage)
	cacheFilePath    string
)

func InitStore(dataDir string) {
	if dataDir == "" {
		dataDir = "data"
	}
	_ = os.MkdirAll(dataDir, 0755)
	cacheFilePath = filepath.Join(dataDir, "broadcasts_cache.json")
	loadFromDisk()
}

func loadFromDisk() {
	if cacheFilePath == "" {
		return
	}
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return
	}
	var list []BroadcastMessage
	if err := json.Unmarshal(data, &list); err == nil {
		storeMu.Lock()
		for _, b := range list {
			activeBroadcasts[b.ID] = b
		}
		storeMu.Unlock()
	}
}

func saveToDisk() {
	if cacheFilePath == "" {
		return
	}
	storeMu.RLock()
	var list []BroadcastMessage
	for _, b := range activeBroadcasts {
		list = append(list, b)
	}
	storeMu.RUnlock()

	data, err := json.Marshal(list)
	if err == nil {
		_ = os.WriteFile(cacheFilePath, data, 0644)
	}
}

// StoreActiveBroadcast adds or updates a broadcast message received from the server
func StoreActiveBroadcast(bc BroadcastMessage) {
	storeMu.Lock()
	activeBroadcasts[bc.ID] = bc
	storeMu.Unlock()
	saveToDisk()
}

// GetActiveBroadcasts returns all currently active broadcast messages
func GetActiveBroadcasts() []BroadcastMessage {
	storeMu.RLock()
	defer storeMu.RUnlock()

	var list []BroadcastMessage
	for _, b := range activeBroadcasts {
		list = append(list, b)
	}
	return list
}

// DismissBroadcast removes a broadcast from the local store
func DismissBroadcast(id string) {
	storeMu.Lock()
	delete(activeBroadcasts, id)
	storeMu.Unlock()
	saveToDisk()
}

// RegisterRoutes registers broadcast endpoints on the Agent Fiber app
func RegisterRoutes(app *fiber.App, onLog func(bLog BroadcastLogPayload)) {
	// API for agent web UI to fetch active broadcasts
	app.Get("/api/broadcasts/active", func(c *fiber.Ctx) error {
		return c.JSON(GetActiveBroadcasts())
	})

	// Also accessible under /radius/api/broadcasts/active
	app.Get("/radius/api/broadcasts/active", func(c *fiber.Ctx) error {
		return c.JSON(GetActiveBroadcasts())
	})

	// API for agent web UI to log view or click
	app.Post("/api/broadcasts/log", func(c *fiber.Ctx) error {
		var payload struct {
			BroadcastID    string `json:"broadcast_id"`
			UserIdentifier string `json:"user_identifier"`
			Clicked        int    `json:"clicked"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		if onLog != nil {
			onLog(BroadcastLogPayload{
				BroadcastID:    payload.BroadcastID,
				UserIdentifier: payload.UserIdentifier,
				ViewedAt:       time.Now().UTC(),
				Clicked:        payload.Clicked,
			})
		}
		return c.JSON(fiber.Map{"success": true})
	})

	app.Post("/radius/api/broadcasts/log", func(c *fiber.Ctx) error {
		var payload struct {
			BroadcastID    string `json:"broadcast_id"`
			UserIdentifier string `json:"user_identifier"`
			Clicked        int    `json:"clicked"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		if onLog != nil {
			onLog(BroadcastLogPayload{
				BroadcastID:    payload.BroadcastID,
				UserIdentifier: payload.UserIdentifier,
				ViewedAt:       time.Now().UTC(),
				Clicked:        payload.Clicked,
			})
		}
		return c.JSON(fiber.Map{"success": true})
	})
}
