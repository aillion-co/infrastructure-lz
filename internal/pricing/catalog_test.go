package pricing

import (
	"context"
	"net"
	"testing"

	"cloud.google.com/go/billing/apiv1/billingpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/aillion-co/infrastructure-lz/internal/models"
)

// fakeCatalogServer is an in-process Cloud Billing Catalog API backend.
type fakeCatalogServer struct {
	billingpb.UnimplementedCloudCatalogServer

	services []*billingpb.Service
	// skusByService maps a service ID (e.g. "svc-compute") to its SKUs.
	skusByService map[string][]*billingpb.Sku

	listSkusCalls int
}

func (f *fakeCatalogServer) ListServices(ctx context.Context, req *billingpb.ListServicesRequest) (*billingpb.ListServicesResponse, error) {
	return &billingpb.ListServicesResponse{Services: f.services}, nil
}

func (f *fakeCatalogServer) ListSkus(ctx context.Context, req *billingpb.ListSkusRequest) (*billingpb.ListSkusResponse, error) {
	f.listSkusCalls++
	serviceID := req.GetParent()[len("services/"):]
	return &billingpb.ListSkusResponse{Skus: f.skusByService[serviceID]}, nil
}

func sku(description string, units int64, nanos int32) *billingpb.Sku {
	return &billingpb.Sku{
		Description: description,
		PricingInfo: []*billingpb.PricingInfo{{
			PricingExpression: &billingpb.PricingExpression{
				TieredRates: []*billingpb.PricingExpression_TierRate{{
					UnitPrice: &money.Money{CurrencyCode: "USD", Units: units, Nanos: nanos},
				}},
			},
		}},
	}
}

// newTestCatalogProvider serves fake on a local gRPC listener and returns a
// CatalogProvider connected to it.
func newTestCatalogProvider(t *testing.T, fake *fakeCatalogServer) *CatalogProvider {
	t.Helper()

	lis, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	billingpb.RegisterCloudCatalogServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	provider, err := NewCatalogProvider(context.Background(), NewStaticProvider(),
		option.WithEndpoint(lis.Addr().String()),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Close() })
	return provider
}

func defaultFake() *fakeCatalogServer {
	return &fakeCatalogServer{
		services: []*billingpb.Service{
			{Name: "services/svc-compute", DisplayName: "Compute Engine"},
			{Name: "services/svc-run", DisplayName: "Cloud Run"},
			{Name: "services/svc-unrelated", DisplayName: "Cloud Spanner"},
		},
		skusByService: map[string][]*billingpb.Sku{
			// $0.10/h NAT gateway; bootstrap-org multiplies by 730h.
			"svc-compute": {
				sku("Instance core running", 1, 0),
				sku("NAT Gateway uptime", 0, 100_000_000),
			},
		},
	}
}

func TestNewCatalogProvider_CachesWantedServiceIDs(t *testing.T) {
	provider := newTestCatalogProvider(t, defaultFake())

	assert.Equal(t, "svc-compute", provider.serviceIDs["Compute Engine"])
	assert.Equal(t, "svc-run", provider.serviceIDs["Cloud Run"])
	// Services not referenced by any assumption are not cached.
	assert.NotContains(t, provider.serviceIDs, "Cloud Spanner")
}

func TestEstimateFeature_RepricesFromCatalog(t *testing.T) {
	provider := newTestCatalogProvider(t, defaultFake())

	cost := provider.EstimateFeature(context.Background(), models.FeatureBootstrapOrg)

	static := NewStaticProvider().EstimateFeature(context.Background(), models.FeatureBootstrapOrg)
	require.Len(t, cost.Items, len(static.Items))

	var natUSD, totalOthers float64
	for _, item := range cost.Items {
		if item.Resource == "Cloud NAT" {
			natUSD = item.MonthlyUSD
		} else {
			totalOthers += item.MonthlyUSD
		}
	}
	// $0.10/h * 730h = $73.00 live price replaces the $32.12 static value.
	assert.InDelta(t, 73.00, natUSD, 0.001)
	assert.InDelta(t, natUSD+totalOthers, cost.MonthlyUSD, 0.001,
		"feature total must be recomputed from repriced items")
}

func TestEstimateFeature_NoSkuMatch_UsesStaticFallback(t *testing.T) {
	fake := defaultFake()
	fake.skusByService["svc-compute"] = []*billingpb.Sku{sku("Instance core running", 1, 0)}
	provider := newTestCatalogProvider(t, fake)

	cost := provider.EstimateFeature(context.Background(), models.FeatureBootstrapOrg)

	for _, item := range cost.Items {
		if item.Resource == "Cloud NAT" {
			// StaticFallback from the assumption table, not a live price.
			assert.InDelta(t, 32.12, item.MonthlyUSD, 0.001)
			return
		}
	}
	t.Fatal("Cloud NAT line item missing")
}

func TestEstimateFeature_UnknownFeature_ReturnsStaticEstimate(t *testing.T) {
	provider := newTestCatalogProvider(t, defaultFake())

	got := provider.EstimateFeature(context.Background(), models.FeatureDeveloperPortal)
	want := NewStaticProvider().EstimateFeature(context.Background(), models.FeatureDeveloperPortal)
	assert.Equal(t, want, got, "features with no assumptions pass through the static estimate")
}

func TestLookupUnitPrice_CachesAcrossCalls(t *testing.T) {
	fake := defaultFake()
	provider := newTestCatalogProvider(t, fake)

	first := provider.EstimateFeature(context.Background(), models.FeatureBootstrapOrg)
	callsAfterFirst := fake.listSkusCalls
	second := provider.EstimateFeature(context.Background(), models.FeatureBootstrapOrg)

	assert.Equal(t, callsAfterFirst, fake.listSkusCalls,
		"second estimate should be served from the SKU cache")
	assert.Equal(t, first, second)
}

func TestEstimateFeature_ZeroPricedSku_FallsBack(t *testing.T) {
	fake := defaultFake()
	// A SKU that matches but has no non-zero tier is skipped, so the
	// lookup fails and the static fallback applies.
	fake.skusByService["svc-compute"] = []*billingpb.Sku{sku("NAT Gateway uptime", 0, 0)}
	provider := newTestCatalogProvider(t, fake)

	cost := provider.EstimateFeature(context.Background(), models.FeatureBootstrapOrg)
	for _, item := range cost.Items {
		if item.Resource == "Cloud NAT" {
			assert.InDelta(t, 32.12, item.MonthlyUSD, 0.001)
			return
		}
	}
	t.Fatal("Cloud NAT line item missing")
}
