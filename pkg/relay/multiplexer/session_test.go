package multiplexer

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

func TestMultiplexerStreaming(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	serverSess := NewSession(serverConn, true)
	clientSess := NewSession(clientConn, false)
	defer serverSess.Close()
	defer clientSess.Close()

	// Channel to signal server receipt
	done := make(chan struct{})

	go func() {
		stream, err := serverSess.AcceptStream()
		if err != nil {
			t.Errorf("Server failed to accept stream: %v", err)
			return
		}
		defer stream.Close()

		if stream.Meta().TargetHost != "cinemana.shabakaty.cc:443" {
			t.Errorf("Target host mismatch: %s", stream.Meta().TargetHost)
		}

		// Echo server
		buf := make([]byte, 1024)
		n, err := stream.Read(buf)
		if err != nil && err != io.EOF {
			t.Errorf("Server read error: %v", err)
			return
		}

		_, _ = stream.Write(buf[:n])
		close(done)
	}()

	clientStream, err := clientSess.OpenStream(StreamMeta{
		ServiceID:  "cinemana",
		TargetHost: "cinemana.shabakaty.cc:443",
		SNI:        "cinemana.shabakaty.cc",
	})
	if err != nil {
		t.Fatalf("Client failed to open stream: %v", err)
	}
	defer clientStream.Close()

	payload := []byte("Hello SASMAN Relay Multiplexer!")
	if _, err := clientStream.Write(payload); err != nil {
		t.Fatalf("Client write error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for server echo")
	}

	echoBuf := make([]byte, 1024)
	n, err := clientStream.Read(echoBuf)
	if err != nil && err != io.EOF {
		t.Fatalf("Client read error: %v", err)
	}

	if !bytes.Equal(echoBuf[:n], payload) {
		t.Fatalf("Echo mismatch! got %q, want %q", string(echoBuf[:n]), string(payload))
	}
}
