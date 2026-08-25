//go:build !linux

package sniproxy

import (
	"fmt"
	"net"
)

// GetOriginalDst fallback on non-Linux platforms
func GetOriginalDst(conn net.Conn) (string, error) {
	return "", fmt.Errorf("SO_ORIGINAL_DST only supported on Linux")
}
