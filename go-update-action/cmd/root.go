package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/github"
	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/goversion"
	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/updater"
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
			ctx := cmd.Context()
			paths := parseNewlineSeparated(pathsRaw)
			labels := parseNewlineSeparated(labelsRaw)
			excludes := parseNewlineSeparated(excludeRaw)

			opts := &slog.HandlerOptions{Level: slog.LevelInfo}
			if debug {
				opts.Level = slog.LevelDebug
			}
			logger := slog.New(slog.NewTextHandler(cmd.OutOrStdout(), opts))

			current, err := readCurrentVersion(goVersionFile)
			if err != nil {
				return fmt.Errorf("reading current Go version: %w", err)
			}
			logger.Info("current Go version", "version", current.String())

			releases, err := goversion.FetchLatestReleases(ctx)
			if err != nil {
				return fmt.Errorf("fetching Go releases: %w", err)
			}
			logger.Debug("fetched releases", "count", len(releases))

			var target *goversion.Version
			switch updateType {
			case "patch":
				target = goversion.FindPatchUpdate(current, releases)
			case "minor":
				target = goversion.FindMinorUpdate(current, releases)
			default:
				return fmt.Errorf("invalid update type %q: must be 'patch' or 'minor'", updateType)
			}

			if target == nil {
				logger.Info("already on latest", "type", updateType, "current", current.String())
				setOutput("action-taken", "skipped")
				setOutput("pr-url", "")
				return nil
			}
			logger.Info("update available", "type", updateType, "from", current.String(), "to", target.String())

			ghClient := github.NewClient(ctx, token, owner, repo)

			branchName := fmt.Sprintf("go-update-%s", target.String())
			prTitle := fmt.Sprintf("chore: bump Go from %s to %s", current.String(), target.String())

			actionTaken, err := handleExistingPRs(ctx, logger, ghClient, labels[0], branchName, current, *target, updateType)
			if err != nil {
				return err
			}
			if actionTaken == "skipped" {
				logger.Info("PR already exists for target version", "version", target.String())
				setOutput("action-taken", "skipped")
				setOutput("pr-url", "")
				return nil
			}

			root := "."
			result, err := updater.ScanAndUpdate(root, current.String(), target.String(), paths, excludes)
			if err != nil {
				return fmt.Errorf("scanning files: %w", err)
			}

			if len(result.Changes) == 0 {
				logger.Info("no files need updating")
				setOutput("action-taken", "skipped")
				setOutput("pr-url", "")
				return nil
			}

			logger.Info("files to update", "count", len(result.Changes))
			for _, c := range result.Changes {
				logger.Debug("file changed", "path", c.Path)
			}

			ghChanges, tidyWarning, err := prepareChangesWithTidy(ctx, logger, root, result.Changes, *target)
			if err != nil {
				return err
			}

			baseSHA, err := ghClient.CreateBranchFromDefault(ctx, branchName)
			if err != nil {
				return fmt.Errorf("creating branch: %w", err)
			}

			commitMsg := prTitle
			_, err = ghClient.CreateCommit(ctx, branchName, baseSHA, commitMsg, ghChanges)
			if err != nil {
				return fmt.Errorf("creating commit: %w", err)
			}

			prBody := formatPRBody(current, *target, result.Changes, tidyWarning)
			prURL, err := ghClient.CreatePR(ctx, prTitle, prBody, branchName, labels)
			if err != nil {
				return fmt.Errorf("creating PR: %w", err)
			}

			logger.Info("PR created", "url", prURL)
			if actionTaken == "replaced" {
				setOutput("action-taken", "replaced")
			} else {
				setOutput("action-taken", "created")
			}
			setOutput("pr-url", prURL)

			if len(result.Suggestions) > 0 {
				handleSuggestionPR(ctx, logger, ghClient, result.Suggestions)
			}

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

var goDirectiveRe = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+(?:\.\d+)?)\s*$`)

func readCurrentVersion(goModPath string) (goversion.Version, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return goversion.Version{}, fmt.Errorf("reading %s: %w", goModPath, err)
	}
	matches := goDirectiveRe.FindSubmatch(content)
	if matches == nil {
		return goversion.Version{}, fmt.Errorf("no go directive found in %s", goModPath)
	}
	return goversion.Parse(string(matches[1]))
}

func handleExistingPRs(ctx context.Context, logger *slog.Logger, ghClient *github.Client, label, _ string, current, target goversion.Version, updateType string) (string, error) {
	existingPRs, err := ghClient.FindOpenPRByLabel(ctx, label)
	if err != nil {
		return "", fmt.Errorf("finding existing PRs: %w", err)
	}

	branchVersionRe := regexp.MustCompile(`^go-update-(\d+\.\d+\.\d+)$`)
	actionTaken := "created"

	for _, pr := range existingPRs {
		head := pr.GetHead().GetRef()
		matches := branchVersionRe.FindStringSubmatch(head)
		if matches == nil {
			continue
		}
		prVersion, err := goversion.Parse(matches[1])
		if err != nil {
			continue
		}

		if prVersion.Compare(target) == 0 {
			return "skipped", nil
		}

		shouldClose := false
		switch updateType {
		case "patch":
			shouldClose = prVersion.SameMinor(current) && prVersion.Compare(target) < 0
		case "minor":
			shouldClose = prVersion.Minor > current.Minor && prVersion.Compare(target) < 0
		}

		if shouldClose {
			logger.Info("closing outdated PR", "number", pr.GetNumber(), "version", prVersion.String())
			if err := ghClient.ClosePR(ctx, pr.GetNumber()); err != nil {
				logger.Warn("failed to close PR", "number", pr.GetNumber(), "error", err)
			}
			oldBranch := fmt.Sprintf("go-update-%s", prVersion.String())
			if err := ghClient.DeleteBranch(ctx, oldBranch); err != nil {
				logger.Debug("failed to delete old branch", "branch", oldBranch, "error", err)
			}
			actionTaken = "replaced"
		}
	}

	return actionTaken, nil
}

func prepareChangesWithTidy(ctx context.Context, logger *slog.Logger, root string, changes []updater.FileChange, target goversion.Version) ([]github.FileChange, string, error) {
	ghChanges := make([]github.FileChange, 0, len(changes))
	var tidyWarning string

	gomodDirs := map[string]bool{}
	for _, c := range changes {
		if filepath.Base(c.Path) == "go.mod" {
			gomodDirs[filepath.Dir(c.Path)] = true
		}
	}

	for dir := range gomodDirs {
		gomodPath := filepath.Join(root, dir, "go.mod")
		for _, c := range changes {
			if c.Path == filepath.Join(dir, "go.mod") {
				if err := os.WriteFile(gomodPath, []byte(c.NewContent), 0o644); err != nil {
					return nil, "", fmt.Errorf("writing go.mod for tidy: %w", err)
				}
				break
			}
		}

		tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
		tidyCmd.Dir = filepath.Join(root, dir)
		tidyCmd.Env = append(os.Environ(), fmt.Sprintf("GOTOOLCHAIN=go%s", target.String()))
		output, err := tidyCmd.CombinedOutput()
		if err != nil {
			logger.Warn("go mod tidy failed", "dir", dir, "error", err, "output", string(output))
			tidyWarning = fmt.Sprintf("⚠️ `go mod tidy` failed in `%s`:\n```\n%s\n```\nManual intervention may be required.", dir, strings.TrimSpace(string(output)))
		} else {
			logger.Debug("go mod tidy succeeded", "dir", dir)
		}
	}

	for _, c := range changes {
		content := c.NewContent
		if filepath.Base(c.Path) == "go.mod" && gomodDirs[filepath.Dir(c.Path)] {
			tidied, err := os.ReadFile(filepath.Join(root, c.Path))
			if err == nil {
				content = string(tidied)
			}
		}
		ghChanges = append(ghChanges, github.FileChange{
			Path:    c.Path,
			Content: content,
		})
	}

	for dir := range gomodDirs {
		goSumPath := filepath.Join(root, dir, "go.sum")
		if sumContent, err := os.ReadFile(goSumPath); err == nil {
			sumRel := filepath.Join(dir, "go.sum")
			alreadyIncluded := false
			for _, c := range ghChanges {
				if c.Path == sumRel {
					alreadyIncluded = true
					break
				}
			}
			if !alreadyIncluded {
				ghChanges = append(ghChanges, github.FileChange{
					Path:    sumRel,
					Content: string(sumContent),
				})
			}
		}

		gomodPath := filepath.Join(root, dir, "go.mod")
		for _, c := range changes {
			if c.Path == filepath.Join(dir, "go.mod") {
				_ = os.WriteFile(gomodPath, []byte(c.OldContent), 0o644)
				break
			}
		}
	}

	return ghChanges, tidyWarning, nil
}

func formatPRBody(current goversion.Version, target goversion.Version, changes []updater.FileChange, tidyWarning string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Go version update: %s → %s\n\n", current.String(), target.String()))

	releaseURL := fmt.Sprintf("https://go.dev/doc/devel/release#go%s", target.MinorString())
	sb.WriteString(fmt.Sprintf("[Release notes](%s)\n\n", releaseURL))

	sb.WriteString("### Changed files\n\n")
	for _, c := range changes {
		sb.WriteString(fmt.Sprintf("- `%s`\n", c.Path))
	}

	if tidyWarning != "" {
		sb.WriteString(fmt.Sprintf("\n### Warnings\n\n%s\n", tidyWarning))
	}

	sb.WriteString("\n---\n*This PR was automatically created by the go-update-action.*\n")
	return sb.String()
}

func handleSuggestionPR(ctx context.Context, logger *slog.Logger, ghClient *github.Client, suggestions []updater.SuggestionChange) {
	existingPRs, err := ghClient.FindOpenPRByLabel(ctx, "go-version-file")
	if err != nil {
		logger.Warn("failed to check for existing suggestion PRs", "error", err)
		return
	}
	if len(existingPRs) > 0 {
		logger.Debug("suggestion PR already exists, skipping")
		return
	}

	branchName := "go-update-use-go-version-file"
	baseSHA, err := ghClient.CreateBranchFromDefault(ctx, branchName)
	if err != nil {
		logger.Warn("failed to create suggestion branch", "error", err)
		return
	}

	var ghChanges []github.FileChange
	for _, s := range suggestions {
		ghChanges = append(ghChanges, github.FileChange{
			Path:    s.Path,
			Content: s.NewContent,
		})
	}

	_, err = ghClient.CreateCommit(ctx, branchName, baseSHA, "chore: use go-version-file instead of hardcoded go-version", ghChanges)
	if err != nil {
		logger.Warn("failed to create suggestion commit", "error", err)
		return
	}

	var sb strings.Builder
	sb.WriteString("## Use `go-version-file` instead of hardcoded `go-version`\n\n")
	sb.WriteString("This PR replaces hardcoded `go-version` values in GitHub Actions workflows with ")
	sb.WriteString("`go-version-file: 'go.mod'`, which automatically reads the Go version from `go.mod`.\n\n")
	sb.WriteString("This ensures your CI always uses the same Go version specified in your project.\n\n")
	sb.WriteString("### Changed files\n\n")
	for _, s := range suggestions {
		sb.WriteString(fmt.Sprintf("- `%s`\n", s.Path))
	}
	sb.WriteString("\n---\n*This PR was automatically created by the go-update-action.*\n")

	prURL, err := ghClient.CreatePR(ctx, "chore: use go-version-file instead of hardcoded go-version", sb.String(), branchName, []string{"go-version-file"})
	if err != nil {
		logger.Warn("failed to create suggestion PR", "error", err)
		return
	}

	logger.Info("suggestion PR created", "url", prURL)
}

func setOutput(key, value string) {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		return
	}
	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s=%s\n", key, value)
}

func parseNewlineSeparated(raw string) []string {
	var result []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return []string{}
	}
	return result
}
