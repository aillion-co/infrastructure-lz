package kcc

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// parseAllDocs unmarshals every "---"-separated YAML document in b, failing
// the test on any parse error. Returns the parsed documents.
func parseAllDocs(t *testing.T, name string, b []byte) []map[string]any {
	t.Helper()
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	var docs []map[string]any
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("%s: invalid YAML: %v", name, err)
		}
		if doc != nil {
			docs = append(docs, doc)
		}
	}
	return docs
}

// TestGeneratedYAML_AllBuildersParse renders every feature builder with a
// valid config and asserts that all generated resources are valid YAML,
// including the YAML embedded in ConfigMap block scalars (Backstage
// templates and Rego). This turns the golden files from change-detectors
// into correctness checks: a bad block-scalar indent or a ": " in a
// free-text field would fail here.
func TestGeneratedYAML_AllBuildersParse(t *testing.T) {
	cases := []struct {
		name    string
		builder FeatureBuilder
		config  any
	}{
		{"bootstrap-org", NewBootstrapOrgBuilder(), &models.BootstrapOrgConfig{
			CustomerName: "acme", WorkloadName: "web", RootLevel: "organization",
			RootID: "123456789012", BillingAccount: "AAAAAA-BBBBBB-CCCCCC",
			Region: "us-central1", Envs: "dev,prod", OrgPolicies: true, SharedVPC: true,
		}},
		{"bigquery", NewBigQueryAnalyticsBuilder(), &models.BigQueryAnalyticsConfig{
			ProjectName: "acme", ProjectID: "acme-1", Region: "eu",
			DatasetDescription: "Customer: Acme Corp", DataViewerGroup: "viewers@example.com",
			EnableDataTransfer: true,
		}},
		{"developer-portal", NewDeveloperPortalBuilder(), &models.DeveloperPortalConfig{
			ProjectName: "acme", GCPProjectID: "acme-1", GCPRegion: "eu",
			GitRepoSSH: "git@github.com:acme/config.git",
		}},
		{"governance", NewGovernanceBuilder(), &models.GovernanceConfig{
			ProjectID: "acme-1",
			Regimes:   []string{"cis", "gdpr", "pci-dss", "iso27001", "nis2", "eucra"},
		}},
		{"hardened-image", NewHardenedImageBakeryBuilder(), &models.HardenedImageBakeryConfig{
			ProjectName: "acme", ProjectID: "acme-1", Region: "us-central1", Zone: "us-central1-a",
		}},
		{"secure-inferencing", NewSecureInferencingBuilder(), &models.SecureInferencingConfig{
			ProjectName: "acme", ProjectID: "acme-1", Region: "us-central1", EnableGemini: true,
		}},
		{"skaffold", NewSkaffoldAppDevBuilder(), &models.SkaffoldAppDevConfig{
			ProjectName: "acme", ServiceName: "api", Region: "us-central1",
			ClusterType: "autopilot", ReleaseChannel: "REGULAR", SQLDB: "yes", AllowIngress: "yes",
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resources, err := tc.builder.Build(tc.config)
			if err != nil {
				t.Fatalf("build failed: %v", err)
			}
			for _, r := range resources {
				docs := parseAllDocs(t, r.Name, r.Content)
				for _, doc := range docs {
					if doc["kind"] != "ConfigMap" {
						continue
					}
					data, _ := doc["data"].(map[string]any)
					for key, val := range data {
						s, ok := val.(string)
						if !ok || !strings.HasSuffix(key, ".yaml") {
							continue
						}
						// Embedded YAML block scalar must itself parse.
						parseAllDocs(t, r.Name+"/"+key, []byte(s))
					}
				}
			}
		})
	}
}

// TestGovernanceRego_StructuralValidity checks each generated
// ConstraintTemplate carries a Rego module that is structurally sound:
// declares a package, defines a violation rule, and has balanced braces.
// A lightweight guard against malformed Rego without a full OPA dependency.
func TestGovernanceRego_StructuralValidity(t *testing.T) {
	resources, err := NewGovernanceBuilder().Build(&models.GovernanceConfig{
		ProjectID: "acme-1",
		Regimes:   []string{"cis", "gdpr", "pci-dss", "iso27001", "nis2", "eucra"},
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	seen := 0
	for _, r := range resources {
		if !strings.HasPrefix(r.Name, "policy-") {
			continue
		}
		docs := parseAllDocs(t, r.Name, r.Content)
		for _, doc := range docs {
			if doc["kind"] != "ConstraintTemplate" {
				continue
			}
			spec, _ := doc["spec"].(map[string]any)
			targets, _ := spec["targets"].([]any)
			if len(targets) == 0 {
				t.Fatalf("%s: ConstraintTemplate has no targets", r.Name)
			}
			target, _ := targets[0].(map[string]any)
			rego, _ := target["rego"].(string)
			if !strings.Contains(rego, "package ") {
				t.Errorf("%s: Rego missing package declaration", r.Name)
			}
			if !strings.Contains(rego, "violation[") {
				t.Errorf("%s: Rego missing violation rule", r.Name)
			}
			if strings.Count(rego, "{") != strings.Count(rego, "}") {
				t.Errorf("%s: Rego has unbalanced braces", r.Name)
			}
			seen++
		}
	}
	if seen == 0 {
		t.Fatal("no ConstraintTemplates found")
	}
}
