package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/action"
	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/github"
	"github.com/spf13/cobra"
)

func Execute() {
	err := NewGoUpdateCmd().Execute()
	if err != nil {
		os.Exit(1)
	}
}

func NewGoUpdateCmd() *cobra.Command {
	var (
		updateType    string
		token         string
		owner         string
		repo          string
		goVersionFile string
		pathsRaw      string
		labelsRaw     string
		excludeRaw    string
		debug         bool
	)

	cmd := &cobra.Command{
		Use:          "go-update",
		Short:        "Check for Go version updates and open PRs",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := action.Config{
				UpdateType:    updateType,
				Paths:         action.ParseNewlineSeparated(pathsRaw),
				Labels:        action.ParseNewlineSeparated(labelsRaw),
				Excludes:      action.ParseNewlineSeparated(excludeRaw),
				Root:          ".",
				GoVersionFile: goVersionFile,
			}

			opts := &slog.HandlerOptions{Level: slog.LevelInfo}
			if debug {
				opts.Level = slog.LevelDebug
			}
			logger := slog.New(slog.NewTextHandler(cmd.OutOrStdout(), opts))

			ghClient := github.NewClient(owner, repo, token)

			result, err := action.Run(cmd.Context(), logger, cfg, ghClient, action.ExecGoModTidy)
			if err != nil {
				return err
			}

			setOutput("action-taken", result.ActionTaken)
			setOutput("pr-url", result.PRURL)

			return nil
		},
	}

	cmd.Flags().StringVar(&updateType, "type", "", "update type: patch or minor")
	_ = cmd.MarkFlagRequired("type")
	cmd.Flags().StringVar(&token, "token", "", "GitHub token")
	_ = cmd.MarkFlagRequired("token")
	cmd.Flags().StringVar(&owner, "owner", "", "repository owner")
	_ = cmd.MarkFlagRequired("owner")
	cmd.Flags().StringVar(&repo, "repo", "", "repository name")
	_ = cmd.MarkFlagRequired("repo")
	cmd.Flags().StringVar(&goVersionFile, "go-version-file", "go.mod", "path to go.mod for version detection")
	cmd.Flags().StringVar(&pathsRaw, "paths", ".", "directories to scan (newline-separated)")
	cmd.Flags().StringVar(&labelsRaw, "labels", "go-update", "labels for PRs (newline-separated)")
	cmd.Flags().StringVar(&excludeRaw, "exclude", "", "glob patterns to exclude (newline-separated)")
	cmd.Flags().BoolVar(&debug, "debug", false, "debug mode")

	return cmd
}

func setOutput(key, value string) {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		return
	}
	f, err := os.OpenFile(filepath.Clean(outputFile), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s=%s\n", key, value)
}
