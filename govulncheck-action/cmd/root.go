package cmd

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"

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
	var debug bool
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
			opts := &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}
			if debug {
				opts.Level = slog.LevelDebug
			}
			handler := slog.NewTextHandler(cmd.OutOrStdout(), opts)
			logger := slog.New(handler)
			// check the current working directory
			workingDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			logger.Debug("working directory", "path", workingDir)
			// check that there is a `go.mod` file in the path
			// (required by the underlying govulncheck command, but here we can collect insights of failures)
			gomodCmd := exec.CommandContext(cmd.Context(), "go", "env", "GOMOD")
			gomodCmd.Dir = path
			output, err := gomodCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get `go.mod` file: %w", err)
			}
			logger.Debug("`go.mod` file", "path", string(output))
			vulns, outdatedVulns, err := govulncheck.Scan(cmd.Context(), logger, govulncheck.DefaultScan(cmd.OutOrStderr()), path, config)
			switch {
			case err != nil:
				return err
			case len(vulns) > 0 || len(outdatedVulns) > 0:
				govulncheck.PrintVulnerabilities(cmd.OutOrStdout(), vulns)
				govulncheck.PrintOutdatedVulnerabilities(cmd.OutOrStdout(), outdatedVulns)
				return fmt.Errorf("%d vulnerabilities found and %d outdated vulnerabilities found", len(vulns), len(outdatedVulns))
			default:
				logger.Info("no vulnerabilities found")
				return nil
			}
		},
	}
	cmd.Flags().StringVar(&configFile, "config", "", "path to the ignored vulnerabilities config file")
	if err := cmd.MarkFlagRequired("config"); err != nil {
		log.Fatalf("failed to mark flag required: %v", err)
	}
	cmd.Flags().StringVar(&path, "path", ".", "path to the repository root directory to scan")
	if err := cmd.MarkFlagRequired("path"); err != nil {
		log.Fatalf("failed to mark flag required: %v", err)
	}
	cmd.Flags().BoolVar(&debug, "debug", false, "debug mode")
	return cmd
}
