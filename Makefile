.PHONY: help run dev up down build clean secrets

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
	@set -a && source .env && printenv && set +a && air

## ui: Start frontend
ui: 
	cd web && pnpm dev

## db: Start local docker db
db:
	cd deploy && docker-compose -f db.yml up
