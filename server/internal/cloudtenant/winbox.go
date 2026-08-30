package cloudtenant

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type WinboxProxyManager struct {
	mu        sync.Mutex
	listeners map[string]net.Listener // subdomain -> listener
	ports     map[string]int          // subdomain -> port
	domain    string
}

var globalWinboxProxyMgr = &WinboxProxyManager{
	listeners: make(map[string]net.Listener),
	ports:     make(map[string]int),
	domain:    "sas-man.net",
}

func GetGlobalWinboxProxyMgr() *WinboxProxyManager {
	return globalWinboxProxyMgr
}

// StartForwarder starts or updates a Winbox TCP proxy on the given public port for a tenant.
func (w *WinboxProxyManager) StartForwarder(subdomain string, listenPort int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	sub := strings.ToLower(strings.TrimSpace(subdomain))
	if sub == "" || listenPort <= 0 {
		return fmt.Errorf("invalid subdomain or port")
	}

	// If already running on this port, return
	if curPort, ok := w.ports[sub]; ok && curPort == listenPort {
		if _, hasListener := w.listeners[sub]; hasListener {
			return nil
		}
	}

	// Close existing listener if port changed
	if l, ok := w.listeners[sub]; ok {
		_ = l.Close()
		delete(w.listeners, sub)
		delete(w.ports, sub)
	}

	addr := fmt.Sprintf("0.0.0.0:%d", listenPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[Winbox-Cloud] ⚠️ Could not bind Winbox port %d for [%s]: %v", listenPort, sub, err)
		return err
	}

	w.listeners[sub] = listener
	w.ports[sub] = listenPort
	log.Printf("[Winbox-Cloud] 🚀 Winbox TCP proxy active for [%s] on port %d -> MikroTik :8291", sub, listenPort)

	go func(subdomain string, l net.Listener, port int) {
		for {
			clientConn, err := l.Accept()
			if err != nil {
				// Listener closed
				return
			}
			go handleWinboxConnection(subdomain, clientConn)
		}
	}(sub, listener, listenPort)

	return nil
}

// StopForwarder stops a Winbox listener
func (w *WinboxProxyManager) StopForwarder(subdomain string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	sub := strings.ToLower(strings.TrimSpace(subdomain))
	if l, ok := w.listeners[sub]; ok {
		_ = l.Close()
		delete(w.listeners, sub)
		delete(w.ports, sub)
		log.Printf("[Winbox-Cloud] Stopped Winbox proxy for [%s]", sub)
	}
}

func handleWinboxConnection(subdomain string, clientConn net.Conn) {
	defer clientConn.Close()

	// Find the MikroTik tunnel IP for this subdomain
	targetIP := resolveMikroTikTunnelIP(subdomain)
	if targetIP == "" {
		log.Printf("[Winbox-Cloud] ❌ Rejected Winbox connection for [%s]: Router tunnel not connected", subdomain)
		return
	}

	targetAddr := fmt.Sprintf("%s:8291", targetIP)
	routerConn, err := net.DialTimeout("tcp", targetAddr, 6*time.Second)
	if err != nil {
		log.Printf("[Winbox-Cloud] ❌ Failed to connect to MikroTik Winbox at %s for [%s]: %v", targetAddr, subdomain, err)
		return
	}
	defer routerConn.Close()

	log.Printf("[Winbox-Cloud] 🔌 Active Winbox session opened for [%s] via %s", subdomain, targetAddr)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(routerConn, clientConn)
		_ = routerConn.Close()
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, routerConn)
		_ = clientConn.Close()
	}()

	wg.Wait()
	log.Printf("[Winbox-Cloud] 🏁 Winbox session closed for [%s]", subdomain)
}

// resolveMikroTikTunnelIP looks up the VPN IP in openvpn-status.log
// resolveMikroTikTunnelIP looks up the VPN IP in openvpn-status.log or dynamic probes
func resolveMikroTikTunnelIP(subdomain string) string {
	sub := strings.ToLower(strings.TrimSpace(subdomain))
	cnTarget := fmt.Sprintf("agent-%s-SASMAN", sub)

	// Status log paths
	statusPaths := []string{
		"/app/data/openvpn-status.log",
		"/root/sasman-central/server/data/openvpn-status.log",
		"data/openvpn-status.log",
		"/etc/openvpn/openvpn-status.log",
		"/run/openvpn/server.status",
		"/tmp/openvpn-status.log",
	}

	for _, path := range statusPaths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		isRoutingTable := false

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "ROUTING TABLE" {
				isRoutingTable = true
				continue
			}
			if line == "GLOBAL STATS" || line == "END" {
				isRoutingTable = false
			}

			if isRoutingTable {
				parts := strings.Split(line, ",")
				if len(parts) >= 2 {
					ip := strings.TrimSpace(parts[0])
					cn := strings.TrimSpace(parts[1])
					if strings.EqualFold(cn, cnTarget) {
						return ip
					}
				}
			}
		}
	}

	// Dynamic probe active Winbox listeners on the /30 net30 range
	candidates := []string{"10.250.0.6", "10.250.0.10", "10.250.0.14", "10.250.0.18", "10.250.0.2"}
	for _, ip := range candidates {
		conn, err := net.DialTimeout("tcp", ip+":8291", 400*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return ip
		}
	}

	return "10.250.0.6"
}
