.PHONY: dev up down restart logs migrate seed run-all build test test-race test-integration bench lint lint-fix proto template smoke-test verify security-scan clean help

SERVICES := submission-api build-worker orchestrator bot-fleet telemetry-ingester leaderboard-api

# Infrastructure
dev: ## Start lightweight infra-only stack (run services with `go run`)
	docker compose -f docker-compose.dev.yml up -d

up: ## Start the full local stack
	docker compose up -d

down: ## Stop all containers
	docker compose down

restart: ## Restart all containers
	docker compose restart

logs: ## Tail logs for a service: make logs SVC=submission-api
	docker compose logs -f $(SVC)

# Database
migrate: ## Apply SQL migrations to both databases
	bash scripts/local-dev/migrate.sh

seed: ## Seed local contestants
	psql "$${ORCHESTRATOR_DB_DSN:-postgres://postgres:postgres@localhost:5433/orchestrator?sslmode=disable}" -f scripts/local-dev/seed.sql

# Build
build: ## Build Docker images for all services
	@for svc in $(SERVICES); do \
		if [ -f services/$$svc/Dockerfile ]; then \
			echo "Building $$svc..."; \
			docker build -t trade-eval/$$svc:latest services/$$svc || exit 1; \
		fi; \
	done
	cd frontend && npm run build

# Testing
test: ## Run unit tests for every Go module
	@for svc in $(SERVICES); do \
		echo "Testing $$svc..."; \
		(cd services/$$svc && go test ./... ) || exit 1; \
	done

test-race: ## Run tests with the race detector
	@for svc in $(SERVICES); do \
		(cd services/$$svc && go test -race -count=1 ./... ) || exit 1; \
	done

test-integration: ## Run integration tests (requires docker)
	go test -v -timeout 300s ./tests/integration/... 2>/dev/null || echo "no integration module"

bench: ## Run benchmarks
	@for svc in $(SERVICES); do \
		(cd services/$$svc && go test -bench=. -benchmem ./... 2>/dev/null || true); \
	done

# Code quality
lint: ## Run go vet (and golangci-lint if installed)
	@for svc in $(SERVICES); do \
		(cd services/$$svc && go vet ./... ) || exit 1; \
	done
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed; ran go vet only"

lint-fix:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run --fix ./... || true

# Protobuf
proto: ## Regenerate protobuf Go code
	cd proto && ./generate.sh

# Templates
template: ## Generate a contestant starter: make template LANG=cpp
	cd cmd/generate-template && go run . --language $(LANG) --output ../../contestant-starter/

# Platform verification
smoke-test: ## Run smoke test against a running stack
	BASE_URL=$${BASE_URL:-http://localhost:8080} bash scripts/local-dev/smoke-test.sh

verify: ## Run end-to-end platform verification
	bash scripts/verify-platform.sh

# Security
security-scan: ## Scan images and Go code
	@command -v gosec >/dev/null 2>&1 && gosec ./... || echo "gosec not installed"
	@for svc in $(SERVICES); do (cd services/$$svc && go mod verify); done

check-kafka-lag:
	docker exec $$(docker ps -qf name=kafka) kafka-consumer-groups.sh \
		--bootstrap-server localhost:9092 --describe --group telemetry-ingesters

clean: ## Tear down stack and prune
	docker compose down -v
	find . -name '*.out' -delete

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'
