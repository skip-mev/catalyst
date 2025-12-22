golangci_lint_cmd=golangci-lint
golangci_version=v2.6.2

GO_BIN_DIR = $(shell go env GOPATH)/bin

help: ## List of commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

test: ## Run tests
	go test ./... -v

build: ## Build the binary
	@echo "Building load test binary..."
	@mkdir -p ./build
	go build -o ./build/ ./cmd/catalyst/...

build-docker: ## Build local docker image
	docker build -t catalyst .

install: ## Install the binary
	@echo "Installing binary to $(GO_BIN_DIR)/catalyst"
	go install ./cmd/catalyst/...

fmt: ## Format the code TODO use golangci-lint for formatting
	@find . -name '*.go' -type f -not -path "*.git*" -not -path "*/mocks/*" -not -name '*.pb.go' -not -name '*.pulsar.go' -not -name '*.gw.go' | xargs go run mvdan.cc/gofumpt -w .
	@find . -name '*.go' -type f -not -path "*.git*" -not -path "*/mocks/*" -not -name '*.pb.go' -not -name '*.pulsar.go' -not -name '*.gw.go' | xargs go run github.com/client9/misspell/cmd/misspell -w
	@find . -name '*.go' -type f -not -path "*.git*" -not -path "/*mocks/*" -not -name '*.pb.go' -not -name '*.pulsar.go' -not -name '*.gw.go' | xargs go run golang.org/x/tools/cmd/goimports -w -local github.com/skip-mev/catalyst

lint: ## Run the linter
	@echo "--> Running linter"
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(golangci_version)
	@$(golangci_lint_cmd) run --timeout=15m

# lint-fix: ## Run the linter and fix the code
# @go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(golangci_version)
# @$(golangci_lint_cmd) run --timeout=15m --fix

lint-markdown: ## Lint markdown files
	@echo "--> Running markdown linter"
	@markdownlint **/*.md

govulncheck: ## Run govulncheck
	@echo "--> Running govulncheck"
	@go run golang.org/x/vuln/cmd/govulncheck -test ./...


.PHONY: help test build build-docker install fmt lint lint-markdown govulncheck