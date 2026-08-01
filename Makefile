SHELL := /bin/sh
DOCKER_COMPOSE ?= $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")
GOCACHE ?= $(CURDIR)/.cache/go-build

.PHONY: install check test build dev-app dev-api dev-worker dev-infra dev down status

install:
	cd app && pnpm install --frozen-lockfile
	cd server && GOCACHE=$(GOCACHE) go mod download

check:
	cd app && pnpm type-check
	cd server && GOCACHE=$(GOCACHE) go vet ./...

test:
	cd app && pnpm test
	cd server && GOCACHE=$(GOCACHE) go test ./...

build:
	cd app && pnpm build:h5
	cd server && GOCACHE=$(GOCACHE) go build ./cmd/api ./cmd/worker ./cmd/admin

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
