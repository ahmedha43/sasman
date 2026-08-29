package devices

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Monitor runs periodic polling for registered network devices
type Monitor struct {
	repo       *Repository
	running    int32
	stopCh     chan struct{}
	pollTicker *time.Ticker
}

func NewMonitor(repo *Repository) *Monitor {
	return &Monitor{
		repo:   repo,
		stopCh: make(chan struct{}),
	}
}

func (m *Monitor) Start() {
	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		return
	}

	log.Printf("[NetworkDevices Monitor] Background monitoring engine started.")
	go m.loop()
}

func (m *Monitor) Stop() {
	if atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		close(m.stopCh)
		log.Printf("[NetworkDevices Monitor] Background monitoring engine stopped.")
	}
}

func (m *Monitor) loop() {
	// Check every 10 seconds for devices that need polling based on their PollIntervalSec
	m.pollTicker = time.NewTicker(10 * time.Second)
	defer m.pollTicker.Stop()

	// Initial poll immediately
	m.pollAll()

	for {
		select {
		case <-m.stopCh:
			return
		case <-m.pollTicker.C:
			m.pollAll()
		}
	}
}

func (m *Monitor) pollAll() {
	devicesList, err := m.repo.ListDevices("", "", "")
	if err != nil {
		log.Printf("[NetworkDevices Monitor] Error listing devices for polling: %v", err)
		return
	}

	now := time.Now()
	var wg sync.WaitGroup

	for _, dev := range devicesList {
		if !dev.IsMonitored {
			continue
		}

		interval := dev.PollIntervalSec
		if interval <= 0 {
			interval = 30
		}

		// Check if it's time to poll this device
		if dev.LastSeen != nil && now.Sub(*dev.LastSeen) < time.Duration(interval)*time.Second {
			continue
		}

		wg.Add(1)
		go func(d Device) {
			defer wg.Done()
			m.PollDevice(d.ID)
		}(dev)
	}

	wg.Wait()
}

// PollDevice executes an on-demand or periodic poll for a specific device
func (m *Monitor) PollDevice(deviceID int64) error {
	dev, err := m.repo.GetDevice(deviceID)
	if err != nil {
		return err
	}

	driver, err := GetDriver(dev.VendorSlug)
	if err != nil {
		_ = m.repo.UpdateDeviceOnlineStatus(deviceID, false, false, err.Error(), time.Now())
		return err
	}

	target, err := m.repo.GetDeviceCredentials(deviceID)
	if err != nil {
		_ = m.repo.UpdateDeviceOnlineStatus(deviceID, false, false, "Missing or invalid credentials", time.Now())
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()

	switch dev.TypeSlug {
	case "switch", "router":
		data, err := driver.PollSwitch(ctx, *target)
		if err != nil {
			_ = m.repo.UpdateDeviceOnlineStatus(deviceID, false, false, err.Error(), now)
			return err
		}

		// Update metrics & sys info
		data.DeviceMetrics.DeviceID = deviceID
		_ = m.repo.RecordMetric(&data.DeviceMetrics)
		_ = m.repo.UpdateDeviceSystemInfo(deviceID, &Device{
			OSVersion:     dev.OSVersion,
			CPULoad:       data.DeviceMetrics.CPULoad,
			MemoryUsed:    data.DeviceMetrics.MemoryUsed,
			MemoryTotal:   data.DeviceMetrics.MemoryTotal,
			StorageUsed:   data.DeviceMetrics.StorageUsed,
			StorageTotal:  data.DeviceMetrics.StorageTotal,
			Temperature:   data.DeviceMetrics.Temperature,
			Voltage:       data.DeviceMetrics.Voltage,
			UptimeSeconds: data.DeviceMetrics.UptimeSeconds,
		})

		if len(data.Interfaces) > 0 {
			_ = m.repo.SaveInterfaces(deviceID, data.Interfaces)
		}

		_ = m.repo.UpdateDeviceOnlineStatus(deviceID, true, false, "", now)
		return nil

	case "link":
		data, err := driver.PollLink(ctx, *target)
		if err != nil {
			_ = m.repo.UpdateDeviceOnlineStatus(deviceID, false, false, err.Error(), now)
			return err
		}

		data.DeviceMetrics.DeviceID = deviceID
		_ = m.repo.RecordMetric(&data.DeviceMetrics)
		_ = m.repo.UpdateDeviceSystemInfo(deviceID, &Device{
			OSVersion:     dev.OSVersion,
			CPULoad:       data.DeviceMetrics.CPULoad,
			MemoryUsed:    data.DeviceMetrics.MemoryUsed,
			MemoryTotal:   data.DeviceMetrics.MemoryTotal,
			StorageUsed:   data.DeviceMetrics.StorageUsed,
			StorageTotal:  data.DeviceMetrics.StorageTotal,
			Temperature:   data.DeviceMetrics.Temperature,
			Voltage:       data.DeviceMetrics.Voltage,
			UptimeSeconds: data.DeviceMetrics.UptimeSeconds,
		})

		data.Wireless.DeviceID = deviceID
		_, _ = m.repo.SaveWirelessInfo(&data.Wireless)

		if len(data.EthernetPorts) > 0 {
			_ = m.repo.SaveInterfaces(deviceID, data.EthernetPorts)
		}

		isWarning := data.Wireless.CCQ > 0 && data.Wireless.CCQ < 70
		_ = m.repo.UpdateDeviceOnlineStatus(deviceID, true, isWarning, "", now)
		return nil

	case "sector":
		data, err := driver.PollSector(ctx, *target)
		if err != nil {
			_ = m.repo.UpdateDeviceOnlineStatus(deviceID, false, false, err.Error(), now)
			return err
		}

		data.DeviceMetrics.DeviceID = deviceID
		_ = m.repo.RecordMetric(&data.DeviceMetrics)
		_ = m.repo.UpdateDeviceSystemInfo(deviceID, &Device{
			OSVersion:     dev.OSVersion,
			CPULoad:       data.DeviceMetrics.CPULoad,
			MemoryUsed:    data.DeviceMetrics.MemoryUsed,
			MemoryTotal:   data.DeviceMetrics.MemoryTotal,
			StorageUsed:   data.DeviceMetrics.StorageUsed,
			StorageTotal:  data.DeviceMetrics.StorageTotal,
			Temperature:   data.DeviceMetrics.Temperature,
			Voltage:       data.DeviceMetrics.Voltage,
			UptimeSeconds: data.DeviceMetrics.UptimeSeconds,
		})

		data.Wireless.DeviceID = deviceID
		wID, _ := m.repo.SaveWirelessInfo(&data.Wireless)

		if len(data.Clients) > 0 {
			_ = m.repo.SaveWirelessClients(deviceID, wID, data.Clients)
		}

		_ = m.repo.UpdateDeviceOnlineStatus(deviceID, true, false, "", now)
		return nil

	default:
		// Fallback to switch/generic poll
		data, err := driver.PollSwitch(ctx, *target)
		if err != nil {
			_ = m.repo.UpdateDeviceOnlineStatus(deviceID, false, false, err.Error(), now)
			return err
		}
		data.DeviceMetrics.DeviceID = deviceID
		_ = m.repo.RecordMetric(&data.DeviceMetrics)
		_ = m.repo.UpdateDeviceOnlineStatus(deviceID, true, false, "", now)
		return nil
	}
}
