package shared

import (
	_ "embed"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

//go:embed default_routing_data.json
var defaultRoutingDataBytes []byte

// RoutingData models
type GameConfig struct {
	Protocol string `json:"protocol"`
	Ports    string `json:"ports"`
}

type RoutingData struct {
	Apps  map[string][]string   `json:"apps"`
	Ips   map[string][]string   `json:"ips"`
	Games map[string]GameConfig `json:"games"`
}

var RoutingDataState RoutingData

// RouterConfig models
type RouterConfig struct {
	Address          string `json:"address"`
	Username         string `json:"user"`
	Password         string `json:"pass"`
	License          string `json:"license"`
	Serial           string `json:"serial"`
	NgrokToken       string `json:"ngrok_token"`
	TunnelMode       string `json:"tunnel_mode"`
	TunnelSubdomain  string `json:"tunnel_subdomain"`
	TunnelToken      string `json:"tunnel_token"`
	TunnelGatewayURL string `json:"tunnel_gateway_url"`
	CentralDomain    string `json:"central_domain"`
	OwnerName             string `json:"owner_name"`
	OwnerPhone            string `json:"owner_phone"`
	WinboxPort            int    `json:"winbox_port"`
	CloudLicenseStatus    string `json:"cloud_license_status"`
	CloudLicenseExpiresAt string `json:"cloud_license_expires_at"`
	CloudLicenseDaysLeft  int    `json:"cloud_license_days_left"`
	CloudLicenseValid     bool   `json:"cloud_license_valid"`
	SetupCompleted        bool   `json:"setup_completed"`
}

var RouterConfigState RouterConfig

const PUBLIC_KEY_HEX = "7c29c8f0bb2041bfb51d5590ad7cc5e17d1fb520d3465f5a2891f5d80ad4413e"

func moveFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	_ = in.Close()
	return os.Remove(src)
}

func GetDataDir() string {
	if os.Getenv("SASMAN_DATA_DIR") != "" {
		return os.Getenv("SASMAN_DATA_DIR")
	}
	if _, err := os.Stat("/app/data"); err == nil {
		return "/app/data"
	}
	return "data"
}

func LoadData() {
	dataDir := GetDataDir()
	_ = os.MkdirAll(dataDir, 0755)
	routingPath := filepath.Join(dataDir, "routing_data.json")

	// Migrate if old file exists in root
	if _, err := os.Stat("routing_data.json"); err == nil && routingPath != "routing_data.json" {
		if _, errNew := os.Stat(routingPath); os.IsNotExist(errNew) {
			_ = moveFile("routing_data.json", routingPath)
		}
	}

	file, err := ioutil.ReadFile(routingPath)
	if err != nil {
		log.Printf("[Init] routing_data.json not found at %s, seeding from embedded defaults.\n", routingPath)
		if len(defaultRoutingDataBytes) > 0 {
			_ = ioutil.WriteFile(routingPath, defaultRoutingDataBytes, 0644)
			json.Unmarshal(defaultRoutingDataBytes, &RoutingDataState)
			log.Printf("[Init] Successfully seeded and loaded default app groups and ip groups\n")
			return
		}
		RoutingDataState.Apps = make(map[string][]string)
		RoutingDataState.Ips = make(map[string][]string)
		RoutingDataState.Games = make(map[string]GameConfig)
		return
	}
	json.Unmarshal(file, &RoutingDataState)
	log.Printf("[Init] Successfully loaded app groups and ip groups\n")
}

func LoadConfig() {
	dataDir := GetDataDir()
	_ = os.MkdirAll(dataDir, 0755)
	configPath := filepath.Join(dataDir, "config.json")

	// Migrate if old file exists in root
	if _, err := os.Stat("config.json"); err == nil && configPath != "config.json" {
		if _, errNew := os.Stat(configPath); os.IsNotExist(errNew) {
			_ = moveFile("config.json", configPath)
		}
	}

	file, err := ioutil.ReadFile(configPath)
	if err != nil {
		return
	}
	json.Unmarshal(file, &RouterConfigState)

	if os.Getenv("SASMAN_TUNNEL_MODE") != "" {
		RouterConfigState.TunnelMode = os.Getenv("SASMAN_TUNNEL_MODE")
	}
	if os.Getenv("SASMAN_SUBDOMAIN") != "" {
		RouterConfigState.TunnelSubdomain = os.Getenv("SASMAN_SUBDOMAIN")
	}
	if os.Getenv("SASMAN_TUNNEL_TOKEN") != "" {
		RouterConfigState.TunnelToken = os.Getenv("SASMAN_TUNNEL_TOKEN")
	}
	if os.Getenv("SASMAN_TUNNEL_GATEWAY_URL") != "" {
		RouterConfigState.TunnelGatewayURL = os.Getenv("SASMAN_TUNNEL_GATEWAY_URL")
	}
	if os.Getenv("SASMAN_CENTRAL_DOMAIN") != "" {
		RouterConfigState.CentralDomain = os.Getenv("SASMAN_CENTRAL_DOMAIN")
	}

	log.Printf("[Config] Loaded previous session for %s\n", RouterConfigState.Address)
}

var OnConfigSaved func()

func SaveConfig() {
	data, _ := json.MarshalIndent(RouterConfigState, "", "  ")
	configPath := filepath.Join(GetDataDir(), "config.json")
	_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	_ = ioutil.WriteFile(configPath, data, 0644)
	if OnConfigSaved != nil {
		go OnConfigSaved()
	}
}

func RemoveConfig() {
	RouterConfigState.Address = ""
	RouterConfigState.Username = ""
	RouterConfigState.Password = ""
	RouterConfigState.Serial = ""
	configPath := filepath.Join(GetDataDir(), "config.json")
	_ = os.Remove(configPath)
	if OnConfigSaved != nil {
		go OnConfigSaved()
	}
}
