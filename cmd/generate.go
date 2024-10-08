package cmd

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/thisisibrahimd/opensloctl/internal/generator/prometheusgenerator"
	"github.com/thisisibrahimd/opensloctl/pkg/specstore"
)

var (
	// generator string
	outputDirectory string
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate monitoring resources from openslo specs",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("generate called")

		specStore := specstore.NewSpecStore(filenames, recursive)
		specStore.LoadSpecs()

		pg := prometheusgenerator.NewPrometheusGenerator(specStore)
		err := pg.Generate(outputDirectory)
		if err != nil {
			log.Fatal("unable to generate files", "err", err)
		}

	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringArrayVarP(&filenames, "filename", "f", []string{}, "The files that contain the openslo specs to load.")
	generateCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Whether to recursively look into the directory.")
	generateCmd.Flags().StringVarP(&outputDirectory, "output-directory", "o", "", "directory to write to")
	// generateCmd.Flags().StringVarP(&generator, "generator", "g", "", "select the generator you would like to use")
}
