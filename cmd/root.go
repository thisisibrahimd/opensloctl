package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

// rootCmd represents the base command when called without any subcommands
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "opensloctl",
	}

	cmd.AddCommand(newLoadCommand())
	cmd.AddCommand(newGenerateCommand())

	return cmd

}
