package cmd

import (
	"github.com/charmbracelet/log"
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
	log.Info("running generate command")

	specStore := specstore.NewSpecStore(specstore.WithFilenames(flags.filenames), specstore.WithRecursive(flags.recursive))
	specs, err := specStore.GetSpecs()
	if err != nil {
		log.Fatal(err)
	}

	pg := prometheusgenerator.NewPrometheusGenerator(specs)
	err = pg.Generate(flags.outputDirectory)
	if err != nil {
		log.Fatal("unable to generate files", "err", err)
	}
}
