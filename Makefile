.PHONY: build run dev test test-verbose test-integration test-integration-local test-e2e test-all test-coverage \
       lint lint-fix fmt vet generate validate-helm validate-kcc \
       docker-build docker-run preview-deploy preview-logs preview-teardown \
       deploy-production clean

# ─── Variables ────────────────────────────────────────────────────────────────

GO         := go
GOFLAGS    := -race
BINARY_DIR := bin
SERVER_BIN := $(BINARY_DIR)/server
MODULE     := github.com/aillion-co/infrastructure-lz
IMAGE_NAME := iac-generator
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -X $(MODULE)/internal/config.Version=$(VERSION) \
              -X $(MODULE)/internal/config.Commit=$(COMMIT) \
              -X $(MODULE)/internal/config.BuildTime=$(BUILD_TIME)

# ─── Build & Run ──────────────────────────────────────────────────────────────

build:
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $(SERVER_BIN) ./cmd/server

run: build
	$(SERVER_BIN)

dev:
	@command -v air >/dev/null 2>&1 || { echo "Install air: go install github.com/air-verse/air@latest"; exit 1; }
	air

# ─── Test ─────────────────────────────────────────────────────────────────────

test:
	$(GO) test $(GOFLAGS) ./...

test-verbose:
	$(GO) test $(GOFLAGS) -v ./...

test-integration:
	$(GO) test $(GOFLAGS) -v -tags=integration -count=1 -timeout=5m ./test/integration/...

test-integration-local: build
	@echo "Starting server for local integration tests..."
	@OTEL_EXPORTER_OTLP_ENDPOINT="" PORT=8081 $(SERVER_BIN) & \
		SERVER_PID=$$!; \
		for i in $$(seq 1 30); do \
			if curl -sf http://localhost:8081/healthz > /dev/null 2>&1; then break; fi; \
			sleep 1; \
		done; \
		INTEGRATION_BASE_URL=http://localhost:8081 $(GO) test $(GOFLAGS) -v -tags=integration -count=1 -timeout=5m ./test/integration/...; \
		TEST_EXIT=$$?; \
		kill $$SERVER_PID 2>/dev/null || true; \
		exit $$TEST_EXIT

test-e2e:
	$(GO) test $(GOFLAGS) -v -tags=e2e -count=1 -timeout=5m ./test/e2e/...

test-all: test test-integration-local test-e2e

test-coverage:
	$(GO) test $(GOFLAGS) -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ─── Lint & Format ───────────────────────────────────────────────────────────

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Install: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

fmt:
	$(GO) fmt ./...
	gofumpt -l -w .

vet:
	$(GO) vet ./...

# ─── Generate & Validate ─────────────────────────────────────────────────────

generate:
	$(GO) generate ./...

validate-helm:
	helm lint deploy/helm/iac-generator/

validate-kcc:
	@echo "Validating generated KCC manifests..."
	$(GO) test -v -run 'TestGolden_' ./internal/generator/kcc/...

# ─── Docker ───────────────────────────────────────────────────────────────────

docker-build:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(IMAGE_NAME):$(VERSION) -t $(IMAGE_NAME):latest .

docker-run: docker-build
	docker run --rm -p 8080:8080 $(IMAGE_NAME):latest

# ─── Deploy ───────────────────────────────────────────────────────────────────

preview-deploy:
	@echo "Deploying preview to GKE..."
	skaffold run -p preview -f deploy/skaffold/skaffold.yaml

preview-logs:
	skaffold run -p preview -f deploy/skaffold/skaffold.yaml --tail

preview-teardown:
	skaffold delete -p preview -f deploy/skaffold/skaffold.yaml

deploy-production:
	skaffold run -p production -f deploy/skaffold/skaffold.yaml

# ─── Utility ──────────────────────────────────────────────────────────────────

clean:
	rm -rf $(BINARY_DIR) coverage.out coverage.html tmp_plan/
	$(GO) clean -testcache
