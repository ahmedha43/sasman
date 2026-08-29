package devices

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

var (
	GlobalService *Service
)

type Service struct {
	repo    *Repository
	monitor *Monitor
}

func NewService(db *sql.DB) *Service {
	repo := NewRepository(db)
	monitor := NewMonitor(repo)
	return &Service{
		repo:    repo,
		monitor: monitor,
	}
}

// Init initializes the database schema, starts the monitoring engine, and sets the global service instance
func Init(db *sql.DB) (*Service, error) {
	if err := EnsureSchema(db); err != nil {
		return nil, fmt.Errorf("init devices schema: %w", err)
	}

	svc := NewService(db)
	svc.monitor.Start()
	GlobalService = svc
	return svc, nil
}

func (s *Service) GetRepository() *Repository {
	return s.repo
}

func (s *Service) GetMonitor() *Monitor {
	return s.monitor
}

// DiscoverAndProbe tests connection and probes the remote device to return discovered identity and suggested type
func (s *Service) DiscoverAndProbe(ctx context.Context, vendorSlug string, target TargetConfig) (*DeviceDiscoveryResult, error) {
	driver, err := GetDriver(vendorSlug)
	if err != nil {
		return nil, err
	}

	if target.Timeout <= 0 {
		target.Timeout = 8 * time.Second
	}

	return driver.Discover(ctx, target)
}

type AddDeviceRequest struct {
	VendorSlug      string `json:"vendor_slug"`
	TypeSlug        string `json:"type_slug"`
	Name            string `json:"name"`
	IP              string `json:"ip"`
	Port            int    `json:"port"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	AuthType        string `json:"auth_type"`
	PollIntervalSec int    `json:"poll_interval_sec"`
	IsMonitored     bool   `json:"is_monitored"`
	Location        string `json:"location"`
	Notes           string `json:"notes"`
	AdminID         int64  `json:"admin_id"`
}

func (s *Service) AddDevice(ctx context.Context, req AddDeviceRequest) (*Device, error) {
	vendor, err := s.repo.GetVendorBySlug(req.VendorSlug)
	if err != nil {
		return nil, fmt.Errorf("vendor '%s' not found: %w", req.VendorSlug, err)
	}

	devType, err := s.repo.GetDeviceTypeBySlug(req.TypeSlug)
	if err != nil {
		return nil, fmt.Errorf("device type '%s' not found: %w", req.TypeSlug, err)
	}

	port := req.Port
	if port <= 0 {
		port = 8728
	}

	authType := req.AuthType
	if authType == "" {
		authType = "api"
	}

	pollInterval := req.PollIntervalSec
	if pollInterval <= 0 {
		pollInterval = 30
	}

	target := TargetConfig{
		IP:       req.IP,
		Port:     port,
		Username: req.Username,
		Password: req.Password,
		AuthType: authType,
		Timeout:  8 * time.Second,
	}

	// 1. Discover to get initial hardware info
	driver, err := GetDriver(req.VendorSlug)
	var discResult *DeviceDiscoveryResult
	if err == nil {
		discResult, _ = driver.Discover(ctx, target)
	}

	d := &Device{
		VendorID:        vendor.ID,
		VendorSlug:      vendor.Slug,
		VendorName:      vendor.Name,
		TypeID:          devType.ID,
		TypeSlug:        devType.Slug,
		TypeName:        devType.Name,
		Name:            req.Name,
		IP:              req.IP,
		Port:            port,
		PollIntervalSec: pollInterval,
		IsMonitored:     req.IsMonitored,
		Location:        req.Location,
		Notes:           req.Notes,
		AdminID:         req.AdminID,
	}

	if discResult != nil {
		if d.Name == "" {
			d.Name = discResult.DeviceName
		}
		d.ModelName = discResult.ModelName
		d.BoardName = discResult.BoardName
		d.OSVersion = discResult.OSVersion
		d.SerialNumber = discResult.SerialNumber
		d.Architecture = discResult.Architecture
		d.CPULoad = discResult.CPULoad
		d.MemoryUsed = discResult.MemoryUsed
		d.MemoryTotal = discResult.MemoryTotal
		d.StorageUsed = discResult.StorageUsed
		d.StorageTotal = discResult.StorageTotal
		d.Temperature = discResult.Temperature
		d.Voltage = discResult.Voltage
		d.UptimeSeconds = discResult.UptimeSeconds
	}

	if d.Name == "" {
		d.Name = fmt.Sprintf("%s-%s", vendor.Name, req.IP)
	}

	cred := &DeviceCredential{
		Username:          req.Username,
		PasswordEncrypted: req.Password,
		AuthType:          authType,
	}

	deviceID, err := s.repo.CreateDevice(d, cred)
	if err != nil {
		return nil, err
	}

	if discResult != nil && len(discResult.Interfaces) > 0 {
		_ = s.repo.SaveInterfaces(deviceID, discResult.Interfaces)
	}

	// Run immediate first poll in background
	go func() {
		_ = s.monitor.PollDevice(deviceID)
	}()

	return s.repo.GetDevice(deviceID)
}

type DeviceDetailResponse struct {
	Device          *Device                `json:"device"`
	Interfaces      []DeviceInterface      `json:"interfaces"`
	Wireless        *DeviceWireless        `json:"wireless,omitempty"`
	Clients         []DeviceWirelessClient `json:"clients,omitempty"`
	MetricsHistory  []DeviceMetric         `json:"metrics_history"`
	RecentEvents    []DeviceEvent          `json:"recent_events"`
	SwitchProfile   *SwitchProfileData     `json:"switch_profile,omitempty"`
	LinkProfile     *LinkProfileData       `json:"link_profile,omitempty"`
	SectorProfile   *SectorProfileData     `json:"sector_profile,omitempty"`
}

func (s *Service) GetDeviceDetail(deviceID int64) (*DeviceDetailResponse, error) {
	d, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return nil, err
	}

	resp := &DeviceDetailResponse{
		Device: d,
	}

	if ifaces, err := s.repo.ListDeviceInterfaces(deviceID); err == nil {
		resp.Interfaces = ifaces
	}

	if w, err := s.repo.GetDeviceWireless(deviceID); err == nil && w != nil {
		resp.Wireless = w
	}

	if clients, err := s.repo.ListWirelessClients(deviceID); err == nil {
		resp.Clients = clients
	}

	if metrics, err := s.repo.GetMetricsHistory(deviceID, 30); err == nil {
		resp.MetricsHistory = metrics
	}

	if events, err := s.repo.ListEvents(deviceID, 20); err == nil {
		resp.RecentEvents = events
	}

	return resp, nil
}

type NetworkSummaryStats struct {
	TotalDevices    int `json:"total_devices"`
	OnlineDevices   int `json:"online_devices"`
	OfflineDevices  int `json:"offline_devices"`
	WarningDevices  int `json:"warning_devices"`
	SwitchesCount   int `json:"switches_count"`
	LinksCount      int `json:"links_count"`
	SectorsCount    int `json:"sectors_count"`
	TotalClients    int `json:"total_clients"`
	TotalInterfaces int `json:"total_interfaces"`
}

func (s *Service) GetSummaryStats() (*NetworkSummaryStats, error) {
	devicesList, err := s.repo.ListDevices("", "", "")
	if err != nil {
		return nil, err
	}

	stats := &NetworkSummaryStats{
		TotalDevices: len(devicesList),
	}

	for _, d := range devicesList {
		switch d.Status {
		case "online":
			stats.OnlineDevices++
		case "offline":
			stats.OfflineDevices++
		case "warning":
			stats.WarningDevices++
		}

		switch d.TypeSlug {
		case "switch":
			stats.SwitchesCount++
		case "link":
			stats.LinksCount++
		case "sector":
			stats.SectorsCount++
		}

		stats.TotalClients += d.ClientsCount
		stats.TotalInterfaces += d.InterfacesCount
	}

	return stats, nil
}
