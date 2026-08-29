package radius

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/wxccs/radius/v2/packet"
	"github.com/wxccs/radius/v2/types"

	"mikrotik-manager/pkg/pki"
)

func TestRadSecAndReverseDisconnectEndToEnd(t *testing.T) {
	// 1. Setup Test DB
	_ = os.MkdirAll("data_test", 0755)
	testDBPath := "data_test/radsec_test.db"
	_ = os.Remove(testDBPath)
	defer os.Remove(testDBPath)

	db, err := sql.Open("sqlite", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test SQLite: %v", err)
	}
	defer db.Close()
	DB = db

	// Ensure Schema
	EnsureSchema()

	// 2. Initialize PKI
	testPKIDir := "data_test/pki_radsec_test"
	_ = os.RemoveAll(testPKIDir)
	defer os.RemoveAll(testPKIDir)

	// Override PKI dir for test
	pkiDirForTest := testPKIDir
	_ = os.MkdirAll(pkiDirForTest, 0700)
	if err := pki.InitPKI(); err != nil {
		t.Fatalf("InitPKI failed: %v", err)
	}

	// 3. Insert test NAS & Generate Client Cert
	_, err = DB.Exec("INSERT INTO nas (nasname, shortname, secret) VALUES (?, ?, ?)", "192.168.88.1", "Branch-1", "123456")
	if err != nil {
		t.Fatalf("Insert NAS failed: %v", err)
	}

	commonName := "agent-1-Branch-1"
	bundle, err := pki.GenerateClientCertificate(commonName, 365)
	if err != nil {
		t.Fatalf("GenerateClientCertificate failed: %v", err)
	}

	_, err = DB.Exec(`
		INSERT INTO nas_certificates (nas_id, nas_name, common_name, serial_number, cert_pem, key_pem, ca_pem, expires_at, revoked)
		VALUES (1, '192.168.88.1', ?, ?, ?, ?, ?, datetime('now', '+1 year'), 0)
	`, commonName, bundle.SerialNumber, bundle.CertPEM, bundle.KeyPEM, bundle.CAPEM)
	if err != nil {
		t.Fatalf("Insert nas_certificates failed: %v", err)
	}

	// Insert test user in radcheck
	_, _ = DB.Exec("INSERT INTO radcheck (username, attribute, op, value) VALUES (?, 'Cleartext-Password', ':=', ?)", "testuser", "secretpass")

	// 4. Start RadSec Listener on ephemeral port
	tlsConfig, err := pki.GetServerTLSConfig(nil)
	if err != nil {
		t.Fatalf("GetServerTLSConfig failed: %v", err)
	}

	listener, err := tls.Listen("tcp4", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("tls.Listen failed: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleRadSecConnection(conn)
		}
	}()

	// 5. Connect Client via mTLS
	clientTLS, err := pki.GetClientTLSConfig(bundle.CertPEM, bundle.KeyPEM, bundle.CAPEM)
	if err != nil {
		t.Fatalf("GetClientTLSConfig failed: %v", err)
	}

	clientConn, err := tls.Dial("tcp", serverAddr, clientTLS)
	if err != nil {
		t.Fatalf("Client tls.Dial failed: %v", err)
	}
	defer clientConn.Close()

	time.Sleep(100 * time.Millisecond)

	// 6. Verify agent registered in registry
	agent := GetRadSecAgentByCN(commonName)
	if agent == nil {
		t.Fatalf("Expected agent %s to be registered in AgentRegistry", commonName)
	}
	if agent.NASName != "192.168.88.1" {
		t.Fatalf("Expected NASName 192.168.88.1, got %s", agent.NASName)
	}

	agents := ListRadSecAgents()
	if len(agents) == 0 {
		t.Fatal("Expected ListRadSecAgents to return active agents")
	}

	// 7. Client sends Access-Request framed over TLS
	secret := []byte("123456")

	reqPkt := &packet.Packet{
		Code:          types.AccessRequest,
		Identifier:    10,
		Authenticator: [16]byte{1, 2, 3, 4},
	}
	reqPkt.Attributes = append(reqPkt.Attributes, packet.NewString(types.AttrUserName, "testuser"))
	reqPkt.Attributes = append(reqPkt.Attributes, packet.NewString(types.AttrUserPassword, "secretpass"))
	reqPkt.Attributes = append(reqPkt.Attributes, packet.NewIPAddr(types.AttrNASIPAddress, net.ParseIP("192.168.88.1")))

	wireReq, err := reqPkt.Marshal(secret)
	if err != nil {
		t.Fatalf("Marshal req failed: %v", err)
	}

	if _, err := clientConn.Write(wireReq); err != nil {
		t.Fatalf("Client write req failed: %v", err)
	}

	respRaw, err := ReadFramedPacket(clientConn, 2*time.Second, clientConn)
	if err != nil {
		t.Fatalf("Client read resp failed: %v", err)
	}

	var respPkt packet.Packet
	if err := respPkt.Unmarshal(respRaw, secret); err != nil {
		t.Fatalf("Unmarshal reply failed: %v", err)
	}

	if respPkt.Code != types.AccessAccept {
		t.Fatalf("Expected AccessAccept (2), got %v", respPkt.Code)
	}

	// 8. Test Reverse Disconnect from Server to Client
	reverseDone := make(chan error, 1)

	// Simulate Client receiving Disconnect-Request and replying Disconnect-ACK
	go func() {
		disconnRaw, err := ReadFramedPacket(clientConn, 3*time.Second, clientConn)
		if err != nil {
			reverseDone <- fmt.Errorf("client failed to read disconnect request: %w", err)
			return
		}

		var disconnPkt packet.Packet
		if err := disconnPkt.Unmarshal(disconnRaw, secret); err != nil {
			reverseDone <- fmt.Errorf("client failed to unmarshal disconnect request: %w", err)
			return
		}

		if disconnPkt.Code != types.DisconnectRequest {
			reverseDone <- fmt.Errorf("expected DisconnectRequest, got %v", disconnPkt.Code)
			return
		}

		// Build Disconnect-ACK reply
		ackPkt := &packet.Packet{
			Code:          types.DisconnectACK,
			Identifier:    disconnPkt.Identifier,
			Authenticator: disconnPkt.Authenticator,
		}
		ackWire, err := ackPkt.Marshal(secret)
		if err != nil {
			reverseDone <- fmt.Errorf("marshal ACK failed: %w", err)
			return
		}

		if _, err := clientConn.Write(ackWire); err != nil {
			reverseDone <- fmt.Errorf("write ACK failed: %w", err)
			return
		}

		reverseDone <- nil
	}()

	// Trigger SendReverseDisconnect from server
	sessInfo := SessionInfo{
		NASIP:     "192.168.88.1",
		IP:        "192.168.88.254",
		SessionID: "sess-8899",
	}

	err = SendReverseDisconnect(agent, "testuser", sessInfo, "123456")
	if err != nil {
		t.Fatalf("SendReverseDisconnect failed: %v", err)
	}

	if err := <-reverseDone; err != nil {
		t.Fatalf("Client reverse disconnect handling error: %v", err)
	}
}
