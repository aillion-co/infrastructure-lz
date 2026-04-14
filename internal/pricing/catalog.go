package pricing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	billing "cloud.google.com/go/billing/apiv1"
	"cloud.google.com/go/billing/apiv1/billingpb"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// cacheTTL is how long a SKU unit price stays cached before re-fetching.
const cacheTTL = 24 * time.Hour

// skuCacheEntry holds a cached USD-per-base-unit price.
type skuCacheEntry struct {
	usdPerUnit float64
	cachedAt   time.Time
}

// CatalogProvider looks up live list pricing from the GCP Cloud Billing
// Catalog API. It falls back to a static provider on any lookup failure.
type CatalogProvider struct {
	client     *billing.CloudCatalogClient
	fallback   Provider
	serviceIDs map[string]string // display name -> service ID

	mu       sync.Mutex
	skuCache map[string]skuCacheEntry // cache key -> entry
}

// catalogAssumption is a single billable line item that the catalog
// provider knows how to look up. ServiceDisplay matches the GCP service's
// display name (e.g. "Compute Engine"); SkuMatch is a case-insensitive
// substring filter against the SKU description.
type catalogAssumption struct {
	Resource        string
	Description     string
	ServiceDisplay  string
	SkuMatch        string
	MonthlyQuantity float64 // multiplier applied to unit price
	StaticFallback  float64 // used when SKU lookup fails
}

// featureAssumptions maps a feature ID to the set of line items the
// catalog provider can reprice. Only the items listed here are
// dynamically priced; other items in the static cost (such as $0
// placeholders) are preserved as-is.
var featureAssumptions = map[models.FeatureID][]catalogAssumption{
	models.FeatureBootstrapOrg: {
		// Cloud NAT charges per gateway-hour; ~730h/month.
		{Resource: "Cloud NAT", Description: "NAT gateway per environment", ServiceDisplay: "Compute Engine", SkuMatch: "nat gateway", MonthlyQuantity: 730, StaticFallback: 32.12},
	},
	models.FeatureHardenedImageBakery: {
		{Resource: "Artifact Registry", Description: "Image storage (first 500MB free)", ServiceDisplay: "Artifact Registry", SkuMatch: "storage", MonthlyQuantity: 5, StaticFallback: 0.50},
	},
	models.FeatureSecureInferencing: {
		// Cloud Run vCPU-second billing; 1 vCPU * 730h * 3600s/h.
		{Resource: "Cloud Run", Description: "LiteLLM proxy (1 vCPU, 512Mi, min 1 instance)", ServiceDisplay: "Cloud Run", SkuMatch: "cpu allocation time", MonthlyQuantity: 730 * 3600, StaticFallback: 25.00},
		// Secret Manager active version-month.
		{Resource: "Secret Manager", Description: "Master key secret (6 versions)", ServiceDisplay: "Secret Manager", SkuMatch: "active secret version", MonthlyQuantity: 6, StaticFallback: 0.06},
	},
	models.FeatureSkaffoldAppDev: {
		// Cloud SQL PostgreSQL db-f1-micro instance-hour.
		{Resource: "Cloud SQL", Description: "PostgreSQL db-f1-micro (if enabled)", ServiceDisplay: "Cloud SQL", SkuMatch: "db-f1-micro", MonthlyQuantity: 730, StaticFallback: 10.00},
	},
}

// NewCatalogProvider constructs a CatalogProvider, validating that the
// catalog can be queried and caching the service-ID map for the services
// referenced by featureAssumptions. Returns an error if the client cannot
// be constructed or no required services can be discovered.
func NewCatalogProvider(ctx context.Context, fallback Provider, opts ...option.ClientOption) (*CatalogProvider, error) {
	client, err := billing.NewCloudCatalogClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating cloud catalog client: %w", err)
	}

	p := &CatalogProvider{
		client:     client,
		fallback:   fallback,
		serviceIDs: map[string]string{},
		skuCache:   map[string]skuCacheEntry{},
	}

	if err := p.refreshServiceIDs(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("listing services: %w", err)
	}

	slog.InfoContext(ctx, "catalog provider initialized", "services_known", len(p.serviceIDs))
	return p, nil
}

// Close releases the underlying catalog client.
func (p *CatalogProvider) Close() error {
	if p.client == nil {
		return nil
	}
	return p.client.Close()
}

// refreshServiceIDs lists all GCP services and caches the IDs of services
// referenced by featureAssumptions.
func (p *CatalogProvider) refreshServiceIDs(ctx context.Context) error {
	wanted := map[string]bool{}
	for _, items := range featureAssumptions {
		for _, a := range items {
			wanted[a.ServiceDisplay] = true
		}
	}

	it := p.client.ListServices(ctx, &billingpb.ListServicesRequest{})
	for {
		svc, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return err
		}
		if wanted[svc.GetDisplayName()] {
			// service name is "services/{id}"; strip prefix
			p.serviceIDs[svc.GetDisplayName()] = strings.TrimPrefix(svc.GetName(), "services/")
		}
	}
	return nil
}

// EstimateFeature returns a feature cost. It starts from the static
// estimate (so that $0 placeholders and unmodelled items survive),
// then for each known assumption it looks up live pricing and replaces
// the matching line item. Any lookup failure leaves the static value.
func (p *CatalogProvider) EstimateFeature(ctx context.Context, featureID models.FeatureID) FeatureCost {
	cost := p.fallback.EstimateFeature(ctx, featureID)

	assumptions, ok := featureAssumptions[featureID]
	if !ok {
		return cost
	}

	var total float64
	for i := range cost.Items {
		p.applyAssumption(ctx, &cost.Items[i], assumptions)
		total += cost.Items[i].MonthlyUSD
	}
	cost.MonthlyUSD = total
	return cost
}

// applyAssumption finds an assumption that matches the given line item
// (by Resource name) and refreshes its MonthlyUSD from the catalog.
// Returns true if the line item was updated from live data.
func (p *CatalogProvider) applyAssumption(ctx context.Context, item *LineItem, assumptions []catalogAssumption) bool {
	for _, a := range assumptions {
		if a.Resource != item.Resource {
			continue
		}
		unit, err := p.lookupUnitPrice(ctx, a.ServiceDisplay, a.SkuMatch)
		if err != nil {
			slog.WarnContext(ctx, "sku lookup failed, using static fallback",
				"resource", a.Resource, "service", a.ServiceDisplay, "error", err)
			item.MonthlyUSD = a.StaticFallback
			return false
		}
		item.MonthlyUSD = unit * a.MonthlyQuantity
		return true
	}
	return false
}

// lookupUnitPrice returns the USD-per-base-unit price for a SKU whose
// description contains skuMatch (case-insensitive) under the given service.
// Results are cached for cacheTTL.
func (p *CatalogProvider) lookupUnitPrice(ctx context.Context, serviceDisplay, skuMatch string) (float64, error) {
	cacheKey := serviceDisplay + "|" + skuMatch

	p.mu.Lock()
	if entry, ok := p.skuCache[cacheKey]; ok && time.Since(entry.cachedAt) < cacheTTL {
		p.mu.Unlock()
		return entry.usdPerUnit, nil
	}
	p.mu.Unlock()

	serviceID, ok := p.serviceIDs[serviceDisplay]
	if !ok {
		return 0, fmt.Errorf("service %q not found in catalog", serviceDisplay)
	}

	needle := strings.ToLower(skuMatch)
	it := p.client.ListSkus(ctx, &billingpb.ListSkusRequest{
		Parent: "services/" + serviceID,
	})
	for {
		sku, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return 0, err
		}
		if !strings.Contains(strings.ToLower(sku.GetDescription()), needle) {
			continue
		}
		price, ok := firstUnitPrice(sku)
		if !ok {
			continue
		}

		p.mu.Lock()
		p.skuCache[cacheKey] = skuCacheEntry{usdPerUnit: price, cachedAt: time.Now()}
		p.mu.Unlock()
		return price, nil
	}
	return 0, fmt.Errorf("no sku matching %q under %q", skuMatch, serviceDisplay)
}

// firstUnitPrice extracts the first non-zero tier rate from a SKU,
// returning USD-per-usage-unit.
func firstUnitPrice(sku *billingpb.Sku) (float64, bool) {
	for _, info := range sku.GetPricingInfo() {
		expr := info.GetPricingExpression()
		if expr == nil {
			continue
		}
		for _, tier := range expr.GetTieredRates() {
			price := tier.GetUnitPrice()
			if price == nil {
				continue
			}
			usd := float64(price.GetUnits()) + float64(price.GetNanos())/1e9
			if usd > 0 {
				return usd, true
			}
		}
	}
	return 0, false
}
