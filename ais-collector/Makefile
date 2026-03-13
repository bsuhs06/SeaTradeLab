.PHONY: help build run test clean db-up db-down db-migrate install

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install Go dependencies
	go mod download
	go mod tidy

build: ## Build the collector binary
	go build -o bin/collector ./cmd/collector

run: ## Run the collector
	go run ./cmd/collector/main.go

test: ## Run tests
	go test -v ./...

clean: ## Clean build artifacts
	rm -rf bin/
	go clean

db-up: ## Start PostgreSQL with Docker Compose
	docker-compose up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 5

db-down: ## Stop PostgreSQL
	docker-compose down

db-reset: ## Reset database (WARNING: destroys all data — backs up first)
	@echo "⚠️  This will DESTROY all data in the database."
	@printf "Are you sure? [y/N] " && read ans && [ "$$ans" = "y" ] || (echo "Aborted."; exit 1)
	@echo "Backing up before reset..."
	@cd .. && bash scripts/backup.sh || echo "Warning: backup failed, but continuing with reset"
	docker-compose down -v
	docker-compose up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 5

db-logs: ## Show database logs
	docker-compose logs -f postgres

db-shell: ## Open psql shell
	docker-compose exec postgres psql -U aisuser -d ais_data

dev: db-up ## Start development environment
	@echo "Database is running"
	@echo "Run 'make run' to start the collector"

lint: ## Run linter
	golangci-lint run

fmt: ## Format code
	go fmt ./...
	gofmt -s -w .

backup: ## Backup the AIS database
	@cd .. && bash scripts/backup.sh

restore: ## Restore from backup (usage: make restore FILE=backups/ais_data_xxx.sql.gz)
	@cd .. && bash scripts/restore.sh $(FILE)
