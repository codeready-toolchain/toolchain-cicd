# go-update-action

A GitHub Action (composite, Go-based) that checks for new Go versions and opens PRs to update `go.mod`, Dockerfiles, Containerfiles, and GitHub Actions workflow files.

## Features

- Detects both **patch** (1.26.1 → 1.26.2) and **minor** (1.26 → 1.27) Go updates
- Fetches latest stable versions from the official [Go downloads API](https://go.dev/dl/?mode=json)
- Updates:
  - `go` directive in all `go.mod` files
  - `FROM golang:X.Y.Z` lines in Dockerfiles/Containerfiles (preserving suffixes like `-alpine` and multi-stage `AS` clauses)
  - `ARG`/`ENV` variables (`GO_VERSION`, `GOLANG_VERSION`, `GO_VER`)
  - Hardcoded `go-version:` in `actions/setup-go` workflow steps
- Runs `go mod tidy` with the target Go version via `GOTOOLCHAIN`
- Creates commits via the GitHub Git Data API (no local git needed)
- Handles duplicate PRs: skips if one already exists, closes-and-replaces if a newer version is available
- Automatically opens a separate suggestion PR to replace hardcoded `go-version:` with `go-version-file: 'go.mod'`
- PR bodies include links to Go release notes and a list of changed files

## Inputs

| Input | Required | Default | Description |
|---|---|---|---|
| `type` | Yes | — | Update type: `patch` or `minor` |
| `token` | Yes | — | GitHub token (GitHub App token recommended to trigger downstream workflows) |
| `go-version-file` | No | `go.mod` | Path to `go.mod` for current version detection |
| `paths` | No | `.` | Directories to scan for files to update (newline-separated) |
| `labels` | No | `go-update` | Labels to apply to PRs (newline-separated) |
| `exclude` | No | — | Glob patterns to exclude files from updates (newline-separated) |

## Outputs

| Output | Description |
|---|---|
| `pr-url` | URL of the created PR (empty if skipped) |
| `action-taken` | `created`, `skipped`, or `replaced` |

## Usage

Create a workflow with two separate jobs — one for patch updates, one for minor updates — running on a daily cron schedule:

```yaml
name: Go Version Update

on:
  schedule:
    - cron: '0 8 * * *'  # daily at 8am UTC

jobs:
  go-patch-update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/create-github-app-token@v1
        id: app-token
        with:
          app-id: ${{ secrets.APP_ID }}
          private-key: ${{ secrets.APP_PRIVATE_KEY }}
      - uses: codeready-toolchain/toolchain-cicd/go-update-action@master
        with:
          type: patch
          token: ${{ steps.app-token.outputs.token }}

  go-minor-update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/create-github-app-token@v1
        id: app-token
        with:
          app-id: ${{ secrets.APP_ID }}
          private-key: ${{ secrets.APP_PRIVATE_KEY }}
      - uses: codeready-toolchain/toolchain-cicd/go-update-action@master
        with:
          type: minor
          token: ${{ steps.app-token.outputs.token }}
```

### Why a GitHub App token?

PRs created with the default `GITHUB_TOKEN` do **not** trigger other workflows (e.g., CI on the PR). Using a [GitHub App token](https://github.com/actions/create-github-app-token) ensures your CI pipeline runs on the update PRs.

### Scanning multiple directories

```yaml
- uses: codeready-toolchain/toolchain-cicd/go-update-action@master
  with:
    type: patch
    token: ${{ steps.app-token.outputs.token }}
    paths: |
      .
      tools/
      services/api/
```

### Excluding files

```yaml
- uses: codeready-toolchain/toolchain-cicd/go-update-action@master
  with:
    type: patch
    token: ${{ steps.app-token.outputs.token }}
    exclude: |
      legacy/*
      test/fixtures/*
```

## Architecture

```
go-update-action/
├── action.yml                    # Composite action definition
├── main.go                       # Entry point
├── cmd/
│   └── root.go                   # Cobra CLI — orchestrates the full flow
├── internal/
│   ├── goversion/
│   │   ├── version.go            # Version parsing and comparison
│   │   └── detect.go             # Fetch latest releases from Go API
│   ├── updater/
│   │   ├── updater.go            # File scanning orchestration
│   │   ├── gomod.go              # go.mod updates
│   │   ├── dockerfile.go         # Dockerfile/Containerfile updates
│   │   └── workflow.go           # GitHub Actions workflow updates
│   └── github/
│       └── client.go             # GitHub API client (Git Data API, PRs, labels)
└── Makefile
```

**Flow:** CLI parses flags → reads current version from `go.mod` → fetches latest releases → determines target version → checks for existing PRs → scans and edits files → runs `go mod tidy` → creates commit via Git Data API → opens PR.

## PR conventions

- **Branch names:** `go-update-1.26.2` (version only, no type prefix)
- **PR titles:** `chore: bump Go from 1.26.1 to 1.26.2`
- **Labels:** `go-update` for version update PRs, `go-version-file` for suggestion PRs
- **PR body:** includes release notes link, list of changed files, and warnings if `go mod tidy` failed

## Build & Test

```bash
make build        # compile binary to ./bin/go-update
make test         # run all tests: go test ./... -v --failfast
make lint         # run golangci-lint
make install      # build and move binary to $GOPATH/bin
```
