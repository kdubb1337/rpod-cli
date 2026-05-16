package cmd

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kdubb1337/rpod-cli/internal/api"
	"github.com/kdubb1337/rpod-cli/internal/output"
)

var datacenterCmd = &cobra.Command{
	Use:     "datacenter",
	Short:   "Inspect RunPod datacenters and the GPU SKUs surfaced in each",
	Aliases: []string{"datacenters", "dc"},
}

var (
	dcListGPUFilter  string
	dcListLocation   string
	dcListListedOnly bool
	dcListStorage    bool
	dcListInStock    bool
)

var datacenterListCmd = &cobra.Command{
	Use:   "list",
	Short: "List datacenters and the GPU SKUs available in each",
	Long: "Returns every datacenter RunPod exposes along with its per-DC GPU\n" +
		"availability. Use --gpu-type to invert the question and answer\n" +
		"\"which datacenters carry this SKU?\"\n\n" +
		"Like `gpu list`, this hits RunPod's public GraphQL endpoint and works\n" +
		"without an API key.",
	Example: `  rpod datacenter list --json
  rpod datacenter list --gpu-type "NVIDIA GeForce RTX 4090"
  rpod datacenter list --location Europe --storage --listed-only
  rpod datacenter list --gpu-type H100 --in-stock --compact`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dcs, err := newClient().ListDataCenters(cmd.Context())
		if err != nil {
			return err
		}
		dcs = filterDataCenters(dcs, dcListGPUFilter, dcListLocation,
			dcListListedOnly, dcListStorage, dcListInStock)
		sort.SliceStable(dcs, func(i, j int) bool { return dcs[i].ID < dcs[j].ID })
		return output.Emit(dcs)
	},
}

// filterDataCenters narrows the DC list and prunes each DC's GpuAvailability
// to entries that match --gpu-type. A DC with zero matching GPUs is dropped
// when --gpu-type is given.
func filterDataCenters(dcs []api.DataCenter, gpuFilter, location string,
	listedOnly, storageOnly, inStock bool) []api.DataCenter {
	needle := strings.ToLower(gpuFilter)
	loc := strings.ToLower(location)
	out := dcs[:0]
	for _, dc := range dcs {
		if listedOnly && !dc.Listed {
			continue
		}
		if storageOnly && !dc.StorageSupport {
			continue
		}
		if loc != "" && !strings.Contains(strings.ToLower(dc.Location), loc) {
			continue
		}
		if needle != "" || inStock {
			kept := dc.GpuAvailability[:0]
			for _, g := range dc.GpuAvailability {
				if needle != "" &&
					!strings.Contains(strings.ToLower(g.GpuTypeID), needle) &&
					!strings.Contains(strings.ToLower(g.DisplayName), needle) {
					continue
				}
				if inStock && !g.Available {
					continue
				}
				kept = append(kept, g)
			}
			if needle != "" && len(kept) == 0 {
				continue
			}
			dc.GpuAvailability = kept
		}
		out = append(out, dc)
	}
	return out
}

func init() {
	datacenterListCmd.Flags().StringVar(&dcListGPUFilter, "gpu-type", "",
		"only DCs whose gpuAvailability includes this GPU (substring on id or displayName)")
	datacenterListCmd.Flags().StringVar(&dcListLocation, "location", "",
		"substring match on the DC's location (e.g. 'United States', 'Europe', 'Canada')")
	datacenterListCmd.Flags().BoolVar(&dcListListedOnly, "listed-only", false,
		"only DCs flagged `listed=true` (user-visible to pod create)")
	datacenterListCmd.Flags().BoolVar(&dcListStorage, "storage", false,
		"only DCs that support network volumes (storageSupport=true)")
	datacenterListCmd.Flags().BoolVar(&dcListInStock, "in-stock", false,
		"prune GPUs where available=false (often paired with --gpu-type)")

	datacenterCmd.AddCommand(datacenterListCmd)
	rootCmd.AddCommand(datacenterCmd)
}
