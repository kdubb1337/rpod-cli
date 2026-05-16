package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kdubb1337/rpod-cli/internal/api"
	"github.com/kdubb1337/rpod-cli/internal/config"
	"github.com/kdubb1337/rpod-cli/internal/output"
)

// Cloud type enum — used to validate `--cloud-type` and emit valid_values on rejection.
var validCloudTypes = []string{"SECURE", "COMMUNITY"}

func newClient() *api.Client {
	return api.New(config.Current().APIKey)
}

var podCmd = &cobra.Command{
	Use:   "pod",
	Short: "Manage RunPod pods (GPU containers)",
}

// --- pod list ---------------------------------------------------------------

var (
	podListStatus string
	podListLimit  int
)

var podListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pods on the account",
	Example: `  rpod pod list --json
  rpod pod list --status RUNNING --compact
  rpod pod list --limit 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		pods, err := newClient().ListPods(ctx, api.ListPodsOptions{DesiredStatus: podListStatus})
		if err != nil {
			return err
		}
		if podListLimit > 0 && len(pods) > podListLimit {
			return output.EmitPage(pods[:podListLimit], "",
				fmt.Sprintf("truncated to %d of %d; raise --limit to see more", podListLimit, len(pods)))
		}
		return output.Emit(pods)
	},
}

// --- pod get ----------------------------------------------------------------

var podGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a single pod by ID",
	Args:  cobra.ExactArgs(1),
	Example: `  rpod pod get abc123 --json
  rpod pod get abc123 --compact`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pod, err := newClient().GetPod(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return output.Emit(pod)
	},
}

// --- pod create -------------------------------------------------------------

var (
	podCreateName            string
	podCreateImage           string
	podCreateGPUTypes        []string
	podCreateGPUCount        int
	podCreateCloudType       string
	podCreateContainerDisk   int
	podCreateVolumeSize      int
	podCreateVolumeMountPath string
	podCreateVolumeID        string
	podCreatePorts           []string
	podCreateEnv             []string
	podCreateDataCenters     []string
	podCreatePublicIP        bool
	podCreateMinVCPUPerGPU   int
	podCreateMinRAMPerGPU    int
	podCreateTemplateID      string
	podCreateInterruptible   bool
	podCreateSSHKeyFile      string
	podCreateWait            bool
	podCreateWaitPorts       []int
	podCreateWaitTimeout     time.Duration
)

var podCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new pod",
	Long: `Create a new RunPod pod. --image and --gpu-type are required.

--gpu-type is repeatable; RunPod will pick the first SKU with capacity:

  rpod pod create --gpu-type "NVIDIA H200" --gpu-type "NVIDIA H200 NVL" ...

If every SKU is out of capacity the call fails with code=capacity
(exit 6); see ` + "`rpod agent-context`" + ` for the full taxonomy.

Discover valid --gpu-type values with: rpod gpu list`,
	Example: `  # Dry-run first
  rpod pod create --image runpod/pytorch:2.4.0 \
    --gpu-type NVIDIA_GEFORCE_RTX_4090 --gpu-count 1 --dry-run

  # Real create with a network volume + block until SSH is reachable
  rpod pod create --name my-pod --image runpod/pytorch:2.4.0 \
    --gpu-type "NVIDIA RTX A6000" --container-disk 20 \
    --volume-id vol_abc --volume-mount /workspace \
    --ssh-key-file ~/.ssh/id_ed25519.pub --public-ip \
    --wait --wait-port 22`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if podCreateImage == "" && podCreateTemplateID == "" {
			return output.Errorf(2, "missing_flag",
				"one of --image or --template-id is required")
		}
		if len(podCreateGPUTypes) == 0 {
			return output.Errorf(2, "missing_flag",
				"--gpu-type is required (discover with: rpod gpu list)")
		}

		if podCreateCloudType != "" {
			ct := strings.ToUpper(podCreateCloudType)
			valid := false
			for _, v := range validCloudTypes {
				if ct == v {
					valid = true
					break
				}
			}
			if !valid {
				return output.ErrorfEnum(9, "invalid_enum", validCloudTypes,
					"--cloud-type=%q is not valid", podCreateCloudType)
			}
			podCreateCloudType = ct
		}

		env, err := parseEnv(podCreateEnv)
		if err != nil {
			return err
		}
		if podCreateSSHKeyFile != "" {
			if env == nil {
				env = map[string]string{}
			}
			key, err := os.ReadFile(podCreateSSHKeyFile)
			if err != nil {
				return output.Errorf(2, "bad_ssh_key_file",
					"could not read --ssh-key-file %q: %v", podCreateSSHKeyFile, err)
			}
			trimmed := strings.TrimSpace(string(key))
			if trimmed == "" {
				return output.Errorf(2, "bad_ssh_key_file",
					"--ssh-key-file %q is empty", podCreateSSHKeyFile)
			}
			if existing, ok := env["PUBLIC_KEY"]; ok && existing != trimmed {
				return output.Errorf(2, "ssh_key_conflict",
					"both --env PUBLIC_KEY=... and --ssh-key-file were set with different values")
			}
			env["PUBLIC_KEY"] = trimmed
		}

		req := api.CreatePodRequest{
			Name:              podCreateName,
			ImageName:         podCreateImage,
			GPUTypeIDs:        podCreateGPUTypes,
			GPUCount:          podCreateGPUCount,
			CloudType:         podCreateCloudType,
			ContainerDiskInGb: podCreateContainerDisk,
			VolumeInGb:        podCreateVolumeSize,
			VolumeMountPath:   podCreateVolumeMountPath,
			NetworkVolumeID:   podCreateVolumeID,
			Ports:             podCreatePorts,
			Env:               env,
			DataCenterIDs:     podCreateDataCenters,
			SupportPublicIP:   podCreatePublicIP,
			MinVCPUPerGPU:     podCreateMinVCPUPerGPU,
			MinRAMPerGPU:      podCreateMinRAMPerGPU,
			TemplateID:        podCreateTemplateID,
			Interruptible:     podCreateInterruptible,
		}

		if flagDryRun {
			return output.EmitDryRun(map[string]any{
				"would_create": req,
				"endpoint":     "POST /pods",
				"wait":         podCreateWait,
				"wait_ports":   podCreateWaitPorts,
			})
		}

		pod, err := newClient().CreatePod(cmd.Context(), req)
		if err != nil {
			return err
		}
		if podCreateWait {
			pod, err = waitForPod(cmd.Context(), pod.ID, waitOptions{
				Ports:   podCreateWaitPorts,
				Timeout: podCreateWaitTimeout,
			})
			if err != nil {
				return err
			}
		}
		return output.Emit(pod)
	},
}

// --- pod delete -------------------------------------------------------------

var podDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a pod (destructive; requires --force or --yes)",
	Args:  cobra.ExactArgs(1),
	Example: `  rpod pod delete abc123 --dry-run
  rpod pod delete abc123 --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		if flagDryRun {
			return output.EmitDryRun(map[string]any{"would_delete_pod": id})
		}
		if !flagForce && !flagYes {
			return output.ErrorfHint(2, "confirmation_required",
				"pass --force or --yes to confirm; --dry-run to preview",
				"refusing to delete pod %s without confirmation", id)
		}
		if err := newClient().DeletePod(cmd.Context(), id); err != nil {
			return err
		}
		return output.Emit(map[string]any{"deleted": id})
	},
}

// --- pod start / stop ------------------------------------------------------

var podStartCmd = &cobra.Command{
	Use:     "start <id>",
	Short:   "Resume a stopped pod",
	Args:    cobra.ExactArgs(1),
	Example: `  rpod pod start abc123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagDryRun {
			return output.EmitDryRun(map[string]any{"would_start_pod": args[0]})
		}
		pod, err := newClient().StartPod(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return output.Emit(pod)
	},
}

var podStopCmd = &cobra.Command{
	Use:     "stop <id>",
	Short:   "Stop a running pod (preserves volume)",
	Args:    cobra.ExactArgs(1),
	Example: `  rpod pod stop abc123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagDryRun {
			return output.EmitDryRun(map[string]any{"would_stop_pod": args[0]})
		}
		pod, err := newClient().StopPod(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return output.Emit(pod)
	},
}

// --- parsing helpers --------------------------------------------------------

// parseEnv turns "KEY=VAL" pairs into a map; rejects malformed entries.
func parseEnv(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, output.Errorf(2, "bad_env",
				"--env entries must be KEY=VALUE (got %q)", p)
		}
		m[k] = v
	}
	return m, nil
}

func init() {
	podListCmd.Flags().StringVar(&podListStatus, "status", "",
		"filter by desiredStatus (e.g. RUNNING, EXITED)")
	podListCmd.Flags().IntVar(&podListLimit, "limit", 0,
		"max items to return (0 = no client-side limit)")

	pf := podCreateCmd.Flags()
	pf.StringVar(&podCreateName, "name", "", "pod name (optional, defaults to a generated name)")
	pf.StringVar(&podCreateImage, "image", "", "container image (e.g. runpod/pytorch:2.4.0)")
	pf.StringSliceVar(&podCreateGPUTypes, "gpu-type", nil,
		"GPU type ID (repeatable; RunPod picks the first with capacity)")
	pf.IntVar(&podCreateGPUCount, "gpu-count", 1, "number of GPUs to attach")
	pf.StringVar(&podCreateCloudType, "cloud-type", "", "SECURE or COMMUNITY (default: SECURE)")
	pf.IntVar(&podCreateContainerDisk, "container-disk", 10, "container disk size in GB")
	pf.IntVar(&podCreateVolumeSize, "volume-size", 0, "ephemeral volume size in GB (use --volume-id for persistent)")
	pf.StringVar(&podCreateVolumeMountPath, "volume-mount", "/workspace", "mount path for the volume")
	pf.StringVar(&podCreateVolumeID, "volume-id", "", "attach an existing network volume by ID")
	pf.StringSliceVar(&podCreatePorts, "port", nil, "expose a port (repeatable, e.g. --port 8888/http)")
	pf.StringArrayVar(&podCreateEnv, "env", nil, "environment variable KEY=VALUE (repeatable)")
	pf.StringSliceVar(&podCreateDataCenters, "data-center", nil, "preferred data-center IDs (repeatable)")
	pf.BoolVar(&podCreatePublicIP, "public-ip", false, "request a public IP")
	pf.IntVar(&podCreateMinVCPUPerGPU, "min-vcpus-per-gpu", 0, "minimum vCPUs per GPU")
	pf.IntVar(&podCreateMinRAMPerGPU, "min-ram-per-gpu", 0, "minimum RAM (GB) per GPU")
	pf.StringVar(&podCreateTemplateID, "template-id", "", "use a Runpod template (omits --image)")
	pf.BoolVar(&podCreateInterruptible, "interruptible", false, "create a spot/interruptible pod")
	pf.StringVar(&podCreateSSHKeyFile, "ssh-key-file", "",
		"path to a public key; populates PUBLIC_KEY env so sshd accepts it")
	pf.BoolVar(&podCreateWait, "wait", false,
		"after creating, block until the pod reaches RUNNING (see --wait-port, --wait-timeout)")
	pf.IntSliceVar(&podCreateWaitPorts, "wait-port", nil,
		"with --wait, also require these private ports to appear in runtime.ports (repeatable)")
	pf.DurationVar(&podCreateWaitTimeout, "wait-timeout", 5*time.Minute,
		"with --wait, give up after this much wall time")

	podCmd.AddCommand(podListCmd, podGetCmd, podCreateCmd, podDeleteCmd, podStartCmd, podStopCmd)
	rootCmd.AddCommand(podCmd)
}
