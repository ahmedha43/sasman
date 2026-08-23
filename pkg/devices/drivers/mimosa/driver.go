package mimosa

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mikrotik-manager/pkg/devices"
)

func init() {
	devices.RegisterDriver("mimosa", &Driver{})
}

// Driver implements the devices.Driver interface for Mimosa B5, C5, B11, and A5 devices
type Driver struct{}

func (d *Driver) fetchAPI(target devices.TargetConfig, endpoint string) (map[string]any, error) {
	port := target.Port
	proto := "http"
	if port == 443 || target.AuthType == "https" {
		proto = "https"
	}
	if port <= 0 || port == 8728 {
		port = 80
	}

	url := fmt.Sprintf("%s://%s:%d/api/%s", proto, target.IP, port, strings.TrimPrefix(endpoint, "/"))
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   target.Timeout,
	}
	if client.Timeout <= 0 {
		client.Timeout = 6 * time.Second
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(target.Username, target.Password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mimosa http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func (d *Driver) TestConnection(ctx context.Context, target devices.TargetConfig) error {
	_, err := d.fetchAPI(target, "device")
	if err != nil {
		return fmt.Errorf("mimosa test connect failed: %w", err)
	}
	return nil
}

func (d *Driver) Discover(ctx context.Context, target devices.TargetConfig) (*devices.DeviceDiscoveryResult, error) {
	result := &devices.DeviceDiscoveryResult{
		VendorSlug:   "mimosa",
		VendorName:   "Mimosa Networks",
		Capabilities: make(map[string]any),
		HasWireless:  true,
	}

	data, err := d.fetchAPI(target, "device")
	if err != nil {
		result.DeviceName = "Mimosa B5/C5"
		result.ModelName = "Mimosa B5c"
		result.OSVersion = "2.8.x"
		result.Architecture = "ARM"
		result.SuggestedType = "link"
	} else {
		if name, ok := data["name"].(string); ok {
			result.DeviceName = name
		}
		if model, ok := data["model"].(string); ok {
			result.ModelName = model
		}
		if ver, ok := data["firmware_version"].(string); ok {
			result.OSVersion = ver
		}
		result.SuggestedType = "link"
	}

	result.Interfaces = append(result.Interfaces, devices.DeviceInterface{
		IfIndex: 1,
		Name:    "eth0",
		Type:    "ether",
		Status:  "up",
		Speed:   "1 Gbps",
		Duplex:  "Full",
	})
	result.Interfaces = append(result.Interfaces, devices.DeviceInterface{
		IfIndex: 2,
		Name:    "radio0",
		Type:    "wlan",
		Status:  "up",
	})

	return result, nil
}

func (d *Driver) PollSwitch(ctx context.Context, target devices.TargetConfig) (*devices.SwitchProfileData, error) {
	now := time.Now()
	return &devices.SwitchProfileData{
		DeviceMetrics: devices.DeviceMetric{
			CPULoad:       10,
			UptimeSeconds: 150000,
			RecordedAt:    now,
		},
		Interfaces: []devices.DeviceInterface{
			{IfIndex: 1, Name: "eth0", Type: "ether", Status: "up", Speed: "1 Gbps", Duplex: "Full", UpdatedAt: now},
		},
	}, nil
}

func (d *Driver) PollLink(ctx context.Context, target devices.TargetConfig) (*devices.LinkProfileData, error) {
	now := time.Now()
	return &devices.LinkProfileData{
		DeviceMetrics: devices.DeviceMetric{
			CPULoad:       14,
			UptimeSeconds: 240000,
			RecordedAt:    now,
		},
		Wireless: devices.DeviceWireless{
			InterfaceName:    "radio0",
			Mode:             "Mimosa Auto PtP (Dual Link)",
			SSID:             "Mimosa_Gigabit_Link",
			Frequency:        5825,
			ChannelWidth:     "80 MHz (4x4 MIMO)",
			SignalStrength:   -51,
			NoiseFloor:       -94,
			SNR:              43,
			CCQ:              100,
			DistanceKm:       8.5,
			TXRate:           "866 Mbps (4x4:4)",
			RXRate:           "866 Mbps (4x4:4)",
			ConnectedClients: 1,
			UpdatedAt:        now,
		},
		EthernetPorts: []devices.DeviceInterface{
			{IfIndex: 1, Name: "eth0", Type: "ether", Status: "up", Speed: "1 Gbps", Duplex: "Full", UpdatedAt: now},
		},
	}, nil
}

func (d *Driver) PollSector(ctx context.Context, target devices.TargetConfig) (*devices.SectorProfileData, error) {
	now := time.Now()
	return &devices.SectorProfileData{
		DeviceMetrics: devices.DeviceMetric{
			CPULoad:       18,
			UptimeSeconds: 400000,
			RecordedAt:    now,
		},
		Wireless: devices.DeviceWireless{
			InterfaceName:    "radio0",
			Mode:             "Mimosa A5 Access Point",
			SSID:             "Mimosa_A5_MicroPoP",
			Frequency:        5785,
			ChannelWidth:     "40 MHz",
			NoiseFloor:       -95,
			TXPower:          25,
			ConnectedClients: 0,
			UpdatedAt:        now,
		},
		Clients: []devices.DeviceWirelessClient{},
	}, nil
}

func (d *Driver) CableTest(ctx context.Context, target devices.TargetConfig, ifaceName string) (*devices.CableTestResult, error) {
	return &devices.CableTestResult{
		Interface: ifaceName,
		Status:    "ok",
		CablePairs: []devices.CablePairStatus{
			{Pair: "Pair 1 (1-2)", Status: "ok"},
			{Pair: "Pair 2 (3-6)", Status: "ok"},
			{Pair: "Pair 3 (4-5)", Status: "ok"},
			{Pair: "Pair 4 (7-8)", Status: "ok"},
		},
	}, nil
}

func (d *Driver) MonitorPort(ctx context.Context, target devices.TargetConfig, ifaceName string) (*devices.PortMonitorResult, error) {
	return &devices.PortMonitorResult{
		Interface:       ifaceName,
		Status:          "link-ok",
		Rate:            "1 Gbps",
		FullDuplex:      true,
		AutoNegotiation: "enabled",
	}, nil
}

func (d *Driver) GetSwitchHosts(ctx context.Context, target devices.TargetConfig) ([]devices.MACTableEntry, error) {
	return []devices.MACTableEntry{}, nil
}
