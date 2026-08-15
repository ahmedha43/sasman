package sniproxy

import (
	"bytes"
	"crypto/tls"
	"net"
	"testing"
)

func TestParseSNIFromClientHello(t *testing.T) {
	domain := "cinemana.shabakaty.cc"

	// Create a pipe and trigger a TLS ClientHello
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		tlsConfig := &tls.Config{
			ServerName: domain,
		}
		tlsClient := tls.Client(client, tlsConfig)
		_ = tlsClient.Handshake()
	}()

	sni, peeked, err := ExtractSNI(server)
	if err != nil {
		t.Fatalf("Failed to extract SNI: %v", err)
	}

	if sni != domain {
		t.Errorf("Expected SNI %q, got %q", domain, sni)
	}

	if len(peeked) == 0 {
		t.Errorf("Expected peeked bytes, got empty")
	}

	// Verify peek connection preserves the entire payload
	peekConn := NewPeekConn(server, peeked)
	buf := make([]byte, len(peeked))
	n, err := peekConn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read from PeekConn: %v", err)
	}
	if n != len(peeked) || !bytes.Equal(buf, peeked) {
		t.Errorf("PeekConn read mismatch")
	}
}

func TestMatchDomain(t *testing.T) {
	tests := []struct {
		host    string
		pattern string
		match   bool
	}{
		{"cinemana.shabakaty.cc", "cinemana.shabakaty.cc", true},
		{"tv.shabakaty.cc", "*.shabakaty.cc", true},
		{"cdn.sub.shabakaty.cc", "*.shabakaty.cc", true},
		{"shabakaty.cc", "*.shabakaty.cc", false},
		{"google.com", "*.shabakaty.cc", false},
		{"cinemana.shabakaty.com", "*.shabakaty.cc", false},
	}

	for _, tt := range tests {
		got := MatchDomain(tt.host, tt.pattern)
		if got != tt.match {
			t.Errorf("MatchDomain(%q, %q) = %v; want %v", tt.host, tt.pattern, got, tt.match)
		}
	}
}
