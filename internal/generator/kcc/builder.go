package kcc

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// Resource represents a generated KCC YAML manifest.
type Resource struct {
	Name    string
	Content []byte
}

type Builder struct {
	templates *template.Template
}

func NewBuilder() *Builder {
	tmpl := template.Must(template.New("kcc").Parse(""))

	// Register all KCC resource templates
	for name, content := range resourceTemplates {
		template.Must(tmpl.New(name).Parse(content))
	}

	return &Builder{templates: tmpl}
}

func (b *Builder) Build(cfg *models.ProjectConfig) ([]Resource, error) {
	var resources []Resource

	// Always generate project and network resources
	projectRes, err := b.render("project.yaml", cfg)
	if err != nil {
		return nil, fmt.Errorf("rendering project: %w", err)
	}
	resources = append(resources, Resource{Name: "project.yaml", Content: projectRes})

	networkRes, err := b.render("network.yaml", cfg)
	if err != nil {
		return nil, fmt.Errorf("rendering network: %w", err)
	}
	resources = append(resources, Resource{Name: "network.yaml", Content: networkRes})

	subnetRes, err := b.render("subnet.yaml", cfg)
	if err != nil {
		return nil, fmt.Errorf("rendering subnet: %w", err)
	}
	resources = append(resources, Resource{Name: "subnet.yaml", Content: subnetRes})

	// Conditionally generate GKE resources
	if cfg.GKE != nil {
		gkeRes, err := b.render("gke.yaml", cfg)
		if err != nil {
			return nil, fmt.Errorf("rendering GKE: %w", err)
		}
		resources = append(resources, Resource{Name: "gke.yaml", Content: gkeRes})
	}

	// Conditionally generate CloudSQL resources
	if cfg.CloudSQL != nil {
		sqlRes, err := b.render("cloudsql.yaml", cfg)
		if err != nil {
			return nil, fmt.Errorf("rendering CloudSQL: %w", err)
		}
		resources = append(resources, Resource{Name: "cloudsql.yaml", Content: sqlRes})
	}

	// IAM resources
	iamRes, err := b.render("iam.yaml", cfg)
	if err != nil {
		return nil, fmt.Errorf("rendering IAM: %w", err)
	}
	resources = append(resources, Resource{Name: "iam.yaml", Content: iamRes})

	return resources, nil
}

func (b *Builder) render(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := b.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("executing template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}
