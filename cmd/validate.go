package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/thisisibrahimd/opensloctl/internal/generator/prometheusgenerator"
	"github.com/thisisibrahimd/opensloctl/pkg/specstore"
)

type validateFlags struct {
	filenames []string
	recursive bool
}

func newValidateCommand() *cobra.Command {
	flags := validateFlags{}

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate OpenSlo specs without writing generated files",
		Run: func(cmd *cobra.Command, args []string) {
			runValidate(cmd, args, flags)
		},
	}

	cmd.Flags().StringArrayVarP(&flags.filenames, "filename", "f", []string{}, "The files that contain the openslo specs to load.")
	cmd.Flags().BoolVarP(&flags.recursive, "recursive", "r", false, "Whether to recursively look into the directory.")

	return cmd
}

func runValidate(cmd *cobra.Command, args []string, flags validateFlags) {
	slog.Info("validating specs", "files", len(flags.filenames))

	specs, err := specstore.GetSpecs(flags.filenames, flags.recursive)
	if err != nil {
		slog.Error("load-time validation failed", "err", err)
		os.Exit(1)
	}

	pg := prometheusgenerator.NewPrometheusGenerator(specs)
	if err := pg.Validate(); err != nil {
		slog.Error("generator validation failed", "err", err)
		os.Exit(1)
	}

	slog.Info("all validations passed", "slos", len(specs.V1.SLOs))
}
