package radius

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"testing"

	layehRadius "layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2869"
)

func TestSignMessageAuthenticator(t *testing.T) {
	secret := []byte("testing123")
	request := layehRadius.New(layehRadius.CodeAccessRequest, secret)
	rfc2865.UserName_AddString(request, "test")

	requestWire, err := request.Encode()
	if err != nil {
		t.Fatal(err)
	}

	response := request.Response(layehRadius.CodeAccessAccept)
	rfc2865.UserName_AddString(response, "test")
	rfc2865.ServiceType_Add(response, rfc2865.ServiceType_Value_FramedUser)

	if err := signMessageAuthenticator(response); err != nil {
		t.Fatal(err)
	}

	messageAuthenticator := rfc2869.MessageAuthenticator_Get(response)
	if bytes.Equal(messageAuthenticator, make([]byte, md5.Size)) {
		t.Fatal("Message-Authenticator was left as all zeroes")
	}

	wireWithZeroedMessageAuthenticator, err := response.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expectedOffset := bytes.Index(wireWithZeroedMessageAuthenticator, append([]byte{byte(rfc2869.MessageAuthenticator_Type), 18}, messageAuthenticator...))
	if expectedOffset == -1 {
		t.Fatal("signed Message-Authenticator attribute not found in response")
	}
	copy(wireWithZeroedMessageAuthenticator[expectedOffset+2:expectedOffset+18], make([]byte, md5.Size))

	mac := hmac.New(md5.New, secret)
	mac.Write(wireWithZeroedMessageAuthenticator)
	if !hmac.Equal(messageAuthenticator, mac.Sum(nil)) {
		t.Fatal("Message-Authenticator HMAC mismatch")
	}

	responseWire, err := response.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !layehRadius.IsAuthenticResponse(responseWire, requestWire, secret) {
		t.Fatal("response authenticator is invalid after signing Message-Authenticator")
	}
}

func TestSignAccessRejectMessageAuthenticator(t *testing.T) {
	secret := []byte("testing123")
	request := layehRadius.New(layehRadius.CodeAccessRequest, secret)
	rfc2865.UserName_AddString(request, "disabled-user")

	requestWire, err := request.Encode()
	if err != nil {
		t.Fatal(err)
	}

	response := request.Response(layehRadius.CodeAccessReject)
	rfc2865.UserName_AddString(response, "disabled-user")
	rfc2865.ReplyMessage_AddString(response, "user disabled")

	if err := signMessageAuthenticator(response); err != nil {
		t.Fatal(err)
	}

	messageAuthenticator := rfc2869.MessageAuthenticator_Get(response)
	if bytes.Equal(messageAuthenticator, make([]byte, md5.Size)) {
		t.Fatal("Access-Reject Message-Authenticator was left as all zeroes")
	}

	responseWire, err := response.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !layehRadius.IsAuthenticResponse(responseWire, requestWire, secret) {
		t.Fatal("Access-Reject response authenticator is invalid")
	}
}
