package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

func TestFeatureRegistry_CoversAllFeatureIDs(t *testing.T) {
	registered := map[models.FeatureID]bool{}
	for _, meta := range models.FeatureRegistry() {
		assert.False(t, registered[meta.ID], "duplicate registry entry for %s", meta.ID)
		registered[meta.ID] = true
	}

	for _, id := range models.AllFeatureIDs() {
		assert.True(t, registered[id], "feature %s missing from registry", id)
	}
	assert.Len(t, registered, len(models.AllFeatureIDs()))
}

func TestFeatureRegistry_DependenciesAreKnownFeatures(t *testing.T) {
	known := map[models.FeatureID]bool{}
	for _, id := range models.AllFeatureIDs() {
		known[id] = true
	}

	for _, meta := range models.FeatureRegistry() {
		for _, dep := range meta.DependsOn {
			assert.True(t, known[dep], "feature %s depends on unknown feature %s", meta.ID, dep)
			assert.NotEqual(t, meta.ID, dep, "feature %s depends on itself", meta.ID)
		}
	}
}

func TestFeatureRegistry_MetadataComplete(t *testing.T) {
	for _, meta := range models.FeatureRegistry() {
		assert.NotEmpty(t, meta.Name, "feature %s has no name", meta.ID)
		assert.NotEmpty(t, meta.Description, "feature %s has no description", meta.ID)
		assert.NotEmpty(t, meta.Category, "feature %s has no category", meta.ID)
	}
}
