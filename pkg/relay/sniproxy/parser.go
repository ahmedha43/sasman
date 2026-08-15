package sniproxy

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

var (
	ErrNotTLS           = errors.New("not a valid TLS handshake record")
	ErrNoSNI            = errors.New("SNI extension not found in ClientHello")
	ErrIncompleteRecord = errors.New("incomplete TLS record")
)

// ExtractSNI reads the TLS ClientHello from the reader without consuming or modifying it.
// It returns the extracted Server Name Indication (SNI) string and a peeked buffer that must be
// prepended to subsequent stream forwarding.
func ExtractSNI(reader io.Reader) (string, []byte, error) {
	// Read TLS Record Header (5 bytes: ContentType[1], Version[2], Length[2])
	header := make([]byte, 5)
	if _, err := io.ReadFull(reader, header); err != nil {
		return "", header, fmt.Errorf("read TLS record header: %w", err)
	}

	// TLS Handshake ContentType is 22 (0x16)
	if header[0] != 0x16 {
		return "", header, ErrNotTLS
	}

	recordLen := binary.BigEndian.Uint16(header[3:5])
	if recordLen == 0 || recordLen > 16384 {
		return "", header, fmt.Errorf("invalid TLS record length: %d", recordLen)
	}

	body := make([]byte, recordLen)
	if _, err := io.ReadFull(reader, body); err != nil {
		full := append(header, body...)
		return "", full, fmt.Errorf("read TLS handshake body: %w", err)
	}

	fullPayload := append(header, body...)

	sni, err := ParseSNIFromClientHello(body)
	if err != nil {
		return "", fullPayload, err
	}

	return sni, fullPayload, nil
}

// ParseSNIFromClientHello parses the TLS Handshake payload to locate the SNI hostname.
func ParseSNIFromClientHello(data []byte) (string, error) {
	// HandshakeType must be 1 (ClientHello)
	if len(data) < 4 || data[0] != 0x01 {
		return "", ErrNotTLS
	}

	// Skip Handshake Header (Type[1] + Length[3])
	pos := 4
	if len(data) < pos+2+32 { // Version[2] + Random[32]
		return "", ErrIncompleteRecord
	}
	pos += 2 + 32

	// Skip Session ID (Len[1] + ID[len])
	if len(data) < pos+1 {
		return "", ErrIncompleteRecord
	}
	sessionIDLen := int(data[pos])
	pos += 1 + sessionIDLen
	if len(data) < pos {
		return "", ErrIncompleteRecord
	}

	// Skip Cipher Suites (Len[2] + Suites[len])
	if len(data) < pos+2 {
		return "", ErrIncompleteRecord
	}
	cipherLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2 + cipherLen
	if len(data) < pos {
		return "", ErrIncompleteRecord
	}

	// Skip Compression Methods (Len[1] + Methods[len])
	if len(data) < pos+1 {
		return "", ErrIncompleteRecord
	}
	compressionLen := int(data[pos])
	pos += 1 + compressionLen
	if len(data) < pos {
		return "", ErrIncompleteRecord
	}

	// Extensions (Len[2] + Extensions[len])
	if len(data) < pos+2 {
		return "", ErrNoSNI
	}
	extensionsLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	if len(data) < pos+extensionsLen {
		return "", ErrIncompleteRecord
	}

	extEnd := pos + extensionsLen
	for pos+4 <= extEnd {
		extType := binary.BigEndian.Uint16(data[pos : pos+2])
		extLen := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		pos += 4

		if pos+extLen > extEnd {
			return "", ErrIncompleteRecord
		}

		// SNI Extension Type is 0 (0x0000)
		if extType == 0x0000 {
			sniData := data[pos : pos+extLen]
			return parseServerNameList(sniData)
		}

		pos += extLen
	}

	return "", ErrNoSNI
}

func parseServerNameList(data []byte) (string, error) {
	if len(data) < 2 {
		return "", ErrIncompleteRecord
	}
	listLen := int(binary.BigEndian.Uint16(data[0:2]))
	pos := 2
	if len(data) < pos+listLen {
		return "", ErrIncompleteRecord
	}

	for pos+3 <= 2+listLen {
		nameType := data[pos]
		nameLen := int(binary.BigEndian.Uint16(data[pos+1 : pos+3]))
		pos += 3

		if pos+nameLen > len(data) {
			return "", ErrIncompleteRecord
		}

		// NameType 0 is host_name
		if nameType == 0 {
			host := string(data[pos : pos+nameLen])
			return strings.ToLower(strings.TrimSpace(host)), nil
		}

		pos += nameLen
	}

	return "", ErrNoSNI
}

// MatchDomain checks if a given hostname matches a domain pattern (supports wildcards e.g. *.shabakaty.cc)
func MatchDomain(hostname, pattern string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	pattern = strings.ToLower(strings.TrimSpace(pattern))

	if hostname == pattern {
		return true
	}

	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // e.g. ".shabakaty.cc"
		if strings.HasSuffix(hostname, suffix) {
			return true
		}
	}

	return false
}

// PeekConn wraps a net.Conn with a buffered prefix that was read during SNI extraction
type PeekConn struct {
	net.Conn
	peekReader io.Reader
}

func NewPeekConn(conn net.Conn, peeked []byte) net.Conn {
	return &PeekConn{
		Conn:       conn,
		peekReader: io.MultiReader(bytes.NewReader(peeked), conn),
	}
}

func (p *PeekConn) Read(b []byte) (n int, err error) {
	return p.peekReader.Read(b)
}
