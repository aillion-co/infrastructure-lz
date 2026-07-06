package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aillion-co/infrastructure-lz/internal/generator/kcc"
	"github.com/aillion-co/infrastructure-lz/internal/models"
)

func buildTestActivation(t *testing.T) []FileEntry {
	t.Helper()
	req := &models.ActivationRequest{
		Customer: models.CustomerDetails{CustomerName: "Acme Corp"},
	}
	resources := map[models.FeatureID][]kcc.Resource{
		models.FeatureBootstrapOrg: {
			{Name: "folder.yaml", Content: []byte("kind: Folder\n")},
		},
		models.FeatureBigQueryAnalytics: {
			{Name: "dataset.yaml", Content: []byte("kind: BigQueryDataset\n")},
		},
		models.FeatureSecureInferencing: {
			{Name: "runservice.yaml", Content: []byte("kind: RunService\n")},
		},
	}

	files, err := NewActivationBuilder().BuildActivation(req, resources)
	require.NoError(t, err)
	return files
}

func TestBuildActivation_Deterministic_SameBytesEveryRun(t *testing.T) {
	first := buildTestActivation(t)

	for run := 0; run < 5; run++ {
		again := buildTestActivation(t)
		require.Len(t, again, len(first))
		for i := range first {
			assert.Equal(t, first[i].Path, again[i].Path, "file order differs on run %d", run)
			assert.Equal(t, string(first[i].Content), string(again[i].Content),
				"content differs for %s on run %d", first[i].Path, run)
		}
	}
}

func TestBuildActivation_UmbrellaStructure(t *testing.T) {
	files := buildTestActivation(t)

	paths := make(map[string]string, len(files))
	for _, f := range files {
		paths[f.Path] = string(f.Content)
	}

	require.Contains(t, paths, "acme-corp-activation/Chart.yaml")
	require.Contains(t, paths, "acme-corp-activation/values.yaml")
	assert.Contains(t, paths, "acme-corp-activation/charts/bootstrap-org/Chart.yaml")
	assert.Contains(t, paths, "acme-corp-activation/charts/bootstrap-org/templates/folder.yaml")
	assert.Contains(t, paths, "acme-corp-activation/charts/bigquery-analytics/templates/dataset.yaml")
	assert.Contains(t, paths, "acme-corp-activation/charts/secure-inferencing/templates/runservice.yaml")

	chartYAML := paths["acme-corp-activation/Chart.yaml"]
	assert.Contains(t, chartYAML, "name: acme-corp-activation")
	assert.Contains(t, chartYAML, "- name: bootstrap-org")
	assert.Contains(t, chartYAML, "condition: bootstrap_org.enabled")

	valuesYAML := paths["acme-corp-activation/values.yaml"]
	assert.Contains(t, valuesYAML, "bootstrap_org:")
	assert.Contains(t, valuesYAML, "bigquery_analytics:")
}

func TestSlugify_StripsPathTraversalAndUnsafeChars(t *testing.T) {
	cases := map[string]string{
		"Acme Corp":   "acme-corp",
		"../../evil":  "evil",
		"a/b/c":       "a-b-c",
		"UPPER_case":  "upper-case",
		"  trim  ":    "trim",
		"!!!":         "customer",
		"good-name-1": "good-name-1",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildActivation_ZipPathsHaveNoTraversal(t *testing.T) {
	req := &models.ActivationRequest{
		Customer: models.CustomerDetails{CustomerName: "../../etc"},
	}
	resources := map[models.FeatureID][]kcc.Resource{
		models.FeatureBootstrapOrg: {{Name: "folder.yaml", Content: []byte("kind: Folder\n")}},
	}
	files, err := NewActivationBuilder().BuildActivation(req, resources)
	require.NoError(t, err)
	for _, f := range files {
		if strings.Contains(f.Path, "..") {
			t.Errorf("zip entry path contains traversal: %q", f.Path)
		}
	}
}
