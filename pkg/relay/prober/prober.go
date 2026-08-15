package prober

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"runtime"
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

	return &Prober{
		services:    make(map[string]relay.ServiceDefinition),
		results:     make(map[string]relay.HealthProbe),
		httpClient:  &http.Client{Transport: transport, Timeout: 4 * time.Second},
		stopCh:      make(chan struct{}),
		onTelemetry: onTelemetry,
		agentID:     agentID,
		subdomain:   subdomain,
	}
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
	p.mu.Unlock()

	if p.onTelemetry != nil {
		telemetry := relay.ServiceTelemetry{
			AgentID:     p.agentID,
			Subdomain:   p.subdomain,
			Timestamp:   time.Now().UTC(),
			Services:    results,
			NodeMetrics: p.collectNodeMetrics(),
		}
		p.onTelemetry(telemetry)
	}
}

func (p *Prober) probeService(svc relay.ServiceDefinition) relay.HealthProbe {
	probeType := svc.ProbeConfig.Type
	if probeType == "" {
		if len(svc.Domains) > 0 {
			probeType = "https"
		} else {
			probeType = "tcp_ping"
		}
	}

	target := svc.ProbeConfig.TargetURL
	if target == "" && len(svc.Domains) > 0 {
		target = "https://" + svc.Domains[0]
	}

	start := time.Now()
	switch probeType {
	case "http", "https":
		req, err := http.NewRequestWithContext(context.Background(), "GET", target, nil)
		if err != nil {
			return relay.HealthProbe{Available: false, LatencyMs: 0, PacketLoss: 100, LastChecked: time.Now().UTC(), ErrorMessage: err.Error()}
		}
		if svc.ProbeConfig.HostHeader != "" {
			req.Host = svc.ProbeConfig.HostHeader
		}

		resp, err := p.httpClient.Do(req)
		latency := float64(time.Since(start).Microseconds()) / 1000.0

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

	case "tcp_ping":
		host := target
		if host == "" && len(svc.Domains) > 0 {
			port := 443
			if len(svc.Ports) > 0 {
				port = svc.Ports[0]
			}
			host = fmt.Sprintf("%s:%d", svc.Domains[0], port)
		}
		conn, err := net.DialTimeout("tcp", host, 3*time.Second)
		latency := float64(time.Since(start).Microseconds()) / 1000.0
		if err != nil {
			return relay.HealthProbe{Available: false, LatencyMs: latency, PacketLoss: 100, LastChecked: time.Now().UTC(), ErrorMessage: err.Error()}
		}
		_ = conn.Close()
		return relay.HealthProbe{Available: true, LatencyMs: latency, PacketLoss: 0, LastChecked: time.Now().UTC()}

	default:
		return relay.HealthProbe{Available: true, LatencyMs: 1.0, LastChecked: time.Now().UTC()}
	}
}

func (p *Prober) collectNodeMetrics() relay.NodeMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return relay.NodeMetrics{
		MemoryMB: float64(m.Alloc) / (1024 * 1024),
	}
}
