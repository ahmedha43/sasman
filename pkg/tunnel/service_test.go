package tunnel

import (
	"strings"
	"testing"
)

func TestExtractSubdomain(t *testing.T) {
	if got := ExtractSubdomainForHost("client1.sas-man.net", ""); got != "client1" {
		t.Fatalf("expected client1, got %s", got)
	}

	if got := ExtractSubdomainForHost("localhost", ""); got != "" {
		t.Fatalf("expected empty subdomain for localhost, got %s", got)
	}

	if got := ExtractSubdomainForHost("client1.sas-man.net", "sas-man.net"); got != "client1" {
		t.Fatalf("expected client1 with sas-man.net, got %s", got)
	}

	if got := ExtractSubdomainForHost("client1.localhost:8080", "localhost:8080"); got != "client1" {
		t.Fatalf("expected client1 with localhost:8080, got %s", got)
	}

	if got := ExtractSubdomainForHost("sas-man.net", "sas-man.net"); got != "" {
		t.Fatalf("expected empty subdomain for exact central domain, got %s", got)
	}
}

func TestRegisterAgentGeneratesSubdomainAndToken(t *testing.T) {
	svc := NewService(nil)
	agent := svc.RegisterAgent("", "")
	if agent == nil {
		t.Fatal("expected agent to be generated")
	}

	if strings.TrimSpace(agent.Subdomain) == "" {
		t.Fatalf("expected generated subdomain")
	}
	if strings.TrimSpace(agent.Token) == "" {
		t.Fatalf("expected generated token")
	}
	if !strings.HasPrefix(agent.Subdomain, "sasman-") {
		t.Fatalf("expected generated subdomain prefix, got %s", agent.Subdomain)
	}
	if !strings.HasPrefix(agent.Token, "tok-") {
		t.Fatalf("expected generated token prefix, got %s", agent.Token)
	}
	if agent.WinboxPort < 10001 {
		t.Fatalf("expected allocated winbox port >= 10001, got %d", agent.WinboxPort)
	}

	svc.RemoveAgent(agent.Subdomain)
}

func TestRemoveAndRotateAgent(t *testing.T) {
	svc := NewService(nil)
	agent := svc.RegisterAgent("demo-sub", "old-token")
	if agent == nil {
		t.Fatal("expected agent to be generated")
	}

	if !svc.RemoveAgent("demo-sub") {
		t.Fatal("expected remove to succeed")
	}
	if svc.GetAgentBySubdomain("demo-sub") != nil {
		t.Fatal("expected agent to be removed")
	}

	agent = svc.RegisterAgent("demo-sub", "old-token")
	token, ok := svc.RotateToken("demo-sub")
	if !ok {
		t.Fatal("expected token rotation to succeed")
	}
	if token == "old-token" {
		t.Fatal("expected token to change on rotation")
	}
	if !strings.HasPrefix(token, "tok-") {
		t.Fatalf("expected rotated token prefix, got %s", token)
	}
	if svc.sessions[agent.ID].Token != token {
		t.Fatal("expected service to store rotated token")
	}

	svc.RemoveAgent("demo-sub")
}
