package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/kdubb1337/rpod-cli/internal/api"
	"github.com/kdubb1337/rpod-cli/internal/output"
)

// pod_remote.go — `pod exec` and `pod cp`. Thin wrappers around the system's
// `ssh`/`scp` binaries; we deliberately do NOT vendor a Go SSH library. Reasons:
//   1. Users already have ssh keyrings, known_hosts, and ssh_config set up;
//      shelling out reuses all of that.
//   2. Interactive sessions need a real PTY — exec.Command with os.Std{in,out,err}
//      attached is the most reliable cross-platform answer.
//   3. The only RunPod-specific work is endpoint discovery, which lives in
//      api/ssh.go and is reused by `pod ssh-info`.

// --- shared flags -----------------------------------------------------------

var (
	remoteIdentity   string
	remoteUseProxy   bool
	remoteStrictHost string
	remoteExtraOpts  []string
)

func addRemoteFlags(cmd *cobra.Command) {
	pf := cmd.Flags()
	pf.StringVar(&remoteIdentity, "identity", "",
		"path to ssh private key (`-i` for ssh/scp)")
	pf.BoolVar(&remoteUseProxy, "use-proxy", false,
		"force proxy SSH (<podID>@ssh.runpod.io) even when a direct endpoint is available")
	pf.StringVar(&remoteStrictHost, "strict-host-key-checking", "accept-new",
		"StrictHostKeyChecking value (yes|accept-new|no)")
	pf.StringArrayVar(&remoteExtraOpts, "ssh-opt", nil,
		"extra `-o` option passed to ssh/scp (repeatable, e.g. --ssh-opt ServerAliveInterval=15)")
}

// resolveEndpoint fetches the pod and picks the right SSH endpoint. Returns
// (endpoint, isProxy) so the caller can adjust user/host expectations.
func resolveEndpoint(ctx context.Context, podID string) (*api.SSHEndpoint, bool, error) {
	pod, err := newClient().GetPod(ctx, podID)
	if err != nil {
		return nil, false, err
	}
	info, ready := pod.DeriveSSHInfo()
	if remoteUseProxy || !ready {
		return info.Proxy, true, nil
	}
	return info.Direct, false, nil
}

// baseSSHArgs returns the `-o`/`-i` options every ssh/scp invocation gets.
// Order matters: identity/options first, then the host-form.
func baseSSHArgs() []string {
	args := []string{
		"-o", "StrictHostKeyChecking=" + remoteStrictHost,
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
	}
	if remoteIdentity != "" {
		args = append(args, "-i", remoteIdentity)
	}
	for _, opt := range remoteExtraOpts {
		args = append(args, "-o", opt)
	}
	return args
}

// --- pod exec ---------------------------------------------------------------

var podExecCmd = &cobra.Command{
	Use:   "exec <id> [-- <cmd...>]",
	Short: "Run a command on a pod over SSH (or open an interactive shell)",
	Long: `Looks up the pod's SSH endpoint, then execs the system ssh binary.
With no command, opens an interactive shell. Use -- to separate rpod flags
from the remote command:

  rpod pod exec abc123 -- bash setup.sh
  rpod pod exec abc123 -- nvidia-smi
  rpod pod exec abc123                          # interactive shell

Direct SSH is preferred when --public-ip is set; otherwise proxy SSH
(<podID>@ssh.runpod.io) is used. Force proxy with --use-proxy.`,
	Args: cobra.MinimumNArgs(1),
	Example: `  rpod pod exec abc123 -- nvidia-smi
  rpod pod exec abc123 --identity ~/.ssh/id_ed25519 -- python infer.py
  rpod pod exec abc123 --use-proxy`,
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		podID := args[0]
		remoteCmd := args[1:]

		endpoint, isProxy, err := resolveEndpoint(cmd.Context(), podID)
		if err != nil {
			return err
		}
		sshArgs := baseSSHArgs()
		if !isProxy {
			sshArgs = append(sshArgs, "-p", fmt.Sprintf("%d", endpoint.Port))
		}
		sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", endpoint.User, endpoint.Host))
		sshArgs = append(sshArgs, remoteCmd...)

		output.Verbose("ssh %s", strings.Join(sshArgs, " "))
		if flagDryRun {
			return output.EmitDryRun(map[string]any{
				"would_exec": append([]string{"ssh"}, sshArgs...),
				"endpoint":   endpoint,
				"proxy":      isProxy,
			})
		}
		return runForeground("ssh", sshArgs)
	},
}

// --- pod cp -----------------------------------------------------------------

var podCpCmd = &cobra.Command{
	Use:   "cp <src> <dst>",
	Short: "Copy files to/from a pod over scp",
	Long: `Either <src> or <dst> must be of the form <podID>:<path>; the
other side is a local path. Wraps the system scp binary with the same endpoint
discovery as ` + "`pod exec`" + `.

  rpod pod cp ./inference.py abc123:/workspace/inference.py
  rpod pod cp abc123:/workspace/out.json ./out.json
  rpod pod cp -r ./src abc123:/workspace/src   # recursive`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, dst := args[0], args[1]
		srcPodID, srcPath, srcIsRemote := splitRemoteSpec(src)
		dstPodID, dstPath, dstIsRemote := splitRemoteSpec(dst)

		switch {
		case srcIsRemote && dstIsRemote:
			return output.Errorf(2, "bad_args",
				"both <src> and <dst> are remote; scp pod→pod is not supported")
		case !srcIsRemote && !dstIsRemote:
			return output.Errorf(2, "bad_args",
				"neither <src> nor <dst> is of the form <podID>:<path>")
		}

		podID := srcPodID
		if dstIsRemote {
			podID = dstPodID
		}
		endpoint, isProxy, err := resolveEndpoint(cmd.Context(), podID)
		if err != nil {
			return err
		}

		scpArgs := baseSSHArgs()
		if podCpRecursive {
			scpArgs = append(scpArgs, "-r")
		}
		if !isProxy {
			// scp uses -P (capital) for port. ssh uses -p (lowercase).
			scpArgs = append(scpArgs, "-P", fmt.Sprintf("%d", endpoint.Port))
		}
		remoteHost := fmt.Sprintf("%s@%s", endpoint.User, endpoint.Host)
		if srcIsRemote {
			scpArgs = append(scpArgs, fmt.Sprintf("%s:%s", remoteHost, srcPath), dst)
		} else {
			scpArgs = append(scpArgs, src, fmt.Sprintf("%s:%s", remoteHost, dstPath))
		}

		output.Verbose("scp %s", strings.Join(scpArgs, " "))
		if flagDryRun {
			return output.EmitDryRun(map[string]any{
				"would_exec": append([]string{"scp"}, scpArgs...),
				"endpoint":   endpoint,
				"proxy":      isProxy,
			})
		}
		return runForeground("scp", scpArgs)
	},
}

var podCpRecursive bool

// splitRemoteSpec parses "<podID>:<path>" into (id, path, true) or returns
// ("", spec, false) for purely local paths. Bare ":" without a podID does
// not count as remote.
func splitRemoteSpec(spec string) (id, path string, remote bool) {
	// Heuristic: a remote spec contains a ':' that is not part of a Windows
	// drive letter (`C:\...`). Pod IDs are alphanumeric, so the part before
	// the colon must be non-empty alnum.
	idx := strings.Index(spec, ":")
	if idx <= 0 {
		return "", spec, false
	}
	left := spec[:idx]
	// Reject local Windows-style drive letters.
	if len(left) == 1 {
		return "", spec, false
	}
	// Reject anything with a path separator on the left (e.g. ./foo:bar).
	if strings.ContainsAny(left, "/\\") {
		return "", spec, false
	}
	return left, spec[idx+1:], true
}

// runForeground execs cmd attached to the user's stdio. Returns a CLIError
// with exit=124 on PTY/signal issues; otherwise propagates the child's exit
// code via the ExitCoder interface so main.go forwards it.
func runForeground(bin string, args []string) error {
	c := exec.Command(bin, args...) // #nosec G204 — args are constructed from typed flags
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code := 1
			if status, ok := ee.Sys().(syscall.WaitStatus); ok {
				code = status.ExitStatus()
			}
			return output.Errorf(code, "remote_exit",
				"%s exited with code %d", bin, code)
		}
		return output.Errorf(8, "remote_spawn",
			"could not spawn %s: %v", bin, err)
	}
	return nil
}

func init() {
	addRemoteFlags(podExecCmd)
	addRemoteFlags(podCpCmd)
	podCpCmd.Flags().BoolVarP(&podCpRecursive, "recursive", "r", false,
		"copy directories recursively (-r on scp)")
	podCmd.AddCommand(podExecCmd, podCpCmd)
}
