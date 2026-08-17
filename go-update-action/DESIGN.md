# Design decisions for go-update-action

This document captures the design discussion that shaped the go-update-action. Each decision is recorded with the alternatives considered and the rationale for the choice made.

## Round 1 — Root decisions

### Q1: Deployment model

**Decision:** Per-repo installation.

Each target repo runs the action in its own workflows using the default `GITHUB_TOKEN` (or a GitHub App token). This follows GitHub Actions conventions and avoids cross-repo token management. The alternative — a central scanner that opens PRs across many repos — would require a PAT or GitHub App with write access to all target repos, making permissions harder to scope.

### Q2: Trigger mechanism

**Decision:** Scheduled cron (daily).

The action runs on a daily schedule, checks the current latest Go version against what's in the repo, and opens a PR if there's a mismatch. This is self-contained with no external infrastructure needed. Go releases are infrequent enough that a daily check is cheap. Alternatives considered: external webhook/dispatch (adds infrastructure) and manual-only (misses updates).

### Q3: Version scope

**Decision:** Both patch and minor updates, as separate PRs.

The action detects both patch updates (1.26.1 → 1.26.2) and minor updates (1.26 → 1.27). These are opened as separate PRs because they serve different purposes: patch updates are typically safe security/bug fixes, while minor updates may require code changes and more review. Minor update PRs target the latest patch of the next minor line (e.g., 1.26.1 → 1.27.2, not 1.27.0).

### Q4: Code location

**Decision:** New directory `go-update-action/` in the existing toolchain-cicd monorepo.

This follows the existing pattern of the monorepo (which already has `govulncheck-action/`, `gomod-check/`, etc.) and shares CI infrastructure.

## Round 2 — Shape of the action

### Q5: Version detection source

**Decision:** Official Go API at `go.dev/dl/?mode=json`.

This is the authoritative source for stable Go releases, requires no authentication, and returns structured JSON with version strings and file metadata. It typically returns the two latest minor release lines (e.g., 1.26.x and 1.25.x). Version parsing (extracting major/minor/patch from strings like `go1.26.6`) is handled by the action since the API does not provide separate version fields.

### Q6: File discovery scope

**Decision:** Scan `go.mod`, `Dockerfile*`, `Containerfile*`, and hardcoded `go-version` in `actions/setup-go` workflow steps.

The action scans broadly but stays focused on Go-specific files. Files like `.go-version` (goenv), `.tool-versions` (asdf), and other tool-specific files are excluded from the initial version to keep it focused — they can be added later.

### Q7: PR strategy

**Decision:** Separate PRs for patch and minor updates.

One invocation per update type via a `--type=patch|minor` flag. The calling workflow runs two separate jobs. This keeps each PR focused and allows different review processes.

### Q8: Duplicate PR handling

**Decision:** Skip if a PR for the same target version exists; close-and-replace if the target version has moved.

When the action runs daily and finds an open PR for "update to Go 1.26.2", it skips. If 1.26.3 is released while the 1.26.2 PR is still open, the action closes the old PR, deletes its branch, and opens a new one with the updated title.

### Q9: go.mod update depth

**Decision:** Edit the `go` directive and run `go mod tidy`.

The action edits the `go` directive in `go.mod`, then runs `go mod tidy` with the target Go version to ensure `go.sum` is consistent. If `go mod tidy` fails, the PR is still created with a warning in the body. Testing is left to the repo's CI pipeline on the resulting PR.

## Round 3 — Implementation details

### Q10: Dockerfile/Containerfile pattern matching

**Decision:** Cover all common patterns.

The action matches:
- `FROM golang:X.Y.Z` lines (preserving suffixes like `-alpine`, `-bookworm` and multi-stage `AS builder` clauses)
- `ARG GO_VERSION=X.Y.Z` and `ENV GO_VERSION=X.Y.Z` (also `GOLANG_VERSION`, `GO_VER`)

### Q11: Setup-go version references

**Decision:** Update hardcoded versions and open a separate suggestion PR.

When the action finds hardcoded `go-version: '1.26.1'` in workflow files, it updates the version in the main PR. It also opens a separate PR (labeled `go-version-file`) suggesting the switch to `go-version-file: 'go.mod'`, which is the better long-term pattern. This keeps the version update PR clean while nudging toward the better approach.

### Q12: Branch naming

**Decision:** `go-update-X.Y.Z` (e.g., `go-update-1.26.2`).

Simple, no type prefix (patch/minor). The PR title already conveys the nature of the update.

### Q13: PR labels

**Decision:** `go-update` label on version update PRs, `go-version-file` on suggestion PRs.

Lightweight, useful for filtering and dashboard queries. Reviewers are not auto-assigned — that's left to CODEOWNERS or other automation.

### Q14: Action inputs

**Decision:** Five inputs with defaults.

| Input | Default | Format |
|---|---|---|
| `type` | (required) | `patch` or `minor` |
| `token` | (required) | GitHub App token |
| `go-version-file` | `go.mod` | path |
| `paths` | `.` | newline-separated |
| `labels` | `go-update` | newline-separated |
| `exclude` | (empty) | newline-separated glob patterns |

### Q15: Go toolchain source

**Decision:** Composite action with `actions/setup-go`.

The action is a composite action (not Docker-based) that uses `actions/setup-go` to install a stable Go version, then runs the Go binary. The binary uses `GOTOOLCHAIN=goX.Y.Z` to let Go auto-download the target version for `go mod tidy`.

## Round 4 — Mechanics

### Q16: `paths` input format

**Decision:** Newline-separated.

This is the convention used by most GitHub Actions (e.g., `paths`, `labels` in actions like `labeler`) and reads cleanly in multiline YAML:
```yaml
paths: |
  .
  tools/
  services/api/
```

### Q17: PR creation mechanism

**Decision:** `google/go-github` library for all GitHub API interactions.

No third-party actions (like `peter-evans/create-pull-request`) are used, for security reasons. The Go binary handles the full lifecycle: finding existing PRs, creating branches, creating commits via the Git Data API, and opening PRs — all through the `go-github` library.

### Q18: go.mod directives

**Decision:** Update the `go` directive only; let `go mod tidy` handle the `toolchain` directive.

Since Go 1.21, `go.mod` has both a `go` directive (minimum version) and an optional `toolchain` directive. Running `go mod tidy` with the new Go version will set the toolchain directive correctly, avoiding the need to encode Go's evolving toolchain logic into the action.

### Q19: PR body content

**Decision:** Changelog-linked.

PR body includes:
- Version bump summary (from → to)
- Link to Go release notes at `go.dev/doc/devel/release`
- List of changed files
- Warning section if `go mod tidy` failed

### Q20: Invocation model

**Decision:** One update per invocation with a `--type` flag.

The binary handles one update type per run. The composite action's calling workflow runs two separate jobs (one for patch, one for minor), giving clean isolation — if one fails, the other still runs.

## Round 5 — Execution flow and auth

### Q21: Authentication

**Decision:** GitHub App token via `actions/create-github-app-token`.

A GitHub App token avoids personal access tokens and ensures that PRs created by the action trigger downstream workflows (CI, linters, etc.). The `actions/create-github-app-token` action is official GitHub infrastructure (under the `actions` org), not third-party.

### Q22: Git operations

**Decision:** Git Data API (fully API-driven).

Commits are created via the GitHub Git Data API (create blobs → create tree → create commit → update ref). This is fully API-driven with no dependency on the `git` CLI. More complex code, but more portable and avoids local git state management.

### Q23: Existing PR detection

**Decision:** Search by label.

The action searches for open PRs with the `go-update` label, then parses the branch name to determine the target version. This is reliable and independent of branch naming conventions or title formatting.

### Q24: Suggestion PR lifecycle

**Decision:** Automatic side-effect with a dedicated label.

When the binary scans workflow files and finds hardcoded `go-version:`, it opens a suggestion PR automatically — but only if no PR with the `go-version-file` label already exists. This prevents duplicates across the patch and minor invocations running in separate jobs.

### Q25: Working tree between invocations

**Decision:** Separate jobs.

Each update type runs in its own job with a fresh checkout. No working tree reset is needed between invocations. If one job fails, the other still runs independently. The ~30s overhead of a second checkout + setup-go is worth the clean isolation.

## Round 6 — Execution model and auth details

### Q26: GitHub App token provisioning

**Decision:** Use `actions/create-github-app-token`.

This official GitHub action handles JWT generation and token exchange. It's battle-tested and maintained by GitHub, saving the effort of writing/maintaining JWT exchange code in the Go binary.

### Q27: Single job vs. separate jobs

**Decision:** Separate jobs.

Confirmed from Q25. Each job is independent: if the minor update has a `go mod tidy` error, the patch update still runs. No shared state to manage.

### Q28: Go version for `go mod tidy`

**Decision:** `GOTOOLCHAIN=goX.Y.Z` environment variable.

The binary sets this env var before running `go mod tidy`. Go's built-in toolchain management (since Go 1.21) automatically downloads and uses the right version. No two-phase composite action needed, no extra setup steps.

### Q29: Suggestion PR label

**Decision:** `go-version-file`.

Specific, self-explanatory, and distinct from `go-update`. Used to detect existing suggestion PRs and prevent duplicates.

### Q30: Action outputs

**Decision:** `pr-url` and `action-taken`.

- `pr-url`: URL of the created PR (empty if skipped)
- `action-taken`: `created`, `skipped`, or `replaced`

Useful for downstream notification steps (e.g., "Slack me when a Go update PR is created") even on cron-triggered runs.

## Round 7 — Edge cases and final details

### Q31: PR title format

**Decision:** `chore: bump Go from 1.26.1 to 1.26.2`.

Follows conventional commits. Including the "from" version makes the PR list scannable at a glance.

### Q32: Multiple go.mod files

**Decision:** Update all `go.mod` files found under configured paths.

If a repo has multiple Go modules (e.g., `./go.mod` and `tools/go.mod`), the action updates all of them for consistency. The `exclude` input handles exceptions.

### Q33: Docker image scope

**Decision:** Only `golang:` official images.

The action only matches `golang:` images (the official Docker Hub image). Custom registry images (e.g., `custom-registry.io/myteam/go-builder:1.26.1`) are not updated. An `additional-images` input can be added later if needed.

### Q34: `go mod tidy` failure handling

**Decision:** Create the PR anyway with a warning in the body.

If `go mod tidy` fails with the target Go version, the PR is still created. The PR body includes a warning section with the error output. This ensures the team is notified about the new Go version even if tidy fails, and the developer can fix the issue manually.

### Q35: Already on latest version

**Decision:** Log and exit 0.

If the repo is already on the latest Go version (or ahead of what the API returns, e.g., using a release candidate), the action logs "already on latest" and exits cleanly. No PR is created, and `action-taken` is set to `skipped`.

### Q36: Version pinning

**Decision:** No `max-version` input for now.

If a repo is constrained to a specific Go version due to a dependency, it should simply not run the action. Adding a `max-version` input later is trivial if the need arises.
