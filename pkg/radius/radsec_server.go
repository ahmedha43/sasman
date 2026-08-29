package radius

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/wxccs/radius/v2/packet"
	"github.com/wxccs/radius/v2/server"

	"mikrotik-manager/pkg/pki"
)

type serverRequestAdapter struct {
	pkt        *packet.Packet
	secret     []byte
	remoteAddr net.Addr
}

func (a *serverRequestAdapter) toServerRequest() *server.Request {
	return &server.Request{
		Packet:     a.pkt,
		Secret:     a.secret,
		RemoteAddr: a.remoteAddr,
	}
}

// StartRadSecServer initializes and runs the RadSec (TLS / mTLS) listener
func StartRadSecServer() {
	radSecPort := 2083
	if envPort := os.Getenv("RADSEC_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			radSecPort = p
		}
	}

	// 1. Initialize PKI
	if err := pki.InitPKI(); err != nil {
		log.Printf("[radsec] ❌ Failed to initialize PKI: %v", err)
		return
	}

	// 2. Build TLS Server Config with Client Certificate Verification & Revocation Check
	verifyClientFunc := func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(verifiedChains) == 0 || len(verifiedChains[0]) == 0 {
			return fmt.Errorf("no verified client certificate chain")
		}
		clientCert := verifiedChains[0][0]
		cn := clientCert.Subject.CommonName
		if cn == "" {
			return fmt.Errorf("client certificate is missing CommonName")
		}

		// Check if certificate is revoked in DB
		if DB != nil {
			var revoked int
			err := DB.QueryRow("SELECT revoked FROM nas_certificates WHERE common_name = ?", cn).Scan(&revoked)
			if err == nil && revoked == 1 {
				return fmt.Errorf("certificate for [%s] has been REVOKED", cn)
			}
		}
		return nil
	}

	tlsConfig, err := pki.GetServerTLSConfig(verifyClientFunc)
	if err != nil {
		log.Printf("[radsec] ❌ Failed to get Server TLS Config: %v", err)
		return
	}

	bindAddr := fmt.Sprintf("0.0.0.0:%d", radSecPort)
	listener, err := tls.Listen("tcp4", bindAddr, tlsConfig)
	if err != nil {
		log.Printf("[radsec] ❌ Failed to bind RadSec TLS listener on %s: %v", bindAddr, err)
		return
	}

	log.Printf("[radsec] 🛡️ Starting RadSec Server (TLS/mTLS RFC 6614) on %s...", bindAddr)

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("[radsec] Accept error: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			go handleRadSecConnection(conn)
		}
	}()
}

func handleRadSecConnection(conn net.Conn) {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		_ = conn.Close()
		return
	}

	// Complete handshake to inspect peer certificate
	if err := tlsConn.Handshake(); err != nil {
		radiusLogger.Printf("[radsec] ❌ Handshake failed from %s: %v", conn.RemoteAddr(), err)
		_ = conn.Close()
		return
	}

	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		radiusLogger.Printf("[radsec] ❌ No client certificate presented from %s", conn.RemoteAddr())
		_ = conn.Close()
		return
	}

	clientCert := state.PeerCertificates[0]
	commonName := clientCert.Subject.CommonName
	remoteAddrStr := conn.RemoteAddr().String()

	// Lookup NAS name associated with this certificate
	nasName := commonName
	var nasSecret string
	if DB != nil {
		_ = DB.QueryRow("SELECT nas_name FROM nas_certificates WHERE common_name = ?", commonName).Scan(&nasName)
		_ = DB.QueryRow("SELECT secret FROM nas WHERE nasname = ? OR shortname = ?", nasName, nasName).Scan(&nasSecret)
	}

	sharedSecret := []byte("radsec") // Standard RFC 6614 secret
	if nasSecret != "" {
		sharedSecret = []byte(nasSecret)
	}

	agent := &RadSecAgent{
		CommonName:  commonName,
		NASName:     nasName,
		RemoteAddr:  remoteAddrStr,
		Conn:        conn,
		ConnectedAt: time.Now(),
		LastPing:    time.Now(),
	}

	RegisterRadSecAgent(agent)
	defer UnregisterRadSecAgent(commonName)

	radiusLogger.Printf("[radsec] 🚀 Established persistent mTLS session: CN=%s, NAS=%s, Remote=%s", commonName, nasName, remoteAddrStr)

	// Read loop for framed RADIUS packets
	for {
		raw, err := ReadFramedPacket(conn, 120*time.Second, conn)
		if err != nil {
			radiusLogger.Printf("[radsec] Connection ended for CN=%s: %v", commonName, err)
			break
		}

		reply, err := agent.HandleInboundPacket(raw, sharedSecret)
		if err != nil {
			radiusLogger.Printf("[radsec] Error handling packet from CN=%s: %v", commonName, err)
			continue
		}

		if reply != nil {
			replyWire, err := reply.Marshal(sharedSecret)
			if err != nil {
				radiusLogger.Printf("[radsec] Error marshaling reply for CN=%s: %v", commonName, err)
				continue
			}

			agent.writeMu.Lock()
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, writeErr := conn.Write(replyWire)
			agent.writeMu.Unlock()

			if writeErr != nil {
				radiusLogger.Printf("[radsec] Error writing reply to CN=%s: %v", commonName, writeErr)
				break
			}
		}
	}
}
