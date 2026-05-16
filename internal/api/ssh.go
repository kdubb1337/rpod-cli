package api

import "fmt"

// SSHInfo summarises every way to reach a pod over SSH plus the HTTP/TCP
// surface RunPod exposes. Derived purely from Pod.Runtime — no extra API call.
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

// DeriveSSHInfo computes every reachable endpoint from the pod's runtime data.
// Returns (info, ok). When ok is false the pod has no runtime data yet (still
// starting) — callers typically chain `pod wait` and retry.
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
	if p.Runtime == nil || len(p.Runtime.Ports) == 0 {
		// Proxy SSH still works without any port mapping, but we report
		// `ok=false` so callers know they may need to wait.
		return info, false
	}
	for _, port := range p.Runtime.Ports {
		// SSH: privatePort=22 with a public IP → direct endpoint
		if port.PrivatePort == 22 && port.IsIPPublic && port.IP != "" && port.PublicPort != 0 {
			info.Direct = &SSHEndpoint{
				Host:    port.IP,
				Port:    port.PublicPort,
				User:    "root",
				Command: fmt.Sprintf("ssh -p %d root@%s", port.PublicPort, port.IP),
			}
			continue
		}
		// HTTP: any port advertised as http → reverse-proxied URL
		if port.Type == "http" && port.PrivatePort != 0 {
			if info.HTTPProxy == nil {
				info.HTTPProxy = map[int]string{}
			}
			info.HTTPProxy[port.PrivatePort] = fmt.Sprintf(HTTPProxyTemplate, p.ID, port.PrivatePort)
			continue
		}
		// TCP with public IP
		if port.IsIPPublic && port.IP != "" && port.PublicPort != 0 {
			if info.PublicTCP == nil {
				info.PublicTCP = map[int]string{}
			}
			info.PublicTCP[port.PrivatePort] = fmt.Sprintf("%s:%d", port.IP, port.PublicPort)
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

// HasPort returns true when the pod's runtime advertises a given privatePort.
// Used by `pod wait` to know when "the http service is up", not just "the pod
// is running".
func (p *Pod) HasPort(privatePort int) bool {
	if p == nil || p.Runtime == nil {
		return false
	}
	for _, port := range p.Runtime.Ports {
		if port.PrivatePort == privatePort {
			return true
		}
	}
	return false
}
