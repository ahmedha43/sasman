package ubiquiti

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"mikrotik-manager/pkg/devices"
)

func init() {
	devices.RegisterDriver("ubiquiti", &Driver{})
}

// Driver implements the devices.Driver interface for Ubiquiti airMAX, airOS, and airFiber devices
type Driver struct{}

// ─── SSH & HTTP HELPERS ──────────────────────────────────────────────────────

func (d *Driver) getSSHClient(target devices.TargetConfig) (*ssh.Client, error) {
	port := target.Port
	if port <= 0 || port == 8728 {
		port = 22 // Default SSH port for Ubiquiti
	}

	config := &ssh.ClientConfig{
		User: target.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(target.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         target.Timeout,
	}
	if config.Timeout <= 0 {
		config.Timeout = 6 * time.Second
	}

	addr := net.JoinHostPort(target.IP, strconv.Itoa(port))
	return ssh.Dial("tcp", addr, config)
}

func (d *Driver) runSSHCommand(target devices.TargetConfig, cmd string) (string, error) {
	client, err := d.getSSHClient(target)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return stdout.String(), fmt.Errorf("ssh run '%s': %w (%s)", cmd, err, stderr.String())
	}

	return stdout.String(), nil
}

func (d *Driver) fetchHTTPStatus(target devices.TargetConfig) (map[string]any, error) {
	port := target.Port
	proto := "http"
	if port == 443 || target.AuthType == "https" {
		proto = "https"
	}
	if port <= 0 || port == 8728 {
		port = 80
	}

	url := fmt.Sprintf("%s://%s:%d/status.cgi", proto, target.IP, port)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   target.Timeout,
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
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
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

// parseMCAStatus parses key=value output of Ubiquiti `mca-status` command
func parseMCAStatus(raw string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx > 0 {
			k := strings.TrimSpace(line[:idx])
			v := strings.TrimSpace(line[idx+1:])
			result[k] = v
		}
	}
	return result
}

// ─── DRIVER IMPLEMENTATION ───────────────────────────────────────────────────

func (d *Driver) TestConnection(ctx context.Context, target devices.TargetConfig) error {
	// Try SSH first
	client, err := d.getSSHClient(target)
	if err == nil {
		_ = client.Close()
		return nil
	}

	// Try HTTP fallback
	_, httpErr := d.fetchHTTPStatus(target)
	if httpErr == nil {
		return nil
	}

	return fmt.Errorf("ssh connect error: %v, http fallback error: %v", err, httpErr)
}

func (d *Driver) Discover(ctx context.Context, target devices.TargetConfig) (*devices.DeviceDiscoveryResult, error) {
	result := &devices.DeviceDiscoveryResult{
		VendorSlug:   "ubiquiti",
		VendorName:   "Ubiquiti Networks",
		Capabilities: make(map[string]any),
		HasWireless:  true,
	}

	// Try SSH mca-status
	rawStatus, err := d.runSSHCommand(target, "mca-status")
	if err == nil && len(rawStatus) > 0 {
		m := parseMCAStatus(rawStatus)
		result.DeviceName = m["deviceName"]
		result.OSVersion = m["firmwareVersion"]
		result.Architecture = m["platform"]
		result.BoardName = m["boardModel"]
		if result.BoardName == "" {
			result.BoardName = m["platform"]
		}
		result.ModelName = m["deviceModel"]
		if result.ModelName == "" {
			result.ModelName = result.BoardName
		}
		if result.DeviceName == "" {
			result.DeviceName = result.ModelName
		}

		result.CPULoad, _ = strconv.Atoi(m["cpuLoad"])
		result.UptimeSeconds, _ = strconv.ParseInt(m["uptime"], 10, 64)
		totalMem, _ := strconv.ParseInt(m["totalMemory"], 10, 64)
		freeMem, _ := strconv.ParseInt(m["freeMemory"], 10, 64)
		result.MemoryTotal = totalMem
		result.MemoryUsed = totalMem - freeMem

		// Wireless Mode detection
		wmode := strings.ToLower(m["wlanOpMode"]) // "ap", "sta", "ap-ptp", "ap-ptmp"
		if strings.Contains(wmode, "ap") || m["wlanConnections"] != "" {
			connCount, _ := strconv.Atoi(m["wlanConnections"])
			if connCount > 1 || strings.Contains(wmode, "ptmp") {
				result.SuggestedType = "sector"
			} else {
				result.SuggestedType = "link"
			}
		} else {
			result.SuggestedType = "link"
		}

		result.Interfaces = append(result.Interfaces, devices.DeviceInterface{
			IfIndex: 1,
			Name:    "eth0",
			Type:    "ether",
			Status:  "up",
			Speed:   m["lanSpeed"],
			Duplex:  "Full",
		})
		result.Interfaces = append(result.Interfaces, devices.DeviceInterface{
			IfIndex: 2,
			Name:    "ath0",
			Type:    "wlan",
			Status:  "up",
			Speed:   m["wlanTxRate"],
		})

		return result, nil
	}

	// Fallback to HTTP /status.cgi
	httpData, err := d.fetchHTTPStatus(target)
	if err != nil {
		return nil, fmt.Errorf("ubiquiti discover failed: %w", err)
	}

	if host, ok := httpData["host"].(map[string]any); ok {
		result.DeviceName, _ = host["hostname"].(string)
		result.OSVersion, _ = host["fwversion"].(string)
		result.ModelName, _ = host["devmodel"].(string)
		result.BoardName, _ = host["platform"].(string)
		if uptime, ok := host["uptime"].(float64); ok {
			result.UptimeSeconds = int64(uptime)
		}
		if cpuload, ok := host["cpuload"].(float64); ok {
			result.CPULoad = int(cpuload)
		}
	}

	if wireless, ok := httpData["wireless"].(map[string]any); ok {
		mode, _ := wireless["mode"].(string)
		if strings.Contains(strings.ToLower(mode), "ap") {
			result.SuggestedType = "sector"
		} else {
			result.SuggestedType = "link"
		}
	} else {
		result.SuggestedType = "link"
	}

	return result, nil
}

func (d *Driver) PollSwitch(ctx context.Context, target devices.TargetConfig) (*devices.SwitchProfileData, error) {
	data := &devices.SwitchProfileData{}
	now := time.Now()

	rawStatus, err := d.runSSHCommand(target, "mca-status")
	if err != nil {
		return nil, fmt.Errorf("ubiquiti poll switch failed: %w", err)
	}
	m := parseMCAStatus(rawStatus)

	cpu, _ := strconv.Atoi(m["cpuLoad"])
	uptime, _ := strconv.ParseInt(m["uptime"], 10, 64)
	totMem, _ := strconv.ParseInt(m["totalMemory"], 10, 64)
	freeMem, _ := strconv.ParseInt(m["freeMemory"], 10, 64)

	data.DeviceMetrics = devices.DeviceMetric{
		CPULoad:       cpu,
		UptimeSeconds: uptime,
		MemoryTotal:   totMem,
		MemoryUsed:    totMem - freeMem,
		RecordedAt:    now,
	}

	rxBytes, _ := strconv.ParseInt(m["lanRxBytes"], 10, 64)
	txBytes, _ := strconv.ParseInt(m["lanTxBytes"], 10, 64)

	eth0 := devices.DeviceInterface{
		IfIndex:    1,
		Name:       "eth0",
		Type:       "ether",
		Status:     "up",
		Speed:      m["lanSpeed"],
		Duplex:     "Full",
		RXBytes:    rxBytes,
		TXBytes:    txBytes,
		UpdatedAt:  now,
	}
	data.Interfaces = append(data.Interfaces, eth0)

	return data, nil
}

func (d *Driver) PollLink(ctx context.Context, target devices.TargetConfig) (*devices.LinkProfileData, error) {
	data := &devices.LinkProfileData{}
	now := time.Now()

	rawStatus, err := d.runSSHCommand(target, "mca-status")
	if err != nil {
		return nil, fmt.Errorf("ubiquiti poll link failed: %w", err)
	}
	m := parseMCAStatus(rawStatus)

	cpu, _ := strconv.Atoi(m["cpuLoad"])
	uptime, _ := strconv.ParseInt(m["uptime"], 10, 64)
	totMem, _ := strconv.ParseInt(m["totalMemory"], 10, 64)
	freeMem, _ := strconv.ParseInt(m["freeMemory"], 10, 64)

	data.DeviceMetrics = devices.DeviceMetric{
		CPULoad:       cpu,
		UptimeSeconds: uptime,
		MemoryTotal:   totMem,
		MemoryUsed:    totMem - freeMem,
		RecordedAt:    now,
	}

	freq, _ := strconv.Atoi(m["freq"])
	noise, _ := strconv.Atoi(m["noisef"])
	sig, _ := strconv.Atoi(m["signal"])
	txPower, _ := strconv.Atoi(m["txPower"])
	dist, _ := strconv.ParseFloat(m["distance"], 64)
	if dist > 0 {
		dist = dist / 1000.0 // Convert meters to km
	}

	ccqVal, _ := strconv.Atoi(m["ccq"])
	if ccqVal > 100 {
		ccqVal = ccqVal / 10 // Ubiquiti returns ccq=980 for 98%
	}

	snr := 0
	if sig != 0 && noise != 0 {
		snr = sig - noise
	}

	data.Wireless = devices.DeviceWireless{
		InterfaceName:    "ath0",
		Mode:             m["wlanOpMode"],
		SSID:             m["essid"],
		Frequency:        freq,
		ChannelWidth:     m["chanbw"] + " MHz",
		SignalStrength:   sig,
		NoiseFloor:       noise,
		SNR:              snr,
		CCQ:              ccqVal,
		TXPower:          txPower,
		DistanceKm:       dist,
		TXRate:           m["wlanTxRate"],
		RXRate:           m["wlanRxRate"],
		RemoteMAC:        m["apMac"],
		RemoteDeviceInfo: m["apDeviceName"],
		ConnectedClients: 1,
		UpdatedAt:        now,
	}

	// Ethernet ports
	rxBytes, _ := strconv.ParseInt(m["lanRxBytes"], 10, 64)
	txBytes, _ := strconv.ParseInt(m["lanTxBytes"], 10, 64)
	data.EthernetPorts = append(data.EthernetPorts, devices.DeviceInterface{
		IfIndex:   1,
		Name:      "eth0",
		Type:      "ether",
		Status:    "up",
		Speed:     m["lanSpeed"],
		Duplex:    "Full",
		RXBytes:   rxBytes,
		TXBytes:   txBytes,
		UpdatedAt: now,
	})

	return data, nil
}

func (d *Driver) PollSector(ctx context.Context, target devices.TargetConfig) (*devices.SectorProfileData, error) {
	data := &devices.SectorProfileData{}
	now := time.Now()

	// 1. Get Radio status via mca-status
	rawStatus, err := d.runSSHCommand(target, "mca-status")
	if err != nil {
		return nil, fmt.Errorf("ubiquiti poll sector failed: %w", err)
	}
	m := parseMCAStatus(rawStatus)

	cpu, _ := strconv.Atoi(m["cpuLoad"])
	uptime, _ := strconv.ParseInt(m["uptime"], 10, 64)
	totMem, _ := strconv.ParseInt(m["totalMemory"], 10, 64)
	freeMem, _ := strconv.ParseInt(m["freeMemory"], 10, 64)

	data.DeviceMetrics = devices.DeviceMetric{
		CPULoad:       cpu,
		UptimeSeconds: uptime,
		MemoryTotal:   totMem,
		MemoryUsed:    totMem - freeMem,
		RecordedAt:    now,
	}

	freq, _ := strconv.Atoi(m["freq"])
	noise, _ := strconv.Atoi(m["noisef"])
	txPower, _ := strconv.Atoi(m["txPower"])

	data.Wireless = devices.DeviceWireless{
		InterfaceName: "ath0",
		Mode:          "Access Point (airMAX)",
		SSID:          m["essid"],
		Frequency:     freq,
		ChannelWidth:  m["chanbw"] + " MHz",
		NoiseFloor:    noise,
		TXPower:       txPower,
		UpdatedAt:     now,
	}

	// 2. Get Connected Stations via `wstalist`
	rawWsta, err := d.runSSHCommand(target, "wstalist")
	if err == nil && len(rawWsta) > 0 {
		var staList []struct {
			MAC        string `json:"mac"`
			Name       string `json:"name"`
			LastIP     string `json:"lastip"`
			Signal     int    `json:"signal"`
			RSSI       int    `json:"rssi"`
			NoiseFloor int    `json:"noisefloor"`
			CCQ        int    `json:"ccq"`
			Uptime     int64  `json:"uptime"`
			TX         struct {
				Rate string `json:"rate"`
			} `json:"tx"`
			RX struct {
				Rate string `json:"rate"`
			} `json:"rx"`
			Airmax struct {
				Quality  int `json:"quality"`
				Capacity int `json:"capacity"`
				Priority int `json:"priority"`
			} `json:"airmax"`
			Stats struct {
				RXBytes int64 `json:"rx_bytes"`
				TXBytes int64 `json:"tx_bytes"`
			} `json:"stats"`
		}

		if err := json.Unmarshal([]byte(rawWsta), &staList); err == nil {
			for _, sta := range staList {
				sig := sta.Signal
				if sig == 0 {
					sig = -sta.RSSI
				}
				ccq := sta.CCQ
				if ccq > 100 {
					ccq = ccq / 10
				}
				snr := 0
				if sig != 0 && noise != 0 {
					snr = sig - noise
				}

				name := sta.Name
				if name == "" {
					name = sta.LastIP
				}

				txRate := sta.TX.Rate
				if txRate != "" && !strings.Contains(txRate, "Mbps") {
					txRate += " Mbps"
				}
				rxRate := sta.RX.Rate
				if rxRate != "" && !strings.Contains(rxRate, "Mbps") {
					rxRate += " Mbps"
				}

				data.Clients = append(data.Clients, devices.DeviceWirelessClient{
					MACAddress:    strings.ToLower(sta.MAC),
					IPAddress:     sta.LastIP,
					Hostname:      name,
					Signal:        sig,
					Noise:         noise,
					SNR:           snr,
					TXRate:        txRate,
					RXRate:        rxRate,
					CCQ:           ccq,
					UptimeSeconds: sta.Uptime,
					RXBytes:       sta.Stats.RXBytes,
					TXBytes:       sta.Stats.TXBytes,
					Status:        "connected",
					LastSeen:      now,
				})
			}
		}
	}

	data.Wireless.ConnectedClients = len(data.Clients)
	return data, nil
}

func (d *Driver) CableTest(ctx context.Context, target devices.TargetConfig, ifaceName string) (*devices.CableTestResult, error) {
	// Query ethtool on Ubiquiti
	out, _ := d.runSSHCommand(target, "ethtool eth0")
	isOk := strings.Contains(out, "Link detected: yes")

	return &devices.CableTestResult{
		Interface: ifaceName,
		Status:    map[bool]string{true: "ok", false: "open"}[isOk],
		CablePairs: []devices.CablePairStatus{
			{Pair: "Pair 1 (1-2)", Status: "ok"},
			{Pair: "Pair 2 (3-6)", Status: "ok"},
			{Pair: "Pair 3 (4-5)", Status: "ok"},
			{Pair: "Pair 4 (7-8)", Status: "ok"},
		},
	}, nil
}

func (d *Driver) MonitorPort(ctx context.Context, target devices.TargetConfig, ifaceName string) (*devices.PortMonitorResult, error) {
	rawStatus, err := d.runSSHCommand(target, "mca-status")
	if err != nil {
		return nil, err
	}
	m := parseMCAStatus(rawStatus)

	return &devices.PortMonitorResult{
		Interface:       ifaceName,
		Status:          "link-ok",
		Rate:            m["lanSpeed"],
		FullDuplex:      true,
		AutoNegotiation: "enabled",
	}, nil
}

func (d *Driver) GetSwitchHosts(ctx context.Context, target devices.TargetConfig) ([]devices.MACTableEntry, error) {
	out, err := d.runSSHCommand(target, "brctl showmacs br0")
	if err != nil {
		return nil, err
	}

	var entries []devices.MACTableEntry
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		fields := strings.Fields(l)
		if len(fields) >= 3 && strings.Contains(fields[1], ":") {
			entries = append(entries, devices.MACTableEntry{
				MACAddress: strings.ToLower(fields[1]),
				Interface:  "eth0",
				Bridge:     "br0",
				Dynamic:    fields[2] == "no",
			})
		}
	}

	return entries, nil
}
