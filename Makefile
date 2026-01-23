# push-validator-cli Makefile

COVERAGE_DIR := .coverage
COVERAGE_PROFILE := $(COVERAGE_DIR)/coverage.out
COVERAGE_HTML := $(COVERAGE_DIR)/coverage.html
PACKAGES := ./cmd/... ./internal/...
MIN_COVERAGE ?= 10

.PHONY: test coverage coverage-html coverage-check coverage-summary clean-coverage build lint help

## test: Run all tests with race detection
test:
	go test -v -race $(PACKAGES)

## coverage: Run tests with coverage and show per-package breakdown
coverage: clean-coverage
	@mkdir -p $(COVERAGE_DIR)
	go test -v -race -covermode=atomic -coverprofile=$(COVERAGE_PROFILE) $(PACKAGES)
	@echo ""
	@echo "=== Coverage Summary ==="
	@go tool cover -func=$(COVERAGE_PROFILE) | grep total:
	@echo ""
	@echo "=== Per-Package Coverage ==="
	@go tool cover -func=$(COVERAGE_PROFILE)
	@echo ""
	@echo "Coverage profile: $(COVERAGE_PROFILE)"

## coverage-html: Generate HTML coverage report and open it
coverage-html: coverage
	go tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	@echo "HTML report: $(COVERAGE_HTML)"
	@command -v open >/dev/null 2>&1 && open $(COVERAGE_HTML) || true

## coverage-check: Verify coverage meets minimum threshold
coverage-check: coverage
	@bash scripts/check-coverage.sh $(MIN_COVERAGE)

## coverage-summary: Print only the total coverage percentage
coverage-summary:
	@mkdir -p $(COVERAGE_DIR)
	@go test -covermode=atomic -coverprofile=$(COVERAGE_PROFILE) $(PACKAGES) > /dev/null 2>&1
	@go tool cover -func=$(COVERAGE_PROFILE) | grep total: | awk '{print $$3}'

## clean-coverage: Remove coverage artifacts
clean-coverage:
	@rm -rf $(COVERAGE_DIR)

## build: Build the push-validator binary
build:
	go build -o push-validator ./cmd/push-validator/

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## help: Show available targets
help:
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
