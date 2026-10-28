package updater_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/codeready-toolchain/toolchain-cicd/go-update-action/internal/updater"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	perm := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	require.NoError(t, os.WriteFile(path, []byte(content), perm))
}

func TestGoModUpdate(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", `module example.com/foo

go 1.26.1

require (
	github.com/some/pkg v1.0.0
)
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	require.Len(t, result.Changes, 1)

	assert.Equal(t, "go.mod", result.Changes[0].Path)
	assert.Contains(t, result.Changes[0].NewContent, "go 1.26.2")
	assert.Contains(t, result.Changes[0].NewContent, "github.com/some/pkg v1.0.0")
	assert.NotContains(t, result.Changes[0].NewContent, "go 1.26.1")
}

func TestGoModNoChange(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", `module example.com/foo

go 1.25.0
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	assert.Empty(t, result.Changes)
}

func TestGoModToolchain(t *testing.T) {
	t.Run("toolchain directive is updated, language version is kept", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "go.mod", "module example.com/foo\n\ngo 1.26\n\ntoolchain go1.26.5\n")

		result, err := updater.ScanAndUpdate(root, "1.26.5", "1.26.10", []string{"."}, nil)
		require.NoError(t, err)
		require.Len(t, result.Changes, 1)
		assert.Equal(t, "module example.com/foo\n\ngo 1.26\n\ntoolchain go1.26.10\n", result.Changes[0].NewContent)
	})

	t.Run("go and toolchain directives with same version are both updated", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "go.mod", "module example.com/foo\n\ngo 1.26.5\n\ntoolchain go1.26.5\n")

		result, err := updater.ScanAndUpdate(root, "1.26.5", "1.26.10", []string{"."}, nil)
		require.NoError(t, err)
		require.Len(t, result.Changes, 1)
		assert.Equal(t, "module example.com/foo\n\ngo 1.26.10\n\ntoolchain go1.26.10\n", result.Changes[0].NewContent)
	})

	t.Run("go directive lower than toolchain is kept", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "go.mod", "module example.com/foo\n\ngo 1.26.1\n\ntoolchain go1.26.5\n")

		result, err := updater.ScanAndUpdate(root, "1.26.5", "1.26.10", []string{"."}, nil)
		require.NoError(t, err)
		require.Len(t, result.Changes, 1)
		assert.Equal(t, "module example.com/foo\n\ngo 1.26.1\n\ntoolchain go1.26.10\n", result.Changes[0].NewContent)
	})

	t.Run("toolchain directive is added after language version", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "go.mod", "module example.com/foo\n\ngo 1.26\n\nrequire github.com/some/pkg v1.0.0\n")

		result, err := updater.ScanAndUpdate(root, "1.26.0", "1.26.10", []string{"."}, nil)
		require.NoError(t, err)
		require.Len(t, result.Changes, 1)
		assert.Equal(t, "module example.com/foo\n\ngo 1.26\n\ntoolchain go1.26.10\n\nrequire github.com/some/pkg v1.0.0\n", result.Changes[0].NewContent)
	})

	t.Run("go directive is not updated when toolchain directive has other version", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "go.mod", "module example.com/foo\n\ngo 1.26.5\n\ntoolchain go1.26.8\n")

		result, err := updater.ScanAndUpdate(root, "1.26.5", "1.26.10", []string{"."}, nil)
		require.NoError(t, err)
		assert.Empty(t, result.Changes)
	})

	t.Run("toolchain directive with other version is not touched", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "go.mod", "module example.com/foo\n\ngo 1.26\n\ntoolchain go1.25.3\n")

		result, err := updater.ScanAndUpdate(root, "1.26.0", "1.26.10", []string{"."}, nil)
		require.NoError(t, err)
		assert.Empty(t, result.Changes)
	})

	t.Run("minor upgrade", func(t *testing.T) {
		for name, tc := range map[string]struct {
			from, content, expected string
		}{
			"go directive lower than toolchain is kept": {
				from:     "1.26.8",
				content:  "module example.com/foo\n\ngo 1.26.4\n\ntoolchain go1.26.8\n",
				expected: "module example.com/foo\n\ngo 1.26.4\n\ntoolchain go1.27.3\n",
			},
			"go directive with older minor version than toolchain is kept": {
				from:     "1.26.8",
				content:  "module example.com/foo\n\ngo 1.25.4\n\ntoolchain go1.26.8\n",
				expected: "module example.com/foo\n\ngo 1.25.4\n\ntoolchain go1.27.3\n",
			},
			"go directive with same version as toolchain is kept": {
				from:     "1.26.8",
				content:  "module example.com/foo\n\ngo 1.26.8\n\ntoolchain go1.26.8\n",
				expected: "module example.com/foo\n\ngo 1.26.8\n\ntoolchain go1.27.3\n",
			},
			"language version is kept when toolchain directive is updated": {
				from:     "1.26.8",
				content:  "module example.com/foo\n\ngo 1.26\n\ntoolchain go1.26.8\n",
				expected: "module example.com/foo\n\ngo 1.26\n\ntoolchain go1.27.3\n",
			},
			"language version is kept when toolchain directive is added": {
				from:     "1.26.0",
				content:  "module example.com/foo\n\ngo 1.26\n",
				expected: "module example.com/foo\n\ngo 1.26\n\ntoolchain go1.27.3\n",
			},
			"go directive is kept when toolchain directive is added": {
				from:     "1.26.8",
				content:  "module example.com/foo\n\ngo 1.26.8\n",
				expected: "module example.com/foo\n\ngo 1.26.8\n\ntoolchain go1.27.3\n",
			},
			"toolchain directive is updated with CRLF line endings": {
				from:     "1.26.8",
				content:  "module example.com/foo\r\n\r\ngo 1.26\r\n\r\ntoolchain go1.26.8\r\n",
				expected: "module example.com/foo\r\n\r\ngo 1.26\r\n\r\ntoolchain go1.27.3\r\n",
			},
			"toolchain directive is added after full version with CRLF line endings": {
				from:     "1.26.8",
				content:  "module example.com/foo\r\n\r\ngo 1.26.8\r\n",
				expected: "module example.com/foo\r\n\r\ngo 1.26.8\r\n\r\ntoolchain go1.27.3\r\n",
			},
			"toolchain directive is added with CRLF line endings": {
				from:     "1.26.0",
				content:  "module example.com/foo\r\n\r\ngo 1.26\r\n",
				expected: "module example.com/foo\r\n\r\ngo 1.26\r\n\r\ntoolchain go1.27.3\r\n",
			},
		} {
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, root, "go.mod", tc.content)

				result, err := updater.ScanAndUpdate(root, tc.from, "1.27.3", []string{"."}, nil)
				require.NoError(t, err)
				require.Len(t, result.Changes, 1)
				assert.Equal(t, tc.expected, result.Changes[0].NewContent)
			})
		}
	})

	t.Run("minor upgrade does not touch go directive when toolchain directive has other version", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "go.mod", "module example.com/foo\n\ngo 1.26.4\n\ntoolchain go1.26.5\n")

		result, err := updater.ScanAndUpdate(root, "1.26.8", "1.27.3", []string{"."}, nil)
		require.NoError(t, err)
		assert.Empty(t, result.Changes)
	})
}

func TestGoModCRLF(t *testing.T) {
	for name, tc := range map[string]struct {
		from, content, expected string
	}{
		"go directive is updated": {
			from:     "1.26.5",
			content:  "module example.com/foo\r\n\r\ngo 1.26.5\r\n",
			expected: "module example.com/foo\r\n\r\ngo 1.26.10\r\n",
		},
		"toolchain directive is updated": {
			from:     "1.26.5",
			content:  "module example.com/foo\r\n\r\ngo 1.26\r\n\r\ntoolchain go1.26.5\r\n",
			expected: "module example.com/foo\r\n\r\ngo 1.26\r\n\r\ntoolchain go1.26.10\r\n",
		},
		"toolchain directive is added": {
			from:     "1.26.0",
			content:  "module example.com/foo\r\n\r\ngo 1.26\r\n",
			expected: "module example.com/foo\r\n\r\ngo 1.26\r\n\r\ntoolchain go1.26.10\r\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "go.mod", tc.content)

			result, err := updater.ScanAndUpdate(root, tc.from, "1.26.10", []string{"."}, nil)
			require.NoError(t, err)
			require.Len(t, result.Changes, 1)
			assert.Equal(t, tc.expected, result.Changes[0].NewContent)
		})
	}
}

func TestGoModMultiple(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", `module example.com/foo

go 1.26.1
`)
	writeFile(t, root, "tools/go.mod", `module example.com/foo/tools

go 1.26.1
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{".", "tools"}, nil)
	require.NoError(t, err)
	require.Len(t, result.Changes, 2)
}

func TestDockerfileFromGolang(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Dockerfile", `FROM golang:1.26.1-alpine AS builder
RUN go build -o /app .
FROM alpine:3.18
COPY --from=builder /app /app
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	require.Len(t, result.Changes, 1)

	assert.Equal(t, "Dockerfile", result.Changes[0].Path)
	assert.Contains(t, result.Changes[0].NewContent, "FROM golang:1.26.2-alpine AS builder")
	assert.Contains(t, result.Changes[0].NewContent, "FROM alpine:3.18")
}

func TestDockerfileMultiStage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Containerfile", `FROM golang:1.26.1 AS build
RUN go build .
FROM golang:1.26.1-bookworm AS test
RUN go test .
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	require.Len(t, result.Changes, 1)

	assert.Contains(t, result.Changes[0].NewContent, "FROM golang:1.26.2 AS build")
	assert.Contains(t, result.Changes[0].NewContent, "FROM golang:1.26.2-bookworm AS test")
}

func TestDockerfileArgEnv(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Dockerfile.ci", `ARG GO_VERSION=1.26.1
FROM golang:${GO_VERSION}
ENV GOLANG_VERSION=1.26.1
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	require.Len(t, result.Changes, 1)

	assert.Contains(t, result.Changes[0].NewContent, "ARG GO_VERSION=1.26.2")
	assert.Contains(t, result.Changes[0].NewContent, "ENV GOLANG_VERSION=1.26.2")
}

func TestDockerfileNoMatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Dockerfile", `FROM node:18
RUN npm install
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	assert.Empty(t, result.Changes)
}

func TestWorkflowUpdate(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".github/workflows/ci.yml", `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26.1'
      - run: go test ./...
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	require.Len(t, result.Changes, 1)

	assert.Equal(t, filepath.Join(".github", "workflows", "ci.yml"), result.Changes[0].Path)
	assert.Contains(t, result.Changes[0].NewContent, "go-version: '1.26.2'")
}

func TestWorkflowUnquotedVersion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".github/workflows/ci.yaml", `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@v5
        with:
          go-version: 1.26.1
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	require.Len(t, result.Changes, 1)

	assert.Contains(t, result.Changes[0].NewContent, "go-version: 1.26.2")
}

func TestWorkflowSuggestion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".github/workflows/ci.yml", `name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26.1'
      - run: go test ./...
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	require.Len(t, result.Suggestions, 1)

	assert.Equal(t, filepath.Join(".github", "workflows", "ci.yml"), result.Suggestions[0].Path)
	assert.Contains(t, result.Suggestions[0].NewContent, "go-version-file: 'go.mod'")
	assert.NotContains(t, result.Suggestions[0].NewContent, "go-version:")
}

func TestWorkflowVariableNotTouched(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".github/workflows/ci.yml", `name: CI
on: push
jobs:
  test:
    strategy:
      matrix:
        go-version: ['1.25', '1.26']
    steps:
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go-version }}
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	assert.Empty(t, result.Changes)
	assert.Empty(t, result.Suggestions)
}

func TestExcludePattern(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", `module example.com/foo

go 1.26.1
`)
	writeFile(t, root, "Dockerfile", `FROM golang:1.26.1
`)
	writeFile(t, root, "legacy/Dockerfile", `FROM golang:1.26.1
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, []string{"legacy/*"})
	require.NoError(t, err)
	require.Len(t, result.Changes, 2)

	paths := make([]string, len(result.Changes))
	for i, c := range result.Changes {
		paths[i] = c.Path
	}
	assert.Contains(t, paths, "go.mod")
	assert.Contains(t, paths, "Dockerfile")
	assert.NotContains(t, paths, filepath.Join("legacy", "Dockerfile"))
}

func TestExcludeNestedPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/foo\n\ngo 1.26.1\n")
	writeFile(t, root, "legacy/sub/Dockerfile", "FROM golang:1.26.1\n")

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, []string{"legacy/*"})
	require.NoError(t, err)
	require.Len(t, result.Changes, 1)
	assert.Equal(t, "go.mod", result.Changes[0].Path)
}

func TestExcludeByBasename(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/foo\n\ngo 1.26.1\n")
	writeFile(t, root, "sub/Dockerfile.dev", "FROM golang:1.26.1\n")

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, []string{"Dockerfile.*"})
	require.NoError(t, err)
	require.Len(t, result.Changes, 1)
	assert.Equal(t, "go.mod", result.Changes[0].Path)
}

func TestExcludeDeepNestedFixtures(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/foo\n\ngo 1.26.1\n")
	writeFile(t, root, "test/fixtures/a/b/go.mod", "module fixtures\n\ngo 1.26.1\n")

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, []string{"test/fixtures/*"})
	require.NoError(t, err)
	require.Len(t, result.Changes, 1)
	assert.Equal(t, "go.mod", result.Changes[0].Path)
}

func TestScanPathOutsideRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/foo\n\ngo 1.26.1\n")

	_, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"../../etc"}, []string{"*.txt"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "escapes root directory")
}

func TestUnrelatedFilesNotTouched(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", `# Project
Uses Go 1.26.1
`)
	writeFile(t, root, "main.go", `package main

func main() {}
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	assert.Empty(t, result.Changes)
}

func TestContainerfile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "Containerfile.build", `FROM golang:1.26.1 AS builder
RUN go build .
`)

	result, err := updater.ScanAndUpdate(root, "1.26.1", "1.26.2", []string{"."}, nil)
	require.NoError(t, err)
	require.Len(t, result.Changes, 1)

	assert.Equal(t, "Containerfile.build", result.Changes[0].Path)
	assert.Contains(t, result.Changes[0].NewContent, "FROM golang:1.26.2 AS builder")
}
