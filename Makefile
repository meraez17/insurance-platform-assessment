.PHONY: up down logs test

up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=100

test:
	docker compose run --rm orchestrator go test ./...
	docker compose run --rm issuer-service dotnet test

