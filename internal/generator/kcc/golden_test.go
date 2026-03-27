package kcc

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/aillion-co/infrastructure-lz/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

// goldenTest is a helper that compares builder output to golden files
func goldenTest(t *testing.T, featureID string, resources []Resource) {
	t.Helper()
	dir := filepath.Join("testdata", "golden", featureID)
	if *update {
		require.NoError(t, os.MkdirAll(dir, 0o755))
		for _, r := range resources {
			require.NoError(t, os.WriteFile(filepath.Join(dir, r.Name), r.Content, 0o644))
		}
		return
	}
	for _, r := range resources {
		path := filepath.Join(dir, r.Name)
		expected, err := os.ReadFile(path)
		require.NoError(t, err, "golden file missing: %s (run with -update)", path)
		assert.Equal(t, string(expected), string(r.Content), "mismatch in %s/%s", featureID, r.Name)
	}
}

func TestGolden_BootstrapOrg(t *testing.T) {
	builder := NewBootstrapOrgBuilder()
	cfg := &models.BootstrapOrgConfig{
		CustomerName:   "acme",
		WorkloadName:   "platform",
		RootLevel:      "organization",
		RootID:         "123456789012",
		BillingAccount: "AAAAAA-BBBBBB-CCCCCC",
		Region:         "australia-southeast1",
		Zone:           "australia-southeast1-a",
		OrgPolicies:    true,
		VPCSC:          true,
		SharedVPC:      true,
		ServiceProjects: true,
		VCS:            "github",
		VCSOrg:         "acme-corp",
		VCSUsername:     "acme-bot",
		Pipeline:       "cloudbuild",
		Envs:           "dev,test,prod",
		EnvFolders:     true,
		GKECluster:     true,
	}

	resources, err := builder.Build(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, resources)

	goldenTest(t, "bootstrap-org", resources)
}

func TestGolden_BigQueryAnalytics(t *testing.T) {
	builder := NewBigQueryAnalyticsBuilder()
	cfg := &models.BigQueryAnalyticsConfig{
		ProjectName:            "analytics-proj",
		ProjectID:              "acme-analytics-prod",
		Region:                 "us-central1",
		DatasetID:              "customer_events",
		DatasetDescription:     "Customer event analytics dataset",
		DefaultTableExpMS:      "7776000000",
		DeleteContentsOnDestroy: false,
		EnableDataTransfer:     true,
		DataSourceType:         "google_cloud_storage",
		DataViewerGroup:        "data-analysts@acme.com",
		EnableScheduledQueries: true,
	}

	resources, err := builder.Build(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, resources)

	goldenTest(t, "bigquery-analytics", resources)
}

func TestGolden_DeveloperPortal(t *testing.T) {
	builder := NewDeveloperPortalBuilder()
	cfg := &models.DeveloperPortalConfig{
		ProjectName:  "devportal",
		GCPProjectID: "acme-devportal-prod",
		GCPRegion:    "us-central1",
		GCPNetwork:   "shared-vpc",
		GCPSubnet:    "devportal-subnet",
		GitRepoSSH:   "git@github.com:acme-corp/platform-config.git",
	}

	resources, err := builder.Build(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, resources)

	goldenTest(t, "dynamic-developer-portal", resources)
}

func TestGolden_HardenedImageBakery(t *testing.T) {
	builder := NewHardenedImageBakeryBuilder()
	cfg := &models.HardenedImageBakeryConfig{
		ProjectName:        "img-bakery",
		ProjectID:          "acme-images-prod",
		Region:             "us-central1",
		Zone:               "us-central1-a",
		Network:            "shared-vpc",
		Subnetwork:         "bakery-subnet",
		BuildSA:            "image-builder",
		LogsBucket:         "acme-images-prod-build-logs",
		ArtifactRepository: "hardened-images",
		PackerVersion:      "1.9.4",
		AnsibleVersion:     "2.15.0",
	}

	resources, err := builder.Build(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, resources)

	goldenTest(t, "hardened-image-bakery", resources)
}

func TestGolden_SecureInferencing(t *testing.T) {
	builder := NewSecureInferencingBuilder()
	cfg := &models.SecureInferencingConfig{
		ProjectName:        "ai-proxy",
		ProjectID:          "acme-ai-prod",
		Region:             "us-central1",
		LiteLLMImage:       "ghcr.io/berriai/litellm:main-v1.35.0",
		EnableGemini:       true,
		GeminiModel:        "gemini-2.0-flash",
		EnableAuditLogging: true,
		EnableCostTracking: true,
		CloudRunCPU:        "4",
		CloudRunMemory:     "2Gi",
		CloudRunMaxInstances: 10,
		AllowedDomains:     "acme.com,partner.com",
	}

	resources, err := builder.Build(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, resources)

	goldenTest(t, "secure-inferencing", resources)
}

func TestGolden_SkaffoldAppDev(t *testing.T) {
	builder := NewSkaffoldAppDevBuilder()
	cfg := &models.SkaffoldAppDevConfig{
		ProjectName:       "myapp",
		ServiceName:       "api-server",
		Description:       "Main API service",
		Authors:           "platform-team@acme.com",
		ClusterType:       "standard",
		ClusterTopology:   "us-central1",
		MachineType:       "e2-standard-4",
		DiskSizeGB:        100,
		EnableAutoscaling: true,
		MinNodes:          2,
		MaxNodes:          8,
		InitialNodeCount:  3,
		SpotVMs:           true,
		ReleaseChannel:    "STABLE",
		ServicePort:       80,
		TargetPort:        8080,
		ServiceNamespace:  "api",
		RepoAddress:       "us-central1-docker.pkg.dev/acme-platform/images",
		SQLDB:             "yes",
		AllowIngress:      "yes",
	}

	resources, err := builder.Build(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, resources)

	goldenTest(t, "skaffold-application-development", resources)
}
