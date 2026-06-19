package model

// SearchRequest represents the incoming search request body.
type SearchRequest struct {
	// Common fields
	Passengers int            `json:"passengers" binding:"required,min=1"`
	CabinClass string         `json:"cabinClass" binding:"required"`
	TripType   string         `json:"tripType,omitempty"` // one_way (default), round_trip, multi_city
	Filters    *SearchFilters `json:"filters,omitempty"`
	Sort       *SortOption    `json:"sort,omitempty"`

	// Fields for one_way and round_trip
	Origin        string  `json:"origin"`
	Destination   string  `json:"destination"`
	DepartureDate string  `json:"departureDate"`
	ReturnDate    *string `json:"returnDate"`

	// Fields for multi_city
	Segments []SearchSegment `json:"segments,omitempty"`
}

// SearchSegment defines a single leg for multi-city trips.
type SearchSegment struct {
	Origin        string `json:"origin" binding:"required"`
	Destination   string `json:"destination" binding:"required"`
	DepartureDate string `json:"departureDate" binding:"required"`
}

// EffectiveTripType returns the resolved trip type, defaulting to one_way.
func (r SearchRequest) EffectiveTripType() string {
	if r.TripType == "" {
		return "one_way"
	}
	return r.TripType
}

type SearchFilters struct {
	MinPrice        *int64   `json:"minPrice,omitempty"`
	MaxPrice        *int64   `json:"maxPrice,omitempty"`
	MaxStops        *int     `json:"maxStops,omitempty"`
	Airlines        []string `json:"airlines,omitempty"`
	DepartureAfter  *string  `json:"departureAfter,omitempty"`
	DepartureBefore *string  `json:"departureBefore,omitempty"`
	ArrivalAfter    *string  `json:"arrivalAfter,omitempty"`
	ArrivalBefore   *string  `json:"arrivalBefore,omitempty"`
	MaxDuration     *int     `json:"maxDuration,omitempty"`
}

type SortOption struct {
	By    string `json:"by" binding:"required,oneof=price duration departure arrival best_value"`
	Order string `json:"order" binding:"required,oneof=asc desc"`
}

// --- Response Types ---

// SearchResponse is the one-way response (backward compatible).
type SearchResponse struct {
	SearchCriteria SearchCriteria `json:"search_criteria"`
	Metadata       Metadata       `json:"metadata"`
	Flights        []Flight       `json:"flights"`
}

// RoundTripResponse is the round-trip response with separate outbound/return arrays.
type RoundTripResponse struct {
	SearchCriteria  SearchCriteria `json:"search_criteria"`
	Metadata        MultiMetadata  `json:"metadata"`
	OutboundFlights []Flight       `json:"outbound_flights"`
	ReturnFlights   []Flight       `json:"return_flights"`
}

// MultiCityResponse is the multi-city response with per-segment results.
type MultiCityResponse struct {
	SearchCriteria MultiCitySearchCriteria  `json:"search_criteria"`
	Metadata       MultiMetadata            `json:"metadata"`
	Segments       map[string]SegmentResult `json:"segments"`
}

// SegmentResult holds the flights for one multi-city/round-trip segment.
type SegmentResult struct {
	Route   string   `json:"route"`
	Flights []Flight `json:"flights"`
}

type SearchCriteria struct {
	Origin        string `json:"origin"`
	Destination   string `json:"destination"`
	DepartureDate string `json:"departure_date"`
	Passengers    int    `json:"passengers"`
	CabinClass    string `json:"cabin_class"`
}

type MultiCitySearchCriteria struct {
	TripType   string          `json:"trip_type"`
	Passengers int             `json:"passengers"`
	CabinClass string          `json:"cabin_class"`
	Segments   []SearchSegment `json:"segments"`
}

type Metadata struct {
	TotalResults       int      `json:"total_results"`
	ProvidersQueried   int      `json:"providers_queried"`
	ProvidersSucceeded int      `json:"providers_succeeded"`
	ProvidersFailed    int      `json:"providers_failed"`
	FailedProviders    []string `json:"failed_providers,omitempty"`
	SearchTimeMs       int64    `json:"search_time_ms"`
	CacheHit           bool     `json:"cache_hit"`
}

// SegmentMetadata holds metadata for a single search leg.
type SegmentMetadata struct {
	Route              string   `json:"route"`
	Results            int      `json:"results"`
	ProvidersSucceeded int      `json:"providers_succeeded"`
	ProvidersFailed    int      `json:"providers_failed"`
	FailedProviders    []string `json:"failed_providers,omitempty"`
	CacheHit           bool     `json:"cache_hit"`
}

// MultiMetadata is the top-level metadata for round-trip and multi-city responses.
type MultiMetadata struct {
	TotalResults     int               `json:"total_results"`
	ProvidersQueried int               `json:"providers_queried"`
	SearchTimeMs     int64             `json:"search_time_ms"`
	Segments         []SegmentMetadata `json:"segments"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
