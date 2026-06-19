package provider

import (
	"context"

	"github.com/service/flight-search/internal/model"
)

// The interface for fetching flights from a provider.
type FlightProvider interface {
	Name() string
	SearchFlights(ctx context.Context, req model.SearchRequest) ([]model.Flight, error)
}
