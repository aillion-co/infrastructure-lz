package pricing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// TestStaticProviderMatchesLegacyEstimates pins the static provider to
// the values originally hardcoded in handlers/cost.go, so a refactor
// can't silently shift the published numbers.
func TestStaticProviderMatchesLegacyEstimates(t *testing.T) {
	p := NewStaticProvider()
	ctx := context.Background()

	cases := []struct {
		id       models.FeatureID
		monthly  float64
		numItems int
	}{
		{models.FeatureBootstrapOrg, 105.12, 5},
		{models.FeatureBigQueryAnalytics, 2.00, 3},
		{models.FeatureDeveloperPortal, 124.70, 3},
		{models.FeatureHardenedImageBakery, 5.50, 3},
		{models.FeatureSecureInferencing, 25.06, 3},
		{models.FeatureSkaffoldAppDev, 178.00, 3},
	}

	for _, c := range cases {
		t.Run(string(c.id), func(t *testing.T) {
			cost := p.EstimateFeature(ctx, c.id)
			assert.Equal(t, c.id, cost.FeatureID)
			assert.NotEmpty(t, cost.FeatureName)
			assert.Len(t, cost.Items, c.numItems)
			assert.InDelta(t, c.monthly, cost.MonthlyUSD, 0.001)
		})
	}
}

func TestStaticProviderUnknownFeature(t *testing.T) {
	p := NewStaticProvider()
	cost := p.EstimateFeature(context.Background(), models.FeatureID("nonexistent"))
	assert.Equal(t, 0.0, cost.MonthlyUSD)
	assert.Empty(t, cost.Items)
}
