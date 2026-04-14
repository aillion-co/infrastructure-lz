# CLAUDE.md — AI Development Harness

This file configures Claude Code to efficiently build and maintain this codebase.
It defines the project, conventions, commands, and the full development feedback loop.

## Project Overview

**infrastructure-lz** is a Go-based web application that generates Infrastructure-as-Code
configurations for Google Cloud. Users fill out a web UI with configuration options, and the
system produces Google Config Connector (KCC) resources wrapped in Helm charts, output as a
downloadable zip file.

### Architecture

```
┌─────────────┐     ┌──────────────┐     ┌───────────────────┐     ┌──────────┐
│   Web UI    │────▶│  API Server  │────▶│  IAC Generator    │────▶│ ZIP File │
│  (Go templ) │     │  (net/http)  │     │  (KCC + Helm)     │     │ download │
└─────────────┘     └──────────────┘     └───────────────────┘     └──────────┘
```

- **Web UI**: Go templates + HTMX for interactive config forms
- **API Server**: Go net/http with middleware for auth, telemetry, validation
- **IAC Generator**: Produces KCC YAML manifests wrapped in Helm chart structure
- **Output**: Zip archive containing a complete Helm chart

### Services

| Service | Path | Description |
|---------|------|-------------|
| `cmd/server` | Main HTTP server | Serves UI and API endpoints |
| `cmd/generator` | CLI generator | Standalone CLI for IAC generation |

## Directory Structure

```
infrastructure-lz/
├── cmd/                    # Application entrypoints
│   ├── server/             # HTTP server (main service)
│   └── generator/          # CLI tool for offline generation
├── internal/               # Private application code
│   ├── api/                # HTTP handlers and middleware
│   │   ├── handlers/       # Request handlers per resource
│   │   └── middleware/     # Auth, logging, telemetry middleware
│   ├── config/             # App configuration loading
│   ├── generator/          # IAC generation engine
│   │   ├── kcc/            # Google Config Connector resource builders
│   │   ├── helm/           # Helm chart scaffolding and packaging
│   │   └── templates/      # Go templates for KCC/Helm output
│   ├── models/             # Domain models (config specs, project defs)
│   ├── telemetry/          # OpenTelemetry setup, metrics, tracing
│   └── web/                # Web UI (templates, static assets)
│       ├── templates/      # Go HTML templates
│       └── static/         # CSS, JS, images
├── pkg/                    # Public/reusable packages
│   ├── archive/            # Zip file creation utilities
│   └── validator/          # Input validation helpers
├── deploy/                 # Deployment configurations
│   ├── helm/               # Helm chart for this application
│   ├── k8s/                # Kustomize overlays
│   │   ├── base/           # Base manifests
│   │   └── overlays/       # Per-environment overrides
│   └── skaffold/           # Skaffold profiles
├── test/                   # Test suites
│   ├── e2e/                # End-to-end browser/API tests
│   ├── integration/        # Integration tests (with real deps)
│   └── fixtures/           # Test data and golden files
├── scripts/                # Development and CI helper scripts
├── docs/                   # Architecture docs and ADRs
├── .github/                # GitHub Actions workflows
│   └── workflows/          # CI/CD and agent workflows
└── .claude/                # Claude Code agent configurations
```

## Commands

These are the commands Claude should use throughout development. All are runnable
from the repository root.

### Build & Run

```bash
# Build all binaries
make build

# Run the server locally (port 8080)
make run

# Run with hot reload (requires air)
make dev
```

### Test

```bash
# Run all unit tests with race detector
make test

# Run tests with verbose output
make test-verbose

# Run integration tests (requires GCP credentials)
make test-integration

# Run e2e tests
make test-e2e

# Run all tests with coverage report
make test-coverage

# Run a specific test by name
go test -v -run TestGenerateHelmChart ./internal/generator/...
```

### Lint & Format

```bash
# Run all linters (golangci-lint)
make lint

# Auto-fix lint issues where possible
make lint-fix

# Format all Go code
make fmt

# Run go vet
make vet
```

### Generate & Validate

```bash
# Generate any code (templates, mocks, etc.)
make generate

# Validate Helm chart output
make validate-helm

# Validate KCC manifests
make validate-kcc
```

### Deploy (GKE Preview)

```bash
# Deploy preview environment via Skaffold
make preview-deploy

# Tail preview logs
make preview-logs

# Tear down preview
make preview-teardown

# Deploy to production
make deploy-production
```

### Docker

```bash
# Build Docker image
make docker-build

# Run Docker image locally
make docker-run
```

## Development Workflow — The Feedback Loop

Claude operates in a continuous implement → verify → deploy → observe → iterate loop.
Each cycle should be tight and produce measurable progress.

### 1. Implement

- Write code in small, testable increments
- Each change should be a single logical unit (one handler, one generator function, one template)
- Always write the test alongside or before the implementation
- Follow the patterns established in existing code

### 2. Verify (mandatory before any commit)

```bash
make lint && make test
```

Both must pass. If lint fails, fix it immediately. If tests fail, fix the code, not the test
(unless the test expectation is wrong).

### 3. Deploy to Preview

For changes that affect the web UI or API behavior:

```bash
make preview-deploy
```

The preview deploy goes to a GKE namespace `preview-{branch}`. Skaffold handles image
building, pushing, and deploying.

### 4. Observe Telemetry

After deploying, check that:
- The service starts without errors: `make preview-logs`
- Health endpoint responds: `curl http://<preview-url>/healthz`
- Metrics are being emitted (check OpenTelemetry collector)

### 5. Iterate

If anything is wrong, go back to step 1. If everything passes:

### 6. Open PR

Once a feature is complete and all checks pass, the CI pipeline will:
1. Run lint + test + build
2. Deploy a preview environment
3. Run e2e tests against the preview
4. Post results as PR comments
5. On merge to `main`, deploy to production

## Code Conventions

### Go Style

- **Go version**: 1.25+
- **Module path**: `github.com/aillion-co/infrastructure-lz`
- **Error handling**: Always wrap errors with `fmt.Errorf("context: %w", err)`
- **Naming**: Follow standard Go conventions. No stuttering (`config.Config` not `config.ConfigStruct`)
- **Packages**: Keep packages focused. If a package has >10 files, consider splitting
- **Interfaces**: Define at the consumer, not the implementer
- **Context**: All public functions that do I/O take `context.Context` as first parameter
- **Logging**: Use `slog` (structured logging) everywhere
- **Testing**: Table-driven tests, use `testify/assert` for assertions

### Observability (MANDATORY)

All code MUST be instrumented with OpenTelemetry. This is not optional.

**Traces**: Every public function that does I/O or non-trivial work MUST create a span:
```go
ctx, span := telemetry.Tracer().Start(ctx, "package.FunctionName")
defer span.End()
```

**Span attributes**: Add meaningful attributes to spans:
```go
span.SetAttributes(
    attribute.String("project.id", projectID),
    attribute.Int("result.count", len(results)),
)
```

**Error recording**: All errors MUST be recorded on the span:
```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, "description")
    return fmt.Errorf("context: %w", err)
}
```

**Metrics**: Use pre-registered metrics from `telemetry.GetMetrics()`:
- Counters for totals (requests, errors, resources generated)
- Histograms for durations and sizes
- Register new metrics in `internal/telemetry/metrics.go`

**Logging**: Use context-aware logging (`slog.InfoContext(ctx, ...)`) so trace IDs
are correlated with log entries.

**HTTP handlers**: All handlers get automatic tracing via the `middleware.Telemetry`
middleware. Add handler-specific spans for sub-operations.

**Adding new metrics**: Add fields to the `Metrics` struct in
`internal/telemetry/metrics.go` and register them in `GetMetrics()`.

**Google Cloud Observability**: The OTLP exporter sends traces and metrics to the
Google Cloud OTEL Collector, which forwards to Cloud Trace and Cloud Monitoring.
Set `OTEL_EXPORTER_OTLP_ENDPOINT` to the collector address.

### HTTP API

- All API endpoints under `/api/v1/`
- Use `net/http` stdlib — no frameworks
- JSON request/response with proper Content-Type headers
- Health check at `/healthz`, readiness at `/readyz`
- Metrics at `/metrics` (Prometheus format)

### IAC Generation

- KCC resources follow the Google Config Connector CRD specifications
- Helm charts follow Helm v3 best practices
- All generated YAML must pass `helm lint` and `kubectl --dry-run`
- Templates use Go `text/template` with strict mode

### Testing

- Unit tests live next to the code: `foo.go` → `foo_test.go`
- Integration tests in `test/integration/`
- E2E tests in `test/e2e/`
- Golden file tests for generated output in `test/fixtures/`
- Test names: `TestFunctionName_Scenario_ExpectedBehavior`

### Git — Branch-per-Feature (MANDATORY)

Every new feature MUST be developed on its own branch. No feature work directly on `main`.

**Workflow:**
1. Create a feature branch from `main`: `feat/description`, `fix/description`, `refactor/description`
2. Develop, commit, and push to the feature branch
3. Run `make lint && make test` — both MUST pass before opening a PR
4. Open a PR to `main` — CI runs lint, test, build, and integration tests automatically
5. All CI checks MUST pass before the PR can be merged
6. Merge via squash-and-merge to keep `main` history clean

**Branch naming:** `feat/<feature-name>`, `fix/<bug-name>`, `refactor/<scope>`

**Rules:**
- Commit messages: imperative mood, <72 chars first line
- One logical change per commit
- Always rebase on main before opening PR
- NEVER push directly to `main` — all changes go through PRs
- Tests MUST pass on the feature branch before merge

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | `8080` |
| `LOG_LEVEL` | Logging level (debug/info/warn/error) | `info` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry collector endpoint | `localhost:4317` |
| `GCP_PROJECT_ID` | GCP project for preview deploys | — |
| `GKE_CLUSTER` | GKE cluster name | — |
| `GKE_REGION` | GKE cluster region | — |

## CI/CD Pipeline

### On Pull Request (merge gate — ALL must pass)
1. `lint` + `test` + `build` + `validate-helm` (parallel)
2. `integration-test` (after build, runs server binary + integration suite)
3. `merge-gate` — aggregates all checks; PR cannot merge unless this passes
4. `docker-build` — image build (no push)
5. Preview deploy to GKE namespace (preview-deploy workflow)
6. Integration + E2E tests against preview
7. Results posted as PR comments

### On Merge to main
1. Full test suite
2. Docker image build + push (tagged with SHA + `latest`)
3. Deploy to production GKE namespace
4. Smoke test

### Agent Workflows
Claude Code can be triggered as a GitHub Actions agent to:
- Implement features from GitHub issues
- Fix failing tests
- Review and improve PRs
- Update dependencies

## Key Dependencies

- `net/http` — HTTP server (stdlib)
- `html/template` — Web UI templates (stdlib)
- `text/template` — IAC generation templates (stdlib)
- `archive/zip` — Zip file creation (stdlib)
- `slog` — Structured logging (stdlib)
- `go.opentelemetry.io/otel` — Tracing and metrics (OTEL SDK)
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` — OTLP trace exporter
- `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc` — OTLP metric exporter
- `github.com/stretchr/testify` — Test assertions
- `helm.sh/helm/v3` — Helm chart validation
- `sigs.k8s.io/yaml` — YAML marshaling for KCC resources

## Anti-Patterns to Avoid

- Do NOT add frameworks (gin, echo, fiber) — use stdlib `net/http`
- Do NOT add ORMs — this app has no database
- Do NOT add dependency injection frameworks — use constructor functions
- Do NOT generate code that isn't tested by golden file comparison
- Do NOT deploy without running `make lint && make test` first
- Do NOT add optional parameters or feature flags for hypothetical future needs
- Do NOT add comments that restate what the code does
- Do NOT write functions that do I/O without OpenTelemetry spans
- Do NOT use bare `slog.Info()` — use `slog.InfoContext(ctx, ...)` to correlate with traces
- Do NOT create metrics outside of `internal/telemetry/metrics.go`
