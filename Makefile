BINARY := squid-check

.PHONY: help build clean run test

default: help

build: ## Build the application
	@go build ./...

clean: ## Clean build artifacts and coverage files
	@go clean
	@rm -rf $(BINARY) dist

help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[32m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Runs the application in either the local environment (my laptop) or the devcontainer.
run: clean build ## Run the application
	@if [ "$$REMOTE_CONTAINERS" = "true" ] || [ "$$CODESPACES" = "true" ] || [ "$$VSCODE_REMOTE_CONTAINERS_SESSION" = "true" ]; then \
		echo "=> Running in remote container"; \
		./$(BINARY) --proxy-address=squid:3128 --target-address=devcontainer:8080 --log-level=debug; \
	else \
		echo "=> Running in local environment"; \
		./$(BINARY) --log-level=debug; \
	fi

snapshot: clean ## Build a snapshot of the docker image using goreleaser
	@goreleaser release --snapshot --clean

test: ## Run tests
	@echo "=> Running go test --race --shuffle=on ./..."
	@go test -v --race --shuffle=on ./...
