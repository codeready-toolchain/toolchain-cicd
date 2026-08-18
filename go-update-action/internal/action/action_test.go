package action

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/github"
	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/goversion"
	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/updater"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type MockGitHubClient struct {
	prs              []github.PRInfo
	findPRErr        error
	closedPRs        []int
	closePRErr       error
	deletedBranches  []string
	deleteBranchErr  error
	createdBranch    string
	createBranchSHA  string
	createBranchErr  error
	committedChanges []github.FileChange
	createCommitSHA  string
	createCommitErr  error
	createdPRURL     string
	createPRErr      error
	createdPRLabels  []string
}

var _ GitHubClient = &MockGitHubClient{}

func (m *MockGitHubClient) FindOpenPRByLabel(_ context.Context, _ string) ([]github.PRInfo, error) {
	return m.prs, m.findPRErr
}

func (m *MockGitHubClient) ClosePR(_ context.Context, number int) error {
	m.closedPRs = append(m.closedPRs, number)
	return m.closePRErr
}

func (m *MockGitHubClient) DeleteBranch(_ context.Context, branch string) error {
	m.deletedBranches = append(m.deletedBranches, branch)
	return m.deleteBranchErr
}

func (m *MockGitHubClient) CreateBranchFromDefault(_ context.Context, branchName string) (string, error) {
	m.createdBranch = branchName
	return m.createBranchSHA, m.createBranchErr
}

func (m *MockGitHubClient) CreateCommit(_ context.Context, _, _, _ string, changes []github.FileChange) (string, error) {
	m.committedChanges = changes
	return m.createCommitSHA, m.createCommitErr
}

func (m *MockGitHubClient) CreatePR(_ context.Context, _, _, _ string, labels []string) (string, error) {
	m.createdPRLabels = labels
	return m.createdPRURL, m.createPRErr
}

func mockTidy(dirs *[]string, output []byte, err error) TidyFunc {
	return func(_ context.Context, dir string, _ string) ([]byte, error) {
		*dirs = append(*dirs, dir)
		return output, err
	}
}

// ReadCurrentVersion tests

func TestReadCurrentVersion(t *testing.T) {
	t.Run("valid go.mod with patch version", func(t *testing.T) {
		dir := t.TempDir()
		gomod := filepath.Join(dir, "go.mod")
		require.NoError(t, os.WriteFile(gomod, []byte("module example\n\ngo 1.26.3\n"), 0o644)) //nolint:gosec

		v, err := ReadCurrentVersion(gomod)
		require.NoError(t, err)
		assert.Equal(t, goversion.Version{Major: 1, Minor: 26, Patch: 3}, v)
	})

	t.Run("valid go.mod without patch version", func(t *testing.T) {
		dir := t.TempDir()
		gomod := filepath.Join(dir, "go.mod")
		require.NoError(t, os.WriteFile(gomod, []byte("module example\n\ngo 1.26\n"), 0o644)) //nolint:gosec

		v, err := ReadCurrentVersion(gomod)
		require.NoError(t, err)
		assert.Equal(t, goversion.Version{Major: 1, Minor: 26, Patch: 0}, v)
	})

	t.Run("no go directive", func(t *testing.T) {
		dir := t.TempDir()
		gomod := filepath.Join(dir, "go.mod")
		require.NoError(t, os.WriteFile(gomod, []byte("module example\n"), 0o644)) //nolint:gosec

		_, err := ReadCurrentVersion(gomod)
		assert.ErrorContains(t, err, "no go directive found")
	})

	t.Run("file does not exist", func(t *testing.T) {
		_, err := ReadCurrentVersion(filepath.Join(t.TempDir(), "go.mod"))
		assert.Error(t, err)
	})
}

// FindExistingPRs tests

func TestFindExistingPRs(t *testing.T) {
	ctx := context.Background()
	current := goversion.Version{Major: 1, Minor: 26, Patch: 0}

	t.Run("no existing PRs", func(t *testing.T) {
		mock := &MockGitHubClient{}
		result, err := FindExistingPRs(ctx, mock, "go-update", current, goversion.Version{Major: 1, Minor: 26, Patch: 1}, "patch")
		require.NoError(t, err)
		assert.Equal(t, "created", result.Action)
		assert.Empty(t, result.Outdated)
	})

	t.Run("PR exists for same version", func(t *testing.T) {
		mock := &MockGitHubClient{
			prs: []github.PRInfo{
				{Number: 1, HeadRef: "go-update-1.26.1"},
			},
		}
		result, err := FindExistingPRs(ctx, mock, "go-update", current, goversion.Version{Major: 1, Minor: 26, Patch: 1}, "patch")
		require.NoError(t, err)
		assert.Equal(t, "skipped", result.Action)
	})

	t.Run("patch: identifies older patch PR as outdated", func(t *testing.T) {
		mock := &MockGitHubClient{
			prs: []github.PRInfo{
				{Number: 5, HeadRef: "go-update-1.26.1"},
			},
		}
		target := goversion.Version{Major: 1, Minor: 26, Patch: 2}
		result, err := FindExistingPRs(ctx, mock, "go-update", current, target, "patch")
		require.NoError(t, err)
		assert.Equal(t, "created", result.Action)
		require.Len(t, result.Outdated, 1)
		assert.Equal(t, 5, result.Outdated[0].Number)
		assert.Equal(t, "go-update-1.26.1", result.Outdated[0].Branch)
	})

	t.Run("patch: does not flag PR for different minor", func(t *testing.T) {
		mock := &MockGitHubClient{
			prs: []github.PRInfo{
				{Number: 5, HeadRef: "go-update-1.27.0"},
			},
		}
		target := goversion.Version{Major: 1, Minor: 26, Patch: 2}
		result, err := FindExistingPRs(ctx, mock, "go-update", current, target, "patch")
		require.NoError(t, err)
		assert.Equal(t, "created", result.Action)
		assert.Empty(t, result.Outdated)
	})

	t.Run("minor: identifies older minor PR as outdated", func(t *testing.T) {
		mock := &MockGitHubClient{
			prs: []github.PRInfo{
				{Number: 10, HeadRef: "go-update-1.27.0"},
			},
		}
		target := goversion.Version{Major: 1, Minor: 28, Patch: 0}
		result, err := FindExistingPRs(ctx, mock, "go-update", current, target, "minor")
		require.NoError(t, err)
		assert.Equal(t, "created", result.Action)
		require.Len(t, result.Outdated, 1)
		assert.Equal(t, 10, result.Outdated[0].Number)
	})

	t.Run("ignores PR with non-matching branch name", func(t *testing.T) {
		mock := &MockGitHubClient{
			prs: []github.PRInfo{
				{Number: 1, HeadRef: "some-other-branch"},
			},
		}
		result, err := FindExistingPRs(ctx, mock, "go-update", current, goversion.Version{Major: 1, Minor: 26, Patch: 1}, "patch")
		require.NoError(t, err)
		assert.Equal(t, "created", result.Action)
		assert.Empty(t, result.Outdated)
	})

	t.Run("FindOpenPRByLabel error", func(t *testing.T) {
		mock := &MockGitHubClient{
			findPRErr: fmt.Errorf("API error"),
		}
		_, err := FindExistingPRs(ctx, mock, "go-update", current, goversion.Version{Major: 1, Minor: 26, Patch: 1}, "patch")
		assert.ErrorContains(t, err, "finding existing PRs")
	})
}

// CloseOutdatedPRs tests

func TestCloseOutdatedPRs(t *testing.T) {
	ctx := context.Background()

	t.Run("closes PRs and deletes branches", func(t *testing.T) {
		mock := &MockGitHubClient{}
		outdated := []outdatedPR{
			{Number: 5, Branch: "go-update-1.26.1", Version: goversion.Version{Major: 1, Minor: 26, Patch: 1}},
			{Number: 8, Branch: "go-update-1.26.2", Version: goversion.Version{Major: 1, Minor: 26, Patch: 2}},
		}
		CloseOutdatedPRs(ctx, discardLogger, mock, outdated)
		assert.Equal(t, []int{5, 8}, mock.closedPRs)
		assert.Equal(t, []string{"go-update-1.26.1", "go-update-1.26.2"}, mock.deletedBranches)
	})

	t.Run("no-op with empty list", func(t *testing.T) {
		mock := &MockGitHubClient{}
		CloseOutdatedPRs(ctx, discardLogger, mock, nil)
		assert.Empty(t, mock.closedPRs)
		assert.Empty(t, mock.deletedBranches)
	})
}

// PrepareChangesWithTidy tests

func TestPrepareChangesWithTidy(t *testing.T) {
	ctx := context.Background()
	target := goversion.Version{Major: 1, Minor: 26, Patch: 1}

	t.Run("writes go.mod and reads back after tidy", func(t *testing.T) {
		root := t.TempDir()
		gomodPath := filepath.Join(root, "go.mod")
		require.NoError(t, os.WriteFile(gomodPath, []byte("module example\n\ngo 1.26.0\n"), 0o644)) //nolint:gosec

		changes := []updater.FileChange{
			{
				Path:       "go.mod",
				OldContent: "module example\n\ngo 1.26.0\n",
				NewContent: "module example\n\ngo 1.26.1\n",
			},
		}

		var tidyDirs []string
		ghChanges, warning, err := PrepareChangesWithTidy(ctx, discardLogger, root, changes, target, mockTidy(&tidyDirs, nil, nil))
		require.NoError(t, err)
		assert.Empty(t, warning)
		require.Len(t, ghChanges, 1)
		assert.Equal(t, "go.mod", ghChanges[0].Path)
		assert.Equal(t, "module example\n\ngo 1.26.1\n", ghChanges[0].Content)
		assert.Equal(t, []string{root}, tidyDirs)

		restored, err := os.ReadFile(gomodPath)
		require.NoError(t, err)
		assert.Equal(t, "module example\n\ngo 1.26.0\n", string(restored))
	})

	t.Run("tidy failure produces warning", func(t *testing.T) {
		root := t.TempDir()
		gomodPath := filepath.Join(root, "go.mod")
		require.NoError(t, os.WriteFile(gomodPath, []byte("module example\n\ngo 1.26.0\n"), 0o644)) //nolint:gosec

		changes := []updater.FileChange{
			{
				Path:       "go.mod",
				OldContent: "module example\n\ngo 1.26.0\n",
				NewContent: "module example\n\ngo 1.26.1\n",
			},
		}

		var tidyDirs []string
		ghChanges, warning, err := PrepareChangesWithTidy(ctx, discardLogger, root, changes, target, mockTidy(&tidyDirs, []byte("missing module"), fmt.Errorf("exit status 1")))
		require.NoError(t, err)
		assert.Contains(t, warning, "go mod tidy")
		assert.Contains(t, warning, "missing module")
		require.Len(t, ghChanges, 1)
	})

	t.Run("picks up go.sum after tidy", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example\n\ngo 1.26.0\n"), 0o644)) //nolint:gosec
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.sum"), []byte("some.dep v1.0.0 h1:abc\n"), 0o644))      //nolint:gosec

		changes := []updater.FileChange{
			{
				Path:       "go.mod",
				OldContent: "module example\n\ngo 1.26.0\n",
				NewContent: "module example\n\ngo 1.26.1\n",
			},
		}

		var tidyDirs []string
		ghChanges, _, err := PrepareChangesWithTidy(ctx, discardLogger, root, changes, target, mockTidy(&tidyDirs, nil, nil))
		require.NoError(t, err)
		require.Len(t, ghChanges, 2)
		assert.Equal(t, "go.sum", ghChanges[1].Path)
		assert.Equal(t, "some.dep v1.0.0 h1:abc\n", ghChanges[1].Content)
	})

	t.Run("preserves file permissions", func(t *testing.T) {
		root := t.TempDir()
		gomodPath := filepath.Join(root, "go.mod")
		require.NoError(t, os.WriteFile(gomodPath, []byte("module example\n\ngo 1.26.0\n"), 0o600)) //nolint:gosec

		changes := []updater.FileChange{
			{
				Path:       "go.mod",
				OldContent: "module example\n\ngo 1.26.0\n",
				NewContent: "module example\n\ngo 1.26.1\n",
			},
		}

		var tidyDirs []string
		_, _, err := PrepareChangesWithTidy(ctx, discardLogger, root, changes, target, mockTidy(&tidyDirs, nil, nil))
		require.NoError(t, err)

		info, err := os.Stat(gomodPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("non-gomod changes pass through", func(t *testing.T) {
		root := t.TempDir()
		changes := []updater.FileChange{
			{
				Path:       "Dockerfile",
				OldContent: "FROM golang:1.26.0\n",
				NewContent: "FROM golang:1.26.1\n",
			},
		}

		var tidyDirs []string
		ghChanges, _, err := PrepareChangesWithTidy(ctx, discardLogger, root, changes, target, mockTidy(&tidyDirs, nil, nil))
		require.NoError(t, err)
		require.Len(t, ghChanges, 1)
		assert.Equal(t, "Dockerfile", ghChanges[0].Path)
		assert.Equal(t, "FROM golang:1.26.1\n", ghChanges[0].Content)
		assert.Empty(t, tidyDirs)
	})
}

// FormatPRBody tests

func TestFormatPRBody(t *testing.T) {
	current := goversion.Version{Major: 1, Minor: 26, Patch: 0}
	target := goversion.Version{Major: 1, Minor: 26, Patch: 1}

	t.Run("basic formatting", func(t *testing.T) {
		changes := []updater.FileChange{
			{Path: "go.mod"},
			{Path: "Dockerfile"},
		}
		body := FormatPRBody(current, target, changes, "")
		assert.Contains(t, body, "1.26.0 → 1.26.1")
		assert.Contains(t, body, "release#go1.26")
		assert.Contains(t, body, "- `go.mod`")
		assert.Contains(t, body, "- `Dockerfile`")
		assert.NotContains(t, body, "### Warnings")
	})

	t.Run("with tidy warning", func(t *testing.T) {
		changes := []updater.FileChange{
			{Path: "go.mod"},
		}
		body := FormatPRBody(current, target, changes, "some warning")
		assert.Contains(t, body, "### Warnings")
		assert.Contains(t, body, "some warning")
	})
}

// HandleSuggestionPR tests

func TestHandleSuggestionPR(t *testing.T) {
	ctx := context.Background()

	t.Run("creates suggestion PR", func(t *testing.T) {
		mock := &MockGitHubClient{
			createBranchSHA: "abc123",
			createCommitSHA: "def456",
			createdPRURL:    "https://github.com/owner/repo/pull/1",
		}
		suggestions := []updater.SuggestionChange{
			{Path: ".github/workflows/ci.yml", NewContent: "updated content"},
		}

		HandleSuggestionPR(ctx, discardLogger, mock, suggestions)
		assert.Equal(t, "go-update-use-go-version-file", mock.createdBranch)
		require.Len(t, mock.committedChanges, 1)
		assert.Equal(t, ".github/workflows/ci.yml", mock.committedChanges[0].Path)
		assert.Equal(t, []string{"go-version-file"}, mock.createdPRLabels)
	})

	t.Run("skips if suggestion PR already exists", func(t *testing.T) {
		mock := &MockGitHubClient{
			prs: []github.PRInfo{
				{Number: 1, HeadRef: "go-update-use-go-version-file"},
			},
		}
		suggestions := []updater.SuggestionChange{
			{Path: ".github/workflows/ci.yml", NewContent: "updated content"},
		}

		HandleSuggestionPR(ctx, discardLogger, mock, suggestions)
		assert.Empty(t, mock.createdBranch)
	})
}

// ParseNewlineSeparated tests

func TestParseNewlineSeparated(t *testing.T) {
	t.Run("single value", func(t *testing.T) {
		assert.Equal(t, []string{"foo"}, ParseNewlineSeparated("foo"))
	})

	t.Run("multiple values", func(t *testing.T) {
		assert.Equal(t, []string{"foo", "bar", "baz"}, ParseNewlineSeparated("foo\nbar\nbaz"))
	})

	t.Run("trims whitespace", func(t *testing.T) {
		assert.Equal(t, []string{"foo", "bar"}, ParseNewlineSeparated("  foo  \n  bar  "))
	})

	t.Run("skips empty lines", func(t *testing.T) {
		assert.Equal(t, []string{"foo", "bar"}, ParseNewlineSeparated("foo\n\nbar\n"))
	})

	t.Run("empty input", func(t *testing.T) {
		assert.Equal(t, []string{}, ParseNewlineSeparated(""))
	})
}
