BINARY := query-sniper

.PHONY: help bench build check clean coverage coverage-html fmt lint run semgrep test todos

default: help

bench: ## Run benchmarks
	@go test --race --shuffle=on --bench=. --benchmem ./...

build: ## Build the application
	@go build -trimpath -o $(BINARY) ./cmd/query-sniper

check: fmt lint semgrep test ## Run formatting, vetting, and linting (all via golangci-lint), then semgrep, tests (with race detection), and benchmarks

clean: ## Clean build artifacts and coverage files
	@go clean
	@rm -rf dist coverage $(BINARY)

coverage: ## Generate test coverage report
	@mkdir -p coverage
	@go test --race --shuffle=on --coverprofile=coverage/coverage.out ./...
	@go tool cover --func=coverage/coverage.out

coverage-html: coverage ## Generate HTML coverage report and open in browser
	@go tool cover --html=coverage/coverage.out -o coverage/coverage.html
	@open coverage/coverage.html

docker: clean lint
	@docker build -f build/dev.Dockerfile -t persona-id/query-sniper:latest .

fmt: ## Format code
	@echo "=> Running golangci-lint fmt"
	@golangci-lint fmt ./... > /dev/null 2>&1 || golangci-lint-v2 fmt ./...

help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[32m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

lint: ## Run golangci-lint
	@echo "=> Running golangci-lint run --fix"
	@golangci-lint run --fix ./... || golangci-lint-v2 run --fix ./...
	@echo "=> Running errortype checks"
	@errortype -deep-is-check ./... && echo "0 issues."

# Runs the application in either the local environment (my laptop) or the devcontainer. The devcontainer will have databases
# already setup and running. However, the local environment will have to be setup manually; I'm using the local-proxysql project
# and am running a mysql/proxysql k8s cluster locally.
run: clean build ## Run the application
	@if [ "$$REMOTE_CONTAINERS" = "true" ] || [ "$$CODESPACES" = "true" ] || [ "$$VSCODE_REMOTE_CONTAINERS_SESSION" = "true" ]; then \
		echo "=> Running in remote container"; \
		./$(BINARY) --log.format=text --log.level=debug; \
	else \
		echo "=> Running in local environment"; \
		SNIPER_CONFIG_FILE=configs/kubernetes_config.yaml SNIPER_CREDS_FILE=configs/kubernetes_credentials.yaml ./$(BINARY) --log.format=text --log.level=debug --log.include_caller=true; \
	fi

semgrep: ## Run semgrep
	@echo "=> Running semgrep scan"
	@semgrep scan || true

snapshot: clean lint ## Build a snapshot of the docker image using goreleaser
	@goreleaser --snapshot --clean

test: ## Run tests
	@echo "=> Running go test --race --shuffle=on ./..."
	@go test --race --shuffle=on ./...

todos: ## Find TODO and FIXME comments in the codebase. Runs ripgrep if available, otherwise uses egrep.
	@if command -v rg >/dev/null 2>&1; then \
		rg "(//|#)\s+(TODO|FIXME)" .; \
	else \
		egrep -rn '(//|#)\s+(TODO|FIXME)' .; \
	fi
