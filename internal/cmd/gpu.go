package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/kdubb1337/runpod-cli/internal/api"
	"github.com/kdubb1337/runpod-cli/internal/output"
)

var gpuCmd = &cobra.Command{
	Use:   "gpu",
	Short: "Inspect available GPU types and pricing",
}

var (
	gpuListFilter    string
	gpuListMinMem    int
	gpuListSecure    bool
	gpuListCommunity bool
)

var gpuListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available GPU types",
	Long: "Returns every GPU type RunPod offers. Use the returned `id` as\n" +
		"the --gpu-type value for `rpod pod create`.",
	Example: `  rpod gpu list --json
  rpod gpu list --filter A6000
  rpod gpu list --min-memory 48 --community`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gpus, err := newClient().ListGPUTypes(cmd.Context())
		if err != nil {
			return err
		}
		gpus = filterGPUs(gpus, gpuListFilter, gpuListMinMem, gpuListSecure, gpuListCommunity)
		return output.Emit(gpus)
	},
}

func filterGPUs(gpus []api.GPUType, name string, minMem int, secure, community bool) []api.GPUType {
	if name == "" && minMem == 0 && !secure && !community {
		return gpus
	}
	needle := strings.ToLower(name)
	out := gpus[:0]
	for _, g := range gpus {
		if needle != "" &&
			!strings.Contains(strings.ToLower(g.ID), needle) &&
			!strings.Contains(strings.ToLower(g.DisplayName), needle) {
			continue
		}
		if minMem > 0 && g.MemoryInGb < minMem {
			continue
		}
		if secure && !g.SecureCloud {
			continue
		}
		if community && !g.CommunityCloud {
			continue
		}
		out = append(out, g)
	}
	return out
}

func init() {
	gpuListCmd.Flags().StringVar(&gpuListFilter, "filter", "",
		"substring match against id and displayName (case-insensitive)")
	gpuListCmd.Flags().IntVar(&gpuListMinMem, "min-memory", 0, "minimum GPU memory in GB")
	gpuListCmd.Flags().BoolVar(&gpuListSecure, "secure", false, "only GPUs available on Secure Cloud")
	gpuListCmd.Flags().BoolVar(&gpuListCommunity, "community", false, "only GPUs available on Community Cloud")

	gpuCmd.AddCommand(gpuListCmd)
	rootCmd.AddCommand(gpuCmd)
}
