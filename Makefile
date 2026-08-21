SHELL := /bin/sh
DOCKER_COMPOSE ?= $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")
GOCACHE ?= $(CURDIR)/.cache/go-build

.PHONY: install check test test-integration test-restart-persistence recognition-eval build migrate dev-app dev-api dev-worker dev-infra dev down status production-config

install:
	cd app && pnpm install --frozen-lockfile
	cd server && GOCACHE=$(GOCACHE) go mod download

check:
	cd app && pnpm type-check
	cd server && GOCACHE=$(GOCACHE) go vet ./...

test:
	cd app && pnpm test
	cd server && GOCACHE=$(GOCACHE) go test ./...

test-integration:
	cd server && test -n "$(SUPPQ_TEST_DATABASE_URL)" && GOCACHE=$(GOCACHE) SUPPQ_TEST_DATABASE_URL="$(SUPPQ_TEST_DATABASE_URL)" go test ./internal/identity ./internal/catalog ./internal/recognition -count=1 -v

test-restart-persistence:
	./scripts/restart-persistence.sh

recognition-eval:
	test -n "$(INPUT)"
	cd server && GOCACHE=$(GOCACHE) go run ./cmd/eval -input "$(abspath $(INPUT))" $(if $(OUTPUT),-output "$(abspath $(OUTPUT))",)

build:
	cd app && pnpm build:h5
	cd server && GOCACHE=$(GOCACHE) go build ./cmd/api ./cmd/worker ./cmd/admin ./cmd/migrate ./cmd/eval

production-config:
	test -n "$(SUPPQ_HOSTNAME)" && test -n "$(SUPPQ_ACME_EMAIL)"
	SUPPQ_HOSTNAME="$(SUPPQ_HOSTNAME)" SUPPQ_ACME_EMAIL="$(SUPPQ_ACME_EMAIL)" $(DOCKER_COMPOSE) -f deploy/compose.production.yml config

migrate:
	cd server && set -a && . ./.env.local && set +a && GOCACHE=$(GOCACHE) go run ./cmd/migrate

dev-app:
	cd app && pnpm dev:h5

dev-api:
	cd server && set -a && . ./.env.local && set +a && go run ./cmd/api

dev-worker:
	cd server && set -a && . ./.env.local && set +a && go run ./cmd/worker

dev-infra:
	$(DOCKER_COMPOSE) -f deploy/compose.dev.yml up postgres object-storage mailpit

dev:
	$(DOCKER_COMPOSE) -f deploy/compose.dev.yml up --build

down:
	$(DOCKER_COMPOSE) -f deploy/compose.dev.yml down

status:
	$(DOCKER_COMPOSE) -f deploy/compose.dev.yml ps
