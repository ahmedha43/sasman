package broadcast

import "time"

// BroadcastMessage is the wire payload pushed to agents via WebSocket
type BroadcastMessage struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Message           string `json:"message"`
	ImageURL          string `json:"image_url"`
	ActionURL         string `json:"action_url"`
	ActionText        string `json:"action_text"`
	DisplayType       string `json:"display_type"`        // 'banner' | 'modal' | 'splash'
	TargetType        string `json:"target_type"`         // 'agents' | 'users' | 'both'
	TargetProfiles    string `json:"target_profiles"`     // 'ALL' or comma-separated
	Frequency         string `json:"frequency"`           // 'once' | 'daily' | 'always'
	SplashDurationSec int    `json:"splash_duration_sec"` // seconds
	CreatedAt         string `json:"created_at"`
}

// BroadcastLogPayload is the report sent back from agent to server
type BroadcastLogPayload struct {
	BroadcastID    string    `json:"broadcast_id"`
	AgentID        string    `json:"agent_id"`
	UserIdentifier string    `json:"user_identifier"`
	ViewedAt       time.Time `json:"viewed_at"`
	Clicked        int       `json:"clicked"`
}
