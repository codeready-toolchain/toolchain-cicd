# go-update-action

A GitHub Action (Docker container, Go-based) that checks for new Go versions and opens PRs to update `go.mod`, Dockerfiles, Containerfiles, and GitHub Actions workflow files.

## Features

- Detects both **patch** (1.26.1 → 1.26.2) and **minor** (1.26 → 1.27) Go updates
- Fetches latest stable versions from the official [Go downloads API](https://go.dev/dl/?mode=json)
- Updates:
  - `go` and `toolchain` directives in all `go.mod` files (see [go.mod updates](#gomod-updates))
  - `FROM golang:X.Y.Z` lines in Dockerfiles/Containerfiles (preserving suffixes like `-alpine` and multi-stage `AS` clauses)
  - `ARG`/`ENV` variables (`GO_VERSION`, `GOLANG_VERSION`, `GO_VER`)
  - Hardcoded `go-version:` in `actions/setup-go` workflow steps
- Runs `go mod tidy` with the target Go version via `GOTOOLCHAIN`
- Creates commits via the GitHub Git Data API (no local git needed)
- Handles duplicate PRs: skips if one already exists, closes-and-replaces if a newer version is available
- Automatically opens a separate suggestion PR to replace hardcoded `go-version:` with `go-version-file: 'go.mod'`
- PR bodies include links to Go release notes and a list of changed files

## go.mod updates

For both patch and minor updates, the `go` directive is set to the minor version (e.g. `go 1.27`) and the `toolchain` directive to the exact version (e.g. `toolchain go1.27.3`). The `toolchain` directive is added if it is missing.

The current version, from which the target version of the patch and minor updates is computed, is read from the `toolchain` directive of the `go-version-file` if present, from its `go` directive otherwise (`go 1.26` stands for `1.26.0`). The version of each `go.mod` file is read the same way, and all the `go.mod` files on a version older than the target version are updated, whatever their minor version.

As a consequence, a `go.mod` file on an older minor version (e.g. `go 1.25.0`) is updated in both the patch update PR and the minor update PR, and the team can decide which Go version they prefer to move to.

Examples with a current version of `1.26.8` (or `1.26.0` for `go 1.26` without `toolchain` directive):

| `go.mod` before                    | Patch update to `1.26.10`          | Minor update to `1.27.3`          |
|------------------------------------|------------------------------------|-----------------------------------|
| `go 1.26.8`                        | `go 1.26` + `toolchain go1.26.10`  | `go 1.27` + `toolchain go1.27.3`  |
| `go 1.26`                          | `go 1.26` + `toolchain go1.26.10`  | `go 1.27` + `toolchain go1.27.3`  |
| `go 1.26.8` + `toolchain go1.26.8` | `go 1.26` + `toolchain go1.26.10`  | `go 1.27` + `toolchain go1.27.3`  |
| `go 1.26.4` + `toolchain go1.26.8` | `go 1.26` + `toolchain go1.26.10`  | `go 1.27` + `toolchain go1.27.3`  |
| `go 1.26` + `toolchain go1.26.8`   | `go 1.26` + `toolchain go1.26.10`  | `go 1.27` + `toolchain go1.27.3`  |
| `go 1.25.4` + `toolchain go1.26.8` | `go 1.26` + `toolchain go1.26.10`  | `go 1.27` + `toolchain go1.27.3`  |
| `go 1.26.4` + `toolchain go1.26.5` (other patch version) | `go 1.26` + `toolchain go1.26.10` | `go 1.27` + `toolchain go1.27.3` |
| `go 1.25.0` (other minor version)  | `go 1.26` + `toolchain go1.26.10`  | `go 1.27` + `toolchain go1.27.3`  |

The `go` directive is not set to the exact version, as `go mod tidy` would then remove the `toolchain` directive, which would be redundant. Note that `go mod tidy` (which runs after the update) raises the `go` directive if a dependency requires a higher version (e.g. `go 1.26.4`).

Patch updates are only available for the Go versions listed by the [Go downloads API](https://go.dev/dl/?mode=json), i.e., the two latest minor versions.

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
