package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/kdubb1337/rpod-cli/internal/api"
	"github.com/kdubb1337/rpod-cli/internal/output"
)

// --- pod wait ---------------------------------------------------------------

var (
	podWaitPorts    []int
	podWaitTimeout  time.Duration
	podWaitInterval time.Duration
	podWaitStatus   string
)

var podWaitCmd = &cobra.Command{
	Use:   "wait <id>",
	Short: "Block until a pod is RUNNING and the requested ports are exposed",
	Long: `Polls GET /pods/<id> with exponential backoff (capped at --interval)
until: (a) desiredStatus == --status, and (b) every --port has been published
in runtime.ports. Returns the pod payload exactly as ` + "`rpod pod get`" + ` would.

Exit codes:
  0    pod reached the requested state
  3    pod not found
  124  timed out before the pod reached the requested state`,
	Args: cobra.ExactArgs(1),
	Example: `  # Wait up to 5 min for the pod to come up; require SSH+HTTP exposed
  rpod pod wait abc123 --port 22 --port 8000 --timeout 5m

  # Pipe straight into ssh-info to grab the connection details
  rpod pod wait abc123 --port 22 --json && rpod pod ssh-info abc123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pod, err := waitForPod(cmd.Context(), args[0], waitOptions{
			Ports:    podWaitPorts,
			Status:   podWaitStatus,
			Timeout:  podWaitTimeout,
			Interval: podWaitInterval,
		})
		if err != nil {
			return err
		}
		return output.Emit(pod)
	},
}

type waitOptions struct {
	Ports    []int
	Status   string
	Timeout  time.Duration
	Interval time.Duration
}

// waitForPod polls GET /pods/<id> with exponential backoff until the pod
// reaches the desired status and all requested ports are exposed. Returns the
// pod on success or a typed CLIError on timeout / not-found.
func waitForPod(ctx context.Context, id string, opt waitOptions) (*api.Pod, error) {
	if opt.Timeout <= 0 {
		opt.Timeout = 5 * time.Minute
	}
	if opt.Interval <= 0 {
		opt.Interval = 5 * time.Second
	}
	if opt.Status == "" {
		opt.Status = "RUNNING"
	}

	deadline := time.Now().Add(opt.Timeout)
	backoff := time.Second
	client := newClient()

	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		pod, err := client.GetPod(ctx, id)
		if err != nil {
			// 404 is fatal — don't keep polling something that doesn't exist.
			var ec interface{ ExitCode() int }
			if errors.As(err, &ec) && ec.ExitCode() == 3 {
				return nil, err
			}
			output.Verbose("wait[%d]: transient: %v", attempt, err)
		} else if podMatches(pod, opt) {
			output.Progress("pod %s reached %s with ports %v", pod.ID, opt.Status, opt.Ports)
			return pod, nil
		} else if pod != nil {
			output.Progress("pod %s status=%s ports=%s", pod.ID, pod.DesiredStatus, summarizePorts(pod))
		}

		if time.Now().After(deadline) {
			return nil, output.ErrorfHint(124, "timeout",
				fmt.Sprintf("re-run with --timeout > %s, or inspect with `rpod pod get %s`", opt.Timeout, id),
				"pod %s did not reach %s with ports %v within %s", id, opt.Status, opt.Ports, opt.Timeout)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		// Exponential backoff capped at --interval.
		if backoff < opt.Interval {
			backoff *= 2
			if backoff > opt.Interval {
				backoff = opt.Interval
			}
		}
	}
}

func podMatches(p *api.Pod, opt waitOptions) bool {
	if p == nil {
		return false
	}
	if p.DesiredStatus != opt.Status {
		return false
	}
	for _, port := range opt.Ports {
		if !p.HasPort(port) {
			return false
		}
	}
	return true
}

func summarizePorts(p *api.Pod) string {
	if p == nil || p.Runtime == nil || len(p.Runtime.Ports) == 0 {
		return "none"
	}
	out := ""
	for i, port := range p.Runtime.Ports {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("%d/%s", port.PrivatePort, port.Type)
	}
	return out
}

func init() {
	podWaitCmd.Flags().IntSliceVar(&podWaitPorts, "port", nil,
		"private port that must appear in runtime.ports (repeatable)")
	podWaitCmd.Flags().DurationVar(&podWaitTimeout, "timeout", 5*time.Minute,
		"give up after this much wall time (e.g. 5m, 600s)")
	podWaitCmd.Flags().DurationVar(&podWaitInterval, "interval", 5*time.Second,
		"max sleep between polls (exponential backoff caps here)")
	podWaitCmd.Flags().StringVar(&podWaitStatus, "status", "RUNNING",
		"desiredStatus to wait for")
	podCmd.AddCommand(podWaitCmd)
}
