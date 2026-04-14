package kcc

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// FeatureBuilder generates KCC resources for a specific activation feature.
type FeatureBuilder interface {
	FeatureID() models.FeatureID
	Build(config interface{}) ([]Resource, error)
}

// FeatureBuilderRegistry holds all registered feature builders.
type FeatureBuilderRegistry struct {
	builders map[models.FeatureID]FeatureBuilder
}

// NewFeatureBuilderRegistry creates a registry with all feature builders registered.
func NewFeatureBuilderRegistry() *FeatureBuilderRegistry {
	r := &FeatureBuilderRegistry{
		builders: make(map[models.FeatureID]FeatureBuilder),
	}
	r.Register(NewBootstrapOrgBuilder())
	r.Register(NewBigQueryAnalyticsBuilder())
	r.Register(NewDeveloperPortalBuilder())
	r.Register(NewHardenedImageBakeryBuilder())
	r.Register(NewSecureInferencingBuilder())
	r.Register(NewSkaffoldAppDevBuilder())
	return r
}

// Register adds a feature builder to the registry.
func (r *FeatureBuilderRegistry) Register(fb FeatureBuilder) {
	r.builders[fb.FeatureID()] = fb
}

// Build generates KCC resources for the given feature and config.
func (r *FeatureBuilderRegistry) Build(featureID models.FeatureID, config interface{}) ([]Resource, error) {
	fb, ok := r.builders[featureID]
	if !ok {
		return nil, fmt.Errorf("no builder registered for feature %q", featureID)
	}
	return fb.Build(config)
}

// BuildAll generates KCC resources for all enabled features in an activation request.
func (r *FeatureBuilderRegistry) BuildAll(req *models.ActivationRequest) (map[models.FeatureID][]Resource, error) {
	result := make(map[models.FeatureID][]Resource)
	for _, f := range req.Features {
		if !f.Enabled {
			continue
		}
		resources, err := r.Build(f.FeatureID, f.Config)
		if err != nil {
			return nil, fmt.Errorf("building feature %q: %w", f.FeatureID, err)
		}
		result[f.FeatureID] = resources
	}
	return result, nil
}

// renderTemplate is a shared helper for feature builders.
func renderTemplate(name, tmplStr string, data interface{}) ([]byte, error) {
	tmpl, err := template.New(name).Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parsing template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// templateFuncs provides common functions available in all KCC templates.
var templateFuncs = template.FuncMap{
	"join":  strings.Join,
	"lower": strings.ToLower,
	"splitCSV": func(s string) []string {
		parts := strings.Split(s, ",")
		var result []string
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				result = append(result, t)
			}
		}
		return result
	},
	"default": func(def, val string) string {
		if val == "" {
			return def
		}
		return val
	},
	"list": func(items ...string) []string { return items },
}
