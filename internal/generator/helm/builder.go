package helm

import (
	"fmt"
	"path"

	"github.com/aillion-co/infrastructure-lz/internal/generator/kcc"
	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// FileEntry represents a file in the Helm chart output.
type FileEntry struct {
	Path    string
	Content []byte
}

type Builder struct{}

func NewBuilder() *Builder {
	return &Builder{}
}

// Build wraps KCC resources into a Helm chart directory structure.
func (b *Builder) Build(cfg *models.ProjectConfig, resources []kcc.Resource) ([]FileEntry, error) {
	chartName := fmt.Sprintf("%s-infra", cfg.ProjectName)
	var files []FileEntry

	// Chart.yaml
	chartYAML := fmt.Sprintf(`apiVersion: v2
name: %s
description: Infrastructure as Code for %s (%s)
type: application
version: 0.1.0
appVersion: "1.0.0"
`, chartName, cfg.ProjectName, cfg.Environment)

	files = append(files, FileEntry{
		Path:    path.Join(chartName, "Chart.yaml"),
		Content: []byte(chartYAML),
	})

	// values.yaml
	valuesYAML := fmt.Sprintf(`# Default values for %s
projectID: %s
region: %s
environment: %s
`, chartName, cfg.ProjectID, cfg.Region, cfg.Environment)

	files = append(files, FileEntry{
		Path:    path.Join(chartName, "values.yaml"),
		Content: []byte(valuesYAML),
	})

	// templates/_helpers.tpl
	helpers := fmt.Sprintf(`{{- define "%s.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "%s.labels" -}}
app.kubernetes.io/name: {{ include "%s.name" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/environment: {{ .Values.environment }}
{{- end }}
`, chartName, chartName, chartName)

	files = append(files, FileEntry{
		Path:    path.Join(chartName, "templates", "_helpers.tpl"),
		Content: []byte(helpers),
	})

	// templates/ — one file per KCC resource
	for _, res := range resources {
		files = append(files, FileEntry{
			Path:    path.Join(chartName, "templates", res.Name),
			Content: res.Content,
		})
	}

	return files, nil
}
