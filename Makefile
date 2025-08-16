LOADTEST_BIN=./build/loadtest
GO_FILES=$(shell find . -name '*.go' -type f -not -path "./vendor/*")
GO_DEPS=go.mod go.sum


###############################################################################
###                                 Tests                                   ###
###############################################################################

test:
	go test ./... -v
.PHONY: test


###############################################################################
###                                 Builds                                  ###
###############################################################################

.PHONY: tidy deps
tidy:
	go mod tidy

deps:
	go env
	go mod download

${LOADTEST_BIN}: ${GO_FILES} ${GO_DEPS}
	@echo "Building load test binary..."
	@mkdir -p ./build
	go build -o ./build/ github.com/skip-mev/catalyst/cmd/loadtest

.PHONY: build
build: ${LOADTEST_BIN}


###############################################################################
###                                Formatting                               ###
###############################################################################

format:
	@find . -name '*.go' -type f -not -path "*.git*" -not -path "*/mocks/*" -not -name '*.pb.go' -not -name '*.pulsar.go' -not -name '*.gw.go' | xargs go run mvdan.cc/gofumpt -w .
	@find . -name '*.go' -type f -not -path "*.git*" -not -path "*/mocks/*" -not -name '*.pb.go' -not -name '*.pulsar.go' -not -name '*.gw.go' | xargs go run github.com/client9/misspell/cmd/misspell -w
	@find . -name '*.go' -type f -not -path "*.git*" -not -path "/*mocks/*" -not -name '*.pb.go' -not -name '*.pulsar.go' -not -name '*.gw.go' | xargs go run golang.org/x/tools/cmd/goimports -w -local github.com/skip-mev/catalyst

.PHONY: format


###############################################################################
###                                Linting                                  ###
###############################################################################

golangci_lint_cmd=golangci-lint
golangci_version=v2.2.2

lint:
	@echo "--> Running linter"
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(golangci_version)
	@$(golangci_lint_cmd) run --timeout=15m

lint-fix:
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(golangci_version)
	@$(golangci_lint_cmd) run --timeout=15m --fix

lint-markdown: tidy
	@echo "--> Running markdown linter"
	@markdownlint **/*.md

govulncheck: tidy
	@echo "--> Running govulncheck"
	@go run golang.org/x/vuln/cmd/govulncheck -test ./...

.PHONY: lint lint-fix lint-markdown govulncheck 