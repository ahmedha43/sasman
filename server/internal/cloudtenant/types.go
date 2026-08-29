package cloudtenant

import (
	"time"
)

type CloudTenant struct {
	ID           string    `json:"id"`
	Subdomain    string    `json:"subdomain"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	OwnerName    string    `json:"owner_name"`
	Phone        string    `json:"phone"`
	Status       string    `json:"status"` // active, suspended
	Plan         string    `json:"plan"`   // cloud_free, cloud_pro
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RegisterRequest struct {
	Subdomain string `json:"subdomain"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	OwnerName string `json:"owner_name"`
	Phone     string `json:"phone"`
}

type LoginRequest struct {
	Subdomain string `json:"subdomain"` // Can be subdomain or email
	Password  string `json:"password"`
}

type LoginResponse struct {
	Success          bool         `json:"success"`
	Token            string       `json:"token"`
	Tenant           *CloudTenant `json:"tenant"`
	InstallScriptURL string       `json:"install_script_url"`
	RadSecAddress    string       `json:"radsec_address"`
	Error            string       `json:"error,omitempty"`
}

type CloudAccountingPayload struct {
	Username       string `json:"username"`
	StatusType     string `json:"status_type"` // Start, Stop, Interim-Update
	SessionID      string `json:"session_id"`
	UserIP         string `json:"user_ip"`
	UserMAC        string `json:"user_mac"`
	NasIP          string `json:"nas_ip"`
	BytesIn        int64  `json:"bytes_in"`
	BytesOut       int64  `json:"bytes_out"`
	SessionTimeSec int64  `json:"session_time_sec"`
	TerminateCause uint32 `json:"terminate_cause"`
}
