package validator_test

import (
	"testing"

	"github.com/aillion-co/infrastructure-lz/internal/models"
	"github.com/aillion-co/infrastructure-lz/pkg/validator"
)

func validConfig() models.ProjectConfig {
	return models.ProjectConfig{
		ProjectName: "my-project",
		ProjectID:   "my-project-123456",
		Region:      "us-central1",
		Environment: "development",
		Network: models.NetworkConfig{
			VPCName:    "main-vpc",
			SubnetCIDR: "10.0.0.0/20",
		},
	}
}

func TestValidateProjectConfig_Valid(t *testing.T) {
	cfg := validConfig()
	errs := validator.ValidateProjectConfig(&cfg)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateProjectConfig_InvalidProjectID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"too short", "ab"},
		{"uppercase", "My-Project"},
		{"starts with number", "1project"},
		{"special chars", "my_project!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.ProjectID = tt.id
			errs := validator.ValidateProjectConfig(&cfg)
			if len(errs) == 0 {
				t.Error("expected validation error for invalid project ID")
			}
		})
	}
}

func TestValidateProjectConfig_InvalidRegion(t *testing.T) {
	cfg := validConfig()
	cfg.Region = "mars-west1"
	errs := validator.ValidateProjectConfig(&cfg)

	found := false
	for _, e := range errs {
		if e == `region "mars-west1" is not supported` {
			found = true
		}
	}
	if !found {
		t.Errorf("expected region error, got: %v", errs)
	}
}

func TestValidateProjectConfig_InvalidCIDR(t *testing.T) {
	cfg := validConfig()
	cfg.Network.SubnetCIDR = "not-a-cidr"
	errs := validator.ValidateProjectConfig(&cfg)
	if len(errs) == 0 {
		t.Error("expected validation error for invalid CIDR")
	}
}

func TestValidateProjectConfig_GKEValidation(t *testing.T) {
	cfg := validConfig()
	cfg.GKE = &models.GKEConfig{
		ClusterName:    "my-cluster",
		NodeCount:      3,
		MachineType:    "e2-standard-4",
		ReleaseChannel: "REGULAR",
	}
	errs := validator.ValidateProjectConfig(&cfg)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid GKE config, got: %v", errs)
	}
}

func TestValidateProjectConfig_GKEInvalidNodeCount(t *testing.T) {
	cfg := validConfig()
	cfg.GKE = &models.GKEConfig{
		ClusterName:    "my-cluster",
		NodeCount:      0,
		ReleaseChannel: "REGULAR",
	}
	errs := validator.ValidateProjectConfig(&cfg)
	if len(errs) == 0 {
		t.Error("expected validation error for zero node count")
	}
}

func TestValidateProjectConfig_EmptyProjectName(t *testing.T) {
	cfg := validConfig()
	cfg.ProjectName = ""
	errs := validator.ValidateProjectConfig(&cfg)
	if len(errs) == 0 {
		t.Error("expected validation error for empty project name")
	}
}
