# Infrastructure Landing Zone Generator

A Go web application that generates Infrastructure-as-Code configurations for Google Cloud Platform. Users configure their GCP landing zone through a web UI, and the system produces Google Config Connector (KCC) resources wrapped in Helm charts, output as a downloadable zip file.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌───────────────────┐     ┌──────────┐
│   Web UI    │────▶│  API Server  │────▶│  IAC Generator    │────▶│ ZIP File │
│  (Go templ) │     │  (net/http)  │     │  (KCC + Helm)     │     │ download │
└─────────────┘     └──────────────┘     └───────────────────┘     └──────────┘
```

- **Web UI** — Go templates with Pico CSS, daisyUI, and Tailwind for an interactive activation wizard
- **API Server** — Go `net/http` with middleware for auth, telemetry, and validation
- **IAC Generator** — Produces KCC YAML manifests wrapped in Helm chart structure
- **Output** — Zip archive containing an umbrella Helm chart with per-feature sub-charts

### Activation Features

| Feature | Description |
|---------|-------------|
| **Bootstrap Organisation** | GCP org structure, management project, CI/CD, folder hierarchy, org policies, workload environments |
| **BigQuery Analytics** | BigQuery datasets with CRM or Google Analytics integrations |
| **Dynamic Developer Portal** | Backstage portal with Config Controller, shipping golden GCP architecture patterns aligned with the governance regimes |
| **Governance Guardrails** | OPA Gatekeeper policies via GKE Policy Controller with selectable compliance regimes (CIS, GDPR, PCI DSS, ISO 27001, NIS2, EU CRA) |
| **Hardened Image Bakery** | CIS-compliant hardened VM images using Packer/Ansible + Cloud Build |
| **Secure Inferencing** | LiteLLM AI proxy for standardised LLM access with audit logging |
| **Skaffold Application Development** | Kubernetes application scaffold with Skaffold, CIS security, multi-env overlays |

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Health check |
| `GET` | `/readyz` | Readiness check |
| `GET` | `/` | Redirects to `/activate` |
| `GET` | `/activate` | Activation wizard UI |
| `POST` | `/api/v1/activate` | Generate activation (returns zip) |
| `GET` | `/api/v1/features` | List available features |
| `POST` | `/api/v1/cost-estimate` | Estimate monthly GCP costs |
| `POST` | `/api/v1/discovery/evaluate` | Evaluate discovery questionnaire |
| `GET` | `/api/v1/discovery/sections` | Discovery form field metadata |

---

## Prerequisites

- **Go 1.25+**
- **golangci-lint** — `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0`
- **air** (optional, for hot-reload) — `go install github.com/air-verse/air@latest`
- **Docker** (for container builds)
- **Helm v3** (for chart validation)
- **kubectl + GKE credentials** (for deployment)

---

## Building and Running Locally

### Quick start

```bash
# Build the server binary
make build

# Run the server on port 8080
make run
```

Open [http://localhost:8080/activate](http://localhost:8080/activate) for the activation wizard.

### Hot-reload development

```bash
make dev
```

This uses `air` to watch for file changes and automatically rebuild.

### Docker

```bash
# Build the container image
make docker-build

# Run in Docker (port 8080)
make docker-run
```

### Environment variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | `8080` |
| `LOG_LEVEL` | Logging level (`debug`, `info`, `warn`, `error`) | `info` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry collector endpoint (empty = disabled) | — |
| `PRICING_PROVIDER` | Cost estimate backend: `static` or `catalog` (GCP Billing Catalog API) | `static` |
| `AUTH_ENABLED` | Enforce Google ID token auth on API endpoints | `false` |
| `AUTH_ALLOWED_AUDIENCES` | Comma-separated OAuth client IDs accepted in the token `aud` claim (required when auth is enabled) | — |
| `AUTH_ALLOWED_DOMAINS` | Comma-separated email domains permitted (empty = any authenticated user) | — |

> `GCP_PROJECT_ID`, `GKE_CLUSTER`, and `GKE_REGION` are used only by the
> CI/CD workflows for deploys; the server does not read them.

---

## Code Quality Requirements

All submissions must pass lint and test before merge. The CI merge gate enforces this automatically.

### Running checks locally

```bash
# Run all linters (golangci-lint)
make lint

# Run go vet
make vet

# Format code
make fmt

# Run unit tests with race detector
make test

# Run everything
make lint && make test
```

### Test suites

```bash
# Unit tests (fast, no external deps)
make test

# Unit tests with verbose output
make test-verbose

# Integration tests (starts server locally, tests against it)
make test-integration-local

# End-to-end tests
make test-e2e

# All tests (unit + integration + e2e)
make test-all

# Coverage report (opens HTML)
make test-coverage
```

### Running a specific test

```bash
go test -v -run TestGolden_BootstrapOrg ./internal/generator/kcc/...
```

### Golden file tests

KCC resource output is validated against golden files. To update golden files after a template change:

```bash
go test -v -run TestGolden -update ./internal/generator/kcc/...
```

### Helm validation

```bash
# Validate the application's Helm chart
make validate-helm

# Validate generated KCC manifests
make validate-kcc
```

### Code conventions

- **Error handling** — wrap errors with `fmt.Errorf("context: %w", err)`
- **Logging** — use `slog.InfoContext(ctx, ...)` (never bare `slog.Info()`)
- **Telemetry** — all I/O functions must create an OTEL span via `telemetry.Tracer().Start(ctx, "pkg.Function")`
- **Testing** — table-driven tests with `testify/assert`, golden file tests for generated output
- **HTTP** — stdlib `net/http` only, no frameworks
- **Packages** — define interfaces at the consumer, keep packages under 10 files

---

## Deploying to Dev (Preview Environments)

Preview environments are deployed automatically per pull request. You can also deploy manually.

### Automatic (via PR)

1. Open a pull request from a feature branch (`feat/`, `fix/`, `refactor/`, or `claude/`)
2. CI runs lint, test, build, integration tests, and Helm validation
3. The **merge gate** blocks merge if any check fails
4. On PR open/sync, the preview-deploy workflow:
   - Builds and pushes the Docker image to Artifact Registry
   - Creates a namespace `preview-pr-{PR_NUMBER}`
   - Deploys via Helm with 1 replica (128Mi RAM, 100m CPU)
   - Runs integration and E2E tests against the preview
   - Posts the preview URL as a PR comment
5. On PR close, the namespace is automatically cleaned up

### Manual (via Skaffold)

Requires GKE credentials and `skaffold` installed:

```bash
# Deploy preview environment
make preview-deploy

# Tail logs
make preview-logs

# Tear down
make preview-teardown
```

### Production

Production deploys happen automatically on merge to `main`:

```bash
# Manual production deploy (requires appropriate credentials)
make deploy-production
```

Production runs with 3 replicas, autoscaling to 10 at 70% CPU.

---

## Applying the Generated Landing Zone

The zip this service produces is a **Helm umbrella chart of Google Config
Connector (KCC) resources**, with a sub-chart per activated feature plus a
governance sub-chart (OPA Gatekeeper policies and Policy Controller
enablement). Applying it to a cluster has its own prerequisites, separate
from running this generator.

### Prerequisites on the target

- A GKE cluster (or Config Controller instance) with **Config Connector
  installed** — the KCC CRDs (`*.cnrm.cloud.google.com`) must exist, and
  its service account must have the IAM roles to create the requested
  resources (projects, folders, IAM, networking, etc.).
- The cluster **registered to a GKE Hub fleet** as the membership named in
  the governance feature's `membershipId` (default `config-controller`);
  the governance sub-chart installs Policy Controller onto that membership,
  which requires the **Anthos/GKE Enterprise entitlement** and the
  `gkehub` / `anthospolicycontroller` APIs enabled.
- `helm` v3 and `kubectl` configured against the target cluster.
- For the developer portal's golden patterns to scaffold real services,
  populate the referenced skeleton repositories (`./skeletons/<pattern-id>`)
  in your Backstage source repo — the chart ships the `Template` catalog
  entries, not the skeleton contents.

### Steps

```bash
# 1. Unzip the downloaded activation
unzip <customer>-activation.zip -d landing-zone && cd landing-zone

# 2. Inspect what will be applied (no cluster changes)
helm template <customer>-activation/

# 3. Install (or upgrade) the landing zone
helm upgrade --install <customer> <customer>-activation/ \
  --namespace config-connector

# 4. Selectively disable a feature at install time if needed
#    (each feature is gated by <feature>.enabled in values.yaml)
helm upgrade --install <customer> <customer>-activation/ \
  --set governance_guardrails.enabled=false
```

Config Connector then reconciles the KCC resources into GCP. Once Policy
Controller is installed, the governance constraints enforce at admission —
roll out with the feature's `enforcementMode: dryrun` first to observe
violations before switching to `deny`.

---

## Branch Workflow

All changes must go through pull requests. Direct pushes to `main` are not permitted.

```
main ◄─── feat/my-feature    (squash-and-merge)
          fix/bug-name
          refactor/scope
```

1. Create a feature branch: `feat/description`, `fix/description`, or `refactor/scope`
2. Develop and commit (imperative mood, <72 chars, one logical change per commit)
3. Run `make lint && make test` — both must pass
4. Open a PR to `main` — CI runs automatically
5. All merge gate checks must pass before merge
6. Merge via squash-and-merge

---

## Project Structure

```
infrastructure-lz/
├── cmd/server/              # HTTP server entrypoint
├── internal/
│   ├── api/
│   │   ├── handlers/        # HTTP handlers (activate, discovery, cost)
│   │   └── middleware/      # Auth, logging, telemetry, recovery
│   ├── config/              # App configuration
│   ├── generator/
│   │   ├── kcc/             # KCC resource builders (one per feature)
│   │   └── helm/            # Helm chart scaffolding
│   ├── archive/             # Zip utilities
│   ├── models/              # Domain models (features, discovery, activation)
│   ├── telemetry/           # OpenTelemetry setup
│   └── web/                 # Templates and static assets
├── deploy/
│   ├── helm/                # Application Helm chart
│   ├── k8s/                 # Kustomize overlays
│   └── skaffold/            # Skaffold profiles
├── test/
│   ├── integration/         # Integration tests
│   ├── e2e/                 # End-to-end tests
│   └── fixtures/            # Test data and golden files
└── .github/workflows/       # CI/CD pipelines
```
