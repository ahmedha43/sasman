package multiplexer

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
)

const (
	CmdOpen  byte = 0x01
	CmdData  byte = 0x02
	CmdClose byte = 0x03
	CmdPing  byte = 0x04
	CmdPong  byte = 0x05
)

var (
	ErrSessionClosed = errors.New("multiplex session closed")
	ErrStreamClosed  = errors.New("multiplex stream closed")
)

// StreamMeta is exchanged when opening a new multiplexed sub-stream
type StreamMeta struct {
	StreamID   uint64 `json:"stream_id"`
	ServiceID  string `json:"service_id"`
	TargetHost string `json:"target_host"`
	SNI        string `json:"sni"`
}

// Session coordinates multiple logical streams over a single underlying net.Conn / io.ReadWriteCloser
type Session struct {
	conn       io.ReadWriteCloser
	isServer   bool
	streams    map[uint64]*Stream
	streamMu   sync.RWMutex
	writeMu    sync.Mutex
	nextID     uint64
	closed     int32
	closeCh    chan struct{}
	onOpenChan chan *Stream
}

// NewSession wraps an underlying connection into a multiplexed session
func NewSession(conn io.ReadWriteCloser, isServer bool) *Session {
	var startID uint64 = 1
	if isServer {
		startID = 2
	}
	sess := &Session{
		conn:       conn,
		isServer:   isServer,
		streams:    make(map[uint64]*Stream),
		nextID:     startID,
		closeCh:    make(chan struct{}),
		onOpenChan: make(chan *Stream, 256),
	}
	go sess.readLoop()
	return sess
}

// OpenStream initiates a new sub-stream to the remote peer
func (s *Session) OpenStream(meta StreamMeta) (*Stream, error) {
	if atomic.LoadInt32(&s.closed) == 1 {
		return nil, ErrSessionClosed
	}

	streamID := atomic.AddUint64(&s.nextID, 2)
	meta.StreamID = streamID

	stream := newStream(streamID, s)

	s.streamMu.Lock()
	s.streams[streamID] = stream
	s.streamMu.Unlock()

	metaBytes, _ := json.Marshal(meta)
	if err := s.writeFrame(streamID, CmdOpen, metaBytes); err != nil {
		s.removeStream(streamID)
		return nil, err
	}

	return stream, nil
}

// AcceptStream waits for incoming sub-streams opened by the remote peer
func (s *Session) AcceptStream() (*Stream, error) {
	select {
	case <-s.closeCh:
		return nil, ErrSessionClosed
	case stream, ok := <-s.onOpenChan:
		if !ok {
			return nil, ErrSessionClosed
		}
		return stream, nil
	}
}

func (s *Session) Close() error {
	if atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		close(s.closeCh)
		s.streamMu.Lock()
		for _, st := range s.streams {
			st.closeLocal()
		}
		s.streams = make(map[uint64]*Stream)
		s.streamMu.Unlock()
		return s.conn.Close()
	}
	return nil
}

func (s *Session) removeStream(id uint64) {
	s.streamMu.Lock()
	delete(s.streams, id)
	s.streamMu.Unlock()
}

func (s *Session) writeFrame(streamID uint64, cmd byte, payload []byte) error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrSessionClosed
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Header: StreamID (8B) + Cmd (1B) + Length (4B) = 13 Bytes
	var hdr [13]byte
	binary.BigEndian.PutUint64(hdr[0:8], streamID)
	hdr[8] = cmd
	binary.BigEndian.PutUint32(hdr[9:13], uint32(len(payload)))

	if _, err := s.conn.Write(hdr[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := s.conn.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) readLoop() {
	defer s.Close()

	var hdr [13]byte
	for {
		if _, err := io.ReadFull(s.conn, hdr[:]); err != nil {
			return
		}

		streamID := binary.BigEndian.Uint64(hdr[0:8])
		cmd := hdr[8]
		length := binary.BigEndian.Uint32(hdr[9:13])

		var payload []byte
		if length > 0 {
			if length > 16*1024*1024 { // 16MB max safety frame limit
				log.Printf("[Multiplexer] Oversized frame: %d bytes", length)
				return
			}
			payload = make([]byte, length)
			if _, err := io.ReadFull(s.conn, payload); err != nil {
				return
			}
		}

		switch cmd {
		case CmdOpen:
			var meta StreamMeta
			_ = json.Unmarshal(payload, &meta)
			meta.StreamID = streamID

			stream := newStream(streamID, s)
			stream.meta = meta

			s.streamMu.Lock()
			s.streams[streamID] = stream
			s.streamMu.Unlock()

			select {
			case s.onOpenChan <- stream:
			default:
				log.Printf("[Multiplexer] Accept queue full, dropping stream %d", streamID)
				stream.Close()
			}

		case CmdData:
			s.streamMu.RLock()
			st, ok := s.streams[streamID]
			s.streamMu.RUnlock()
			if ok && st != nil {
				st.pushData(payload)
			}

		case CmdClose:
			s.streamMu.Lock()
			st, ok := s.streams[streamID]
			delete(s.streams, streamID)
			s.streamMu.Unlock()
			if ok && st != nil {
				st.closeRemote()
			}

		case CmdPing:
			_ = s.writeFrame(0, CmdPong, nil)

		case CmdPong:
			// Heartbeat received
		}
	}
}

// Stream represents an isolated bidirectional channel within a multiplexed session
type Stream struct {
	id         uint64
	session    *Session
	meta       StreamMeta
	inBuf      chan []byte
	currentBuf []byte
	closed     int32
	closeCh    chan struct{}
}

func newStream(id uint64, sess *Session) *Stream {
	return &Stream{
		id:      id,
		session: sess,
		inBuf:   make(chan []byte, 128),
		closeCh: make(chan struct{}),
	}
}

func (st *Stream) Meta() StreamMeta {
	return st.meta
}

func (st *Stream) pushData(data []byte) {
	if atomic.LoadInt32(&st.closed) == 1 {
		return
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	select {
	case st.inBuf <- cp:
	case <-st.closeCh:
	}
}

func (st *Stream) Read(p []byte) (n int, err error) {
	if len(st.currentBuf) > 0 {
		n = copy(p, st.currentBuf)
		st.currentBuf = st.currentBuf[n:]
		return n, nil
	}

	// 1. Drain available buffer non-blocking first
	select {
	case data, ok := <-st.inBuf:
		if ok && len(data) > 0 {
			n = copy(p, data)
			if n < len(data) {
				st.currentBuf = data[n:]
			}
			return n, nil
		}
	default:
	}

	// 2. Blocking wait
	select {
	case data, ok := <-st.inBuf:
		if !ok {
			return 0, io.EOF
		}
		n = copy(p, data)
		if n < len(data) {
			st.currentBuf = data[n:]
		}
		return n, nil
	case <-st.closeCh:
		// Check one last time if any data arrived before close
		select {
		case data, ok := <-st.inBuf:
			if ok && len(data) > 0 {
				n = copy(p, data)
				if n < len(data) {
					st.currentBuf = data[n:]
				}
				return n, nil
			}
		default:
		}
		return 0, io.EOF
	}
}

func (st *Stream) Write(p []byte) (n int, err error) {
	if atomic.LoadInt32(&st.closed) == 1 {
		return 0, ErrStreamClosed
	}
	if err := st.session.writeFrame(st.id, CmdData, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (st *Stream) Close() error {
	if atomic.CompareAndSwapInt32(&st.closed, 0, 1) {
		close(st.closeCh)
		st.session.removeStream(st.id)
		return st.session.writeFrame(st.id, CmdClose, nil)
	}
	return nil
}

func (st *Stream) closeRemote() {
	if atomic.CompareAndSwapInt32(&st.closed, 0, 1) {
		close(st.closeCh)
	}
}

func (st *Stream) closeLocal() {
	if atomic.CompareAndSwapInt32(&st.closed, 0, 1) {
		close(st.closeCh)
	}
}
