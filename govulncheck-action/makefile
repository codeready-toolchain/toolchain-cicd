.DEFAULT_GOAL := help

.PHONY: help
# Based on https:##gist.github.com/rcmachado/af3db315e31383502660
## Display this help text
help:/
	$(info Available targets)
	$(info -----------------)
	@awk '/^[a-zA-Z\-%\_0-9]+:/ { \
		helpMessage = match(lastLine, /^## (.*)/); \
		helpCommand = substr($$1, 0, index($$1, ":")-1); \
		if (helpMessage) { \
			helpMessage = substr(lastLine, RSTART + 3, RLENGTH); \
			gsub(/##/, "\n                                     ", helpMessage); \
			printf "%-35s - %s\n", helpCommand, helpMessage; \
			lastLine = "" \
		} \
	} \
	{ hasComment = match(lastLine, /^## (.*)/); \
          if(hasComment) { \
            lastLine=lastLine$$0; \
	  } \
          else { \
	    lastLine = $$0 \
          } \
        }' $(MAKEFILE_LIST)

# --------------------------------------
# Testing
# --------------------------------------

.PHONY: test
## run all tests excluding fixtures and vendored packages
test:
	@go test ./... -v --failfast

# --------------------------------------
# Linting
# --------------------------------------

.PHONY: lint
## run golangci-lint against project
lint: install-golangci-lint
	@$(shell go env GOPATH)/bin/golangci-lint run -v -c .golangci.yml ./...

.PHONY: install-golangci-lint
## Install development tools.
install-golangci-lint:
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin


# --------------------------------------
# Installing
# --------------------------------------

.PHONY: build
## build the binary and copy into ./bin
build:
	@go build \
	-o bin/govulncheckx main.go 

.PHONY: install
## build the binary and copy into $(GOPATH)/bin
install: build
	@mv bin/govulncheckx ${GOPATH}/bin/govulncheckx
