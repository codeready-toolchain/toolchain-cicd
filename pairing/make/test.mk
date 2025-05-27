.PHONY: test
test: build
	@echo "running the pairing unit tests..."
	go test -v -failfast ./...
