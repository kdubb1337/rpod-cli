package api

import "testing"

func TestDeriveSSHInfo_RuntimePorts(t *testing.T) {
	p := &Pod{
		ID: "abc123",
		Runtime: &PodRuntime{
			Ports: []PodPort{
				{IP: "1.2.3.4", IsIPPublic: true, PrivatePort: 22, PublicPort: 12885, Type: "tcp"},
				{PrivatePort: 8000, Type: "http"},
			},
		},
	}
	info, ok := p.DeriveSSHInfo()
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	if info.Direct == nil || info.Direct.Host != "1.2.3.4" || info.Direct.Port != 12885 {
		t.Fatalf("direct endpoint not derived from runtime.ports: %+v", info.Direct)
	}
	if got := info.HTTPProxy[8000]; got != "https://abc123-8000.proxy.runpod.net" {
		t.Fatalf("http proxy missing/wrong: %q", got)
	}
}

// Regression for UI-created / resumed pods where runtime.ports is empty but
// REST returns portMappings + publicIp. Before this fix `pod ssh-info` and
// `pod exec` silently fell back to the slow ssh.runpod.io proxy.
func TestDeriveSSHInfo_PortMappingsFallback(t *testing.T) {
	p := &Pod{
		ID:           "abc123",
		PublicIP:     "213.181.104.219",
		Ports:        []string{"22/tcp", "8000/http"},
		PortMappings: map[string]int{"22": 12885, "8000": 12886},
	}
	info, ok := p.DeriveSSHInfo()
	if !ok {
		t.Fatalf("expected ok=true from portMappings fallback, got false")
	}
	if info.Direct == nil || info.Direct.Host != "213.181.104.219" || info.Direct.Port != 12885 {
		t.Fatalf("direct endpoint not synthesised from portMappings: %+v", info.Direct)
	}
	if info.Direct.Command != "ssh -p 12885 root@213.181.104.219" {
		t.Fatalf("unexpected direct command: %q", info.Direct.Command)
	}
	if got := info.HTTPProxy[8000]; got != "https://abc123-8000.proxy.runpod.net" {
		t.Fatalf("http proxy missing for spec-declared http port: %q", got)
	}
	if _, isTCP := info.PublicTCP[8000]; isTCP {
		t.Fatalf("port 8000 should be classified http, not tcp, from spec: %+v", info.PublicTCP)
	}
}

func TestDeriveSSHInfo_NoRuntimeNoMappings(t *testing.T) {
	p := &Pod{ID: "abc123", Ports: []string{"8000/http"}}
	info, ok := p.DeriveSSHInfo()
	if ok {
		t.Fatalf("expected ok=false with no direct endpoint, got true")
	}
	if info.Direct != nil {
		t.Fatalf("direct endpoint should be nil: %+v", info.Direct)
	}
	if info.Proxy == nil || info.Proxy.Host != SSHProxyHost {
		t.Fatalf("proxy endpoint missing: %+v", info.Proxy)
	}
	// HTTP proxy URLs answer for any spec-declared http port even without
	// runtime data — keep this guarantee.
	if got := info.HTTPProxy[8000]; got != "https://abc123-8000.proxy.runpod.net" {
		t.Fatalf("http proxy URL should be derivable from spec alone: %q", got)
	}
}

func TestHasPort_PortMappings(t *testing.T) {
	p := &Pod{PortMappings: map[string]int{"22": 12885}}
	if !p.HasPort(22) {
		t.Fatalf("HasPort(22) should be true via portMappings")
	}
	if p.HasPort(8000) {
		t.Fatalf("HasPort(8000) should be false")
	}
}
