package helm

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/aillion-co/infrastructure-lz/internal/generator/kcc"
	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// ActivationBuilder generates a complete Helm chart package for an activation
// containing sub-charts per enabled feature.
type ActivationBuilder struct{}

// NewActivationBuilder creates an ActivationBuilder.
func NewActivationBuilder() *ActivationBuilder {
	return &ActivationBuilder{}
}

// BuildActivation generates a Helm chart with sub-charts for each feature.
func (b *ActivationBuilder) BuildActivation(req *models.ActivationRequest, resourcesByFeature map[models.FeatureID][]kcc.Resource) ([]FileEntry, error) {
	chartName := fmt.Sprintf("%s-activation", slugify(req.Customer.CustomerName))
	var files []FileEntry

	// Sort feature IDs so the generated chart bytes are deterministic
	// regardless of map iteration order.
	featureIDs := make([]models.FeatureID, 0, len(resourcesByFeature))
	for fid := range resourcesByFeature {
		featureIDs = append(featureIDs, fid)
	}
	sort.Slice(featureIDs, func(i, j int) bool { return featureIDs[i] < featureIDs[j] })

	// Root Chart.yaml (umbrella chart)
	var deps []string
	for _, fid := range featureIDs {
		deps = append(deps, fmt.Sprintf(`    - name: %s
      version: "0.1.0"
      condition: %s.enabled`, string(fid), sanitizeHelmKey(string(fid))))
	}

	customer := req.Customer.CustomerName
	chartYAML := fmt.Sprintf(`apiVersion: v2
name: %s
description: %s
type: application
version: 0.1.0
appVersion: "1.0.0"
dependencies:
%s
`, chartName, yamlQuote("GCP Landing Zone activation for "+customer), strings.Join(deps, "\n"))

	files = append(files, FileEntry{
		Path:    path.Join(chartName, "Chart.yaml"),
		Content: []byte(chartYAML),
	})

	// Root values.yaml with feature toggles and cross-references
	var valuesLines []string
	valuesLines = append(valuesLines, fmt.Sprintf("# Activation values for %s", chartName))
	valuesLines = append(valuesLines, fmt.Sprintf("customerName: %s", yamlQuote(customer)))
	valuesLines = append(valuesLines, "")
	for _, fid := range featureIDs {
		key := sanitizeHelmKey(string(fid))
		valuesLines = append(valuesLines, fmt.Sprintf("%s:", key))
		valuesLines = append(valuesLines, "  enabled: true")
		valuesLines = append(valuesLines, "")
	}

	files = append(files, FileEntry{
		Path:    path.Join(chartName, "values.yaml"),
		Content: []byte(strings.Join(valuesLines, "\n") + "\n"),
	})

	// Per-feature sub-charts
	for _, fid := range featureIDs {
		resources := resourcesByFeature[fid]
		subChartName := string(fid)
		subChartPath := path.Join(chartName, "charts", subChartName)

		// Sub-chart Chart.yaml
		subChartYAML := fmt.Sprintf(`apiVersion: v2
name: %s
description: KCC resources for %s
type: application
version: 0.1.0
appVersion: "1.0.0"
`, subChartName, featureDisplayName(fid))

		files = append(files, FileEntry{
			Path:    path.Join(subChartPath, "Chart.yaml"),
			Content: []byte(subChartYAML),
		})

		// Sub-chart values.yaml
		subValues := fmt.Sprintf("# Default values for %s\nenabled: true\n", subChartName)
		files = append(files, FileEntry{
			Path:    path.Join(subChartPath, "values.yaml"),
			Content: []byte(subValues),
		})

		// Sub-chart _helpers.tpl
		helpers := fmt.Sprintf(`{{- define "%s.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "%s.labels" -}}
app.kubernetes.io/name: {{ include "%s.name" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: %s
feature: %s
{{- end }}
`, subChartName, subChartName, subChartName, chartName, subChartName)

		files = append(files, FileEntry{
			Path:    path.Join(subChartPath, "templates", "_helpers.tpl"),
			Content: []byte(helpers),
		})

		// KCC resource files in templates/. These are static manifests, but
		// Helm treats every file under templates/ as a Go template, so any
		// literal "{{"/"}}" in the content (e.g. Backstage scaffolder
		// "${{ ... }}" syntax) must be escaped or `helm install`/`helm
		// template` fails. Helm renders the escape back to the literal.
		for _, res := range resources {
			files = append(files, FileEntry{
				Path:    path.Join(subChartPath, "templates", res.Name),
				Content: []byte(escapeHelmDelimiters(string(res.Content))),
			})
		}
	}

	return files, nil
}

// slugify converts a customer name into a filesystem- and YAML-safe slug.
// It keeps only lowercase letters, digits, and hyphens so the result can be
// used both as a zip entry path (no "/" or ".." — prevents zip-slip) and as
// an unquoted YAML scalar. Runs of unsafe characters collapse to a single
// hyphen and leading/trailing hyphens are trimmed.
func slugify(name string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "customer"
	}
	return s
}

func sanitizeHelmKey(featureID string) string {
	return strings.ReplaceAll(featureID, "-", "_")
}

// helmDelimiterEscaper rewrites literal Go-template delimiters into Helm
// escape expressions. strings.Replacer does not re-scan its own output, so
// the "}}" introduced by the "{{" replacement is not double-escaped.
var helmDelimiterEscaper = strings.NewReplacer(
	"{{", `{{ "{{" }}`,
	"}}", `{{ "}}" }}`,
)

// escapeHelmDelimiters escapes "{{"/"}}" so static content placed under a
// Helm chart's templates/ directory renders back to the original literal.
func escapeHelmDelimiters(s string) string {
	if !strings.Contains(s, "{{") && !strings.Contains(s, "}}") {
		return s
	}
	return helmDelimiterEscaper.Replace(s)
}

// yamlQuote renders a string as a safe double-quoted YAML scalar so that
// user-supplied values cannot alter document structure.
func yamlQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func featureDisplayName(fid models.FeatureID) string {
	for _, meta := range models.FeatureRegistry() {
		if meta.ID == fid {
			return meta.Name
		}
	}
	return string(fid)
}
