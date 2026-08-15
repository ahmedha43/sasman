package broadcast

import (
	_ "embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"mikrotik-manager/pkg/core"
	"github.com/go-routeros/routeros/v3"
)

//go:embed splash_page.html
var splashPageHTML string

type SplashManager struct {
	mu           sync.RWMutex
	httpServer   *http.Server
	activeSplash *BroadcastMessage
	clientFunc   func() (*routeros.Client, error)
	onLog        func(bLog BroadcastLogPayload)
	running      bool
}

var globalSplashMgr *SplashManager

func InitSplashManager(clientFunc func() (*routeros.Client, error), onLog func(bLog BroadcastLogPayload)) *SplashManager {
	mgr := &SplashManager{
		clientFunc: clientFunc,
		onLog:      onLog,
	}
	globalSplashMgr = mgr
	mgr.startHTTPServer(":18080")
	return mgr
}

func (m *SplashManager) startHTTPServer(addr string) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		m.mu.RLock()
		bc := m.activeSplash
		m.mu.RUnlock()

		if bc == nil {
			// No active splash, tell user and close
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!doctype html><html lang="ar" dir="rtl"><body style="font-family:sans-serif; text-align:center; padding:40px; background:#0f172a; color:#fff;"><h3>✅ تم تفعيل اتصالك بالإنترنت بنجاح. يمكنك المتابعة.</h3><script>setTimeout(()=>{ window.location.href="http://google.com"; }, 2000);</script></body></html>`))
			return
		}

		// Extract client IP
		clientIP := extractIP(r)

		// Render Splash page with dynamic data
		dur := bc.SplashDurationSec
		if dur <= 0 {
			dur = 10
		}

		html := splashPageHTML
		html = strings.ReplaceAll(html, "{{TITLE}}", escapeHTML(bc.Title))
		html = strings.ReplaceAll(html, "{{MESSAGE}}", escapeHTML(bc.Message))
		html = strings.ReplaceAll(html, "{{IMAGE_URL}}", bc.ImageURL)
		html = strings.ReplaceAll(html, "{{ACTION_URL}}", bc.ActionURL)
		html = strings.ReplaceAll(html, "{{ACTION_TEXT}}", escapeHTML(bc.ActionText))
		html = strings.ReplaceAll(html, "{{DURATION}}", fmt.Sprintf("%d", dur))
		html = strings.ReplaceAll(html, "{{CAMPAIGN_ID}}", bc.ID)
		html = strings.ReplaceAll(html, "{{CLIENT_IP}}", clientIP)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	})

	mux.HandleFunc("/broadcast/dismiss", func(w http.ResponseWriter, r *http.Request) {
		clientIP := r.URL.Query().Get("ip")
		if clientIP == "" {
			clientIP = extractIP(r)
		}
		campaignID := r.URL.Query().Get("id")
		clicked := r.URL.Query().Get("clicked") == "1"

		if clientIP != "" {
			m.RemoveUserFromAdList(clientIP)
		}

		if m.onLog != nil && campaignID != "" {
			clk := 0
			if clicked {
				clk = 1
			}
			m.onLog(BroadcastLogPayload{
				BroadcastID:    campaignID,
				UserIdentifier: clientIP,
				ViewedAt:       time.Now().UTC(),
				Clicked:        clk,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	m.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("[Splash] Ad-Engine HTTP Interceptor listening on %s", addr)
		if err := m.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Splash] HTTP server error: %v", err)
		}
	}()
}

// ApplySplashCampaign configures MikroTik NAT and targets user IPs
func (m *SplashManager) ApplySplashCampaign(bc BroadcastMessage) error {
	m.mu.Lock()
	m.activeSplash = &bc
	m.mu.Unlock()

	if m.clientFunc == nil {
		return nil
	}

	rClient, err := m.clientFunc()
	if err != nil || rClient == nil {
		return fmt.Errorf("mikrotik client connect error: %w", err)
	}
	defer rClient.Close()

	// 1. Ensure NAT redirect rule exists for sasman_ad_pending on port 80 -> 18080
	m.ensureNATRedirectRule(rClient)

	// 2. Fetch active PPPoE / Hotspot users from MikroTik
	activeReply, err := core.SafeRun(rClient, "/interface/pppoe-server/print", "?running=true")
	if err == nil && activeReply != nil {
		for _, re := range activeReply.Re {
			ip := re.Map["user-address"]
			if ip == "" {
				ip = re.Map["remote-address"]
			}
			if ip != "" {
				m.AddUserToAdList(rClient, ip, 3*time.Hour)
			}
		}
	}

	return nil
}

func (m *SplashManager) ensureNATRedirectRule(rClient *routeros.Client) {
	// Check if rule already exists
	checkReply, err := core.SafeRun(rClient, "/ip/firewall/nat/print", "?comment=SASMAN-Ad-Splash")
	if err == nil && checkReply != nil && len(checkReply.Re) > 0 {
		return // Rule already present
	}

	// Add redirect rule at top (place-before=0)
	_, _ = core.SafeRun(rClient, "/ip/firewall/nat/add",
		"=chain=dstnat",
		"=src-address-list=sasman_ad_pending",
		"=protocol=tcp",
		"=dst-port=80",
		"=action=redirect",
		"=to-ports=18080",
		"=comment=SASMAN-Ad-Splash",
		"=place-before=0")
}

func (m *SplashManager) AddUserToAdList(rClient *routeros.Client, ip string, timeout time.Duration) {
	timeoutStr := fmt.Sprintf("%dh", int(timeout.Hours()))
	if timeoutStr == "0h" {
		timeoutStr = "3h"
	}

	// Add IP to address-list with timeout
	_, _ = core.SafeRun(rClient, "/ip/firewall/address-list/add",
		"=list=sasman_ad_pending",
		fmt.Sprintf("=address=%s", ip),
		fmt.Sprintf("=timeout=%s", timeoutStr),
		"=comment=SASMAN-Ad-Pending")
}

func (m *SplashManager) RemoveUserFromAdList(ip string) {
	if m.clientFunc == nil {
		return
	}
	rClient, err := m.clientFunc()
	if err != nil || rClient == nil {
		return
	}
	defer rClient.Close()

	// Find entry in address-list and remove
	reply, err := core.SafeRun(rClient, "/ip/firewall/address-list/print", "?list=sasman_ad_pending", fmt.Sprintf("?address=%s", ip))
	if err == nil && reply != nil {
		for _, re := range reply.Re {
			if id := re.Map[".id"]; id != "" {
				_, _ = core.SafeRun(rClient, "/ip/firewall/address-list/remove", fmt.Sprintf("=.id=%s", id))
			}
		}
	}
}

func extractIP(r *http.Request) string {
	remote := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		return host
	}
	return remote
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
