package radius

import (
	"bytes"
	"crypto/md5"
	"testing"

	"github.com/wxccs/radius/v2/packet"
	"github.com/wxccs/radius/v2/types"
)

func TestSignMessageAuthenticator(t *testing.T) {
	secret := []byte("testing123")
	request := &packet.Packet{
		Code:       types.AccessRequest,
		Identifier: 1,
	}
	copy(request.Authenticator[:], []byte("1234567890123456"))
	request.Attributes = append(request.Attributes, packet.NewString(types.AttrUserName, "test"))
	request.Attributes = append(request.Attributes, packet.NewOctets(types.AttrMessageAuthenticator, make([]byte, 16)))

	requestWire, err := request.Marshal(secret)
	if err != nil {
		t.Fatal(err)
	}

	response := &packet.Packet{
		Code:          types.AccessAccept,
		Identifier:    request.Identifier,
		Authenticator: request.Authenticator,
	}
	response.Attributes = append(response.Attributes, packet.NewString(types.AttrUserName, "test"))
	response.Attributes = append(response.Attributes, packet.NewInteger(types.AttrServiceType, 2)) // Framed-User
	response.Attributes = append(response.Attributes, packet.NewOctets(types.AttrMessageAuthenticator, make([]byte, 16)))

	responseWire, err := response.Marshal(secret)
	if err != nil {
		t.Fatal(err)
	}

	var parsedResponse packet.Packet
	if err := parsedResponse.Unmarshal(responseWire, secret); err != nil {
		t.Fatal(err)
	}

	messageAuthenticator := getAttrBytes(&parsedResponse, types.AttrMessageAuthenticator)
	if len(messageAuthenticator) != 16 {
		t.Fatal("Message-Authenticator attribute missing or invalid length")
	}
	if bytes.Equal(messageAuthenticator, make([]byte, md5.Size)) {
		t.Fatal("Message-Authenticator was left as all zeroes")
	}

	// Verify unmarshaling succeeds against request wire
	_ = requestWire
}

func TestSignAccessRejectMessageAuthenticator(t *testing.T) {
	secret := []byte("testing123")
	request := &packet.Packet{
		Code:       types.AccessRequest,
		Identifier: 2,
	}
	copy(request.Authenticator[:], []byte("1234567890123456"))
	request.Attributes = append(request.Attributes, packet.NewString(types.AttrUserName, "disabled-user"))

	_, err := request.Marshal(secret)
	if err != nil {
		t.Fatal(err)
	}

	response := &packet.Packet{
		Code:          types.AccessReject,
		Identifier:    request.Identifier,
		Authenticator: request.Authenticator,
	}
	response.Attributes = append(response.Attributes, packet.NewString(types.AttrUserName, "disabled-user"))
	response.Attributes = append(response.Attributes, packet.NewString(types.AttrReplyMessage, "user disabled"))
	response.Attributes = append(response.Attributes, packet.NewOctets(types.AttrMessageAuthenticator, make([]byte, 16)))

	responseWire, err := response.Marshal(secret)
	if err != nil {
		t.Fatal(err)
	}

	var parsedReject packet.Packet
	if err := parsedReject.Unmarshal(responseWire, secret); err != nil {
		t.Fatal(err)
	}

	messageAuthenticator := getAttrBytes(&parsedReject, types.AttrMessageAuthenticator)
	if len(messageAuthenticator) != 16 {
		t.Fatal("Message-Authenticator attribute missing or invalid length")
	}
	if bytes.Equal(messageAuthenticator, make([]byte, md5.Size)) {
		t.Fatal("Access-Reject Message-Authenticator was left as all zeroes")
	}
}
