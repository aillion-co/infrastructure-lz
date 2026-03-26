package generator_test

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/aillion-co/infrastructure-lz/internal/generator"
	"github.com/aillion-co/infrastructure-lz/internal/models"
)

func TestGenerate_BasicProject(t *testing.T) {
	gen := generator.New()

	cfg := &models.ProjectConfig{
		ProjectName: "test-project",
		ProjectID:   "test-project-123",
		Region:      "us-central1",
		Environment: "development",
		Network: models.NetworkConfig{
			VPCName:       "test-vpc",
			SubnetCIDR:    "10.0.0.0/20",
			EnablePrivate: true,
		},
		IAM: models.IAMConfig{
			AdminGroup: "admins@example.com",
		},
	}

	zipData, err := gen.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("failed to read zip: %v", err)
	}

	expectedFiles := map[string]bool{
		"test-project-infra/Chart.yaml":             false,
		"test-project-infra/values.yaml":            false,
		"test-project-infra/templates/_helpers.tpl":  false,
		"test-project-infra/templates/project.yaml":  false,
		"test-project-infra/templates/network.yaml":  false,
		"test-project-infra/templates/subnet.yaml":   false,
		"test-project-infra/templates/iam.yaml":      false,
	}

	for _, f := range reader.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("expected file %q not found in zip", name)
		}
	}
}

func TestGenerate_WithGKE(t *testing.T) {
	gen := generator.New()

	cfg := &models.ProjectConfig{
		ProjectName: "gke-project",
		ProjectID:   "gke-project-456",
		Region:      "us-central1",
		Environment: "production",
		Network: models.NetworkConfig{
			VPCName:     "gke-vpc",
			SubnetCIDR:  "10.0.0.0/20",
			PodCIDR:     "10.1.0.0/16",
			ServiceCIDR: "10.2.0.0/20",
		},
		GKE: &models.GKEConfig{
			ClusterName:    "main-cluster",
			NodeCount:      3,
			MachineType:    "e2-standard-4",
			ReleaseChannel: "REGULAR",
		},
		IAM: models.IAMConfig{},
	}

	zipData, err := gen.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("failed to read zip: %v", err)
	}

	hasGKE := false
	for _, f := range reader.File {
		if strings.HasSuffix(f.Name, "gke.yaml") {
			hasGKE = true
		}
	}
	if !hasGKE {
		t.Error("expected gke.yaml in zip when GKE is configured")
	}
}

func TestGenerate_WithCloudSQL(t *testing.T) {
	gen := generator.New()

	cfg := &models.ProjectConfig{
		ProjectName: "sql-project",
		ProjectID:   "sql-project-789",
		Region:      "us-central1",
		Environment: "staging",
		Network: models.NetworkConfig{
			VPCName:    "sql-vpc",
			SubnetCIDR: "10.0.0.0/20",
		},
		CloudSQL: &models.CloudSQLConfig{
			InstanceName:     "main-db",
			DatabaseType:     "POSTGRES_15",
			Tier:             "db-custom-2-7680",
			HighAvailability: true,
		},
		IAM: models.IAMConfig{},
	}

	zipData, err := gen.Generate(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("failed to read zip: %v", err)
	}

	hasSQL := false
	for _, f := range reader.File {
		if strings.HasSuffix(f.Name, "cloudsql.yaml") {
			hasSQL = true
		}
	}
	if !hasSQL {
		t.Error("expected cloudsql.yaml in zip when CloudSQL is configured")
	}
}
