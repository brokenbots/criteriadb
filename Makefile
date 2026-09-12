.PHONY: help build build-server build-adapter build-ingest test test-cover proto tidy fmt vet lint clean

# CGO_ENABLED=0 for zero CGO portability across all platforms
CGO_ENABLED ?= 0

help: ## Show available Makefile targets
	@awk 'BEGIN{FS=":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: build-server build-adapter build-ingest ## Build all CriteriaDB binaries (CGO_ENABLED=0)

build-server: ## Build CriteriaDB standalone gRPC server (bin/criteriadb)
	@mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) go build -o bin/criteriadb ./cmd/criteriadb

build-adapter: ## Build CriteriaDB Criteria Adapter binary (bin/criteria-adapter-criteriadb)
	@mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) go build -o bin/criteria-adapter-criteriadb ./cmd/criteria-adapter-criteriadb

build-ingest: ## Build CriteriaDB ND-JSON live event stream ingester (bin/criteriadb-ingest)
	@mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) go build -o bin/criteriadb-ingest ./cmd/criteriadb-ingest

test: ## Run unit and integration tests (CGO_ENABLED=0)
	CGO_ENABLED=$(CGO_ENABLED) go test -v ./...

test-cover: ## Run test suite with coverage output (cover.out)
	CGO_ENABLED=$(CGO_ENABLED) go test -coverprofile=cover.out -v ./...
	go tool cover -func=cover.out

proto: ## Generate Go protobuf structs and gRPC services from proto/
	buf generate proto

tidy: ## Clean and sync go.mod dependencies
	go mod tidy

fmt: ## Format Go source code
	go fmt ./...

vet: ## Run go vet static analysis
	go vet ./...

lint: fmt vet ## Run formatting and static code linting

clean: ## Remove compiled binaries and temporary test artifacts
	rm -rf bin/ cover.out *.db
