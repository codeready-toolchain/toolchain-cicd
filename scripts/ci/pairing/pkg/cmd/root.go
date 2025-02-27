package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = NewRootCmd()

func NewRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test-e2e",
		Short: "test-e2e command-line",
		Long:  `test-e2e command-line that helps you pair PRs`,
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(NewPairCmd())
}
