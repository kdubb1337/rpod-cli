package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kdubb1337/rpod-cli/internal/output"
)

var volumeCmd = &cobra.Command{
	Use:     "volume",
	Short:   "Manage RunPod network volumes",
	Aliases: []string{"volumes"},
}

var volumeListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List network volumes on the account",
	Example: `  rpod volume list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vols, err := newClient().ListVolumes(cmd.Context())
		if err != nil {
			return err
		}
		return output.Emit(vols)
	},
}

var volumeGetCmd = &cobra.Command{
	Use:     "get <id>",
	Short:   "Get a single network volume by ID",
	Args:    cobra.ExactArgs(1),
	Example: `  rpod volume get vol_abc --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := newClient().GetVolume(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return output.Emit(v)
	},
}

func init() {
	volumeCmd.AddCommand(volumeListCmd, volumeGetCmd)
	rootCmd.AddCommand(volumeCmd)
}
