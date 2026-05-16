package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/kdubb1337/rpod-cli/internal/api"
	"github.com/kdubb1337/rpod-cli/internal/output"
)

// --- pod ssh-info -----------------------------------------------------------

var podSSHInfoCmd = &cobra.Command{
	Use:   "ssh-info <id>",
	Short: "Print SSH endpoints + HTTP proxy URLs for a pod",
	Long: `Derives every reachable endpoint from the pod's runtime data. Returns:

  direct      — TCP ssh root@<ip> -p <port>, when --public-ip is set
  proxy       — universal "<podID>@ssh.runpod.io" (always available)
  http_proxy  — {privatePort: "https://<podID>-<port>.proxy.runpod.net"}
  public_tcp  — {privatePort: "<ip>:<publicPort>"} for non-HTTP TCP ports

When the pod has no runtime yet, only the proxy entry is populated and the
exit code is still 0 — use ` + "`rpod pod wait`" + ` to block until reachable.`,
	Args: cobra.ExactArgs(1),
	Example: `  rpod pod ssh-info abc123
  rpod pod ssh-info abc123 --json | jq -r '.data.direct.command'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pod, err := newClient().GetPod(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		info, _ := pod.DeriveSSHInfo()
		return output.Emit(info)
	},
}

// --- pod url ----------------------------------------------------------------

var podURLCmd = &cobra.Command{
	Use:   "url <id> <port>",
	Short: "Print the RunPod HTTP proxy URL for a pod's port",
	Long: `Prints https://<podID>-<port>.proxy.runpod.net.

This is a pure string format — the port does not need to be advertised in the
pod's runtime yet. Output is one line so it can be piped directly into curl.`,
	Args: cobra.ExactArgs(2),
	Example: `  rpod pod url abc123 8000
  curl "$(rpod pod url abc123 8000)/health"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[1])
		if err != nil || port <= 0 {
			return output.Errorf(2, "bad_port",
				"port must be a positive integer (got %q)", args[1])
		}
		fmt.Fprintf(os.Stdout, api.HTTPProxyTemplate+"\n", args[0], port)
		return nil
	},
}

func init() {
	podCmd.AddCommand(podSSHInfoCmd, podURLCmd)
}
