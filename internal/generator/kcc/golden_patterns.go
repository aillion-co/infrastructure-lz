package kcc

import (
	"fmt"
	"strings"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// goldenPattern is a reference GCP architecture shipped in the Backstage
// portal catalog. Each pattern names the governance regimes it is designed
// for and the guardrail policies (governanceCatalog IDs) it stays within,
// so teams can pick an architecture that is compliant by construction.
type goldenPattern struct {
	ID          string
	Title       string
	Description string
	Regimes     []string // governanceRegimes keys this pattern aligns with
	Guardrails  []string // governanceCatalog policy IDs the pattern satisfies
	Components  []string // GCP services used, shown in the pattern card
}

// goldenPatternCatalog is the out-of-the-box pattern library, in fixed
// order for deterministic output.
var goldenPatternCatalog = []goldenPattern{
	{
		ID:          "three-tier-web-app",
		Title:       "Three-Tier Web Application",
		Description: "External HTTPS load balancer fronting Cloud Run services with a private-IP Cloud SQL backend, CMEK encryption end to end",
		Regimes:     []string{"cis", "pci-dss"},
		Guardrails:  []string{"no-public-iam", "require-cmek", "sql-no-public-ip", "firewall-no-open-ingress"},
		Components:  []string{"External HTTPS LB", "Cloud Run", "Cloud SQL (private IP)", "Cloud KMS", "Cloud Armor"},
	},
	{
		ID:          "serverless-event-driven",
		Title:       "Serverless Event-Driven Processing",
		Description: "Pub/Sub-triggered Cloud Run workers writing to versioned Cloud Storage with uniform access controls",
		Regimes:     []string{"cis", "iso27001"},
		Guardrails:  []string{"no-public-iam", "storage-uniform-access", "bucket-versioning"},
		Components:  []string{"Pub/Sub", "Cloud Run", "Eventarc", "Cloud Storage", "Firestore"},
	},
	{
		ID:          "data-platform-eu",
		Title:       "EU Data Platform",
		Description: "BigQuery warehouse with Dataflow ingestion pinned to approved EU regions, CMEK on every store, and mandatory data-classification labels",
		Regimes:     []string{"gdpr", "iso27001", "nis2"},
		Guardrails:  []string{"require-cmek", "allowed-regions", "required-labels", "storage-uniform-access"},
		Components:  []string{"BigQuery (CMEK)", "Dataflow", "Cloud Storage (EU)", "Data Catalog", "Cloud KMS"},
	},
	{
		ID:          "private-gke-microservices",
		Title:       "Private GKE Microservices",
		Description: "Private-node GKE with Workload Identity, Binary Authorization, and internal load balancing only",
		Regimes:     []string{"cis", "nis2", "pci-dss"},
		Guardrails:  []string{"gke-private-cluster", "gke-workload-identity", "binary-authorization", "firewall-no-open-ingress"},
		Components:  []string{"GKE (private nodes)", "Workload Identity", "Binary Authorization", "Internal LB", "Artifact Registry"},
	},
	{
		ID:          "secure-ml-inference",
		Title:       "Secure ML Inference",
		Description: "Vertex AI models served through a private LiteLLM proxy with Secret Manager key handling, EU residency, and attested supply chain",
		Regimes:     []string{"eucra", "gdpr", "nis2"},
		Guardrails:  []string{"no-public-iam", "require-cmek", "allowed-regions", "binary-authorization"},
		Components:  []string{"Vertex AI", "Cloud Run (LiteLLM)", "Secret Manager", "VPC-SC", "Cloud KMS"},
	},
	{
		ID:          "regulated-payments",
		Title:       "Regulated Payments Processing",
		Description: "PCI-scoped processing on private GKE with shielded nodes, CMEK everywhere, and no public network paths",
		Regimes:     []string{"iso27001", "pci-dss"},
		Guardrails:  []string{"no-public-iam", "require-cmek", "sql-no-public-ip", "gke-private-cluster", "firewall-no-open-ingress", "shielded-vms"},
		Components:  []string{"GKE (private, shielded)", "Cloud SQL (private IP)", "Cloud KMS", "Cloud HSM", "VPC-SC"},
	},
}

// renderGoldenPatterns emits the Backstage golden-pattern catalog as a
// ConfigMap of scaffolder Template entities plus the app-config fragment
// that registers them as catalog locations. Both are mounted into the
// Backstage pod, so the patterns appear in the portal out of the box.
func renderGoldenPatterns(cfg *models.DeveloperPortalConfig) []byte {
	var b strings.Builder

	b.WriteString(`apiVersion: v1
kind: ConfigMap
metadata:
  name: backstage-golden-patterns
  namespace: backstage
  labels:
    app.kubernetes.io/part-of: developer-portal
    feature: dynamic-developer-portal
data:
`)

	for _, p := range goldenPatternCatalog {
		fmt.Fprintf(&b, "  %s.yaml: |\n%s", p.ID, indentBlock(renderPatternEntity(p, cfg), 4))
	}

	// app-config fragment registering each pattern file as a catalog
	// location; mounted alongside the pattern files at /golden-patterns.
	b.WriteString(`---
apiVersion: v1
kind: ConfigMap
metadata:
  name: backstage-app-config-patterns
  namespace: backstage
  labels:
    app.kubernetes.io/part-of: developer-portal
    feature: dynamic-developer-portal
data:
  app-config.golden-patterns.yaml: |
    catalog:
      locations:
`)
	for _, p := range goldenPatternCatalog {
		fmt.Fprintf(&b, "        - type: file\n          target: /golden-patterns/%s.yaml\n          rules:\n            - allow: [Template]\n", p.ID)
	}

	return []byte(b.String())
}

// renderPatternEntity renders one pattern as a Backstage scaffolder
// Template entity.
func renderPatternEntity(p goldenPattern, cfg *models.DeveloperPortalConfig) string {
	var b strings.Builder

	tags := append([]string{"gcp", "golden-path"}, p.Regimes...)

	fmt.Fprintf(&b, `apiVersion: scaffolder.backstage.io/v1beta3
kind: Template
metadata:
  name: %s
  title: %s
  description: %s
  tags: [%s]
  annotations:
    compliance.aillion.co/regimes: %s
    compliance.aillion.co/guardrails: %s
    aillion.co/components: %s
spec:
  owner: platform-team
  type: architecture-pattern
  parameters:
    - title: Service details
      required: [name, environment]
      properties:
        name:
          title: Service name
          type: string
          description: Lowercase name for the new service
        environment:
          title: Target environment
          type: string
          enum: [dev, test, prod]
          default: dev
  steps:
    - id: fetch
      name: Fetch pattern skeleton
      action: fetch:template
      input:
        url: ./skeletons/%s
        values:
          name: ${{ parameters.name }}
          environment: ${{ parameters.environment }}
          gcpProjectId: %s
          gcpRegion: %s
    - id: publish
      name: Publish to repository
      action: publish:github
      input:
        repoUrl: github.com?repo=${{ parameters.name }}&owner=platform-team
    - id: register
      name: Register in catalog
      action: catalog:register
      input:
        repoContentsUrl: ${{ steps.publish.output.repoContentsUrl }}
        catalogInfoPath: /catalog-info.yaml
`,
		p.ID, p.Title, p.Description, strings.Join(tags, ", "),
		strings.Join(p.Regimes, ","), strings.Join(p.Guardrails, ","),
		strings.Join(p.Components, ", "),
		p.ID, cfg.GCPProjectID, cfg.GCPRegion)

	return b.String()
}
