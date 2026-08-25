package prober

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mikrotik-manager/pkg/relay"
)

// Prober periodically checks the health of services and collects node telemetry
type Prober struct {
	mu          sync.RWMutex
	services    map[string]relay.ServiceDefinition
	results     map[string]relay.HealthProbe
	httpClient  *http.Client
	running     int32
	stopCh      chan struct{}
	onTelemetry func(telemetry relay.ServiceTelemetry)
	agentID     string
	subdomain   string
	wanPublicIP string
	wanISP      string
	wanASN      string
	wanCountry  string
}

func NewProber(agentID, subdomain string, onTelemetry func(relay.ServiceTelemetry)) *Prober {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Prober only checks connectivity & response time
		},
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 5 * time.Second,
		}).DialContext,
		MaxIdleConns:          50,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 4 * time.Second,
	}

	p := &Prober{
		services:    make(map[string]relay.ServiceDefinition),
		results:     make(map[string]relay.HealthProbe),
		httpClient:  &http.Client{Transport: transport, Timeout: 5 * time.Second},
		stopCh:      make(chan struct{}),
		onTelemetry: onTelemetry,
		agentID:     agentID,
		subdomain:   subdomain,
		wanCountry:  "",
		wanISP:      "",
		wanASN:      "",
	}
	go p.detectWANLoop()
	return p
}

func (p *Prober) detectWANLoop() {
	// Try immediately on start with retries
	for i := 0; i < 5; i++ {
		if p.detectWAN() {
			break
		}
		time.Sleep(2 * time.Second)
	}

	// Periodic re-check every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.detectWAN()
		}
	}
}

func (p *Prober) detectWAN() bool {
	endpoints := []string{
		"https://ipwhois.app/json/",
		"https://ipapi.co/json/",
		"https://ipinfo.io/json",
	}

	for _, ep := range endpoints {
		req, err := http.NewRequest("GET", ep, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SASMAN/5.0; +https://sas-man.net)")

		resp, err := p.httpClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		var data struct {
			IP          string `json:"ip"`
			Org         string `json:"org"`
			ISP         string `json:"isp"`
			ASN         string `json:"asn"`
			CountryCode string `json:"country_code"`
			Country     string `json:"country"`
		}

		decodeErr := json.NewDecoder(resp.Body).Decode(&data)
		resp.Body.Close()

		if decodeErr == nil && data.IP != "" {
			p.mu.Lock()
			p.wanPublicIP = data.IP

			if data.CountryCode != "" {
				p.wanCountry = data.CountryCode
			} else if data.Country == "Iraq" || data.Country == "IQ" {
				p.wanCountry = "IQ"
			} else {
				p.wanCountry = data.Country
			}

			// ISP and Org detection
			if data.ISP != "" {
				p.wanISP = data.ISP
			} else if data.Org != "" {
				p.wanISP = data.Org
			}

			// ASN detection
			if data.ASN != "" {
				p.wanASN = data.ASN
			} else if data.Org != "" {
				parts := strings.Fields(data.Org)
				if len(parts) > 0 && strings.HasPrefix(parts[0], "AS") {
					p.wanASN = parts[0]
				}
			}

			p.mu.Unlock()

			// Trigger immediate telemetry push with true WAN details
			p.runAllProbes()
			return true
		}
	}

	return false
}

// UpdateServices updates the list of services to probe
func (p *Prober) UpdateServices(services []relay.ServiceDefinition) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.services = make(map[string]relay.ServiceDefinition)
	for _, svc := range services {
		if svc.Enabled {
			p.services[svc.ID] = svc
		}
	}
}

// Start launches the background probing worker loop
func (p *Prober) Start() {
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return
	}

	go p.probeLoop()
}

// Stop terminates probing
func (p *Prober) Stop() {
	if atomic.CompareAndSwapInt32(&p.running, 1, 0) {
		close(p.stopCh)
	}
}

func (p *Prober) probeLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Initial probe immediately
	p.runAllProbes()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.runAllProbes()
		}
	}
}

func (p *Prober) TriggerProbeNow(serviceID string) {
	if serviceID == "" || serviceID == "all" {
		go p.runAllProbes()
		return
	}

	p.mu.RLock()
	svc, ok := p.services[serviceID]
	p.mu.RUnlock()

	if !ok {
		return
	}

	go func(s relay.ServiceDefinition) {
		probe := p.probeService(s)
		p.mu.Lock()
		p.results[s.ID] = probe
		resultsCopy := make(map[string]relay.HealthProbe, len(p.results))
		for k, v := range p.results {
			resultsCopy[k] = v
		}
		p.mu.Unlock()

		if p.onTelemetry != nil {
			telemetry := relay.ServiceTelemetry{
				AgentID:     p.agentID,
				Subdomain:   p.subdomain,
				PublicIP:    p.wanPublicIP,
				ISPName:     p.wanISP,
				ASN:         p.wanASN,
				CountryCode: p.wanCountry,
				Timestamp:   time.Now().UTC(),
				Services:    resultsCopy,
				NodeMetrics: p.collectNodeMetrics(),
			}
			p.onTelemetry(telemetry)
		}
	}(svc)
}

func (p *Prober) runAllProbes() {
	p.mu.RLock()
	svcs := make([]relay.ServiceDefinition, 0, len(p.services))
	for _, s := range p.services {
		svcs = append(svcs, s)
	}
	p.mu.RUnlock()

	var wg sync.WaitGroup
	results := make(map[string]relay.HealthProbe)
	var resMu sync.Mutex

	for _, svc := range svcs {
		wg.Add(1)
		go func(s relay.ServiceDefinition) {
			defer wg.Done()
			probe := p.probeService(s)
			resMu.Lock()
			results[s.ID] = probe
			resMu.Unlock()
		}(svc)
	}

	wg.Wait()

	p.mu.Lock()
	p.results = results
	resultsCopy := make(map[string]relay.HealthProbe, len(p.results))
	for k, v := range p.results {
		resultsCopy[k] = v
	}
	p.mu.Unlock()

	if p.onTelemetry != nil {
		telemetry := relay.ServiceTelemetry{
			AgentID:     p.agentID,
			Subdomain:   p.subdomain,
			PublicIP:    p.wanPublicIP,
			ISPName:     p.wanISP,
			ASN:         p.wanASN,
			CountryCode: p.wanCountry,
			Timestamp:   time.Now().UTC(),
			Services:    resultsCopy,
			NodeMetrics: p.collectNodeMetrics(),
		}
		p.onTelemetry(telemetry)
	}
}

func (p *Prober) probeService(svc relay.ServiceDefinition) relay.HealthProbe {
	probeType := strings.ToLower(strings.TrimSpace(svc.ProbeConfig.Type))
	if probeType == "" {
		probeType = "tcp_ping"
	}

	target := strings.TrimSpace(svc.ProbeConfig.TargetURL)
	targetDomain := ""
	if len(svc.Domains) > 0 {
		targetDomain = strings.TrimPrefix(svc.Domains[0], "*.")
	}

	targetPort := 443
	if len(svc.Ports) > 0 && svc.Ports[0] > 0 {
		targetPort = svc.Ports[0]
	}

	switch probeType {
	case "tcp_ping", "ping", "tcp":
		host := target
		if host == "" || strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
			if u, err := url.Parse(host); err == nil && u.Hostname() != "" {
				host = u.Hostname()
				if u.Port() != "" {
					if prt, err := strconv.Atoi(u.Port()); err == nil && prt > 0 {
						targetPort = prt
					}
				}
			} else if targetDomain != "" {
				host = targetDomain
			}
		}

		dialTarget := net.JoinHostPort(host, strconv.Itoa(targetPort))
		dialer := net.Dialer{Timeout: 3 * time.Second}
		start := time.Now()
		conn, err := dialer.Dial("tcp", dialTarget)
		latency := float64(time.Since(start).Microseconds()) / 1000.0

		if err != nil {
			return relay.HealthProbe{
				Available:    false,
				LatencyMs:    latency,
				PacketLoss:   100,
				LastChecked:  time.Now().UTC(),
				ErrorMessage: err.Error(),
			}
		}
		_ = conn.Close()

		return relay.HealthProbe{
			Available:   true,
			LatencyMs:   latency,
			PacketLoss:  0,
			LastChecked: time.Now().UTC(),
		}

	case "http", "https":
		if target == "" && targetDomain != "" {
			target = probeType + "://" + targetDomain
		}
		if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
			target = probeType + "://" + target
		}

		var tcpConnectStart, tcpConnectDone time.Time
		trace := &httptrace.ClientTrace{
			ConnectStart: func(network, addr string) {
				tcpConnectStart = time.Now()
			},
			ConnectDone: func(network, addr string, err error) {
				tcpConnectDone = time.Now()
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), "GET", target, nil)
		if err != nil {
			return relay.HealthProbe{Available: false, LatencyMs: 0, PacketLoss: 100, LastChecked: time.Now().UTC(), ErrorMessage: err.Error()}
		}
		if svc.ProbeConfig.HostHeader != "" {
			req.Host = svc.ProbeConfig.HostHeader
		}

		totalStart := time.Now()
		resp, err := p.httpClient.Do(req)
		totalDuration := float64(time.Since(totalStart).Microseconds()) / 1000.0

		latency := totalDuration
		if !tcpConnectStart.IsZero() && !tcpConnectDone.IsZero() {
			latency = float64(tcpConnectDone.Sub(tcpConnectStart).Microseconds()) / 1000.0
		}

		if err != nil {
			return relay.HealthProbe{Available: false, LatencyMs: latency, PacketLoss: 100, LastChecked: time.Now().UTC(), ErrorMessage: err.Error()}
		}
		_ = resp.Body.Close()

		expected := svc.ProbeConfig.ExpectedCode
		if expected == 0 {
			expected = 200
		}

		isAvailable := (resp.StatusCode >= 200 && resp.StatusCode < 400) || (resp.StatusCode == expected)
		return relay.HealthProbe{
			Available:   isAvailable,
			LatencyMs:   latency,
			PacketLoss:  0,
			LastChecked: time.Now().UTC(),
		}

	default:
		dialTarget := net.JoinHostPort(targetDomain, strconv.Itoa(targetPort))
		dialer := net.Dialer{Timeout: 3 * time.Second}
		start := time.Now()
		conn, err := dialer.Dial("tcp", dialTarget)
		latency := float64(time.Since(start).Microseconds()) / 1000.0
		if err != nil {
			return relay.HealthProbe{Available: false, LatencyMs: latency, PacketLoss: 100, LastChecked: time.Now().UTC(), ErrorMessage: err.Error()}
		}
		_ = conn.Close()
		return relay.HealthProbe{Available: true, LatencyMs: latency, PacketLoss: 0, LastChecked: time.Now().UTC()}
	}
}

func (p *Prober) collectNodeMetrics() relay.NodeMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return relay.NodeMetrics{
		MemoryMB: float64(m.Alloc) / (1024 * 1024),
	}
}
