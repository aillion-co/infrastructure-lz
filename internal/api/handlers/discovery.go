package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/aillion-co/infrastructure-lz/internal/models"
	"github.com/aillion-co/infrastructure-lz/internal/telemetry"
)

// FeatureRecommendation represents a single feature recommendation from discovery evaluation.
type FeatureRecommendation struct {
	FeatureID   models.FeatureID `json:"featureId"`
	Recommended bool             `json:"recommended"`
	Confidence  string           `json:"confidence"` // high, medium, low
	Reason      string           `json:"reason"`
}

// DiscoveryEvaluationResponse is the response from the discovery evaluation endpoint.
type DiscoveryEvaluationResponse struct {
	Recommendations []FeatureRecommendation `json:"recommendations"`
}

// HandleDiscoveryEvaluate returns an HTTP handler that evaluates a discovery
// questionnaire response and returns feature recommendations.
func HandleDiscoveryEvaluate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.Tracer().Start(r.Context(), "handlers.DiscoveryEvaluate")
		defer span.End()

		var req models.DiscoveryResponse
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "invalid request body")
			slog.ErrorContext(ctx, "failed to decode discovery request", "error", err)
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		span.SetAttributes(
			attribute.String("customer.name", req.CustomerInfo.CustomerName),
		)

		recs := evaluateDiscovery(&req)

		span.SetAttributes(
			attribute.Int("recommendations.count", len(recs)),
		)

		resp := DiscoveryEvaluationResponse{Recommendations: recs}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to encode response")
			slog.ErrorContext(ctx, "failed to encode discovery evaluation response", "error", err)
		}
	}
}

// HandleDiscoverySections returns an HTTP handler that serves the discovery
// section metadata for UI rendering.
func HandleDiscoverySections() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.Tracer().Start(r.Context(), "handlers.DiscoverySections")
		defer span.End()

		sections := buildDiscoverySections()

		span.SetAttributes(
			attribute.Int("sections.count", len(sections)),
		)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sections)
	}
}

// evaluateDiscovery examines questionnaire answers and produces feature recommendations.
func evaluateDiscovery(resp *models.DiscoveryResponse) []FeatureRecommendation {
	var recs []FeatureRecommendation

	// bootstrap-org: always recommended
	recs = append(recs, FeatureRecommendation{
		FeatureID:   models.FeatureBootstrapOrg,
		Recommended: true,
		Confidence:  "high",
		Reason:      "Organization bootstrap is always recommended as a foundational step",
	})

	// bigquery-analytics: if DataMgmt mentions BigQuery or analytics
	bqRec := FeatureRecommendation{
		FeatureID:   models.FeatureBigQueryAnalytics,
		Recommended: false,
		Confidence:  "low",
		Reason:      "No BigQuery or analytics requirements detected",
	}
	if containsAny(resp.DataMgmt.EncryptionKeyMgmt, "bigquery", "analytics") ||
		resp.CostControl.BillingExport == "looker" ||
		resp.AIEnablement.EnableBQML {
		bqRec.Recommended = true
		bqRec.Confidence = "high"
		bqRec.Reason = "Data management configuration indicates BigQuery or analytics usage"
	}
	recs = append(recs, bqRec)

	// developer-portal: if IACCICD mentions developer portal
	dpRec := FeatureRecommendation{
		FeatureID:   models.FeatureDeveloperPortal,
		Recommended: false,
		Confidence:  "low",
		Reason:      "No developer portal requirements detected",
	}
	if containsAny(resp.IACCICD.VCS, "developer", "portal") ||
		containsAny(resp.IACCICD.BuildPipeline, "developer", "portal") {
		dpRec.Recommended = true
		dpRec.Confidence = "high"
		dpRec.Reason = "IAC/CICD configuration indicates developer portal usage"
	}
	recs = append(recs, dpRec)

	// hardened-image-bakery: if Security mentions hardened images
	hiRec := FeatureRecommendation{
		FeatureID:   models.FeatureHardenedImageBakery,
		Recommended: false,
		Confidence:  "low",
		Reason:      "No hardened image requirements detected",
	}
	if sliceContainsAny(resp.Security.VMSecurity, "hardened", "image") ||
		containsAny(resp.Security.TeamCapabilities, "hardened", "image") {
		hiRec.Recommended = true
		hiRec.Confidence = "high"
		hiRec.Reason = "Security configuration indicates hardened image requirements"
	}
	recs = append(recs, hiRec)

	// secure-inferencing: if AIEnablement mentions AI/ML
	siRec := FeatureRecommendation{
		FeatureID:   models.FeatureSecureInferencing,
		Recommended: false,
		Confidence:  "low",
		Reason:      "No AI/ML inferencing requirements detected",
	}
	if resp.AIEnablement.EnableVertexAI ||
		len(resp.AIEnablement.GeminiModels) > 0 ||
		len(resp.AIEnablement.VertexEndpoints) > 0 ||
		resp.AIEnablement.EnableModelGarden ||
		resp.AIEnablement.EnableRAGPipeline ||
		resp.AIEnablement.EnableLiteLLM {
		siRec.Recommended = true
		siRec.Confidence = "high"
		siRec.Reason = "AI enablement configuration indicates ML/AI inferencing requirements"
	}
	recs = append(recs, siRec)

	// skaffold-app-dev: if IACCICD mentions Skaffold or app dev
	saRec := FeatureRecommendation{
		FeatureID:   models.FeatureSkaffoldAppDev,
		Recommended: false,
		Confidence:  "low",
		Reason:      "No Skaffold or application development requirements detected",
	}
	if containsAny(resp.IACCICD.BuildPipeline, "skaffold", "app-dev", "app dev") ||
		containsAny(resp.IACCICD.VCS, "skaffold", "app-dev", "app dev") {
		saRec.Recommended = true
		saRec.Confidence = "high"
		saRec.Reason = "IAC/CICD configuration indicates Skaffold or application development usage"
	}
	recs = append(recs, saRec)

	return recs
}

// containsAny checks if s contains any of the given substrings (case-insensitive).
func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// sliceContainsAny checks if any element in the slice contains any of the given substrings.
func sliceContainsAny(slice []string, substrs ...string) bool {
	for _, s := range slice {
		if containsAny(s, substrs...) {
			return true
		}
	}
	return false
}

// buildDiscoverySections returns the full set of discovery sections and fields for UI rendering.
func buildDiscoverySections() []models.DiscoverySectionMeta {
	return []models.DiscoverySectionMeta{
		{
			Title:       "Customer Information",
			Description: "Basic customer and engagement context",
			Fields: []models.DiscoveryFieldMeta{
				{Key: "customerName", Label: "Customer Name", Type: "text", Required: true},
				{Key: "securityContactName", Label: "Security Contact Name", Type: "text"},
				{Key: "dataContactName", Label: "Data Contact Name", Type: "text"},
				{Key: "industryVertical", Label: "Industry Vertical", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "financial-services", Label: "Financial Services"},
					{Value: "public-sector", Label: "Public Sector"},
					{Value: "healthcare", Label: "Healthcare"},
					{Value: "retail", Label: "Retail"},
					{Value: "technology", Label: "Technology"},
					{Value: "other", Label: "Other"},
				}},
				{Key: "engagementType", Label: "Engagement Type", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "workload-migration", Label: "Workload Migration"},
					{Value: "greenfield-defined", Label: "Greenfield Defined"},
					{Value: "greenfield-undefined", Label: "Greenfield Undefined"},
					{Value: "modernization", Label: "Modernization"},
				}},
			},
		},
		{
			Title:       "Identity & Access",
			Description: "Identity provider and access management configuration",
			Fields: []models.DiscoveryFieldMeta{
				{Key: "identityProvider", Label: "Identity Provider", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "ad", Label: "Active Directory"},
					{Value: "entra", Label: "Microsoft Entra"},
					{Value: "google-identity", Label: "Google Identity"},
					{Value: "okta", Label: "Okta"},
					{Value: "other", Label: "Other"},
				}},
				{Key: "consoleUsers", Label: "Console Users", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "0-50", Label: "0-50"},
					{Value: "50-500", Label: "50-500"},
					{Value: "500+", Label: "500+"},
				}},
				{Key: "accessGroups", Label: "Access Groups", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "groups-defined", Label: "Groups Defined"},
					{Value: "groups-need-help", Label: "Need Help Defining Groups"},
					{Value: "no-groups", Label: "No Groups"},
				}},
			},
		},
		{
			Title:       "Resource Management",
			Description: "Organization and resource management preferences",
			Fields: []models.DiscoveryFieldMeta{
				{Key: "existingOrg", Label: "Existing Organization", Type: "boolean"},
				{Key: "foundationsLayout", Label: "Foundations Layout", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "environment-driven", Label: "Environment Driven"},
					{Value: "flexible", Label: "Flexible"},
				}},
				{Key: "namingScheme", Label: "Naming Scheme", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "obj-env-team-hash", Label: "Object-Environment-Team-Hash"},
					{Value: "custom", Label: "Custom"},
				}},
			},
		},
		{
			Title:       "Networking",
			Description: "Network topology and connectivity requirements",
			Fields: []models.DiscoveryFieldMeta{
				{Key: "vpcTopology", Label: "VPC Topology", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "shared", Label: "Shared VPC"},
					{Value: "island", Label: "Island VPCs"},
					{Value: "ncc-hub-spoke", Label: "NCC Hub & Spoke"},
				}},
				{Key: "interconnect", Label: "Interconnect", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "none", Label: "None"},
					{Value: "dedicated", Label: "Dedicated"},
					{Value: "partner", Label: "Partner"},
					{Value: "ha-vpn", Label: "HA VPN"},
					{Value: "classic-vpn", Label: "Classic VPN"},
				}},
				{Key: "natGateway", Label: "NAT Gateway", Type: "boolean"},
				{Key: "secureWebProxy", Label: "Secure Web Proxy", Type: "boolean"},
			},
		},
		{
			Title:       "Data Management",
			Description: "Data management and encryption preferences",
			Fields: []models.DiscoveryFieldMeta{
				{Key: "encryptionKeyManagement", Label: "Encryption Key Management", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "none", Label: "None"},
					{Value: "cloud-kms", Label: "Cloud KMS"},
					{Value: "cloud-kms-autokey", Label: "Cloud KMS Autokey"},
					{Value: "external-kms", Label: "External KMS"},
				}},
			},
		},
		{
			Title:       "Cost Control",
			Description: "Billing and budget preferences",
			Fields: []models.DiscoveryFieldMeta{
				{Key: "billingVisibility", Label: "Billing Visibility", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "centrally", Label: "Centrally"},
					{Value: "by-department", Label: "By Department"},
				}},
				{Key: "billingExport", Label: "Billing Export", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "looker", Label: "Looker"},
					{Value: "no-export", Label: "No Export"},
					{Value: "other", Label: "Other"},
				}},
				{Key: "enableBillingAlerts", Label: "Enable Billing Alerts", Type: "boolean"},
			},
		},
		{
			Title:       "IAC & CI/CD",
			Description: "Infrastructure-as-code and CI/CD tooling preferences",
			Fields: []models.DiscoveryFieldMeta{
				{Key: "vcs", Label: "Version Control System", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "github-saas", Label: "GitHub SaaS"},
					{Value: "github-enterprise", Label: "GitHub Enterprise"},
					{Value: "gitlab-saas", Label: "GitLab SaaS"},
					{Value: "gitlab-self-managed", Label: "GitLab Self-Managed"},
				}},
				{Key: "buildPipeline", Label: "Build Pipeline", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "cloud-build", Label: "Cloud Build"},
					{Value: "atlantis", Label: "Atlantis"},
					{Value: "terraform-cloud", Label: "Terraform Cloud"},
					{Value: "skaffold", Label: "Skaffold"},
				}},
			},
		},
		{
			Title:       "AI Enablement",
			Description: "AI and ML platform requirements",
			Fields: []models.DiscoveryFieldMeta{
				{Key: "enableVertexAi", Label: "Enable Vertex AI", Type: "boolean"},
				{Key: "enableModelGarden", Label: "Enable Model Garden", Type: "boolean"},
				{Key: "enableBqMl", Label: "Enable BigQuery ML", Type: "boolean"},
				{Key: "enableRagPipeline", Label: "Enable RAG Pipeline", Type: "boolean"},
				{Key: "enableLitellmProxy", Label: "Enable LiteLLM Proxy", Type: "boolean"},
				{Key: "aiAuditLogging", Label: "AI Audit Logging", Type: "boolean"},
			},
		},
		{
			Title:       "Security",
			Description: "Security posture and policy requirements",
			Fields: []models.DiscoveryFieldMeta{
				{Key: "cmek", Label: "Customer-Managed Encryption Keys", Type: "boolean"},
				{Key: "securityCommandCenter", Label: "Security Command Center", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "standard", Label: "Standard"},
					{Value: "premium", Label: "Premium"},
					{Value: "enterprise", Label: "Enterprise"},
					{Value: "different-system", Label: "Different System"},
				}},
				{Key: "vmSecurity", Label: "VM Security Policies", Type: "multiselect", Options: []models.DiscoveryFieldOption{
					{Value: "hardened-images", Label: "Hardened Images"},
					{Value: "os-login", Label: "OS Login"},
					{Value: "shielded-vms", Label: "Shielded VMs"},
				}},
			},
		},
		{
			Title:       "Resilience",
			Description: "Uptime and disaster recovery requirements",
			Fields: []models.DiscoveryFieldMeta{
				{Key: "uptimeSlo", Label: "Uptime SLO", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "99.9", Label: "99.9%"},
					{Value: "99.99", Label: "99.99%"},
					{Value: "99.999", Label: "99.999%"},
				}},
				{Key: "rto", Label: "Recovery Time Objective", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "near-zero", Label: "Near Zero"},
					{Value: "less-10-min", Label: "Less than 10 minutes"},
					{Value: "less-30-min", Label: "Less than 30 minutes"},
				}},
				{Key: "rpo", Label: "Recovery Point Objective", Type: "select", Options: []models.DiscoveryFieldOption{
					{Value: "near-zero", Label: "Near Zero"},
					{Value: "hourly", Label: "Hourly"},
					{Value: "nightly", Label: "Nightly"},
				}},
			},
		},
	}
}
