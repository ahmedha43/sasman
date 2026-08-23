package devices

import (
	"time"
)

// Vendor represents a hardware manufacturer (e.g. MikroTik, Ubiquiti, Huawei, Cisco)
type Vendor struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	Website   string    `json:"website,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeviceType represents the functional category of a device
type DeviceType struct {
	ID             int64     `json:"id"`
	Slug           string    `json:"slug"` // "switch", "link", "sector", "router"
	Name           string    `json:"name"` // "Switch / محول", "PtP Link / ربط لاسلكي", "Sector AP / سكتور بث"
	Icon           string    `json:"icon"`
	Description    string    `json:"description,omitempty"`
	DefaultProfile string    `json:"default_profile,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// DeviceModel represents a specific hardware model from a vendor
type DeviceModel struct {
	ID               int64     `json:"id"`
	VendorID         int64     `json:"vendor_id"`
	VendorName       string    `json:"vendor_name,omitempty"`
	ModelName        string    `json:"model_name"`
	DefaultTypeID    int64     `json:"default_type_id"`
	DefaultTypeSlug  string    `json:"default_type_slug,omitempty"`
	CapabilitiesJSON string    `json:"capabilities_json,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// Device represents a network device instance deployed in the infrastructure
type Device struct {
	ID                  int64           `json:"id"`
	VendorID            int64           `json:"vendor_id"`
	VendorSlug          string          `json:"vendor_slug"`
	VendorName          string          `json:"vendor_name"`
	TypeID              int64           `json:"type_id"`
	TypeSlug            string          `json:"type_slug"` // "switch", "link", "sector"
	TypeName            string          `json:"type_name"`
	ModelID             int64           `json:"model_id,omitempty"`
	ModelName           string          `json:"model_name"`
	Name                string          `json:"name"`
	IP                  string          `json:"ip"`
	Port                int             `json:"port"`
	Status              string          `json:"status"` // "online", "offline", "warning", "unknown"
	LastSeen            *time.Time      `json:"last_seen,omitempty"`
	LastError           string          `json:"last_error,omitempty"`
	PollIntervalSec     int             `json:"poll_interval_sec"`
	IsMonitored         bool            `json:"is_monitored"`
	DownSince           *time.Time      `json:"down_since,omitempty"`
	UpSince             *time.Time      `json:"up_since,omitempty"`
	DowntimeDurationSec int64           `json:"downtime_duration_sec"`
	OSVersion           string          `json:"os_version,omitempty"`
	SerialNumber        string          `json:"serial_number,omitempty"`
	Architecture        string          `json:"architecture,omitempty"`
	BoardName           string          `json:"board_name,omitempty"`
	UptimeSeconds       int64           `json:"uptime_seconds,omitempty"`
	CPULoad             int             `json:"cpu_load,omitempty"`
	MemoryUsed          int64           `json:"memory_used,omitempty"`
	MemoryTotal         int64           `json:"memory_total,omitempty"`
	StorageUsed         int64           `json:"storage_used,omitempty"`
	StorageTotal        int64           `json:"storage_total,omitempty"`
	Temperature         float64         `json:"temperature,omitempty"`
	Voltage             float64         `json:"voltage,omitempty"`
	Location            string          `json:"location,omitempty"`
	Notes               string          `json:"notes,omitempty"`
	MetadataJSON        string          `json:"metadata_json,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
	AdminID             int64           `json:"admin_id,omitempty"`
	InterfacesCount     int             `json:"interfaces_count,omitempty"`
	ClientsCount        int             `json:"clients_count,omitempty"`
	LastMetric          *DeviceMetric   `json:"last_metric,omitempty"`
	WirelessInfo        *DeviceWireless `json:"wireless_info,omitempty"`
}

// DeviceCredential stores secure authentication data for a device
type DeviceCredential struct {
	ID                int64     `json:"id"`
	DeviceID          int64     `json:"device_id"`
	Username          string    `json:"username"`
	PasswordEncrypted string    `json:"-"` // Never serialized to JSON
	AuthType          string    `json:"auth_type"` // "api", "api_ssl", "ssh", "snmp_v2", "snmp_v3"
	Community         string    `json:"community,omitempty"`
	APIVersion        string    `json:"api_version,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// DeviceInterface represents a physical or logical port on a network device
type DeviceInterface struct {
	ID            int64     `json:"id"`
	DeviceID      int64     `json:"device_id"`
	IfIndex       int       `json:"if_index"`
	Name          string    `json:"name"`
	Type          string    `json:"type"` // "ether", "sfp", "bridge", "vlan", "wlan"
	MACAddress    string    `json:"mac_address"`
	Status        string    `json:"status"` // "up", "down", "disabled"
	Speed         string    `json:"speed,omitempty"` // "1Gbps", "10Gbps", "100Mbps"
	Duplex        string    `json:"duplex,omitempty"` // "full", "half"
	MTU           int       `json:"mtu"`
	VLANID        int       `json:"vlan_id,omitempty"`
	IsPoE         bool      `json:"is_poe"`
	IsSFP         bool      `json:"is_sfp"`
	PoEStatus     string    `json:"poe_status,omitempty"` // "powered", "off", "fault"
	PoEPowerWatt  float64   `json:"poe_power_watt,omitempty"`
	SFPWavelength string    `json:"sfp_wavelength,omitempty"`
	SFPTemp       float64   `json:"sfp_temp,omitempty"`
	SFPTXPowerDBm float64   `json:"sfp_tx_power_dbm,omitempty"`
	SFPRXPowerDBm float64   `json:"sfp_rx_power_dbm,omitempty"`
	RXBytes       int64     `json:"rx_bytes"`
	TXBytes       int64     `json:"tx_bytes"`
	RXPackets     int64     `json:"rx_packets"`
	TXPackets     int64     `json:"tx_packets"`
	RXErrors      int64     `json:"rx_errors"`
	TXErrors      int64     `json:"tx_errors"`
	RXDrops       int64     `json:"rx_drops"`
	TXDrops       int64     `json:"tx_drops"`
	SFPInfoJSON   string    `json:"sfp_info_json,omitempty"`
	PoEInfoJSON   string    `json:"poe_info_json,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// DeviceMetric holds a historical snapshot of device performance metrics
type DeviceMetric struct {
	ID            int64     `json:"id"`
	DeviceID      int64     `json:"device_id"`
	CPULoad       int       `json:"cpu_load"`
	MemoryUsed    int64     `json:"memory_used"`
	MemoryTotal   int64     `json:"memory_total"`
	StorageUsed   int64     `json:"storage_used"`
	StorageTotal  int64     `json:"storage_total"`
	Temperature   float64   `json:"temperature"`
	Voltage       float64   `json:"voltage"`
	UptimeSeconds int64     `json:"uptime_seconds"`
	RXBytes       int64     `json:"rx_bytes"`
	TXBytes       int64     `json:"tx_bytes"`
	RXPackets     int64     `json:"rx_packets"`
	TXPackets     int64     `json:"tx_packets"`
	RXErrors      int64     `json:"rx_errors"`
	TXErrors      int64     `json:"tx_errors"`
	RXDrops       int64     `json:"rx_drops"`
	TXDrops       int64     `json:"tx_drops"`
	RecordedAt    time.Time `json:"recorded_at"`
}

// DeviceWireless represents wireless radio and link metrics (for PtP Link and Sector AP)
type DeviceWireless struct {
	ID               int64     `json:"id"`
	DeviceID         int64     `json:"device_id"`
	InterfaceName    string    `json:"interface_name"`
	Mode             string    `json:"mode"` // "ap-bridge", "bridge", "station", "station-bridge", "p2p"
	SSID             string    `json:"ssid"`
	Frequency        int       `json:"frequency"`      // e.g. 5805 MHz
	ChannelWidth     string    `json:"channel_width"`  // e.g. "20MHz", "40MHz", "80MHz"
	NoiseFloor       int       `json:"noise_floor"`    // e.g. -96 dBm
	TXPower          int       `json:"tx_power"`       // e.g. 20 dBm
	CCQ              int       `json:"ccq"`            // e.g. 98%
	SignalStrength   int       `json:"signal_strength"` // e.g. -52 dBm
	SNR              int       `json:"snr"`            // Signal-to-Noise Ratio (dB)
	TXRate           string    `json:"tx_rate"`        // e.g. "866 Mbps"
	RXRate           string    `json:"rx_rate"`        // e.g. "780 Mbps"
	DistanceKm       float64   `json:"distance_km"`    // e.g. 3.2 km
	MCS              string    `json:"mcs,omitempty"`
	AirtimeUsage     float64   `json:"airtime_usage,omitempty"` // Percentage 0-100
	RemoteMAC        string    `json:"remote_mac,omitempty"`
	RemoteDeviceInfo string    `json:"remote_device_info,omitempty"`
	ConnectedClients int       `json:"connected_clients"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// DeviceWirelessClient represents a connected station/client on a Sector AP or PtP Link
type DeviceWirelessClient struct {
	ID            int64     `json:"id"`
	DeviceID      int64     `json:"device_id"`
	WirelessID    int64     `json:"wireless_id,omitempty"`
	MACAddress    string    `json:"mac_address"`
	IPAddress     string    `json:"ip_address,omitempty"`
	Hostname      string    `json:"hostname,omitempty"`
	Signal        int       `json:"signal"` // dBm
	Noise         int       `json:"noise"`  // dBm
	SNR           int       `json:"snr"`    // dB
	TXRate        string    `json:"tx_rate"`
	RXRate        string    `json:"rx_rate"`
	CCQ           int       `json:"ccq"` // %
	UptimeSeconds int64     `json:"uptime_seconds"`
	RXBytes       int64     `json:"rx_bytes"`
	TXBytes       int64     `json:"tx_bytes"`
	Status        string    `json:"status"` // "connected", "idle", "disconnected"
	LastSeen      time.Time `json:"last_seen"`
}

// DeviceEvent records lifecycle and operational events for a device
type DeviceEvent struct {
	ID            int64     `json:"id"`
	DeviceID      int64     `json:"device_id"`
	EventType     string    `json:"event_type"` // "online", "offline", "warning", "config_change", "probe_fail"
	Severity      string    `json:"severity"`   // "info", "warning", "critical"
	Message       string    `json:"message"`
	EventDataJSON string    `json:"event_data_json,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// DeviceAlert represents an active or resolved alarm on a device
type DeviceAlert struct {
	ID          int64      `json:"id"`
	DeviceID    int64      `json:"device_id"`
	DeviceName  string     `json:"device_name,omitempty"`
	AlertType   string     `json:"alert_type"` // "offline", "high_cpu", "low_signal", "high_temp", "poe_fault"
	Message     string     `json:"message"`
	Status      string     `json:"status"` // "active", "acknowledged", "resolved"
	TriggeredAt time.Time  `json:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// TargetConfig encapsulates the network and authentication details needed to connect to a device
type TargetConfig struct {
	IP       string        `json:"ip"`
	Port     int           `json:"port"`
	Username string        `json:"username"`
	Password string        `json:"password"`
	AuthType string        `json:"auth_type"` // "api", "api_ssl", "ssh", "snmp"
	Timeout  time.Duration `json:"timeout"`
}

// DeviceDiscoveryResult holds the data returned by automatic device discovery
type DeviceDiscoveryResult struct {
	VendorSlug    string            `json:"vendor_slug"`
	VendorName    string            `json:"vendor_name"`
	SuggestedType string            `json:"suggested_type"` // "switch", "link", "sector"
	ModelName     string            `json:"model_name"`
	BoardName     string            `json:"board_name"`
	OSVersion     string            `json:"os_version"`
	SerialNumber  string            `json:"serial_number"`
	Architecture  string            `json:"architecture"`
	DeviceName    string            `json:"device_name"`
	UptimeSeconds int64             `json:"uptime_seconds"`
	CPULoad       int               `json:"cpu_load"`
	MemoryUsed    int64             `json:"memory_used"`
	MemoryTotal   int64             `json:"memory_total"`
	StorageUsed   int64             `json:"storage_used"`
	StorageTotal  int64             `json:"storage_total"`
	Temperature   float64           `json:"temperature"`
	Voltage       float64           `json:"voltage"`
	Interfaces    []DeviceInterface `json:"interfaces"`
	HasWireless   bool              `json:"has_wireless"`
	HasSwitchChip bool              `json:"has_switch_chip"`
	Capabilities  map[string]any    `json:"capabilities"`
}

// SwitchProfileData holds the complete polled dataset tailored for a Switch
type SwitchProfileData struct {
	DeviceMetrics DeviceMetric      `json:"metrics"`
	Interfaces    []DeviceInterface `json:"interfaces"`
	Bridges       []BridgeInfo      `json:"bridges,omitempty"`
	VLANs         []VLANInfo        `json:"vlans,omitempty"`
	MACTable      []MACTableEntry   `json:"mac_table,omitempty"`
	PoEEntries    []PoEEntry        `json:"poe_entries,omitempty"`
	SFPEntries    []SFPEntry        `json:"sfp_entries,omitempty"`
}

// LinkProfileData holds the complete polled dataset tailored for a Wireless PtP Link
type LinkProfileData struct {
	DeviceMetrics DeviceMetric      `json:"metrics"`
	Wireless      DeviceWireless    `json:"wireless"`
	EthernetPorts []DeviceInterface `json:"ethernet_ports"`
}

// SectorProfileData holds the complete polled dataset tailored for an Access Point Sector
type SectorProfileData struct {
	DeviceMetrics DeviceMetric           `json:"metrics"`
	Wireless      DeviceWireless         `json:"wireless"`
	Clients       []DeviceWirelessClient `json:"clients"`
}

type BridgeInfo struct {
	Name       string   `json:"name"`
	MACAddress string   `json:"mac_address"`
	Ports      []string `json:"ports"`
	MTU        int      `json:"mtu"`
	Running    bool     `json:"running"`
}

type VLANInfo struct {
	Bridge   string   `json:"bridge"`
	VLANIDs  string   `json:"vlan_ids"`
	Tagged   []string `json:"tagged"`
	Untagged []string `json:"untagged"`
	Current  bool     `json:"current"`
}

type MACTableEntry struct {
	MACAddress string `json:"mac_address"`
	Interface  string `json:"interface"`
	Bridge     string `json:"bridge"`
	Dynamic    bool   `json:"dynamic"`
	Age        string `json:"age,omitempty"`
}

type PoEEntry struct {
	Interface string  `json:"interface"`
	Status    string  `json:"status"`
	Voltage   float64 `json:"voltage"`
	CurrentMA int     `json:"current_ma"`
	PowerWatt float64 `json:"power_watt"`
}

type SFPEntry struct {
	Interface    string  `json:"interface"`
	Vendor       string  `json:"vendor,omitempty"`
	PartNumber   string  `json:"part_number,omitempty"`
	Wavelength   string  `json:"wavelength,omitempty"`
	Temperature  float64 `json:"temperature"`
	SupplyVolt   float64 `json:"supply_volt"`
	TXPowerDBm   float64 `json:"tx_power_dbm"`
	RXPowerDBm   float64 `json:"rx_power_dbm"`
	LinkDistance string  `json:"link_distance,omitempty"`
}

type CableTestResult struct {
	Interface   string            `json:"interface"`
	Status      string            `json:"status"` // "ok", "open", "shorted", "failed", "unknown"
	LengthMeter float64           `json:"length_meter"`
	CablePairs  []CablePairStatus `json:"cable_pairs"`
	RawDetails  map[string]string `json:"raw_details,omitempty"`
}

type CablePairStatus struct {
	Pair   string  `json:"pair"`   // "pair-1", "pair-2", "pair-3", "pair-4"
	Status string  `json:"status"` // "ok", "open", "shorted"
	Length float64 `json:"length"` // distance in meters if available
}

type PortMonitorResult struct {
	Interface        string  `json:"interface"`
	Status           string  `json:"status"`
	AutoNegotiation  string  `json:"auto_negotiation"`
	Rate             string  `json:"rate"`
	FullDuplex       bool    `json:"full_duplex"`
	DefaultName      string  `json:"default_name"`
	TXFlowControl    string  `json:"tx_flow_control"`
	RXFlowControl    string  `json:"rx_flow_control"`
	SFPModulePresent string  `json:"sfp_module_present"`
	SFPRXLoss        string  `json:"sfp_rx_loss"`
	SFPTXFault       string  `json:"sfp_tx_fault"`
	SFPTemp          float64 `json:"sfp_temp"`
	SFPSupplyVolt    float64 `json:"sfp_supply_volt"`
	SFPTXBiasCurrent float64 `json:"sfp_tx_bias_current"`
	SFPTXPowerDBm    float64 `json:"sfp_tx_power_dbm"`
	SFPRXPowerDBm    float64 `json:"sfp_rx_power_dbm"`
	SFPWavelength    string  `json:"sfp_wavelength"`
	SFPVendor        string  `json:"sfp_vendor"`
	SFPPartNumber    string  `json:"sfp_part_number"`
}
