# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build        # compile binary to ./bin/govulncheckx
make test         # run all tests: go test ./... -v --failfast
make lint         # run golangci-lint (installs it first via install-golangci-lint)
make install      # build and move binary to $GOPATH/bin

# run a single test
go test ./internal/govulncheck/ -run TestScan -v
```

## Architecture

This is a GitHub Action (Docker-based) that wraps `golang.org/x/vuln/scan` (govulncheck) with support for ignoring known vulnerabilities via a `.govulncheck.yaml` config file. Ignored vulnerabilities have a `silence-until` date; once expired, they resurface as errors.

**Flow:** `main.go` → `cmd/root.go` (cobra CLI) → `govulncheck.Scan()` → `pruneIgnoredVulns()` / `listOutdatedVulns()`.

- `cmd/root.go` — CLI entry point with `--config`, `--path`, `--debug`, and `--edit-config` flags. Orchestrates scanning, output, and configuration updates.
- `internal/configuration/` — Parses the `.govulncheck.yaml` ignore-list (vulnerability ID + silence-until date).
- `internal/govulncheck/scan.go` — Runs govulncheck via `scan.Command` and returns raw JSON. `ScanFunc` type allows test injection.
- `internal/govulncheck/report.go` — Parses govulncheck's streaming JSON output (one JSON object per line, not a single array) into `Report` (findings + OSV entries). Only keeps findings with a function-level trace.
- `internal/govulncheck/vulnerability.go` — Filters detected vulns against the ignore-list, detects outdated ignores, and formats output. Prints copy-pasteable YAML snippets for new ignores.
- `internal/govulncheck/types.go` — Data structures for govulncheck JSON output (`Finding`, `OSV`, `Trace`, `Report`, `Vulnerability`).

**Container runtime:** `Containerfile` builds a multi-stage image from `golang:1.26.0`. `entrypoint.sh` reads the scanned project's `go.mod` to set `GOTOOLCHAIN` so govulncheck uses the project's Go version, not the container's.

Without `--edit-config`, the action exits non-zero when it finds unignored vulnerabilities or outdated ignore entries. With `--edit-config`, it persists the updates and exits successfully.
