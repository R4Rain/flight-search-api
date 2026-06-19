package aggregator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/service/flight-search/internal/cache"
	"github.com/service/flight-search/internal/model"
	"github.com/service/flight-search/internal/provider"
)

// Aggregator queries multiple providers in parallel and merges results.
type Aggregator struct {
	providers []provider.FlightProvider
	cache     *cache.FlightCache
	timeout   time.Duration
}

func New(providers []provider.FlightProvider, c *cache.FlightCache, timeout time.Duration) *Aggregator {
	return &Aggregator{
		providers: providers,
		cache:     c,
		timeout:   timeout,
	}
}

type providerResult struct {
	name    string
	flights []model.Flight
	err     error
}

// LegResult holds the result of searching one leg (segment).
type LegResult struct {
	Flights  []model.Flight
	Metadata model.Metadata
	CacheHit bool
}

// Search executes a single-leg search (used for one-way and internally by SearchLegs).
func (a *Aggregator) Search(ctx context.Context, req model.SearchRequest) ([]model.Flight, model.Metadata, bool) {
	cacheKey := cache.Key(req)

	if flights, meta, ok := a.cache.Get(cacheKey); ok {
		slog.Info("cache hit", "key", cacheKey)
		return flights, meta, true
	}

	start := time.Now()
	results := a.queryProviders(ctx, req)

	var allFlights []model.Flight
	var failedProviders []string
	succeeded := 0

	for _, r := range results {
		if r.err != nil {
			slog.Warn("provider failed",
				"provider", r.name,
				"error", r.err.Error(),
			)
			failedProviders = append(failedProviders, r.name)
			continue
		}
		slog.Info("provider succeeded",
			"provider", r.name,
			"flights", len(r.flights),
		)
		allFlights = append(allFlights, r.flights...)
		succeeded++
	}

	elapsed := time.Since(start)

	meta := model.Metadata{
		TotalResults:       len(allFlights),
		ProvidersQueried:   len(a.providers),
		ProvidersSucceeded: succeeded,
		ProvidersFailed:    len(failedProviders),
		FailedProviders:    failedProviders,
		SearchTimeMs:       elapsed.Milliseconds(),
		CacheHit:           false,
	}

	if succeeded > 0 {
		a.cache.Set(cacheKey, allFlights, meta)
	}

	return allFlights, meta, false
}

// SearchLegs executes multiple legs in parallel, returning per-leg results.
func (a *Aggregator) SearchLegs(ctx context.Context, legs []model.SearchRequest) []LegResult {
	results := make([]LegResult, len(legs))
	var wg sync.WaitGroup

	for i, leg := range legs {
		wg.Add(1)
		go func(idx int, legReq model.SearchRequest) {
			defer wg.Done()
			flights, meta, cacheHit := a.Search(ctx, legReq)
			results[idx] = LegResult{
				Flights:  flights,
				Metadata: meta,
				CacheHit: cacheHit,
			}
		}(i, leg)
	}

	wg.Wait()
	return results
}

// BuildLegRequest creates a SearchRequest for a specific leg.
func BuildLegRequest(origin, destination, date string, base model.SearchRequest) model.SearchRequest {
	return model.SearchRequest{
		Origin:        origin,
		Destination:   destination,
		DepartureDate: date,
		Passengers:    base.Passengers,
		CabinClass:    base.CabinClass,
		Filters:       base.Filters,
		Sort:          base.Sort,
	}
}

// BuildSegmentMetadata creates a SegmentMetadata from a Metadata and route.
func BuildSegmentMetadata(meta model.Metadata, origin, destination string, filteredCount int, cacheHit bool) model.SegmentMetadata {
	return model.SegmentMetadata{
		Route:              fmt.Sprintf("%s→%s", origin, destination),
		Results:            filteredCount,
		ProvidersSucceeded: meta.ProvidersSucceeded,
		ProvidersFailed:    meta.ProvidersFailed,
		FailedProviders:    meta.FailedProviders,
		CacheHit:           cacheHit,
	}
}

func (a *Aggregator) queryProviders(ctx context.Context, req model.SearchRequest) []providerResult {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	results := make([]providerResult, len(a.providers))
	var wg sync.WaitGroup

	for i, p := range a.providers {
		wg.Add(1)
		go func(idx int, prov provider.FlightProvider) {
			defer wg.Done()
			flights, err := prov.SearchFlights(ctx, req)
			results[idx] = providerResult{
				name:    prov.Name(),
				flights: flights,
				err:     err,
			}
		}(i, p)
	}

	wg.Wait()
	return results
}
