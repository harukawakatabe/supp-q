SHELL := /bin/sh
DOCKER_COMPOSE ?= $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")
GOCACHE ?= $(CURDIR)/.cache/go-build

.PHONY: install check test test-integration build migrate dev-app dev-api dev-worker dev-infra dev down status

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
	cd server && test -n "$(SUPPQ_TEST_DATABASE_URL)" && GOCACHE=$(GOCACHE) SUPPQ_TEST_DATABASE_URL="$(SUPPQ_TEST_DATABASE_URL)" go test ./internal/identity -run TestIdentityInvitationAndDemoLifecycle -count=1 -v

build:
	cd app && pnpm build:h5
	cd server && GOCACHE=$(GOCACHE) go build ./cmd/api ./cmd/worker ./cmd/admin ./cmd/migrate

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
