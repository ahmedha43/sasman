//go:build linux

package sniproxy

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

const (
	SO_ORIGINAL_DST      = 80
	IP6T_SO_ORIGINAL_DST = 80
)

// GetOriginalDst extracts the original destination IP:port from a TCP connection
// intercepted via iptables REDIRECT or RouterOS v7 DST-NAT redirect.
func GetOriginalDst(conn net.Conn) (string, error) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return "", fmt.Errorf("not a tcp connection")
	}

	file, err := tcpConn.File()
	if err != nil {
		return "", err
	}
	defer file.Close()

	fd := int(file.Fd())

	// Try IPv4 SO_ORIGINAL_DST
	var raw syscall.RawSockaddrInet4
	rawLen := uint32(syscall.SizeofSockaddrInet4)

	_, _, errno := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		uintptr(fd),
		uintptr(syscall.SOL_IP),
		uintptr(SO_ORIGINAL_DST),
		uintptr(unsafe.Pointer(&raw)),
		uintptr(unsafe.Pointer(&rawLen)),
		0,
	)
	if errno == 0 {
		port := binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&raw.Port))[:])
		ip := net.IPv4(raw.Addr[0], raw.Addr[1], raw.Addr[2], raw.Addr[3])
		return net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port)), nil
	}

	// Try IPv6 SO_ORIGINAL_DST
	var raw6 syscall.RawSockaddrInet6
	raw6Len := uint32(syscall.SizeofSockaddrInet6)

	_, _, errno6 := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		uintptr(fd),
		uintptr(syscall.SOL_IPV6),
		uintptr(IP6T_SO_ORIGINAL_DST),
		uintptr(unsafe.Pointer(&raw6)),
		uintptr(unsafe.Pointer(&raw6Len)),
		0,
	)
	if errno6 == 0 {
		port := binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&raw6.Port))[:])
		ip := net.IP(raw6.Addr[:])
		return net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port)), nil
	}

	return "", errno
}
