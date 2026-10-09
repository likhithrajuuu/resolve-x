COMPOSE := docker compose -f deploy/docker-compose.yml

.PHONY: build test up down logs load fmt kong-check seed incident-demo e2e
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
	$(COMPOSE) logs -f ingestion processor tenancy query incident analysis kong
load:         ## 30s load test against local ingestion
	go run ./cmd/loadgen -url http://localhost:4318 -c 64 -d 30s -spans 100
kong-check:   ## validate deploy/kong/kong.yml
	docker run --rm -v $(PWD)/deploy/kong/kong.yml:/kong.yml:ro -e KONG_DATABASE=off kong:3.9 kong config parse /kong.yml
seed:         ## healthy history + live traffic (2 min)
	go run ./cmd/simulate -backfill 30m -d 2m
incident-demo: ## inject a payments fault right after a deployment
	go run ./cmd/simulate -d 3m -fault payments -deploy payments=v2.14.0
e2e:          ## full product check against a running stack
	./e2e.sh
