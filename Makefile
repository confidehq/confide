.PHONY: help run dev up down build clean secrets

DOCKER_COMPOSE := $(shell docker compose version > /dev/null 2>&1 && echo "docker compose" || echo "docker-compose")

# Set the default goal
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo "Usage: \n make [command]"
	@echo ""
	@echo "Commands:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## dev: Run locally with Air
dev: 
	@echo "Starting local dev with Air..."
	@set -a && source .env.local && set +a && air

## ui: Start frontend
ui: 
	cd web && pnpm dev

## db: Start local docker db
db:
	cd deploy && $(DOCKER_COMPOSE) -f db.yml up

up:
	$(DOCKER_COMPOSE) --env-file .env.docker -f deploy/docker-compose.yml up --build --remove-orphans
