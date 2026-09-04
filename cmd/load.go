package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/thisisibrahimd/opensloctl/pkg/specstore"
)

type loadFlags struct {
	filenames []string
	recursive bool
}

func newLoadCommand() *cobra.Command {
	flags := loadFlags{}

	cmd := &cobra.Command{
		Use:   "load",
		Short: "read openslo spec files",
		Run: func(cmd *cobra.Command, args []string) {
			runLoad(cmd, args, flags)
		},
	}

	cmd.Flags().StringArrayVarP(&flags.filenames, "filename", "f", []string{}, "The files that contain the openslo specs to load.")
	cmd.Flags().BoolVarP(&flags.recursive, "recursive", "r", false, "Whether to recursively look into the directory.")

	return cmd
}

func runLoad(cmd *cobra.Command, args []string, flags loadFlags) {
	slog.Info("reading files/dirs", "number", len(flags.filenames))

	specs, err := specstore.GetSpecs(flags.filenames, flags.recursive)
	if err != nil {
		slog.Error("unable to load specs", "err", err)
		os.Exit(1)
	}

	slog.Info("specs loaded", "count", len(specs.V1.SLOs))
}
