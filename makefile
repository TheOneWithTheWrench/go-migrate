.PHONY: local-up local-down

local-up: ## Start the test database using Docker Compose
	@echo "Starting test database via Docker Compose..."
	docker-compose up -d --wait # -d runs in detached mode, --wait uses the healthcheck

local-down: ## Stop and remove the test database using Docker Compose
	@echo "Stopping test database via Docker Compose..."
	docker-compose down 

help: 
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
