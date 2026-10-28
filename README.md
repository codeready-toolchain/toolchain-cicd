# CodeReady Toolchain CI/CD

This repository contains GitHub Actions (composite and container-based) used in the GitHub Actions workflows of the CodeReady Toolchain repositories.

It also contains scripts used in the CI setup as well as in the CD processes.

## go-update-action

A container-based (Go) GitHub Action that checks for new Go versions (patch and minor) and opens PRs to update the `toolchain`/`go` directives in `go.mod` files, the `FROM golang:...` lines and Go version `ARG`/`ENV` variables in Dockerfiles/Containerfiles, and hardcoded `go-version:` values in `actions/setup-go` workflow steps. It runs `go mod tidy` with the target Go version and creates commits through the GitHub API, handling existing PRs by skipping or replacing them.

See [go-update-action/README.md](go-update-action/README.md) for inputs, outputs and usage.

## gomod-check

A composite action that verifies that the `go.mod` file does not contain `replace` directives for "protected" modules (by default, modules under `github.com/codeready-toolchain/` and `github.com/kubesaw/`). The list of protected module prefixes can be customized with the `protected-modules` input.

## govulncheck-action

> [!NOTE]
> The `govulncheck-action` is not used anymore (at least for now).

A container-based (Go) GitHub Action that wraps `govulncheck` and allows ignoring known vulnerabilities listed in a `.govulncheck.yaml` file. Each ignored vulnerability has a `silence-until` date, after which it is reported again so that it can be reassessed.

See [govulncheck-action/README.md](govulncheck-action/README.md) for the configuration file format and usage.

## prepare-tools-action

A composite action that installs the tools needed to release a Toolchain operator: `podman`, `operator-sdk` (with checksum signature verification) and `opm`. The versions of `operator-sdk` and of the operator registry (`opm`) can be configured via inputs.

## publish-operators-for-e2e-tests

A composite action that logs into quay.io and publishes the current version of the Toolchain operators (host and member) as single bundles to be used by the e2e tests of a PR, by running the `make publish-current-bundles-for-e2e` target with the PR's details (head SHA, number, author and branch).

## release-operator-action

A composite action that releases a new version of a Toolchain operator: it logs into quay.io, builds and pushes the operator image, generates the CD release manifests and pushes the bundle and index images (via the `podman-push`, `generate-cd-release-manifests` and `push-bundle-and-index-image` make targets).

## scripts

Bash scripts used to maintain and manage the OLM catalog files in the CD process:

- `olm-setup.sh`: common functions used by the other scripts.
- `generate-cd-release-manifests.sh`: regenerates the operator bundle manifests for the latest commit in `master`.
- `push-bundle-and-index-image.sh`: builds the bundle image (for the "staging" channel), adds it to an index image and pushes both to quay.io.

See [scripts/cd/README.adoc](scripts/cd/README.adoc) for more details.
