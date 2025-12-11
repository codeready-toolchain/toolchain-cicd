#!/bin/bash

# upgrade the go version to match the `toolchain` directive in the `go.mod` file
# see https://go.dev/doc/toolchain
export GOTOOLCHAIN=auto
go mod verify

# Check the version of Go
go version

# Run the govulncheck command
govulncheckx "$@"