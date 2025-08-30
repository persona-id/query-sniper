BINARY := query-sniper

.PHONY: help bench build check clean coverage coverage-html fmt lint run test

default: help

bench: ## Run benchmarks
	@go test --race --shuffle=on --bench=. --benchmem ./...

build: ## Build the application
	@go build -trimpath -o $(BINARY) ./cmd/query-sniper

check: fmt lint test ## Run formatting (via golangci-lint), vetting (also via golangci-lint), linting, tests, benchmarks, and race detection

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
	@golangci-lint-v2 fmt ./...

help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

lint: ## Run golangci-lint
	@golangci-lint-v2 run --fix

# Runs the application in either the local environment (my laptop) or the devcontainer. The devcontainer will have databases
# already setup and running. However, the local environment will have to be setup manually; I'm using the local-proxysql project
# and am running a mysql/proxysql k8s cluster locally.
run: clean build ## Run the application
	@if [ "$$REMOTE_CONTAINERS" = "true" ] || [ "$$CODESPACES" = "true" ] || [ "$$VSCODE_REMOTE_CONTAINERS_SESSION" = "true" ]; then \
		echo "Running in remote container..."; \
		./$(BINARY) --log.format=text --log.level=debug; \
	else \
		echo "Running in local environment..."; \
		SNIPER_CONFIG_FILE=configs/kubernetes_config.yaml SNIPER_CREDS_FILE=configs/kubernetes_credentials.yaml ./$(BINARY) --log.format=text --log.level=debug --log.include_caller=true; \
	fi

snapshot: clean lint ## Build a snapshot of the docker image using goreleaser
	@goreleaser --snapshot --clean

test: ## Run tests
	@go test --race --shuffle=on ./...
