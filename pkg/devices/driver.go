package devices

import (
	"context"
)

// Driver is the vendor-agnostic interface that all hardware integrations must implement
type Driver interface {
	// TestConnection verifies that the device is reachable and credentials are valid
	TestConnection(ctx context.Context, target TargetConfig) error

	// Discover probes the device to automatically detect its Model, OS version, Architecture, Interfaces, and capabilities
	Discover(ctx context.Context, target TargetConfig) (*DeviceDiscoveryResult, error)

	// PollSwitch collects metrics, interface states, VLANs, bridges, and PoE/SFP info for Switch devices
	PollSwitch(ctx context.Context, target TargetConfig) (*SwitchProfileData, error)

	// PollLink collects radio link parameters, signal levels, CCQ, rates, and remote peer info for PtP Link devices
	PollLink(ctx context.Context, target TargetConfig) (*LinkProfileData, error)

	// PollSector collects radio parameters and all active connected client stations for Sector AP devices
	PollSector(ctx context.Context, target TargetConfig) (*SectorProfileData, error)

	// CableTest executes an ethernet TDR cable diagnostic on a specific interface
	CableTest(ctx context.Context, target TargetConfig, ifaceName string) (*CableTestResult, error)

	// MonitorPort queries live, real-time link parameters (speed, duplex, SFP diagnostics, flow control) for a specific interface
	MonitorPort(ctx context.Context, target TargetConfig, ifaceName string) (*PortMonitorResult, error)

	// GetSwitchHosts fetches switch chip and bridge MAC host tables
	GetSwitchHosts(ctx context.Context, target TargetConfig) ([]MACTableEntry, error)
}
