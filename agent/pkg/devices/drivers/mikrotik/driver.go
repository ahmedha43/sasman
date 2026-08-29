package mikrotik

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"mikrotik-manager/agent/pkg/devices"

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
			name := m["name"]
			ifType := strings.ToLower(m["type"])
			isDynamic := m["dynamic"] == "true" || strings.HasPrefix(name, "<pppoe-") || strings.HasPrefix(name, "<l2tp-") || strings.HasPrefix(name, "<sstp-") || strings.HasPrefix(name, "<ovpn-")

			// Filter out dynamic client tunnels and loopback from main hardware port map
			if isDynamic || ifType == "pppoe-in" || ifType == "l2tp-in" || ifType == "sstp-in" || ifType == "ovpn-in" || ifType == "ppp-in" || ifType == "loopback" {
				continue
			}

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
	if ethReply, err := client.Run("/interface/ethernet/print"); err == nil {
		for _, re := range ethReply.Re {
			ethMap[re.Map["name"]] = re.Map
		}
	}

	// Monitor running ethernet ports to get negotiated speed and duplex
	for name, eth := range ethMap {
		if eth["running"] == "true" || eth["disabled"] != "true" {
			if monReply, err := client.Run("/interface/ethernet/monitor", "=numbers="+name, "=once="); err == nil && len(monReply.Re) > 0 {
				m := monReply.Re[0].Map
				if m["rate"] != "" {
					eth["speed"] = m["rate"]
				}
				if m["rate"] == "" && m["speed"] != "" {
					eth["speed"] = m["speed"]
				}
				if m["full-duplex"] != "" {
					eth["full-duplex"] = m["full-duplex"]
				}
				if m["sfp-temperature"] != "" {
					eth["sfp-temperature"] = m["sfp-temperature"]
				}
				if m["sfp-tx-power"] != "" {
					eth["sfp-tx-power"] = m["sfp-tx-power"]
				}
				if m["sfp-rx-power"] != "" {
					eth["sfp-rx-power"] = m["sfp-rx-power"]
				}
				if m["sfp-wavelength"] != "" {
					eth["sfp-wavelength"] = m["sfp-wavelength"]
				}
			}
		}
	}

	poeMap := make(map[string]map[string]string)
	if poeReply, err := client.Run("/interface/ethernet/poe/print"); err == nil {
		for _, re := range poeReply.Re {
			poeMap[re.Map["name"]] = re.Map
		}
	}

	var totalRXBytes, totalTXBytes int64
	var totalRXPackets, totalTXPackets int64
	var totalRXErrors, totalTXErrors int64
	var totalRXDrops, totalTXDrops int64

	if reply, err := client.Run("/interface/print"); err == nil {
		for idx, re := range reply.Re {
			m := re.Map
			name := m["name"]
			ifType := strings.ToLower(m["type"])
			isDynamic := m["dynamic"] == "true" || strings.HasPrefix(name, "<pppoe-") || strings.HasPrefix(name, "<l2tp-") || strings.HasPrefix(name, "<sstp-") || strings.HasPrefix(name, "<ovpn-")

			// Filter out dynamic PPPoE/VPN client sessions and loopback from physical switch ports
			if isDynamic || ifType == "pppoe-in" || ifType == "l2tp-in" || ifType == "sstp-in" || ifType == "ovpn-in" || ifType == "ppp-in" || ifType == "loopback" {
				continue
			}

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
			if rxBytes == 0 {
				rxBytes, _ = strconv.ParseInt(m["rx-bytes"], 10, 64)
			}
			if rxBytes == 0 {
				rxBytes, _ = strconv.ParseInt(m["rx_byte"], 10, 64)
			}
			if rxBytes == 0 && m["bytes"] != "" {
				rx, _ := cleanBytes(m["bytes"])
				rxBytes = rx
			}

			txBytes, _ := strconv.ParseInt(m["tx-byte"], 10, 64)
			if txBytes == 0 {
				txBytes, _ = strconv.ParseInt(m["tx-bytes"], 10, 64)
			}
			if txBytes == 0 {
				txBytes, _ = strconv.ParseInt(m["tx_byte"], 10, 64)
			}
			if txBytes == 0 && m["bytes"] != "" {
				_, tx := cleanBytes(m["bytes"])
				txBytes = tx
			}

			rxPkts, _ := strconv.ParseInt(m["rx-packet"], 10, 64)
			if rxPkts == 0 {
				rxPkts, _ = strconv.ParseInt(m["rx-packets"], 10, 64)
			}
			if rxPkts == 0 && m["packets"] != "" {
				rxP, _ := cleanBytes(m["packets"])
				rxPkts = rxP
			}

			txPkts, _ := strconv.ParseInt(m["tx-packet"], 10, 64)
			if txPkts == 0 {
				txPkts, _ = strconv.ParseInt(m["tx-packets"], 10, 64)
			}
			if txPkts == 0 && m["packets"] != "" {
				_, txP := cleanBytes(m["packets"])
				txPkts = txP
			}

			rxErr, _ := strconv.ParseInt(m["rx-error"], 10, 64)
			if rxErr == 0 {
				rxErr, _ = strconv.ParseInt(m["rx-errors"], 10, 64)
			}
			txErr, _ := strconv.ParseInt(m["tx-error"], 10, 64)
			if txErr == 0 {
				txErr, _ = strconv.ParseInt(m["tx-errors"], 10, 64)
			}

			rxDrop, _ := strconv.ParseInt(m["rx-drop"], 10, 64)
			if rxDrop == 0 {
				rxDrop, _ = strconv.ParseInt(m["rx-drops"], 10, 64)
			}
			txDrop, _ := strconv.ParseInt(m["tx-drop"], 10, 64)
			if txDrop == 0 {
				txDrop, _ = strconv.ParseInt(m["tx-drops"], 10, 64)
			}

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
				if iface.Speed == "" && iface.Status == "up" && (eth["auto-negotiation"] == "true" || eth["auto-negotiation"] == "yes") {
					iface.Speed = "1 Gbps"
				}

				iface.Duplex = eth["full-duplex"]
				if iface.Duplex == "true" || iface.Duplex == "yes" {
					iface.Duplex = "Full"
				} else if iface.Duplex == "false" || iface.Duplex == "no" {
					iface.Duplex = "Half"
				} else if iface.Status == "up" && iface.Duplex == "" {
					iface.Duplex = "Full"
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
	if reply, err := client.Run("/interface/wireless/print"); err == nil && len(reply.Re) > 0 {
		m := reply.Re[0].Map
		freq, _ := strconv.Atoi(m["frequency"])
		noise := cleanSignal(m["noise-floor"])
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
	var linkRegRows []map[string]string
	if reply, err := client.Run("/interface/wireless/registration-table/print"); err == nil && len(reply.Re) > 0 {
		for _, re := range reply.Re {
			linkRegRows = append(linkRegRows, re.Map)
		}
	}
	if len(linkRegRows) == 0 {
		if reply, err := client.Run("/interface/wifi/registration-table/print"); err == nil && len(reply.Re) > 0 {
			for _, re := range reply.Re {
				linkRegRows = append(linkRegRows, re.Map)
			}
		}
	}
	if len(linkRegRows) == 0 {
		if reply, err := client.Run("/interface/wifiwave2/registration-table/print"); err == nil && len(reply.Re) > 0 {
			for _, re := range reply.Re {
				linkRegRows = append(linkRegRows, re.Map)
			}
		}
	}

	if len(linkRegRows) > 0 {
		m := linkRegRows[0]
		sig := cleanSignal(m["signal-strength"])
		if sig == 0 {
			sig = cleanSignal(m["signal-strength-ch0"])
		}
		if sig == 0 {
			sig = cleanSignal(m["signal"])
		}
		ccq := cleanInt(m["overall-tx-ccq"])
		if ccq == 0 {
			ccq = cleanInt(m["tx-ccq"])
		}
		dist, _ := strconv.ParseFloat(strings.TrimSuffix(m["distance"], "km"), 64)
		snr := cleanInt(m["signal-to-noise"])
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
		data.Wireless.ConnectedClients = len(linkRegRows)
	}

	// 4. Ethernet Interfaces
	if reply, err := client.Run("/interface/print", "?type=ether"); err == nil {
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
			if rxBytes == 0 {
				rxBytes, _ = strconv.ParseInt(m["rx-bytes"], 10, 64)
			}
			if rxBytes == 0 && m["bytes"] != "" {
				rx, _ := cleanBytes(m["bytes"])
				rxBytes = rx
			}

			txBytes, _ := strconv.ParseInt(m["tx-byte"], 10, 64)
			if txBytes == 0 {
				txBytes, _ = strconv.ParseInt(m["tx-bytes"], 10, 64)
			}
			if txBytes == 0 && m["bytes"] != "" {
				_, tx := cleanBytes(m["bytes"])
				txBytes = tx
			}

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
	if reply, err := client.Run("/interface/wireless/print"); err == nil && len(reply.Re) > 0 {
		m := reply.Re[0].Map
		freq, _ := strconv.Atoi(m["frequency"])
		noise := cleanSignal(m["noise-floor"])
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
	var regRows []map[string]string

	// Try standard wireless registration table
	if reply, err := client.Run("/interface/wireless/registration-table/print"); err == nil && len(reply.Re) > 0 {
		for _, re := range reply.Re {
			regRows = append(regRows, re.Map)
		}
	}

	// If empty, try RouterOS v7 wifi
	if len(regRows) == 0 {
		if reply, err := client.Run("/interface/wifi/registration-table/print"); err == nil && len(reply.Re) > 0 {
			for _, re := range reply.Re {
				regRows = append(regRows, re.Map)
			}
		}
	}

	// If empty, try wifiwave2
	if len(regRows) == 0 {
		if reply, err := client.Run("/interface/wifiwave2/registration-table/print"); err == nil && len(reply.Re) > 0 {
			for _, re := range reply.Re {
				regRows = append(regRows, re.Map)
			}
		}
	}

	// If empty, try CAPsMAN
	if len(regRows) == 0 {
		if reply, err := client.Run("/caps-man/registration-table/print"); err == nil && len(reply.Re) > 0 {
			for _, re := range reply.Re {
				regRows = append(regRows, re.Map)
			}
		}
	}

	data.Wireless.ConnectedClients = len(regRows)
	for _, m := range regRows {
		mac := strings.ToLower(m["mac-address"])
		sig := cleanSignal(m["signal-strength"])
		if sig == 0 {
			sig = cleanSignal(m["signal-strength-ch0"])
		}
		if sig == 0 {
			sig = cleanSignal(m["signal"])
		}
		noise := cleanSignal(m["noise-floor"])
		if noise == 0 {
			noise = data.Wireless.NoiseFloor
		}
		snr := cleanInt(m["signal-to-noise"])
		if snr == 0 && sig != 0 && noise != 0 {
			snr = sig - noise
		}
		ccq := cleanInt(m["overall-tx-ccq"])
		if ccq == 0 {
			ccq = cleanInt(m["tx-ccq"])
		}
		if ccq == 0 {
			ccq = cleanInt(m["ccq"])
		}

		rxBytes, txBytes := cleanBytes(m["bytes"])
		if rxBytes == 0 && m["rx-bytes"] != "" {
			rxBytes, _ = strconv.ParseInt(m["rx-bytes"], 10, 64)
			txBytes, _ = strconv.ParseInt(m["tx-bytes"], 10, 64)
		}
		if rxBytes == 0 && m["bytes-received"] != "" {
			rxBytes, _ = strconv.ParseInt(m["bytes-received"], 10, 64)
			txBytes, _ = strconv.ParseInt(m["bytes-sent"], 10, 64)
		}

		txRate := m["tx-rate"]
		rxRate := m["rx-rate"]
		if txRate == "" {
			txRate = m["rate"]
		}

		hostname := m["radio-name"]
		if hostname == "" {
			hostname = m["comment"]
		}
		if hostname == "" {
			hostname = hostMap[mac]
		}
		if hostname == "" {
			hostname = m["interface"]
		}

		clientItem := devices.DeviceWirelessClient{
			MACAddress:    mac,
			IPAddress:     ipMap[mac],
			Hostname:      hostname,
			Signal:        sig,
			Noise:         noise,
			SNR:           snr,
			TXRate:        txRate,
			RXRate:        rxRate,
			CCQ:           ccq,
			UptimeSeconds: parseUptime(m["uptime"]),
			RXBytes:       rxBytes,
			TXBytes:       txBytes,
			Status:        "connected",
			LastSeen:      now,
		}

		data.Clients = append(data.Clients, clientItem)
	}

	return data, nil
}

func cleanSignal(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var res string
	isNegative := false
	if strings.HasPrefix(s, "-") {
		isNegative = true
		s = s[1:]
	}
	for _, c := range s {
		if c >= '0' && c <= '9' {
			res += string(c)
		} else if res != "" {
			break
		}
	}
	if res == "" {
		return 0
	}
	val, _ := strconv.Atoi(res)
	if isNegative {
		return -val
	}
	return val
}

func cleanInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	var res string
	isNegative := false
	if strings.HasPrefix(s, "-") {
		isNegative = true
		s = s[1:]
	}
	for _, c := range s {
		if c >= '0' && c <= '9' {
			res += string(c)
		} else if res != "" {
			break
		}
	}
	if res == "" {
		return 0
	}
	val, _ := strconv.Atoi(res)
	if isNegative {
		return -val
	}
	return val
}

func cleanBytes(s string) (int64, int64) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0
	}
	parts := strings.Split(s, ",")
	if len(parts) == 2 {
		rx, _ := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		tx, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		return rx, tx
	}
	b, _ := strconv.ParseInt(s, 10, 64)
	return b, 0
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

// CableTest executes an ethernet TDR cable diagnostic on a specific interface
func (d *Driver) CableTest(ctx context.Context, target devices.TargetConfig, ifaceName string) (*devices.CableTestResult, error) {
	client, err := d.dial(target)
	if err != nil {
		return nil, fmt.Errorf("connect to device: %w", err)
	}
	defer client.Close()

	reply, err := client.Run("/interface/ethernet/cable-test", "=numbers="+ifaceName, "=once=")
	if err != nil {
		return nil, fmt.Errorf("cable-test failed: %w", err)
	}

	result := &devices.CableTestResult{
		Interface:  ifaceName,
		Status:     "unknown",
		RawDetails: make(map[string]string),
	}

	if len(reply.Re) > 0 {
		m := reply.Re[0].Map
		result.RawDetails = m
		result.Status = m["status"]
		if result.Status == "" {
			result.Status = m["cable-test"]
		}
		if result.Status == "" {
			result.Status = "ok"
		}

		// Parse cable-pairs=open:14,open:14,open:14,open:14 or ok:42,...
		if pairsStr, ok := m["cable-pairs"]; ok && pairsStr != "" {
			pairs := strings.Split(pairsStr, ",")
			for idx, p := range pairs {
				parts := strings.Split(p, ":")
				pairItem := devices.CablePairStatus{
					Pair:   fmt.Sprintf("Pair %d", idx+1),
					Status: parts[0],
				}
				if len(parts) > 1 {
					dist, _ := strconv.ParseFloat(strings.TrimSuffix(parts[1], "m"), 64)
					pairItem.Length = dist
					if result.LengthMeter == 0 && dist > 0 {
						result.LengthMeter = dist
					}
				}
				result.CablePairs = append(result.CablePairs, pairItem)
			}
		}

		// Direct pair keys: pair1-status, pair2-status, etc.
		if len(result.CablePairs) == 0 {
			for pNum := 1; pNum <= 4; pNum++ {
				pKey := fmt.Sprintf("pair%d-status", pNum)
				dKey := fmt.Sprintf("pair%d-distance", pNum)
				if pStat, ok := m[pKey]; ok {
					dist, _ := strconv.ParseFloat(strings.TrimSuffix(m[dKey], "m"), 64)
					result.CablePairs = append(result.CablePairs, devices.CablePairStatus{
						Pair:   fmt.Sprintf("Pair %d", pNum),
						Status: pStat,
						Length: dist,
					})
					if result.LengthMeter == 0 && dist > 0 {
						result.LengthMeter = dist
					}
				}
			}
		}

		if lenStr, ok := m["length"]; ok && result.LengthMeter == 0 {
			dist, _ := strconv.ParseFloat(strings.TrimSuffix(lenStr, "m"), 64)
			result.LengthMeter = dist
		}
	}

	return result, nil
}

// MonitorPort queries live, real-time link parameters (speed, duplex, SFP diagnostics, flow control) for a specific interface
func (d *Driver) MonitorPort(ctx context.Context, target devices.TargetConfig, ifaceName string) (*devices.PortMonitorResult, error) {
	client, err := d.dial(target)
	if err != nil {
		return nil, fmt.Errorf("connect to device: %w", err)
	}
	defer client.Close()

	reply, err := client.Run("/interface/ethernet/monitor", "=numbers="+ifaceName, "=once=")
	if err != nil {
		return nil, fmt.Errorf("monitor failed: %w", err)
	}

	result := &devices.PortMonitorResult{
		Interface: ifaceName,
		Status:    "down",
	}

	if len(reply.Re) > 0 {
		m := reply.Re[0].Map
		result.Status = m["status"]
		result.AutoNegotiation = m["auto-negotiation"]
		result.Rate = m["rate"]
		if result.Rate == "" {
			result.Rate = m["speed"]
		}
		result.FullDuplex = m["full-duplex"] == "true" || m["full-duplex"] == "yes"
		result.DefaultName = m["default-name"]
		result.TXFlowControl = m["tx-flow-control"]
		result.RXFlowControl = m["rx-flow-control"]
		result.SFPModulePresent = m["sfp-module-present"]
		result.SFPRXLoss = m["sfp-rx-loss"]
		result.SFPTXFault = m["sfp-tx-fault"]
		result.SFPWavelength = m["sfp-wavelength"]
		result.SFPVendor = m["sfp-vendor-name"]
		result.SFPPartNumber = m["sfp-vendor-part-number"]

		if f, err := strconv.ParseFloat(m["sfp-temperature"], 64); err == nil {
			result.SFPTemp = f
		}
		if f, err := strconv.ParseFloat(m["sfp-supply-voltage"], 64); err == nil {
			result.SFPSupplyVolt = f
		}
		if f, err := strconv.ParseFloat(m["sfp-tx-bias-current"], 64); err == nil {
			result.SFPTXBiasCurrent = f
		}
		if f, err := strconv.ParseFloat(m["sfp-tx-power"], 64); err == nil {
			result.SFPTXPowerDBm = f
		}
		if f, err := strconv.ParseFloat(m["sfp-rx-power"], 64); err == nil {
			result.SFPRXPowerDBm = f
		}
	}

	return result, nil
}

// GetSwitchHosts fetches switch chip and bridge MAC host tables
func (d *Driver) GetSwitchHosts(ctx context.Context, target devices.TargetConfig) ([]devices.MACTableEntry, error) {
	client, err := d.dial(target)
	if err != nil {
		return nil, fmt.Errorf("connect to device: %w", err)
	}
	defer client.Close()

	var entries []devices.MACTableEntry

	// 1. Switch Chip Host Table
	if reply, err := client.Run("/interface/ethernet/switch/host/print"); err == nil {
		for _, re := range reply.Re {
			m := re.Map
			entries = append(entries, devices.MACTableEntry{
				MACAddress: m["mac-address"],
				Interface:  m["port"],
				Bridge:     m["switch"],
				Dynamic:    m["dynamic"] == "true" || m["dynamic"] == "yes",
				Age:        m["age"],
			})
		}
	}

	// 2. Bridge Host Table
	if reply, err := client.Run("/interface/bridge/host/print"); err == nil {
		for _, re := range reply.Re {
			m := re.Map
			entries = append(entries, devices.MACTableEntry{
				MACAddress: m["mac-address"],
				Interface:  m["interface"],
				Bridge:     m["bridge"],
				Dynamic:    m["dynamic"] == "true" || m["dynamic"] == "yes",
				Age:        m["age"],
			})
		}
	}

	return entries, nil
}
