package radius

import (
	"net"
	"testing"

	"github.com/wxccs/radius/v2/packet"
	"github.com/wxccs/radius/v2/types"
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

	var p packet.Packet
	if err := p.Unmarshal(wire, []byte(secret)); err != nil {
		t.Fatal(err)
	}
	if p.Code != types.DisconnectRequest {
		t.Fatalf("expected Disconnect-Request, got %v", p.Code)
	}
	if getAttrString(&p, types.AttrUserName) != "test" {
		t.Fatal("missing User-Name")
	}
	if !getAttrIP(&p, types.AttrNASIPAddress).Equal(net.ParseIP("192.168.88.1")) {
		t.Fatal("missing NAS-IP-Address")
	}
	if !getAttrIP(&p, types.AttrFramedIPAddress).Equal(net.ParseIP("192.168.88.254")) {
		t.Fatal("missing Framed-IP-Address")
	}
	if getAttrString(&p, types.AttrAcctSessionID) != "84000027" {
		t.Fatal("missing Acct-Session-Id")
	}
	if getAttrString(&p, types.AttrCallingStationID) != "AA:BB:CC:DD:EE:FF" {
		t.Fatal("missing Calling-Station-Id")
	}
}
