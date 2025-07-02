package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/codeready-toolchain/toolchain-cicd/govulncheck-action/internal/configuration"
	"github.com/codeready-toolchain/toolchain-cicd/govulncheck-action/internal/govulncheck"
	"github.com/spf13/cobra"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := NewVulnCheckCmd().Execute()
	if err != nil {
		os.Exit(1)
	}
}

func NewVulnCheckCmd() *cobra.Command {
	var configFile, path string
	var cmd = &cobra.Command{
		Use:          "vuln-check",
		Short:        "Run govulncheck and exclude vulnerabilities listed in the '--ignored' YAML file",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			config, err := configuration.New(configFile)
			if err != nil {
				return err
			}
			logger := log.New(cmd.OutOrStdout(), "", 0)
			vulns, err := govulncheck.Scan(cmd.Context(), logger, govulncheck.DefaultScan, config, path)
			switch {
			case err != nil:
				return err
			case len(vulns) > 0:
				govulncheck.PrintVulnerabilities(logger, vulns)
				return fmt.Errorf("%d vulnerabilities found", len(vulns))
			default:
				logger.Println("no vulnerabilities found")
				return nil
			}
		},
	}
	cmd.Flags().StringVar(&configFile, "config", "", "path to the config file")
	cmd.Flags().StringVar(&path, "path", ".", "path to the config file")
	return cmd
}
