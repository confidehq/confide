.PHONY: help db dev ui up down

DOCKER_COMPOSE := $(shell docker compose version > /dev/null 2>&1 && echo "docker compose" || echo "docker-compose")

# Set the default goal
.DEFAULT_GOAL := help

help:
	@echo "Usage: \n make [command]"
	@echo ""
	@echo "Commands:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## dev: Run locally with Air
dev: 
	@echo "Starting local dev with Air..."
	air

## ui: Start frontend
ui: 
	cd web && pnpm dev

## db: Start local docker db
db:
	$(DOCKER_COMPOSE) -p confide -f database.yml up --remove-orphans 

## up: Start the application using Docker Compose
up:
	VERSION=$(shell git describe --tags --abbrev=0 2>/dev/null || echo dev) \
	COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) \
	$(DOCKER_COMPOSE) -p confide --env-file .env -f docker-compose.yml up --build --remove-orphans

## down: Stop the application using Docker Compose
down:
	$(DOCKER_COMPOSE) -p confide --env-file .env -f docker-compose.yml down
