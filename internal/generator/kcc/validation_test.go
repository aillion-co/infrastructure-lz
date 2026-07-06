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

func TestGovernanceConfig_Validation(t *testing.T) {
	base := func() *models.GovernanceConfig {
		return &models.GovernanceConfig{
			ProjectID: "mgmt-proj",
			Regimes:   []string{"cis", "gdpr"},
		}
	}

	tests := []struct {
		name      string
		mutate    func(*models.GovernanceConfig)
		wantField string
	}{
		{"missing projectId", func(c *models.GovernanceConfig) { c.ProjectID = "" }, "projectId"},
		{"no regimes", func(c *models.GovernanceConfig) { c.Regimes = nil }, "regimes"},
		{"unknown regime", func(c *models.GovernanceConfig) { c.Regimes = []string{"soc2"} }, "regimes"},
		{"bad enforcement mode", func(c *models.GovernanceConfig) { c.EnforcementMode = "warn" }, "enforcementMode"},
	}

	builder := NewGovernanceBuilder()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base()
			tt.mutate(cfg)
			_, err := builder.Build(cfg)
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("expected ValidationError, got %v", err)
			}
			if verr.Field != tt.wantField {
				t.Errorf("expected field %q, got %q", tt.wantField, verr.Field)
			}
		})
	}
}

func TestGovernanceBuilder_RegimeSelection(t *testing.T) {
	builder := NewGovernanceBuilder()

	// CIS only: no data-residency or CMEK policies, but public-IAM and
	// firewall guardrails present.
	resources, err := builder.Build(&models.GovernanceConfig{
		ProjectID: "mgmt-proj",
		Regimes:   []string{"cis"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := map[string]bool{}
	for _, r := range resources {
		names[r.Name] = true
	}
	for _, want := range []string{"policycontroller.yaml", "regime-manifest.yaml", "policy-no-public-iam.yaml", "policy-firewall-no-open-ingress.yaml"} {
		if !names[want] {
			t.Errorf("cis selection missing %s (got %v)", want, names)
		}
	}
	for _, absent := range []string{"policy-allowed-regions.yaml", "policy-require-cmek.yaml", "policy-binary-authorization.yaml"} {
		if names[absent] {
			t.Errorf("cis selection unexpectedly includes %s", absent)
		}
	}

	// GDPR adds residency and CMEK.
	resources, err = builder.Build(&models.GovernanceConfig{
		ProjectID: "mgmt-proj",
		Regimes:   []string{"gdpr"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names = map[string]bool{}
	for _, r := range resources {
		names[r.Name] = true
	}
	for _, want := range []string{"policy-allowed-regions.yaml", "policy-require-cmek.yaml", "policy-required-labels.yaml"} {
		if !names[want] {
			t.Errorf("gdpr selection missing %s", want)
		}
	}
}

func TestGovernanceBuilder_EnforcementMode(t *testing.T) {
	builder := NewGovernanceBuilder()

	resources, err := builder.Build(&models.GovernanceConfig{
		ProjectID:       "mgmt-proj",
		Regimes:         []string{"cis"},
		EnforcementMode: "dryrun",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range resources {
		if !strings.HasPrefix(r.Name, "policy-") {
			continue
		}
		if !strings.Contains(string(r.Content), "enforcementAction: dryrun") {
			t.Errorf("%s should carry enforcementAction: dryrun", r.Name)
		}
	}
}

func TestInjection_NewlineInStringFieldRejected(t *testing.T) {
	builder := NewBigQueryAnalyticsBuilder()
	_, err := builder.Build(&models.BigQueryAnalyticsConfig{
		ProjectName:        "acme",
		ProjectID:          "acme-1",
		Region:             "eu",
		DatasetDescription: "legit\n---\nkind: IAMPolicyMember\nspec:\n  member: allUsers",
	})
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError for newline injection, got %v", err)
	}
	if verr.Field != "datasetDescription" {
		t.Errorf("expected field datasetDescription, got %q", verr.Field)
	}
}

func TestInjection_YamlStructuralCharInIdentifierRejected(t *testing.T) {
	builder := NewBootstrapOrgBuilder()
	_, err := builder.Build(&models.BootstrapOrgConfig{
		CustomerName:   "acme: evil",
		WorkloadName:   "web",
		RootLevel:      "organization",
		RootID:         "123",
		BillingAccount: "AAAAAA-BBBBBB-CCCCCC",
		Region:         "us-central1",
		Envs:           "dev",
	})
	var verr *ValidationError
	if !errors.As(err, &verr) || verr.Field != "customerName" {
		t.Fatalf("expected customerName ValidationError, got %v", err)
	}
}

func TestInjection_BenignFreeTextIsQuotedNotBroken(t *testing.T) {
	builder := NewBigQueryAnalyticsBuilder()
	resources, err := builder.Build(&models.BigQueryAnalyticsConfig{
		ProjectName:        "acme",
		ProjectID:          "acme-1",
		Region:             "eu",
		DatasetDescription: "Customer: Acme Corp",
	})
	if err != nil {
		t.Fatalf("benign free text should be accepted: %v", err)
	}
	for _, r := range resources {
		if r.Name == "bq-dataset.yaml" {
			if !strings.Contains(string(r.Content), `description: "Customer: Acme Corp"`) {
				t.Errorf("free-text description should be YAML-quoted, got:\n%s", r.Content)
			}
			return
		}
	}
	t.Fatal("bq-dataset.yaml not generated")
}
