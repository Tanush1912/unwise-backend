.PHONY: run build test migrate-up migrate-down clean install-migrate seed migrate-force migrate-fix-dirty migrate-version migrate-reset

GOPATH := $(shell go env GOPATH)
MIGRATE := $(GOPATH)/bin/migrate

run:
	go run cmd/server/main.go

build:
	go build -o bin/server cmd/server/main.go

test:
	go test -v ./...

install-migrate:
	@echo "Installing migrate tool..."
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

migrate-up: install-migrate
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs); \
	fi; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "Error: DATABASE_URL is not set. Please set it in your .env file or export it."; \
		exit 1; \
	fi; \
	$(MIGRATE) -path migrations -database "$$DATABASE_URL" up

migrate-down: install-migrate
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs); \
	fi; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "Error: DATABASE_URL is not set. Please set it in your .env file or export it."; \
		exit 1; \
	fi; \
	$(MIGRATE) -path migrations -database "$$DATABASE_URL" down

migrate-force: install-migrate
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs); \
	fi; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "Error: DATABASE_URL is not set. Please set it in your .env file or export it."; \
		exit 1; \
	fi; \
	if [ -z "$$VERSION" ]; then \
		echo "Error: VERSION is required."; \
		echo "Usage: make migrate-force VERSION=<version_number>"; \
		echo "Example: make migrate-force VERSION=4"; \
		exit 1; \
	fi; \
	$(MIGRATE) -path migrations -database "$$DATABASE_URL" force $$VERSION

migrate-fix-dirty: install-migrate
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs); \
	fi; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "Error: DATABASE_URL is not set. Please set it in your .env file or export it."; \
		exit 1; \
	fi; \
	@echo "Fixing dirty database state..."; \
	@echo "This will force the version to the current dirty version to mark it as clean."; \
	@echo "If the error says 'Dirty database version X', run: make migrate-force VERSION=X"; \
	@echo ""; \
	@echo "Current migration version:"; \
	$(MIGRATE) -path migrations -database "$$DATABASE_URL" version || true

migrate-version: install-migrate
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs); \
	fi; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "Error: DATABASE_URL is not set. Please set it in your .env file or export it."; \
		exit 1; \
	fi; \
	$(MIGRATE) -path migrations -database "$$DATABASE_URL" version

migrate-reset: install-migrate
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs); \
	fi; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "Error: DATABASE_URL is not set. Please set it in your .env file or export it."; \
		exit 1; \
	fi; \
	@echo "WARNING: This will reset migrations to version 0 and reapply all migrations."; \
	@echo "Press Ctrl+C to cancel, or Enter to continue..."; \
	read confirm; \
	$(MIGRATE) -path migrations -database "$$DATABASE_URL" drop -f; \
	$(MIGRATE) -path migrations -database "$$DATABASE_URL" up

clean:
	rm -rf bin/

seed:
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs); \
	fi; \
	if [ -z "$$DATABASE_URL" ]; then \
		echo "Error: DATABASE_URL is not set. Please set it in your .env file or export it."; \
		exit 1; \
	fi; \
	go run scripts/seed/main.go

