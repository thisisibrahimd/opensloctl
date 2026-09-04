package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/thisisibrahimd/opensloctl/internal/generator/prometheusgenerator"
	"github.com/thisisibrahimd/opensloctl/pkg/specstore"
)

type generateFlags struct {
	filenames       []string
	recursive       bool
	outputDirectory string
}

func newGenerateCommand() *cobra.Command {
	flags := generateFlags{}

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate monitoring resources from openslo specs",
		Run: func(cmd *cobra.Command, args []string) {
			runGenerate(cmd, args, flags)
		},
	}

	cmd.Flags().StringArrayVarP(&flags.filenames, "filename", "f", []string{}, "The files that contain the openslo specs to load.")
	cmd.Flags().BoolVarP(&flags.recursive, "recursive", "r", false, "Whether to recursively look into the directory.")
	cmd.Flags().StringVarP(&flags.outputDirectory, "output-directory", "o", "", "directory to write to")

	return cmd
}

func runGenerate(cmd *cobra.Command, args []string, flags generateFlags) {
	slog.Info("running generate command")

	specs, err := specstore.GetSpecs(flags.filenames, flags.recursive)
	if err != nil {
		slog.Error("unable to load specs", "err", err)
		os.Exit(1)
	}

	pg := prometheusgenerator.NewPrometheusGenerator(specs)
	err = pg.Generate(flags.outputDirectory)
	if err != nil {
		slog.Error("unable to generate files", "err", err)
		os.Exit(1)
	}
}
