.PHONY: test
## runs the tests (use `test-in-container` on macOS)
test: build
	@echo "running the pairing unit tests..."
	go test -v -failfast ./...