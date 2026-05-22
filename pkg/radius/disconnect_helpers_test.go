package radius

import (
	"net"
	"testing"

	layehRadius "layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
)

func TestBuildDisconnectPacketUsesRadiusLibraryEncoding(t *testing.T) {
	secret := "testing123"
	wire, err := buildDisconnectPacket("test", SessionInfo{
		NASIP:          "192.168.88.1",
		IP:             "192.168.88.254",
		SessionID:      "84000027",
		CallingStation: "AA:BB:CC:DD:EE:FF",
	}, secret)
	if err != nil {
		t.Fatal(err)
	}

	packet, err := layehRadius.Parse(wire, []byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if packet.Code != layehRadius.CodeDisconnectRequest {
		t.Fatalf("expected Disconnect-Request, got %v", packet.Code)
	}
	if rfc2865.UserName_GetString(packet) != "test" {
		t.Fatal("missing User-Name")
	}
	if !rfc2865.NASIPAddress_Get(packet).Equal(net.ParseIP("192.168.88.1")) {
		t.Fatal("missing NAS-IP-Address")
	}
	if !rfc2865.FramedIPAddress_Get(packet).Equal(net.ParseIP("192.168.88.254")) {
		t.Fatal("missing Framed-IP-Address")
	}
	if rfc2866.AcctSessionID_GetString(packet) != "84000027" {
		t.Fatal("missing Acct-Session-Id")
	}
	if rfc2865.CallingStationID_GetString(packet) != "AA:BB:CC:DD:EE:FF" {
		t.Fatal("missing Calling-Station-Id")
	}
	if !layehRadius.IsAuthenticRequest(wire, []byte(secret)) {
		t.Fatal("disconnect request authenticator is invalid")
	}
}
