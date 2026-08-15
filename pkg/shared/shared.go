package shared

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
)

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

func LoadData() {
	// Migrate if old file exists but new one doesn't
	if _, err := os.Stat("routing_data.json"); err == nil {
		if _, errNew := os.Stat("data/routing_data.json"); os.IsNotExist(errNew) {
			if errMigrate := moveFile("routing_data.json", "data/routing_data.json"); errMigrate == nil {
				log.Println("[Migration] Moved routing_data.json to data/ directory")
			} else {
				log.Printf("[Migration] Error moving routing_data.json: %v\n", errMigrate)
			}
		}
	}

	file, err := ioutil.ReadFile("data/routing_data.json")
	if err != nil {
		log.Printf("[Init] Warning: routing_data.json not found, starting fresh.\n")
		RoutingDataState.Apps = make(map[string][]string)
		RoutingDataState.Ips = make(map[string][]string)
		RoutingDataState.Games = make(map[string]GameConfig)
		return
	}
	json.Unmarshal(file, &RoutingDataState)
	log.Printf("[Init] Successfully loaded app groups and ip groups\n")
}

func LoadConfig() {
	// Migrate if old file exists but new one doesn't
	if _, err := os.Stat("config.json"); err == nil {
		if _, errNew := os.Stat("data/config.json"); os.IsNotExist(errNew) {
			if errMigrate := moveFile("config.json", "data/config.json"); errMigrate == nil {
				log.Println("[Migration] Moved config.json to data/ directory")
			} else {
				log.Printf("[Migration] Error moving config.json: %v\n", errMigrate)
			}
		}
	}

	file, err := ioutil.ReadFile("data/config.json")
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

func SaveConfig() {
	data, _ := json.MarshalIndent(RouterConfigState, "", "  ")
	_ = ioutil.WriteFile("data/config.json", data, 0644)
}

func RemoveConfig() {
	RouterConfigState.Address = ""
	RouterConfigState.Username = ""
	RouterConfigState.Password = ""
	RouterConfigState.Serial = ""
	_ = os.Remove("data/config.json")
}
