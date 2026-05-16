package api

import (
	"fmt"
	"strconv"
	"strings"
)

// SSHInfo summarises every way to reach a pod over SSH plus the HTTP/TCP
// surface RunPod exposes. Derived purely from Pod fields — no extra API call.
type SSHInfo struct {
	// PodID is included so agents that only have the SSHInfo blob can still
	// chain into other rpod commands.
	PodID string `json:"pod_id"`
	// Direct is populated when the pod has a public IP and port 22 is exposed
	// as TCP. Always prefer this when present — proxy SSH is slower.
	Direct *SSHEndpoint `json:"direct,omitempty"`
	// Proxy is RunPod's universal SSH proxy. Always available for any pod that
	// has PUBLIC_KEY set, regardless of public-ip / port mapping.
	Proxy *SSHEndpoint `json:"proxy"`
	// HTTPProxy maps {privatePort: "https://<id>-<port>.proxy.runpod.net"} for
	// every HTTP port the pod exposes.
	HTTPProxy map[int]string `json:"http_proxy,omitempty"`
	// PublicTCP maps {privatePort: "ip:publicPort"} for every TCP port with a
	// public IP — useful for reaching a vLLM/Triton/etc. service.
	PublicTCP map[int]string `json:"public_tcp,omitempty"`
}

// SSHEndpoint is one reachable SSH host:port pair.
type SSHEndpoint struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`
	// Command is a ready-to-run `ssh` invocation. Agents can copy it verbatim.
	Command string `json:"command"`
}

// SSHProxyHost is RunPod's universal SSH proxy hostname.
const SSHProxyHost = "ssh.runpod.io"

// HTTPProxyTemplate is the format RunPod uses to reverse-proxy any HTTP port.
// See https://docs.runpod.io/pods/configuration/expose-ports.
const HTTPProxyTemplate = "https://%s-%d.proxy.runpod.net"

// DeriveSSHInfo computes every reachable endpoint from the pod's network data.
// Two REST shapes are supported:
//   - runtime.ports[] (API-created pods, populated once the pod is running)
//   - portMappings{} + publicIp (UI-created / resumed pods — runtime.ports
//     stays empty for these even when direct TCP routing is live)
//
// Returns (info, ok). When ok is false the pod has no reachable direct
// endpoint yet — callers typically chain `pod wait` and retry.
func (p *Pod) DeriveSSHInfo() (*SSHInfo, bool) {
	if p == nil {
		return nil, false
	}
	info := &SSHInfo{
		PodID: p.ID,
		Proxy: &SSHEndpoint{
			Host:    SSHProxyHost,
			Port:    22,
			User:    p.ID,
			Command: fmt.Sprintf("ssh %s@%s", p.ID, SSHProxyHost),
		},
	}

	specTypes := parseSpecPortTypes(p.Ports)

	for _, port := range runtimePorts(p) {
		if port.PrivatePort == 22 && port.IsIPPublic && port.IP != "" && port.PublicPort != 0 {
			info.Direct = &SSHEndpoint{
				Host:    port.IP,
				Port:    port.PublicPort,
				User:    "root",
				Command: fmt.Sprintf("ssh -p %d root@%s", port.PublicPort, port.IP),
			}
			continue
		}
		if port.Type == "http" && port.PrivatePort != 0 {
			if info.HTTPProxy == nil {
				info.HTTPProxy = map[int]string{}
			}
			info.HTTPProxy[port.PrivatePort] = fmt.Sprintf(HTTPProxyTemplate, p.ID, port.PrivatePort)
			continue
		}
		if port.IsIPPublic && port.IP != "" && port.PublicPort != 0 {
			if info.PublicTCP == nil {
				info.PublicTCP = map[int]string{}
			}
			info.PublicTCP[port.PrivatePort] = fmt.Sprintf("%s:%d", port.IP, port.PublicPort)
		}
	}

	// Fall back to portMappings for ports that runtime.ports didn't cover.
	// Common case: UI/resumed pods where runtime.ports is empty entirely.
	for privStr, pub := range p.PortMappings {
		priv, err := strconv.Atoi(privStr)
		if err != nil || priv <= 0 || pub <= 0 {
			continue
		}
		if priv == 22 && info.Direct == nil && p.PublicIP != "" {
			info.Direct = &SSHEndpoint{
				Host:    p.PublicIP,
				Port:    pub,
				User:    "root",
				Command: fmt.Sprintf("ssh -p %d root@%s", pub, p.PublicIP),
			}
			continue
		}
		// portMappings has no type info — fall back to the spec ports list to
		// know http vs tcp; default to tcp when unknown.
		if specTypes[priv] == "http" {
			if _, seen := info.HTTPProxy[priv]; !seen {
				if info.HTTPProxy == nil {
					info.HTTPProxy = map[int]string{}
				}
				info.HTTPProxy[priv] = fmt.Sprintf(HTTPProxyTemplate, p.ID, priv)
			}
			continue
		}
		if p.PublicIP != "" {
			if _, seen := info.PublicTCP[priv]; !seen {
				if info.PublicTCP == nil {
					info.PublicTCP = map[int]string{}
				}
				info.PublicTCP[priv] = fmt.Sprintf("%s:%d", p.PublicIP, pub)
			}
		}
	}

	// HTTP proxy URLs answer for any port declared as http at create-time,
	// even when neither runtime.ports nor portMappings mentions it.
	for priv, typ := range specTypes {
		if typ != "http" {
			continue
		}
		if info.HTTPProxy == nil {
			info.HTTPProxy = map[int]string{}
		}
		if _, ok := info.HTTPProxy[priv]; !ok {
			info.HTTPProxy[priv] = fmt.Sprintf(HTTPProxyTemplate, p.ID, priv)
		}
	}

	return info, info.Direct != nil
}

// HTTPProxyURL returns the proxy URL for a given private port. Doesn't require
// the port to be in runtime.ports — RunPod's HTTP proxy answers for any port
// declared at create-time.
func (p *Pod) HTTPProxyURL(privatePort int) string {
	if p == nil || p.ID == "" || privatePort <= 0 {
		return ""
	}
	return fmt.Sprintf(HTTPProxyTemplate, p.ID, privatePort)
}

// HasPort returns true when either runtime.ports or portMappings advertises
// the given privatePort. Used by `pod wait` to know when "the http service
// is up", not just "the pod is running".
func (p *Pod) HasPort(privatePort int) bool {
	if p == nil {
		return false
	}
	if p.Runtime != nil {
		for _, port := range p.Runtime.Ports {
			if port.PrivatePort == privatePort {
				return true
			}
		}
	}
	if _, ok := p.PortMappings[strconv.Itoa(privatePort)]; ok {
		return true
	}
	return false
}

// runtimePorts returns runtime.ports if present, or an empty slice — letting
// DeriveSSHInfo handle the nil-runtime case in one place.
func runtimePorts(p *Pod) []PodPort {
	if p == nil || p.Runtime == nil {
		return nil
	}
	return p.Runtime.Ports
}

// parseSpecPortTypes turns the spec Ports list (["22/tcp", "8000/http"]) into
// a {privatePort: "tcp"|"http"} map. Entries without a recognisable port
// number are skipped; entries without an explicit type default to "tcp".
func parseSpecPortTypes(spec []string) map[int]string {
	out := map[int]string{}
	for _, s := range spec {
		slash := strings.IndexByte(s, '/')
		var portStr, typ string
		if slash < 0 {
			portStr, typ = s, "tcp"
		} else {
			portStr, typ = s[:slash], strings.ToLower(s[slash+1:])
		}
		port, err := strconv.Atoi(strings.TrimSpace(portStr))
		if err != nil || port <= 0 {
			continue
		}
		if typ != "http" {
			typ = "tcp"
		}
		out[port] = typ
	}
	return out
}
