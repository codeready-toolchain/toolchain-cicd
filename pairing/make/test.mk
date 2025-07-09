.PHONY: test
test: build
	@echo "running the pairing unit tests..."
	go test -v -failfast ./...


# Output directory for coverage information
COV_DIR = $(OUT_DIR)/coverage

.PHONY: test-with-coverage
## runs the tests with coverage
test-with-coverage:
	@echo "running the unit tests with coverage..."
	@-mkdir -p $(COV_DIR)
	@-rm $(COV_DIR)/coverage.txt
	$(Q)go test -vet off ${V_FLAG} $(shell go list ./... | grep -v /cmd/manager) -coverprofile=$(COV_DIR)/coverage.txt -covermode=atomic ./...

