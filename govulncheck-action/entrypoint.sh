#!/bin/bash

# Read the toolchain directive from the scanned project's go.mod so that govulncheck
# analyses the code against the Go stdlib version the project actually uses, not the
# Go version baked into this container.
# See https://go.dev/doc/toolchain
GOMOD_TOOLCHAIN=$(grep '^toolchain ' go.mod 2>/dev/null | awk '{print $2}')
GOMOD_GO=$(grep '^go ' go.mod 2>/dev/null | awk '{print $2}')
if [ -n "$GOMOD_TOOLCHAIN" ]; then
    export GOTOOLCHAIN="$GOMOD_TOOLCHAIN"
elif [ -n "$GOMOD_GO" ]; then
    export GOTOOLCHAIN="go${GOMOD_GO}"
else
    export GOTOOLCHAIN=auto
fi

go mod verify

# Check the version of Go that will be used for scanning
go version

# Run the govulncheck command
govulncheckx "$@"