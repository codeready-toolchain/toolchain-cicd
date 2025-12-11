goarch?=$(shell go env GOARCH) 

.PHONY: build
build:
	@go version
	mkdir -p $(OUT_DIR)/bin || true
	$(Q)CGO_ENABLED=0 GOARCH=${goarch} GOOS=linux \
		go build ${V_FLAG} ./...

.PHONY: lint
lint:
ifeq (, $(shell which golangci-lint 2>/dev/null))
	$(error "golangci-lint not found in PATH. Please install it using instructions on https://golangci-lint.run/usage/install/#local-installation")
endif
	golangci-lint ${V_FLAG} run --config=./.golangci.yml --verbose ./...