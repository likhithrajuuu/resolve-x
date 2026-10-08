COMPOSE := docker compose -f deploy/docker-compose.yml

.PHONY: build test up down logs load fmt
build:        ## compile all services into bin/
	go build -o bin/ ./cmd/...
test:
	go test ./...
fmt:
	gofmt -w . && go vet ./...
up:           ## start the full local stack
	$(COMPOSE) up -d --build
down:
	$(COMPOSE) down -v
logs:
	$(COMPOSE) logs -f ingestion processor gateway
load:         ## 30s load test against local ingestion
	go run ./cmd/loadgen -url http://localhost:4318 -c 64 -d 30s -spans 100
