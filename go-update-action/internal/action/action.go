package action

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
)

type GitHubClient interface {
	FindOpenPRByLabel(ctx context.Context, label string) ([]github.PRInfo, error)
	ClosePR(ctx context.Context, number int) error
	DeleteBranch(ctx context.Context, branch string) error
	CreateBranchFromDefault(ctx context.Context, branchName string) (string, error)
	CreateCommit(ctx context.Context, branch, baseSHA, message string, changes []github.FileChange) (string, error)
	CreatePR(ctx context.Context, logger *slog.Logger, title, body, branch string, labels []string) (string, error)
}

type TidyFunc func(ctx context.Context, dir string, goVersion string) ([]byte, error)

func ExecGoModTidy(ctx context.Context, dir string, goVersion string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOTOOLCHAIN=go%s", goVersion))
	return cmd.CombinedOutput()
}

type Config struct {
	UpdateType    string
	Paths         []string
	Labels        []string
	Excludes      []string
	Root          string
	GoVersionFile string
}

type Result struct {
	ActionTaken string
	PRURL       string
}

var goDirectiveRe = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+(?:\.\d+)?)\s*$`)

func Run(ctx context.Context, logger *slog.Logger, cfg Config, ghClient GitHubClient, tidy TidyFunc) (*Result, error) {
	current, err := ReadCurrentVersion(cfg.GoVersionFile)
	if err != nil {
		return nil, fmt.Errorf("reading current Go version: %w", err)
	}
	logger.Info("current Go version", "version", current.String())

	releases, err := goversion.FetchLatestReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching Go releases: %w", err)
	}
	logger.Debug("fetched releases", "count", len(releases))

	var target *goversion.Version
	switch cfg.UpdateType {
	case "patch":
		target = goversion.FindPatchUpdate(current, releases)
	case "minor":
		target = goversion.FindMinorUpdate(current, releases)
	default:
		return nil, fmt.Errorf("invalid update type %q: must be 'patch' or 'minor'", cfg.UpdateType)
	}

	if target == nil {
		logger.Info("already on latest", "type", cfg.UpdateType, "current", current.String())
		return &Result{ActionTaken: "skipped"}, nil
	}
	logger.Info("update available", "type", cfg.UpdateType, "from", current.String(), "to", target.String())

	branchName := fmt.Sprintf("go-update-%s", target.String())
	prTitle := fmt.Sprintf("chore: bump Go from %s to %s", current.String(), target.String())

	existing, err := FindExistingPRs(ctx, ghClient, cfg.Labels[0], current, *target, cfg.UpdateType)
	if err != nil {
		return nil, err
	}
	if existing.Action == "skipped" {
		logger.Info("PR already exists for target version", "version", target.String())
		return &Result{ActionTaken: "skipped"}, nil
	}

	scanResult, err := updater.ScanAndUpdate(cfg.Root, current.String(), target.String(), cfg.Paths, cfg.Excludes)
	if err != nil {
		return nil, fmt.Errorf("scanning files: %w", err)
	}

	if len(scanResult.Changes) == 0 {
		logger.Info("no files need updating")
		return &Result{ActionTaken: "skipped"}, nil
	}

	logger.Info("files to update", "count", len(scanResult.Changes))
	for _, c := range scanResult.Changes {
		logger.Debug("file changed", "path", c.Path)
	}

	ghChanges, tidyWarning, err := PrepareChangesWithTidy(ctx, logger, cfg.Root, scanResult.Changes, *target, tidy)
	if err != nil {
		return nil, err
	}

	baseSHA, err := ghClient.CreateBranchFromDefault(ctx, branchName)
	if err != nil {
		return nil, fmt.Errorf("creating branch: %w", err)
	}

	_, err = ghClient.CreateCommit(ctx, branchName, baseSHA, prTitle, ghChanges)
	if err != nil {
		return nil, fmt.Errorf("creating commit: %w", err)
	}

	prBody := FormatPRBody(current, *target, scanResult.Changes, tidyWarning)
	prURL, err := ghClient.CreatePR(ctx, logger, prTitle, prBody, branchName, cfg.Labels)
	if err != nil {
		return nil, fmt.Errorf("creating PR: %w", err)
	}

	logger.Info("PR created", "url", prURL)

	CloseOutdatedPRs(ctx, logger, ghClient, existing.Outdated)

	finalAction := "created"
	if len(existing.Outdated) > 0 {
		finalAction = "replaced"
	}

	if len(scanResult.Suggestions) > 0 {
		HandleSuggestionPR(ctx, logger, ghClient, scanResult.Suggestions)
	}

	return &Result{ActionTaken: finalAction, PRURL: prURL}, nil
}

func ReadCurrentVersion(goModPath string) (goversion.Version, error) {
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

type outdatedPR struct {
	Number  int
	Branch  string
	Version goversion.Version
}

type existingPRCheck struct {
	Action   string // "created" or "skipped"
	Outdated []outdatedPR
}

func FindExistingPRs(ctx context.Context, ghClient GitHubClient, label string, current, target goversion.Version, updateType string) (*existingPRCheck, error) {
	existingPRs, err := ghClient.FindOpenPRByLabel(ctx, label)
	if err != nil {
		return nil, fmt.Errorf("finding existing PRs: %w", err)
	}

	branchVersionRe := regexp.MustCompile(`^go-update-(\d+\.\d+\.\d+)$`)
	result := &existingPRCheck{Action: "created"}

	for _, pr := range existingPRs {
		matches := branchVersionRe.FindStringSubmatch(pr.HeadRef)
		if matches == nil {
			continue
		}
		prVersion, err := goversion.Parse(matches[1])
		if err != nil {
			continue
		}

		if prVersion.Compare(target) == 0 {
			return &existingPRCheck{Action: "skipped"}, nil
		}

		shouldClose := false
		switch updateType {
		case "patch":
			shouldClose = prVersion.SameMinor(current) && prVersion.Compare(target) < 0
		case "minor":
			shouldClose = prVersion.Minor > current.Minor && prVersion.Compare(target) < 0
		}

		if shouldClose {
			result.Outdated = append(result.Outdated, outdatedPR{
				Number:  pr.Number,
				Branch:  fmt.Sprintf("go-update-%s", prVersion.String()),
				Version: prVersion,
			})
		}
	}

	return result, nil
}

func CloseOutdatedPRs(ctx context.Context, logger *slog.Logger, ghClient GitHubClient, outdated []outdatedPR) {
	for _, pr := range outdated {
		logger.Info("closing outdated PR", "number", pr.Number, "version", pr.Version.String())
		if err := ghClient.ClosePR(ctx, pr.Number); err != nil {
			logger.Warn("failed to close PR", "number", pr.Number, "error", err)
		}
		if err := ghClient.DeleteBranch(ctx, pr.Branch); err != nil {
			logger.Debug("failed to delete old branch", "branch", pr.Branch, "error", err)
		}
	}
}

func PrepareChangesWithTidy(ctx context.Context, logger *slog.Logger, root string, changes []updater.FileChange, target goversion.Version, tidy TidyFunc) ([]github.FileChange, string, error) {
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
				perm := os.FileMode(0o644)
				if info, err := os.Stat(gomodPath); err == nil {
					perm = info.Mode().Perm()
				}
				if err := os.WriteFile(gomodPath, []byte(c.NewContent), perm); err != nil {
					return nil, "", fmt.Errorf("writing go.mod for tidy: %w", err)
				}
				break
			}
		}

		output, err := tidy(ctx, filepath.Join(root, dir), target.String())
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
				perm := os.FileMode(0o644)
				if info, err := os.Stat(gomodPath); err == nil {
					perm = info.Mode().Perm()
				}
				_ = os.WriteFile(gomodPath, []byte(c.OldContent), perm)
				break
			}
		}
	}

	return ghChanges, tidyWarning, nil
}

func FormatPRBody(current goversion.Version, target goversion.Version, changes []updater.FileChange, tidyWarning string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "## Go version update: %s → %s\n\n", current.String(), target.String())

	releaseURL := fmt.Sprintf("https://go.dev/doc/devel/release#go%s", target.MinorString())
	fmt.Fprintf(&sb, "[Release notes](%s)\n\n", releaseURL)

	fmt.Fprintf(&sb, "### Changed files\n\n")
	for _, c := range changes {
		fmt.Fprintf(&sb, "- `%s`\n", c.Path)
	}

	if tidyWarning != "" {
		fmt.Fprintf(&sb, "\n### Warnings\n\n%s\n", tidyWarning)
	}

	fmt.Fprintf(&sb, "\n---\n*This PR was automatically created by the go-update-action.*\n")
	return sb.String()
}

func HandleSuggestionPR(ctx context.Context, logger *slog.Logger, ghClient GitHubClient, suggestions []updater.SuggestionChange) {
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
		fmt.Fprintf(&sb, "- `%s`\n", s.Path)
	}
	sb.WriteString("\n---\n*This PR was automatically created by the go-update-action.*\n")

	prURL, err := ghClient.CreatePR(ctx, logger, "chore: use go-version-file instead of hardcoded go-version", sb.String(), branchName, []string{"go-version-file"})
	if err != nil {
		logger.Warn("failed to create suggestion PR", "error", err)
		return
	}

	logger.Info("suggestion PR created", "url", prURL)
}

func ParseNewlineSeparated(raw string) []string {
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
