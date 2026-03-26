package kcc

import (
	"strings"
	"testing"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

func TestFeatureBuilderRegistry_BuildAll(t *testing.T) {
	registry := NewFeatureBuilderRegistry()

	req := &models.ActivationRequest{
		Customer: models.CustomerDetails{
			CustomerName: "testcorp",
			ContactEmail: "test@example.com",
		},
		Features: []models.FeatureSelection{
			{
				FeatureID: models.FeatureBootstrapOrg,
				Enabled:   true,
				Config: &models.BootstrapOrgConfig{
					CustomerName:   "testcorp",
					WorkloadName:   "web",
					RootLevel:      "organization",
					RootID:         "123456789",
					BillingAccount: "AAAAAA-BBBBBB-CCCCCC",
					Region:         "europe-west2",
					Zone:           "europe-west2-a",
					OrgPolicies:    true,
					SharedVPC:      true,
					ServiceProjects: true,
					Envs:           "dev,prod",
					EnvFolders:     true,
					GKECluster:     true,
					VCS:            "github",
					Pipeline:       "cloudbuild",
				},
			},
			{
				FeatureID: models.FeatureBigQueryAnalytics,
				Enabled:   false,
			},
		},
	}

	result, err := registry.BuildAll(req)
	if err != nil {
		t.Fatalf("BuildAll failed: %v", err)
	}

	// Only bootstrap-org should be built (bigquery is disabled)
	if len(result) != 1 {
		t.Fatalf("expected 1 feature built, got %d", len(result))
	}

	resources, ok := result[models.FeatureBootstrapOrg]
	if !ok {
		t.Fatal("expected bootstrap-org resources")
	}

	// Should have: mgmt-folder, mgmt-project, cicd-sa, org-policies, shared-vpc, env-projects, env-gke, vpc-sc
	if len(resources) < 5 {
		t.Errorf("expected at least 5 resources, got %d", len(resources))
	}

	// Check resource names
	names := make(map[string]bool)
	for _, r := range resources {
		names[r.Name] = true
	}

	expected := []string{"mgmt-folder.yaml", "mgmt-project.yaml", "cicd-service-account.yaml", "org-policies.yaml", "shared-vpc.yaml", "env-projects.yaml", "env-gke.yaml"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing expected resource %q", name)
		}
	}
}

func TestBootstrapOrgBuilder_Build_MinimalConfig(t *testing.T) {
	builder := NewBootstrapOrgBuilder()

	cfg := &models.BootstrapOrgConfig{
		CustomerName:   "minicorp",
		WorkloadName:   "app",
		RootLevel:      "folder",
		RootID:         "987654321",
		BillingAccount: "AAAAAA-BBBBBB-CCCCCC",
		Region:         "us-central1",
		Zone:           "us-central1-a",
		Envs:           "dev",
	}

	resources, err := builder.Build(cfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Minimal: mgmt-folder, mgmt-project, cicd-sa, env-projects (no org-policies, no shared-vpc, no gke, no vpc-sc)
	if len(resources) != 4 {
		t.Errorf("expected 4 resources for minimal config, got %d", len(resources))
		for _, r := range resources {
			t.Logf("  resource: %s", r.Name)
		}
	}

	// Check folder references the folder (not org)
	for _, r := range resources {
		if r.Name == "mgmt-folder.yaml" {
			if strings.Contains(string(r.Content), "organizationRef") {
				t.Error("folder-level config should not reference organizationRef")
			}
			if !strings.Contains(string(r.Content), "folderRef") {
				t.Error("folder-level config should reference folderRef")
			}
		}
	}
}

func TestBigQueryAnalyticsBuilder_Build(t *testing.T) {
	builder := NewBigQueryAnalyticsBuilder()

	cfg := &models.BigQueryAnalyticsConfig{
		ProjectName:    "analytics-test",
		ProjectID:      "analytics-test-proj",
		Region:         "europe-west2",
		DatasetID:      "sales",
		DataViewerGroup: "viewers@example.com",
	}

	resources, err := builder.Build(cfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// APIs + dataset + IAM (no data transfer)
	if len(resources) != 3 {
		t.Errorf("expected 3 resources, got %d", len(resources))
	}

	for _, r := range resources {
		if r.Name == "bq-dataset.yaml" {
			if !strings.Contains(string(r.Content), "BigQueryDataset") {
				t.Error("dataset should contain BigQueryDataset kind")
			}
			if !strings.Contains(string(r.Content), "europe-west2") {
				t.Error("dataset should contain region")
			}
		}
	}
}

func TestDeveloperPortalBuilder_Build(t *testing.T) {
	builder := NewDeveloperPortalBuilder()

	cfg := &models.DeveloperPortalConfig{
		ProjectName:  "dev-portal",
		GCPProjectID: "portal-project-123",
		GCPRegion:    "europe-west2",
		GCPNetwork:   "shared-vpc",
		GCPSubnet:    "shared-subnet",
		GitRepoSSH:   "git@github.com:org/config.git",
	}

	resources, err := builder.Build(cfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// APIs + config-controller + backstage-namespace + backstage-workloads + config-sync
	if len(resources) != 5 {
		t.Errorf("expected 5 resources, got %d", len(resources))
	}

	for _, r := range resources {
		if r.Name == "config-controller.yaml" {
			if !strings.Contains(string(r.Content), "ContainerCluster") {
				t.Error("config controller should contain ContainerCluster")
			}
			if !strings.Contains(string(r.Content), "configConnectorConfig") {
				t.Error("config controller should enable Config Connector addon")
			}
		}
	}
}

func TestSecureInferencingBuilder_Build(t *testing.T) {
	builder := NewSecureInferencingBuilder()

	cfg := &models.SecureInferencingConfig{
		ProjectName:    "ai-proxy",
		ProjectID:      "ai-proxy-project",
		Region:         "us-central1",
		EnableGemini:   true,
		GeminiModel:    "gemini-2.5-pro",
		EnableAuditLogging: true,
	}

	resources, err := builder.Build(cfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// APIs + SA + secret + cloud run
	if len(resources) != 4 {
		t.Errorf("expected 4 resources, got %d", len(resources))
	}

	for _, r := range resources {
		if r.Name == "inferencing-cloudrun.yaml" {
			content := string(r.Content)
			if !strings.Contains(content, "RunService") {
				t.Error("cloud run should contain RunService kind")
			}
			if !strings.Contains(content, "gemini-2.5-pro") {
				t.Error("should reference configured Gemini model")
			}
			if !strings.Contains(content, "LITELLM_LOG") {
				t.Error("should enable audit logging")
			}
		}
	}
}

func TestSkaffoldAppDevBuilder_Build(t *testing.T) {
	builder := NewSkaffoldAppDevBuilder()

	cfg := &models.SkaffoldAppDevConfig{
		ProjectName:      "myapp",
		ServiceName:      "api",
		ClusterType:      "standard",
		MachineType:      "e2-standard-4",
		EnableAutoscaling: true,
		MinNodes:         1,
		MaxNodes:         5,
		ReleaseChannel:   "REGULAR",
		ServicePort:      80,
		TargetPort:       8080,
		SQLDB:            "yes",
		AllowIngress:     "yes",
	}

	resources, err := builder.Build(cfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// GKE + namespace + network-policy + deployment + cloudsql
	if len(resources) != 5 {
		t.Errorf("expected 5 resources, got %d", len(resources))
	}

	for _, r := range resources {
		if r.Name == "appdev-deployment.yaml" {
			content := string(r.Content)
			if !strings.Contains(content, "runAsNonRoot: true") {
				t.Error("deployment should enforce CIS non-root")
			}
			if !strings.Contains(content, "readOnlyRootFilesystem: true") {
				t.Error("deployment should enforce CIS read-only rootfs")
			}
			if !strings.Contains(content, "LoadBalancer") {
				t.Error("service should be LoadBalancer when ingress enabled")
			}
		}
	}
}

func TestSkaffoldAppDevBuilder_Build_Autopilot_NoSQL(t *testing.T) {
	builder := NewSkaffoldAppDevBuilder()

	cfg := &models.SkaffoldAppDevConfig{
		ProjectName:    "simple",
		ServiceName:    "web",
		ClusterType:    "autopilot",
		ReleaseChannel: "STABLE",
		SQLDB:          "no",
		AllowIngress:   "no",
	}

	resources, err := builder.Build(cfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// GKE + namespace + network-policy + deployment (no cloudsql)
	if len(resources) != 4 {
		t.Errorf("expected 4 resources, got %d", len(resources))
	}

	for _, r := range resources {
		if r.Name == "appdev-gke.yaml" {
			content := string(r.Content)
			if !strings.Contains(content, "enableAutopilot: true") {
				t.Error("should enable autopilot")
			}
			if strings.Contains(content, "ContainerNodePool") {
				t.Error("autopilot should not have a separate node pool")
			}
		}
		if r.Name == "appdev-deployment.yaml" {
			content := string(r.Content)
			if strings.Contains(content, "LoadBalancer") {
				t.Error("service should be ClusterIP when ingress disabled")
			}
		}
	}
}

func TestFeatureBuilderRegistry_UnknownFeature(t *testing.T) {
	registry := NewFeatureBuilderRegistry()

	_, err := registry.Build("nonexistent-feature", nil)
	if err == nil {
		t.Fatal("expected error for unknown feature")
	}
}
