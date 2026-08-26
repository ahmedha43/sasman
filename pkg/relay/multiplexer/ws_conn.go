package multiplexer

import (
	"io"
	"sync"

	"github.com/gorilla/websocket"
)

// WSReadWriteCloser wraps a gorilla/websocket Conn into an io.ReadWriteCloser
type WSReadWriteCloser struct {
	conn    *websocket.Conn
	reader  io.Reader
	writeMu sync.Mutex
}

// NewWSReadWriteCloser creates a new wrapper for gorilla/websocket
func NewWSReadWriteCloser(conn *websocket.Conn) *WSReadWriteCloser {
	return &WSReadWriteCloser{conn: conn}
}

func (w *WSReadWriteCloser) Read(p []byte) (int, error) {
	for {
		if w.reader == nil {
			msgType, r, err := w.conn.NextReader()
			if err != nil {
				return 0, err
			}
			if msgType != websocket.BinaryMessage {
				continue
			}
			w.reader = r
		}
		n, err := w.reader.Read(p)
		if err == io.EOF {
			w.reader = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (w *WSReadWriteCloser) Write(p []byte) (int, error) {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	err := w.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *WSReadWriteCloser) Close() error {
	return w.conn.Close()
}
