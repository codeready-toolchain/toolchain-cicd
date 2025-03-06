goarch?=$(shell go env GOARCH) 

.PHONY: build
build:
	@go version
	mkdir -p $(OUT_DIR)/bin || true
	$(Q)CGO_ENABLED=0 GOARCH=${goarch} GOOS=linux \
		go build ${V_FLAG} ./...