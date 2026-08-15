package sniproxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"mikrotik-manager/pkg/relay"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 32*1024) // 32KB buffer
		return &b
	},
}

// RouteResolver finds the matching service and assigned egress agent for an incoming connection
type RouteResolver interface {
	ResolveRoute(sni string, dstAddr string) (*relay.EgressRoute, *relay.ServiceDefinition, error)
}

// StreamDialer connects or multiplexes to an egress agent
type StreamDialer interface {
	DialEgressStream(ctx context.Context, route *relay.EgressRoute, svc *relay.ServiceDefinition, targetHost string, rawPrefix []byte) (io.ReadWriteCloser, error)
}

// InterceptorListener listens for redirected traffic from MikroTik (port 18443 / 18080)
type InterceptorListener struct {
	activeConns  atomic.Int64
	bytesRelayed atomic.Int64
	running      atomic.Int32
	addr         string
	listener     net.Listener
	resolver     RouteResolver
	dialer       StreamDialer
	mu           sync.Mutex
	stopCh       chan struct{}
}

func NewInterceptorListener(addr string, resolver RouteResolver, dialer StreamDialer) *InterceptorListener {
	return &InterceptorListener{
		addr:     addr,
		resolver: resolver,
		dialer:   dialer,
		stopCh:   make(chan struct{}),
	}
}

func (l *InterceptorListener) Start() error {
	ln, err := net.Listen("tcp", l.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", l.addr, err)
	}

	l.listener = ln
	l.running.Store(1)

	log.Printf("[Relay Interceptor] Listening on %s", l.addr)

	go l.acceptLoop()
	return nil
}

func (l *InterceptorListener) Stop() {
	if l.running.CompareAndSwap(1, 0) {
		close(l.stopCh)
		if l.listener != nil {
			_ = l.listener.Close()
		}
	}
}

func (l *InterceptorListener) ActiveConnections() int {
	return int(l.activeConns.Load())
}

func (l *InterceptorListener) TotalBytesRelayed() int64 {
	return l.bytesRelayed.Load()
}

func (l *InterceptorListener) acceptLoop() {
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			select {
			case <-l.stopCh:
				return
			default:
				log.Printf("[Relay Interceptor] Accept error: %v", err)
				time.Sleep(50 * time.Millisecond)
				continue
			}
		}

		l.activeConns.Add(1)
		go l.handleClientConn(conn)
	}
}

func (l *InterceptorListener) handleClientConn(clientConn net.Conn) {
	defer func() {
		clientConn.Close()
		l.activeConns.Add(-1)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	sni, peeked, err := ExtractSNI(clientConn)
	_ = clientConn.SetReadDeadline(time.Time{})

	targetHost := ""
	if err == nil && sni != "" {
		targetHost = net.JoinHostPort(sni, "443")
	} else {
		// Non-TLS or missing SNI: use original destination address
		targetHost = clientConn.LocalAddr().String()
	}

	route, svc, err := l.resolver.ResolveRoute(sni, targetHost)
	if err != nil || route == nil || svc == nil {
		// No relay route configured: close connection
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	egressStream, err := l.dialer.DialEgressStream(ctx, route, svc, targetHost, peeked)
	if err != nil {
		log.Printf("[Relay Interceptor] Dial egress error for %s (target %s via %s): %v", sni, targetHost, route.PrimaryAgent, err)
		return
	}
	defer egressStream.Close()

	// Bidirectional stream piping
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		bufPtr := bufPool.Get().(*[]byte)
		defer bufPool.Put(bufPtr)
		n, _ := io.CopyBuffer(egressStream, clientConn, *bufPtr)
		l.bytesRelayed.Add(n)
	}()

	go func() {
		defer wg.Done()
		bufPtr := bufPool.Get().(*[]byte)
		defer bufPool.Put(bufPtr)
		n, _ := io.CopyBuffer(clientConn, egressStream, *bufPtr)
		l.bytesRelayed.Add(n)
	}()

	wg.Wait()
}
