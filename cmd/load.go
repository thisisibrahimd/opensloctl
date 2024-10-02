package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/thisisibrahimd/opensloctl/pkg/spec_store"
)

var (
	filenames []string
	recursive bool
)

// loadCmd represents the load command
var loadCmd = &cobra.Command{
	Use:   "load",
	Short: "read openslo spec files",
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("reading files/dirs", "number", len(filenames))

		// Read and load specs
		specStore := spec_store.NewSpecStore(filenames, recursive)
		specStore.LoadSpecs()

	},
}

func init() {
	rootCmd.AddCommand(loadCmd)

	loadCmd.Flags().StringArrayVarP(&filenames, "filename", "f", []string{}, "The files that contain the openslo specs to load.")
	loadCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Whether to recursively look into the directory.")
}
