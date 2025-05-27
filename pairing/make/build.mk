GO_PACKAGE_ORG_NAME ?= codeready-toolchain
GO_PACKAGE_REPO_NAME ?= pairing
GO_PACKAGE_PATH ?= github.com/${GO_PACKAGE_ORG_NAME}/${GO_PACKAGE_REPO_NAME}
goarch ?= amd64

BIN_DIR = $(OUT_DIR)/bin
.PHONY: build
## Build the operator
build: GO_COMMAND=build
build: GO_EXTRA_FLAGS=-o $(BIN_DIR)/
build: clean-bin run-go

.PHONY: install
## installs the binary executable
install: GO_COMMAND=install
install: run-go

run-go:
	$(Q)CGO_ENABLED=0 \
		env GOOS=linux GOARCH=${goarch} go ${GO_COMMAND} ${V_FLAG} \
		-ldflags "-X ${GO_PACKAGE_PATH}/pkg/version.Commit=${GIT_COMMIT_ID} -X ${GO_PACKAGE_PATH}/pkg/version.BuildTime=${BUILD_TIME}" \
        ${GO_EXTRA_FLAGS} ${GO_PACKAGE_PATH}/...

.PHONY: lint
lint:
ifeq (, $(shell which golangci-lint 2>/dev/null))
	$(error "golangci-lint not found in PATH. Please install it using instructions on https://golangci-lint.run/usage/install/#local-installation")
endif
	golangci-lint ${V_FLAG} run --config=./.golangci.yml --verbose ./...