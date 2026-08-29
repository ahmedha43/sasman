package pki

import (
	"crypto/tls"
	"net"
	"os"
	"testing"
	"time"
)

func TestPKIEndToEnd(t *testing.T) {
	// Set temporary test PKI directory
	testDir := "data_test/pki_test"
	_ = os.RemoveAll(testDir)
	defer os.RemoveAll(testDir)

	pkiDir = testDir

	// 1. Initialize PKI
	if err := InitPKI(); err != nil {
		t.Fatalf("InitPKI failed: %v", err)
	}

	// 2. Generate Client Certificate
	clientBundle, err := GenerateClientCertificate("agent-test-1", 365)
	if err != nil {
		t.Fatalf("GenerateClientCertificate failed: %v", err)
	}

	if clientBundle.CommonName != "agent-test-1" {
		t.Fatalf("expected CommonName agent-test-1, got %s", clientBundle.CommonName)
	}
	if clientBundle.CertPEM == "" || clientBundle.KeyPEM == "" || clientBundle.CAPEM == "" {
		t.Fatal("empty PEM files in client bundle")
	}

	// 3. Test mTLS Server & Client Handshake
	serverTLS, err := GetServerTLSConfig(nil)
	if err != nil {
		t.Fatalf("GetServerTLSConfig failed: %v", err)
	}

	clientTLS, err := GetClientTLSConfig(clientBundle.CertPEM, clientBundle.KeyPEM, clientBundle.CAPEM)
	if err != nil {
		t.Fatalf("GetClientTLSConfig failed: %v", err)
	}

	// Start TLS listener on localhost
	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Fatalf("tls.Listen failed: %v", err)
	}
	defer ln.Close()

	serverDone := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		tlsConn := conn.(*tls.Conn)
		if err := tlsConn.Handshake(); err != nil {
			serverDone <- err
			return
		}

		state := tlsConn.ConnectionState()
		if len(state.PeerCertificates) == 0 {
			serverDone <- err
			return
		}
		if state.PeerCertificates[0].Subject.CommonName != "agent-test-1" {
			serverDone <- err
			return
		}

		// Echo test
		buf := make([]byte, 4)
		_, _ = conn.Read(buf)
		_, _ = conn.Write(buf)
		serverDone <- nil
	}()

	// Connect client
	clientConn, err := tls.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}, "tcp", ln.Addr().String(), clientTLS)
	if err != nil {
		t.Fatalf("tls.Dial failed: %v", err)
	}
	defer clientConn.Close()

	if _, err := clientConn.Write([]byte("PING")); err != nil {
		t.Fatalf("client write failed: %v", err)
	}

	buf := make([]byte, 4)
	if _, err := clientConn.Read(buf); err != nil {
		t.Fatalf("client read failed: %v", err)
	}
	if string(buf) != "PING" {
		t.Fatalf("expected PING, got %s", string(buf))
	}

	if err := <-serverDone; err != nil {
		t.Fatalf("server mTLS handshake verification failed: %v", err)
	}
}
