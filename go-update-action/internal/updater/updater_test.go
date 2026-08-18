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
