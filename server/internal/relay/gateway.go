package relay

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"mikrotik-manager/pkg/relay/multiplexer"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// GatewayServer manages VPS egress streaming for agents
type GatewayServer struct {
	tcpListener net.Listener
	running     bool
	mu          sync.Mutex
}

func NewGatewayServer() *GatewayServer {
	return &GatewayServer{}
}

// StartTCPListener starts a background TCP listener on port 18444
func (gs *GatewayServer) StartTCPListener(port int) error {
	if port <= 0 {
		port = 18444
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	gs.tcpListener = ln
	gs.running = true
	log.Printf("[Relay VPS Gateway] TCP Egress Gateway listening on :%d", port)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if !gs.running {
					return
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			go func(c net.Conn) {
				sess := multiplexer.NewSession(c, true)
				HandleServerEgressSession(sess)
			}(conn)
		}
	}()
	return nil
}

// RegisterWebSocketGateway adds Fiber WebSocket endpoint on /api/relay/gateway
func (gs *GatewayServer) RegisterWebSocketGateway(router fiber.Router) {
	router.Use("/gateway", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	router.Get("/gateway", websocket.New(func(c *websocket.Conn) {
		rwc := &fiberWSReadWriteCloser{conn: c}
		sess := multiplexer.NewSession(rwc, true)
		HandleServerEgressSession(sess)
	}))
}

// HandleServerEgressSession processes streams on a multiplexer session, dialing outbound from VPS
func HandleServerEgressSession(sess *multiplexer.Session) {
	defer sess.Close()

	for {
		stream, err := sess.AcceptStream()
		if err != nil {
			return
		}

		go func(s *multiplexer.Stream) {
			defer s.Close()

			meta := s.Meta()
			target := meta.TargetHost
			if target == "" {
				target = meta.SNI
			}
			if target == "" {
				return
			}

			// Ensure port is present
			if !strings.Contains(target, ":") {
				target = net.JoinHostPort(target, "443")
			}

			dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 15 * time.Second}
			outConn, err := dialer.DialContext(context.Background(), "tcp", target)
			if err != nil {
				log.Printf("[Relay VPS Egress] Failed to dial target %s: %v", target, err)
				return
			}
			defer outConn.Close()

			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				_, _ = io.Copy(outConn, s)
			}()

			go func() {
				defer wg.Done()
				_, _ = io.Copy(s, outConn)
			}()

			wg.Wait()
		}(stream)
	}
}

type fiberWSReadWriteCloser struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func (f *fiberWSReadWriteCloser) Read(p []byte) (int, error) {
	for {
		msgType, data, err := f.conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		return copy(p, data), nil
	}
}

func (f *fiberWSReadWriteCloser) Write(p []byte) (int, error) {
	f.writeMu.Lock()
	defer f.writeMu.Unlock()
	err := f.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (f *fiberWSReadWriteCloser) Close() error {
	return f.conn.Close()
}
