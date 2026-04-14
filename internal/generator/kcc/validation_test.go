package kcc

import (
	"errors"
	"strings"
	"testing"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

func TestBootstrapOrgConfig_Validation(t *testing.T) {
	base := func() *models.BootstrapOrgConfig {
		return &models.BootstrapOrgConfig{
			CustomerName:   "acme",
			WorkloadName:   "web",
			RootLevel:      "organization",
			RootID:         "123456789012",
			BillingAccount: "AAAAAA-BBBBBB-CCCCCC",
			Region:         "us-central1",
			Envs:           "dev,prod",
		}
	}

	tests := []struct {
		name      string
		mutate    func(*models.BootstrapOrgConfig)
		wantField string
	}{
		{"missing customer", func(c *models.BootstrapOrgConfig) { c.CustomerName = "" }, "customerName"},
		{"missing workload", func(c *models.BootstrapOrgConfig) { c.WorkloadName = "" }, "workloadName"},
		{"missing rootLevel", func(c *models.BootstrapOrgConfig) { c.RootLevel = "" }, "rootLevel"},
		{"bad rootLevel", func(c *models.BootstrapOrgConfig) { c.RootLevel = "team" }, "rootLevel"},
		{"missing rootId", func(c *models.BootstrapOrgConfig) { c.RootID = "" }, "rootId"},
		{"missing billing", func(c *models.BootstrapOrgConfig) { c.BillingAccount = "" }, "billingAccount"},
		{"missing region", func(c *models.BootstrapOrgConfig) { c.Region = "" }, "region"},
		{"empty envs", func(c *models.BootstrapOrgConfig) { c.Envs = "" }, "envs"},
		{"whitespace envs", func(c *models.BootstrapOrgConfig) { c.Envs = "   " }, "envs"},
		{"orgPolicies at folder", func(c *models.BootstrapOrgConfig) {
			c.RootLevel = "folder"
			c.OrgPolicies = true
		}, "orgPolicies"},
		{"vpcSc at folder", func(c *models.BootstrapOrgConfig) {
			c.RootLevel = "folder"
			c.VPCSC = true
		}, "vpcSc"},
		{"serviceProjects without sharedVpc", func(c *models.BootstrapOrgConfig) {
			c.ServiceProjects = true
			c.SharedVPC = false
		}, "serviceProjects"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base()
			tt.mutate(cfg)
			_, err := toBootstrapOrgConfig(cfg)
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}
			if ve.Field != tt.wantField {
				t.Errorf("expected field %q, got %q (msg=%s)", tt.wantField, ve.Field, ve.Message)
			}
		})
	}
}

func TestBootstrapOrgConfig_Validation_Passes(t *testing.T) {
	cfg := &models.BootstrapOrgConfig{
		CustomerName:   "acme",
		WorkloadName:   "web",
		RootLevel:      "organization",
		RootID:         "123456789012",
		BillingAccount: "AAAAAA-BBBBBB-CCCCCC",
		Region:         "us-central1",
		Envs:           " dev,prod ",
	}
	got, err := toBootstrapOrgConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Envs != "dev,prod" {
		t.Errorf("expected Envs to be trimmed, got %q", got.Envs)
	}
}

func TestSecureInferencingConfig_Validation(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *models.SecureInferencingConfig
		wantField string
	}{
		{"missing projectId", &models.SecureInferencingConfig{ProjectName: "p", Region: "us-central1"}, "projectId"},
		{"missing projectName", &models.SecureInferencingConfig{ProjectID: "id", Region: "us-central1"}, "projectName"},
		{"missing region", &models.SecureInferencingConfig{ProjectID: "id", ProjectName: "p"}, "region"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := toSecureInferencingConfig(tt.cfg)
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.wantField {
				t.Fatalf("want field %q, got err %v", tt.wantField, err)
			}
		})
	}
}

func TestSecureInferencingBuilder_CloudRunSecretSchema(t *testing.T) {
	builder := NewSecureInferencingBuilder()
	cfg := &models.SecureInferencingConfig{
		ProjectName: "ai",
		ProjectID:   "ai-prod",
		Region:      "us-central1",
	}
	resources, err := builder.Build(cfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	var found bool
	for _, r := range resources {
		if r.Name != "inferencing-cloudrun.yaml" {
			continue
		}
		found = true
		content := string(r.Content)
		if !strings.Contains(content, "secret: ai-litellm-master-key") {
			t.Errorf("expected correct KCC secretKeyRef.secret field, got:\n%s", content)
		}
		if !strings.Contains(content, `version: "latest"`) {
			t.Errorf("expected correct KCC secretKeyRef.version field, got:\n%s", content)
		}
		if strings.Contains(content, "secretRef:") {
			t.Errorf("found legacy invalid secretRef field, got:\n%s", content)
		}
		if strings.Contains(content, "versionRef:") {
			t.Errorf("found legacy invalid versionRef field, got:\n%s", content)
		}
		if strings.Contains(content, "@.iam.gserviceaccount.com") {
			t.Errorf("empty project interpolated into SA email, got:\n%s", content)
		}
	}
	if !found {
		t.Fatal("inferencing-cloudrun.yaml not generated")
	}
}

func TestSkaffoldAppDevBuilder_UsesUserRegion(t *testing.T) {
	builder := NewSkaffoldAppDevBuilder()
	cfg := &models.SkaffoldAppDevConfig{
		ProjectName:    "myapp",
		ServiceName:    "api",
		Region:         "europe-west2",
		ClusterType:    "autopilot",
		ReleaseChannel: "REGULAR",
		SQLDB:          "yes",
		AllowIngress:   "no",
	}
	resources, err := builder.Build(cfg)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	for _, r := range resources {
		content := string(r.Content)
		if strings.Contains(content, "us-central1") {
			t.Errorf("%s contains hardcoded us-central1 instead of user region: %s", r.Name, content)
		}
		if r.Name == "appdev-cloudsql.yaml" && !strings.Contains(content, "region: europe-west2") {
			t.Errorf("appdev-cloudsql.yaml missing user region, got:\n%s", content)
		}
	}
}

func TestBigQueryAnalyticsConfig_Validation(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *models.BigQueryAnalyticsConfig
		wantField string
	}{
		{"missing projectId", &models.BigQueryAnalyticsConfig{ProjectName: "p", Region: "us"}, "projectId"},
		{"missing projectName", &models.BigQueryAnalyticsConfig{ProjectID: "id", Region: "us"}, "projectName"},
		{"missing region", &models.BigQueryAnalyticsConfig{ProjectID: "id", ProjectName: "p"}, "region"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := toBigQueryConfig(tt.cfg)
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.wantField {
				t.Fatalf("want field %q, got err %v", tt.wantField, err)
			}
		})
	}
}
