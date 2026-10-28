# go-update-action

A GitHub Action (Docker container, Go-based) that checks for new Go versions and opens PRs to update `go.mod`, Dockerfiles, Containerfiles, and GitHub Actions workflow files.

## Features

- Detects both **patch** (1.26.1 → 1.26.2) and **minor** (1.26 → 1.27) Go updates
- Fetches latest stable versions from the official [Go downloads API](https://go.dev/dl/?mode=json)
- Updates:
  - `toolchain` directive (or `go` directive, if there is no `toolchain` directive) in all `go.mod` files (see [go.mod update strategy](#gomod-update-strategy))
  - `FROM golang:X.Y.Z` lines in Dockerfiles/Containerfiles (preserving suffixes like `-alpine` and multi-stage `AS` clauses)
  - `ARG`/`ENV` variables (`GO_VERSION`, `GOLANG_VERSION`, `GO_VER`)
  - Hardcoded `go-version:` in `actions/setup-go` workflow steps
- Runs `go mod tidy` with the target Go version via `GOTOOLCHAIN`
- Creates commits via the GitHub Git Data API (no local git needed)
- Handles duplicate PRs: skips if one already exists, closes-and-replaces if a newer version is available
- Automatically opens a separate suggestion PR to replace hardcoded `go-version:` with `go-version-file: 'go.mod'`
- PR bodies include links to Go release notes and a list of changed files

## go.mod update strategy

The current version is read from the `toolchain` directive if present, from the `go` directive otherwise (a language version such as `go 1.26` is read as `1.26.0`).
The table below shows how `go.mod` is updated from `1.26.8` to `1.26.9` (patch) or `1.27.3` (minor):

| `toolchain` | Before | After patch update (→ 1.26.9) | After minor update (→ 1.27.3) |
|---|---|---|---|
| present | `go 1.26.8`<br>`toolchain go1.26.8` | `go 1.26.9`<br>`toolchain go1.26.9` | `go 1.26.8` (unchanged)<br>`toolchain go1.27.3` |
| present | `go 1.26.4`<br>`toolchain go1.26.8` | `go 1.26.4` (unchanged)<br>`toolchain go1.26.9` | `go 1.26.4` (unchanged)<br>`toolchain go1.27.3` |
| present | `go 1.26`<br>`toolchain go1.26.8` | `go 1.26` (unchanged)<br>`toolchain go1.26.9` | `go 1.26` (unchanged)<br>`toolchain go1.27.3` |
| missing | `go 1.26.8` | `go 1.26.9` | `go 1.26.8` (unchanged)<br>`toolchain go1.27.3` (added) |
| missing | `go 1.26` | `go 1.26` (unchanged)<br>`toolchain go1.26.9` (added) | `go 1.26` (unchanged)<br>`toolchain go1.27.3` (added) |
| `default` | `go 1.26.8`<br>`toolchain default` | `go 1.26.9`<br>`toolchain default` (unchanged) | no change |
| `default` | `go 1.26`<br>`toolchain default` | no change | no change |

In other words:
- the `go` directive (i.e., the minimum version required by the module and its consumers) is never updated on minor updates, and on patch updates it is only updated if it is a full version and has the same value as the `toolchain` directive, or if there is no `toolchain` directive
- the `toolchain` directive is always set to the target version, and it is added after the `go` directive if it was missing and the `go` directive is not updated
- a `toolchain default` directive (i.e., the module explicitly opts out of toolchain switching) is never changed, and no other `toolchain` directive is added

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
    - cron: '0 6 * * *' # 6 AM UTC

permissions:
  contents: read

jobs:
  go-patch-update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          persist-credentials: false
      - uses: actions/create-github-app-token@v3
        id: app-token
        with:
          client-id: ${{ secrets.GO_VERSION_UPDATER_APP_CLIENT_ID }}
          private-key: ${{ secrets.GO_VERSION_UPDATER_APP_PRIVATE_KEY }}
          permission-contents: write
          permission-pull-requests: write
      - uses: codeready-toolchain/toolchain-cicd/go-update-action@master
        with:
          type: patch
          token: ${{ steps.app-token.outputs.token }}

  go-minor-update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          persist-credentials: false
      - uses: actions/create-github-app-token@v3
        id: app-token
        with:
          client-id: ${{ secrets.GO_VERSION_UPDATER_APP_CLIENT_ID }}
          private-key: ${{ secrets.GO_VERSION_UPDATER_APP_PRIVATE_KEY }}
          permission-contents: write
          permission-pull-requests: write
      - uses: codeready-toolchain/toolchain-cicd/go-update-action@master
        with:
          type: minor
          token: ${{ steps.app-token.outputs.token }}
```

### Runner and version pinning

The action runs in a container, so it requires a Linux runner (e.g. `ubuntu-latest`). There is no need to set up Go in the calling workflow.

Using `@master` runs the image pinned in `action.yml` on the `master` branch, which changes whenever a new image is published.
To control when you pick up a new version, pin the action to a commit SHA instead:

```yaml
- uses: codeready-toolchain/toolchain-cicd/go-update-action@<commit-sha> # master
```

### Why a GitHub App token?

PRs created with the default `GITHUB_TOKEN` do **not** trigger other workflows (e.g., CI on the PR). Using a [GitHub App token](https://github.com/actions/create-github-app-token) ensures your CI pipeline runs on the update PRs.

### Required GitHub App permissions

The GitHub App needs these **repository** permissions:

| Permission | Access | Why |
|---|---|---|
| **Contents** | Read & Write | Create branches and commits via the Git Data API |
| **Pull requests** | Read & Write | Open, close, and list PRs |
| **Metadata** | Read-only | Required by all GitHub Apps (granted automatically) |

Optionally, if you want the action to apply labels to PRs (default: `go-update`):

| Permission | Access | Why |
|---|---|---|
| **Issues** | Read & Write | GitHub's API manages labels through the Issues permission, even on pull requests |

No organization-level permissions are needed.

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
├── action.yml                    # Docker action definition (uses the quay.io image)
├── Containerfile                 # Image published to quay.io/codeready-toolchain/go-update-action
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

## Container image

The action runs the `quay.io/codeready-toolchain/go-update-action` image, built from the `Containerfile`.
On every push to `master` that affects the image, the [publish workflow](../.github/workflows/go-update-action-publish.yml)
pushes an image tagged with the short commit SHA (and `latest`), then opens a PR to pin that tag in `action.yml`.
This way, each commit of the action references a specific image rather than `latest`.

## Build & Test

```bash
make build        # compile binary to ./bin/go-update
make test         # run all tests: go test ./... -v --failfast
make lint         # run golangci-lint
make install      # build and move binary to $GOPATH/bin
```
