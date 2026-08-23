package mikrotik

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"mikrotik-manager/pkg/devices"

	"github.com/go-routeros/routeros/v3"
)

// Driver implements devices.Driver for MikroTik RouterOS hardware
type Driver struct{}

func NewDriver() *Driver {
	return &Driver{}
}

func init() {
	devices.RegisterDriver("mikrotik", NewDriver())
}

func (d *Driver) dial(target devices.TargetConfig) (*routeros.Client, error) {
	port := target.Port
	if port <= 0 {
		port = 8728
	}

	host := target.IP
	if strings.Contains(host, ":") {
		h, p, err := net.SplitHostPort(host)
		if err == nil {
			host = h
			if prt, err := strconv.Atoi(p); err == nil {
				port = prt
			}
		}
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))

	if target.AuthType == "api_ssl" || port == 8729 {
		return routeros.DialTLS(addr, target.Username, target.Password, nil)
	}

	return routeros.Dial(addr, target.Username, target.Password)
}

func (d *Driver) TestConnection(ctx context.Context, target devices.TargetConfig) error {
	client, err := d.dial(target)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer client.Close()

	_, err = client.Run("/system/identity/print")
	if err != nil {
		return fmt.Errorf("authentication error: %w", err)
	}
	return nil
}

func (d *Driver) Discover(ctx context.Context, target devices.TargetConfig) (*devices.DeviceDiscoveryResult, error) {
	client, err := d.dial(target)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", target.IP, err)
	}
	defer client.Close()

	result := &devices.DeviceDiscoveryResult{
		VendorSlug:   "mikrotik",
		VendorName:   "MikroTik",
		Capabilities: make(map[string]any),
	}

	// 1. Identity
	if reply, err := client.Run("/system/identity/print"); err == nil && len(reply.Re) > 0 {
		result.DeviceName = reply.Re[0].Map["name"]
	}

	// 2. Resource
	if reply, err := client.Run("/system/resource/print"); err == nil && len(reply.Re) > 0 {
		m := reply.Re[0].Map
		result.OSVersion = m["version"]
		result.Architecture = m["architecture-name"]
		result.BoardName = m["board-name"]
		result.CPULoad, _ = strconv.Atoi(m["cpu-load"])
		result.UptimeSeconds = parseUptime(m["uptime"])

		totalMem, _ := strconv.ParseInt(m["total-memory"], 10, 64)
		freeMem, _ := strconv.ParseInt(m["free-memory"], 10, 64)
		result.MemoryTotal = totalMem
		result.MemoryUsed = totalMem - freeMem

		totalHdd, _ := strconv.ParseInt(m["total-hdd-space"], 10, 64)
		freeHdd, _ := strconv.ParseInt(m["free-hdd-space"], 10, 64)
		result.StorageTotal = totalHdd
		result.StorageUsed = totalHdd - freeHdd
	}

	// 3. Routerboard
	if reply, err := client.Run("/system/routerboard/print"); err == nil && len(reply.Re) > 0 {
		m := reply.Re[0].Map
		result.ModelName = m["model"]
		result.SerialNumber = m["serial-number"]
		if result.BoardName == "" {
			result.BoardName = m["board-name"]
		}
	}
	if result.ModelName == "" {
		result.ModelName = result.BoardName
	}
	if result.DeviceName == "" {
		result.DeviceName = result.ModelName
	}

	// 4. Health (Temp, Voltage)
	if reply, err := client.Run("/system/health/print"); err == nil {
		for _, re := range reply.Re {
			name := strings.ToLower(re.Map["name"])
			valStr := re.Map["value"]
			if strings.Contains(name, "temperature") || name == "board-temperature1" || name == "cpu-temperature" {
				if f, err := strconv.ParseFloat(valStr, 64); err == nil && (result.Temperature == 0 || name == "board-temperature1") {
					result.Temperature = f
				}
			} else if strings.Contains(name, "voltage") {
				if f, err := strconv.ParseFloat(valStr, 64); err == nil {
					result.Voltage = f
				}
			}
		}
	}

	// 5. Wireless capability check
	if reply, err := client.Run("/interface/wireless/print"); err == nil && len(reply.Re) > 0 {
		result.HasWireless = true
		result.Capabilities["wireless_interfaces"] = len(reply.Re)
		for _, re := range reply.Re {
			mode := re.Map["mode"]
			if mode == "ap-bridge" {
				result.SuggestedType = "sector"
			} else if mode == "bridge" || mode == "station-bridge" || mode == "station" {
				if result.SuggestedType == "" {
					result.SuggestedType = "link"
				}
			}
		}
	}

	// 6. Switch chip capability check
	if reply, err := client.Run("/interface/ethernet/switch/print"); err == nil && len(reply.Re) > 0 {
		result.HasSwitchChip = true
		result.Capabilities["switch_chips"] = len(reply.Re)
	}

	if result.SuggestedType == "" {
		if result.HasSwitchChip || strings.HasPrefix(strings.ToLower(result.ModelName), "crs") || strings.HasPrefix(strings.ToLower(result.ModelName), "css") {
			result.SuggestedType = "switch"
		} else if result.HasWireless {
			result.SuggestedType = "link"
		} else {
			result.SuggestedType = "switch"
		}
	}

	// 7. Interfaces
	if reply, err := client.Run("/interface/print"); err == nil {
		for idx, re := range reply.Re {
			m := re.Map
			iface := devices.DeviceInterface{
				IfIndex:    idx + 1,
				Name:       m["name"],
				Type:       m["type"],
				MACAddress: m["mac-address"],
				Status:     "down",
			}
			if m["running"] == "true" {
				iface.Status = "up"
			}
			if m["disabled"] == "true" {
				iface.Status = "disabled"
			}
			if strings.Contains(strings.ToLower(m["type"]), "sfp") || strings.Contains(strings.ToLower(m["name"]), "sfp") {
				iface.IsSFP = true
			}
			result.Interfaces = append(result.Interfaces, iface)
		}
	}

	return result, nil
}

func (d *Driver) PollSwitch(ctx context.Context, target devices.TargetConfig) (*devices.SwitchProfileData, error) {
	client, err := d.dial(target)
	if err != nil {
		return nil, fmt.Errorf("connect to switch: %w", err)
	}
	defer client.Close()

	data := &devices.SwitchProfileData{}
	now := time.Now()

	// 1. Resources
	if reply, err := client.Run("/system/resource/print"); err == nil && len(reply.Re) > 0 {
		m := reply.Re[0].Map
		cpu, _ := strconv.Atoi(m["cpu-load"])
		totalMem, _ := strconv.ParseInt(m["total-memory"], 10, 64)
		freeMem, _ := strconv.ParseInt(m["free-memory"], 10, 64)
		totalHdd, _ := strconv.ParseInt(m["total-hdd-space"], 10, 64)
		freeHdd, _ := strconv.ParseInt(m["free-hdd-space"], 10, 64)

		data.DeviceMetrics = devices.DeviceMetric{
			CPULoad:       cpu,
			MemoryTotal:   totalMem,
			MemoryUsed:    totalMem - freeMem,
			StorageTotal:  totalHdd,
			StorageUsed:   totalHdd - freeHdd,
			UptimeSeconds: parseUptime(m["uptime"]),
			RecordedAt:    now,
		}
	}

	// 2. Health
	if reply, err := client.Run("/system/health/print"); err == nil {
		for _, re := range reply.Re {
			name := strings.ToLower(re.Map["name"])
			valStr := re.Map["value"]
			if strings.Contains(name, "temperature") || name == "board-temperature1" {
				if f, err := strconv.ParseFloat(valStr, 64); err == nil {
					data.DeviceMetrics.Temperature = f
				}
			} else if strings.Contains(name, "voltage") {
				if f, err := strconv.ParseFloat(valStr, 64); err == nil {
					data.DeviceMetrics.Voltage = f
				}
			}
		}
	}

	// 3. Ethernet / SFP / PoE Interfaces
	ethMap := make(map[string]map[string]string)
	if ethReply, err := client.Run("/interface/ethernet/print", "detail"); err == nil {
		for _, re := range ethReply.Re {
			ethMap[re.Map["name"]] = re.Map
		}
	}

	poeMap := make(map[string]map[string]string)
	if poeReply, err := client.Run("/interface/ethernet/poe/print", "detail"); err == nil {
		for _, re := range poeReply.Re {
			poeMap[re.Map["name"]] = re.Map
		}
	}

	var totalRXBytes, totalTXBytes int64
	var totalRXPackets, totalTXPackets int64
	var totalRXErrors, totalTXErrors int64
	var totalRXDrops, totalTXDrops int64

	if reply, err := client.Run("/interface/print", "detail"); err == nil {
		for idx, re := range reply.Re {
			m := re.Map
			name := m["name"]
			iface := devices.DeviceInterface{
				IfIndex:    idx + 1,
				Name:       name,
				Type:       m["type"],
				MACAddress: m["mac-address"],
				Status:     "down",
			}
			if m["running"] == "true" {
				iface.Status = "up"
			}
			if m["disabled"] == "true" {
				iface.Status = "disabled"
			}
			mtu, _ := strconv.Atoi(m["mtu"])
			iface.MTU = mtu

			rxBytes, _ := strconv.ParseInt(m["rx-byte"], 10, 64)
			txBytes, _ := strconv.ParseInt(m["tx-byte"], 10, 64)
			rxPkts, _ := strconv.ParseInt(m["rx-packet"], 10, 64)
			txPkts, _ := strconv.ParseInt(m["tx-packet"], 10, 64)
			rxErr, _ := strconv.ParseInt(m["rx-error"], 10, 64)
			txErr, _ := strconv.ParseInt(m["tx-error"], 10, 64)
			rxDrop, _ := strconv.ParseInt(m["rx-drop"], 10, 64)
			txDrop, _ := strconv.ParseInt(m["tx-drop"], 10, 64)

			iface.RXBytes = rxBytes
			iface.TXBytes = txBytes
			iface.RXPackets = rxPkts
			iface.TXPackets = txPkts
			iface.RXErrors = rxErr
			iface.TXErrors = txErr
			iface.RXDrops = rxDrop
			iface.TXDrops = txDrop

			totalRXBytes += rxBytes
			totalTXBytes += txBytes
			totalRXPackets += rxPkts
			totalTXPackets += txPkts
			totalRXErrors += rxErr
			totalTXErrors += txErr
			totalRXDrops += rxDrop
			totalTXDrops += txDrop

			if eth, ok := ethMap[name]; ok {
				iface.Speed = eth["speed"]
				if iface.Speed == "" {
					iface.Speed = eth["rate"]
				}
				iface.Duplex = eth["full-duplex"]
				if iface.Duplex == "true" {
					iface.Duplex = "full"
				} else if iface.Duplex == "false" {
					iface.Duplex = "half"
				}
				if strings.Contains(strings.ToLower(eth["sfp-type"]), "sfp") || strings.Contains(strings.ToLower(name), "sfp") {
					iface.IsSFP = true
					iface.SFPWavelength = eth["sfp-wavelength"]
					if sfpT, err := strconv.ParseFloat(eth["sfp-temperature"], 64); err == nil {
						iface.SFPTemp = sfpT
					}
					if sfpTx, err := strconv.ParseFloat(eth["sfp-tx-power"], 64); err == nil {
						iface.SFPTXPowerDBm = sfpTx
					}
					if sfpRx, err := strconv.ParseFloat(eth["sfp-rx-power"], 64); err == nil {
						iface.SFPRXPowerDBm = sfpRx
					}
					data.SFPEntries = append(data.SFPEntries, devices.SFPEntry{
						Interface:    name,
						Vendor:       eth["sfp-vendor-name"],
						PartNumber:   eth["sfp-vendor-part-number"],
						Wavelength:   eth["sfp-wavelength"],
						Temperature:  iface.SFPTemp,
						TXPowerDBm:   iface.SFPTXPowerDBm,
						RXPowerDBm:   iface.SFPRXPowerDBm,
						LinkDistance: eth["sfp-link-length-copper"],
					})
				}
				if eth["poe-out"] != "" && eth["poe-out"] != "none" {
					iface.IsPoE = true
					iface.PoEStatus = eth["poe-out-status"]
					if iface.PoEStatus == "" {
						iface.PoEStatus = eth["poe-out"]
					}
					if v, err := strconv.ParseFloat(eth["poe-out-voltage"], 64); err == nil {
						if p, err := strconv.ParseFloat(eth["poe-out-power"], 64); err == nil {
							iface.PoEPowerWatt = p
						} else {
							iface.PoEPowerWatt = v
						}
					}
				}
			}

			if poe, ok := poeMap[name]; ok {
				iface.IsPoE = true
				iface.PoEStatus = poe["status"]
				v, _ := strconv.ParseFloat(poe["voltage"], 64)
				c, _ := strconv.Atoi(poe["current"])
				p, _ := strconv.ParseFloat(poe["power"], 64)
				iface.PoEPowerWatt = p
				data.PoEEntries = append(data.PoEEntries, devices.PoEEntry{
					Interface: name,
					Status:    poe["status"],
					Voltage:   v,
					CurrentMA: c,
					PowerWatt: p,
				})
			}

			iface.UpdatedAt = now
			data.Interfaces = append(data.Interfaces, iface)
		}
	}

	data.DeviceMetrics.RXBytes = totalRXBytes
	data.DeviceMetrics.TXBytes = totalTXBytes
	data.DeviceMetrics.RXPackets = totalRXPackets
	data.DeviceMetrics.TXPackets = totalTXPackets
	data.DeviceMetrics.RXErrors = totalRXErrors
	data.DeviceMetrics.TXErrors = totalTXErrors
	data.DeviceMetrics.RXDrops = totalRXDrops
	data.DeviceMetrics.TXDrops = totalTXDrops

	// 4. Bridges
	if reply, err := client.Run("/interface/bridge/print"); err == nil {
		for _, re := range reply.Re {
			mtu, _ := strconv.Atoi(re.Map["mtu"])
			data.Bridges = append(data.Bridges, devices.BridgeInfo{
				Name:       re.Map["name"],
				MACAddress: re.Map["mac-address"],
				MTU:        mtu,
				Running:    re.Map["running"] == "true",
			})
		}
	}

	// 5. VLANs
	if reply, err := client.Run("/interface/bridge/vlan/print"); err == nil {
		for _, re := range reply.Re {
			data.VLANs = append(data.VLANs, devices.VLANInfo{
				Bridge:   re.Map["bridge"],
				VLANIDs:  re.Map["vlan-ids"],
				Tagged:   strings.Split(re.Map["tagged"], ","),
				Untagged: strings.Split(re.Map["untagged"], ","),
				Current:  re.Map["current-tagged"] != "" || re.Map["current-untagged"] != "",
			})
		}
	}

	// 6. MAC Table (Bridge Host)
	if reply, err := client.Run("/interface/bridge/host/print"); err == nil {
		count := 0
		for _, re := range reply.Re {
			if count > 200 {
				break
			}
			data.MACTable = append(data.MACTable, devices.MACTableEntry{
				MACAddress: re.Map["mac-address"],
				Interface:  re.Map["interface"],
				Bridge:     re.Map["bridge"],
				Dynamic:    re.Map["dynamic"] == "true",
				Age:        re.Map["age"],
			})
			count++
		}
	}

	return data, nil
}

func (d *Driver) PollLink(ctx context.Context, target devices.TargetConfig) (*devices.LinkProfileData, error) {
	client, err := d.dial(target)
	if err != nil {
		return nil, fmt.Errorf("connect to link device: %w", err)
	}
	defer client.Close()

	data := &devices.LinkProfileData{}
	now := time.Now()

	// 1. Resources
	if reply, err := client.Run("/system/resource/print"); err == nil && len(reply.Re) > 0 {
		m := reply.Re[0].Map
		cpu, _ := strconv.Atoi(m["cpu-load"])
		totalMem, _ := strconv.ParseInt(m["total-memory"], 10, 64)
		freeMem, _ := strconv.ParseInt(m["free-memory"], 10, 64)
		totalHdd, _ := strconv.ParseInt(m["total-hdd-space"], 10, 64)
		freeHdd, _ := strconv.ParseInt(m["free-hdd-space"], 10, 64)

		data.DeviceMetrics = devices.DeviceMetric{
			CPULoad:       cpu,
			MemoryTotal:   totalMem,
			MemoryUsed:    totalMem - freeMem,
			StorageTotal:  totalHdd,
			StorageUsed:   totalHdd - freeHdd,
			UptimeSeconds: parseUptime(m["uptime"]),
			RecordedAt:    now,
		}
	}

	// 2. Wireless Radio
	if reply, err := client.Run("/interface/wireless/print", "detail"); err == nil && len(reply.Re) > 0 {
		m := reply.Re[0].Map
		freq, _ := strconv.Atoi(m["frequency"])
		noise, _ := strconv.Atoi(m["noise-floor"])
		txPower, _ := strconv.Atoi(m["tx-power"])

		data.Wireless = devices.DeviceWireless{
			InterfaceName: m["name"],
			Mode:          m["mode"],
			SSID:          m["ssid"],
			Frequency:     freq,
			ChannelWidth:  m["channel-width"],
			NoiseFloor:    noise,
			TXPower:       txPower,
			UpdatedAt:     now,
		}
	}

	// 3. Wireless Registration Table (Link Peer Status)
	if reply, err := client.Run("/interface/wireless/registration-table/print", "detail"); err == nil && len(reply.Re) > 0 {
		m := reply.Re[0].Map
		sig, _ := strconv.Atoi(m["signal-strength"])
		ccq, _ := strconv.Atoi(strings.TrimSuffix(m["overall-tx-ccq"], "%"))
		if ccq == 0 {
			ccq, _ = strconv.Atoi(strings.TrimSuffix(m["tx-ccq"], "%"))
		}
		dist, _ := strconv.ParseFloat(strings.TrimSuffix(m["distance"], "km"), 64)
		snr, _ := strconv.Atoi(m["signal-to-noise"])
		if snr == 0 && sig != 0 && data.Wireless.NoiseFloor != 0 {
			snr = sig - data.Wireless.NoiseFloor
		}

		data.Wireless.SignalStrength = sig
		data.Wireless.CCQ = ccq
		data.Wireless.SNR = snr
		data.Wireless.DistanceKm = dist
		data.Wireless.TXRate = m["tx-rate"]
		data.Wireless.RXRate = m["rx-rate"]
		data.Wireless.RemoteMAC = m["mac-address"]
		data.Wireless.RemoteDeviceInfo = m["radio-name"]
		if data.Wireless.RemoteDeviceInfo == "" {
			data.Wireless.RemoteDeviceInfo = m["routeros-version"]
		}
		data.Wireless.ConnectedClients = len(reply.Re)
	}

	// 4. Ethernet Interfaces
	if reply, err := client.Run("/interface/print", "detail", "?type=ether"); err == nil {
		for idx, re := range reply.Re {
			m := re.Map
			iface := devices.DeviceInterface{
				IfIndex:    idx + 1,
				Name:       m["name"],
				Type:       m["type"],
				MACAddress: m["mac-address"],
				Status:     "down",
			}
			if m["running"] == "true" {
				iface.Status = "up"
			}
			rxBytes, _ := strconv.ParseInt(m["rx-byte"], 10, 64)
			txBytes, _ := strconv.ParseInt(m["tx-byte"], 10, 64)
			iface.RXBytes = rxBytes
			iface.TXBytes = txBytes
			iface.UpdatedAt = now
			data.EthernetPorts = append(data.EthernetPorts, iface)
		}
	}

	return data, nil
}

func (d *Driver) PollSector(ctx context.Context, target devices.TargetConfig) (*devices.SectorProfileData, error) {
	client, err := d.dial(target)
	if err != nil {
		return nil, fmt.Errorf("connect to sector AP: %w", err)
	}
	defer client.Close()

	data := &devices.SectorProfileData{}
	now := time.Now()

	// 1. Resources
	if reply, err := client.Run("/system/resource/print"); err == nil && len(reply.Re) > 0 {
		m := reply.Re[0].Map
		cpu, _ := strconv.Atoi(m["cpu-load"])
		totalMem, _ := strconv.ParseInt(m["total-memory"], 10, 64)
		freeMem, _ := strconv.ParseInt(m["free-memory"], 10, 64)
		totalHdd, _ := strconv.ParseInt(m["total-hdd-space"], 10, 64)
		freeHdd, _ := strconv.ParseInt(m["free-hdd-space"], 10, 64)

		data.DeviceMetrics = devices.DeviceMetric{
			CPULoad:       cpu,
			MemoryTotal:   totalMem,
			MemoryUsed:    totalMem - freeMem,
			StorageTotal:  totalHdd,
			StorageUsed:   totalHdd - freeHdd,
			UptimeSeconds: parseUptime(m["uptime"]),
			RecordedAt:    now,
		}
	}

	// 2. Wireless Radio
	if reply, err := client.Run("/interface/wireless/print", "detail"); err == nil && len(reply.Re) > 0 {
		m := reply.Re[0].Map
		freq, _ := strconv.Atoi(m["frequency"])
		noise, _ := strconv.Atoi(m["noise-floor"])
		txPower, _ := strconv.Atoi(m["tx-power"])

		data.Wireless = devices.DeviceWireless{
			InterfaceName: m["name"],
			Mode:          m["mode"],
			SSID:          m["ssid"],
			Frequency:     freq,
			ChannelWidth:  m["channel-width"],
			NoiseFloor:    noise,
			TXPower:       txPower,
			UpdatedAt:     now,
		}
	}

	// 3. ARP and DHCP maps to resolve IP & Hostnames for clients
	ipMap := make(map[string]string)
	hostMap := make(map[string]string)

	if arpReply, err := client.Run("/ip/arp/print"); err == nil {
		for _, re := range arpReply.Re {
			mac := strings.ToLower(re.Map["mac-address"])
			if mac != "" {
				ipMap[mac] = re.Map["address"]
			}
		}
	}

	if dhcpReply, err := client.Run("/ip/dhcp-server/lease/print"); err == nil {
		for _, re := range dhcpReply.Re {
			mac := strings.ToLower(re.Map["mac-address"])
			if mac != "" {
				if ipMap[mac] == "" {
					ipMap[mac] = re.Map["address"]
				}
				hostMap[mac] = re.Map["host-name"]
			}
		}
	}

	// 4. Connected Clients
	if reply, err := client.Run("/interface/wireless/registration-table/print", "detail"); err == nil {
		data.Wireless.ConnectedClients = len(reply.Re)
		for _, re := range reply.Re {
			m := re.Map
			mac := strings.ToLower(m["mac-address"])
			sig, _ := strconv.Atoi(m["signal-strength"])
			noise, _ := strconv.Atoi(m["noise-floor"])
			if noise == 0 {
				noise = data.Wireless.NoiseFloor
			}
			snr, _ := strconv.Atoi(m["signal-to-noise"])
			if snr == 0 && sig != 0 && noise != 0 {
				snr = sig - noise
			}
			ccq, _ := strconv.Atoi(strings.TrimSuffix(m["overall-tx-ccq"], "%"))
			if ccq == 0 {
				ccq, _ = strconv.Atoi(strings.TrimSuffix(m["tx-ccq"], "%"))
			}

			rxBytes, _ := strconv.ParseInt(m["bytes"], 10, 64)
			txBytes, _ := strconv.ParseInt(m["tx-bytes"], 10, 64)
			if rxBytes == 0 {
				// Sometimes formatted as "rx,tx"
				parts := strings.Split(m["bytes"], ",")
				if len(parts) == 2 {
					rxBytes, _ = strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
					txBytes, _ = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
				}
			}

			clientItem := devices.DeviceWirelessClient{
				MACAddress:    mac,
				IPAddress:     ipMap[mac],
				Hostname:      m["radio-name"],
				Signal:        sig,
				Noise:         noise,
				SNR:           snr,
				TXRate:        m["tx-rate"],
				RXRate:        m["rx-rate"],
				CCQ:           ccq,
				UptimeSeconds: parseUptime(m["uptime"]),
				RXBytes:       rxBytes,
				TXBytes:       txBytes,
				Status:        "connected",
				LastSeen:      now,
			}

			if clientItem.Hostname == "" {
				clientItem.Hostname = hostMap[mac]
			}

			data.Clients = append(data.Clients, clientItem)
		}
	}

	return data, nil
}

func parseUptime(s string) int64 {
	if s == "" {
		return 0
	}

	var totalSec int64
	var numStr string

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			numStr += string(c)
		} else {
			if numStr != "" {
				val, _ := strconv.ParseInt(numStr, 10, 64)
				switch c {
				case 'w':
					totalSec += val * 7 * 24 * 3600
				case 'd':
					totalSec += val * 24 * 3600
				case 'h':
					totalSec += val * 3600
				case 'm':
					totalSec += val * 60
				case 's':
					totalSec += val
				}
				numStr = ""
			}
		}
	}

	if numStr != "" {
		val, _ := strconv.ParseInt(numStr, 10, 64)
		totalSec += val
	}

	return totalSec
}
