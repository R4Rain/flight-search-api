package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/service/flight-search/internal/aggregator"
	"github.com/service/flight-search/internal/cache"
	"github.com/service/flight-search/internal/model"
	"github.com/service/flight-search/internal/provider"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	providers := []provider.FlightProvider{
		provider.NewGarudaProvider(),
		provider.NewLionAirProvider(),
		provider.NewBatikAirProvider(),
		provider.NewAirAsiaProvider(3),
	}

	flightCache := cache.New(5 * time.Minute)
	agg := aggregator.New(providers, flightCache, 5*time.Second)
	h := NewSearchHandler(agg)

	r := gin.New()
	r.POST("/api/v1/flights/search", h.HandleSearch)
	return r
}

func doRequest(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/flights/search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- One-Way Tests (backward compatible) ---

func TestSearchEndpoint_OneWay_Success(t *testing.T) {
	r := setupRouter()
	w := doRequest(r, `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"cabinClass":"economy"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.SearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.SearchCriteria.Origin != "CGK" {
		t.Errorf("origin = %q, want CGK", resp.SearchCriteria.Origin)
	}
	if resp.Metadata.ProvidersQueried != 4 {
		t.Errorf("providers_queried = %d, want 4", resp.Metadata.ProvidersQueried)
	}
	if resp.Metadata.ProvidersSucceeded < 3 {
		t.Errorf("providers_succeeded = %d, want >= 3", resp.Metadata.ProvidersSucceeded)
	}
	if resp.Metadata.TotalResults == 0 {
		t.Error("expected some flights in results")
	}

	for _, f := range resp.Flights {
		if f.ID == "" || f.Provider == "" || f.Departure.Airport == "" || f.Price.Amount <= 0 {
			t.Error("flight has missing required fields")
		}
	}
}

func TestSearchEndpoint_OneWay_WithFiltersAndSort(t *testing.T) {
	r := setupRouter()
	w := doRequest(r, `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"cabinClass":"economy","filters":{"maxPrice":1000000,"maxStops":0},"sort":{"by":"price","order":"asc"}}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.SearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	for _, f := range resp.Flights {
		if f.Price.Amount > 1000000 {
			t.Errorf("flight %s price %d exceeds maxPrice", f.FlightNumber, f.Price.Amount)
		}
		if f.Stops > 0 {
			t.Errorf("flight %s has %d stops, expected 0", f.FlightNumber, f.Stops)
		}
	}

	for i := 1; i < len(resp.Flights); i++ {
		if resp.Flights[i].Price.Amount < resp.Flights[i-1].Price.Amount {
			t.Errorf("flights not sorted by price asc at index %d", i)
		}
	}
}

func TestSearchEndpoint_OneWay_BackwardCompatible(t *testing.T) {
	r := setupRouter()
	// No tripType field - should default to one_way
	w := doRequest(r, `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"cabinClass":"economy"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp model.SearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Metadata.TotalResults == 0 {
		t.Error("expected flights for backward-compatible one-way request")
	}
}

func TestSearchEndpoint_CacheHit(t *testing.T) {
	r := setupRouter()
	body := `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"cabinClass":"economy"}`

	doRequest(r, body)
	w2 := doRequest(r, body)

	var resp2 model.SearchResponse
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	if !resp2.Metadata.CacheHit {
		t.Error("expected cache hit on second request")
	}
}

// --- Round-Trip Tests ---

func TestSearchEndpoint_RoundTrip_Success(t *testing.T) {
	r := setupRouter()
	w := doRequest(r, `{"tripType":"round_trip","origin":"CGK","destination":"DPS","departureDate":"2025-12-15","returnDate":"2025-12-20","passengers":1,"cabinClass":"economy"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.RoundTripResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse round-trip response: %v", err)
	}

	if len(resp.OutboundFlights) == 0 {
		t.Error("expected outbound flights")
	}
	if len(resp.ReturnFlights) == 0 {
		t.Error("expected return flights")
	}
	if resp.Metadata.TotalResults != len(resp.OutboundFlights)+len(resp.ReturnFlights) {
		t.Errorf("total_results %d != outbound(%d) + return(%d)",
			resp.Metadata.TotalResults, len(resp.OutboundFlights), len(resp.ReturnFlights))
	}
	if len(resp.Metadata.Segments) != 2 {
		t.Fatalf("expected 2 segment metadata entries, got %d", len(resp.Metadata.Segments))
	}
	if resp.Metadata.Segments[0].Route != "CGK→DPS" {
		t.Errorf("segment 0 route = %q, want CGK→DPS", resp.Metadata.Segments[0].Route)
	}
	if resp.Metadata.Segments[1].Route != "DPS→CGK" {
		t.Errorf("segment 1 route = %q, want DPS→CGK", resp.Metadata.Segments[1].Route)
	}
}

func TestSearchEndpoint_RoundTrip_MissingReturnDate(t *testing.T) {
	r := setupRouter()
	w := doRequest(r, `{"tripType":"round_trip","origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"cabinClass":"economy"}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSearchEndpoint_RoundTrip_InvalidReturnDate(t *testing.T) {
	r := setupRouter()
	// Return date before departure
	w := doRequest(r, `{"tripType":"round_trip","origin":"CGK","destination":"DPS","departureDate":"2025-12-15","returnDate":"2025-12-10","passengers":1,"cabinClass":"economy"}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- Multi-City Tests ---

func TestSearchEndpoint_MultiCity_Success(t *testing.T) {
	r := setupRouter()
	w := doRequest(r, `{
"tripType":"multi_city",
"passengers":1,
"cabinClass":"economy",
"segments":[
{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15"},
{"origin":"DPS","destination":"SUB","departureDate":"2025-12-18"},
{"origin":"SUB","destination":"CGK","departureDate":"2025-12-20"}
]
}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.MultiCityResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse multi-city response: %v", err)
	}

	if len(resp.Segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(resp.Segments))
	}
	if resp.Segments["0"].Route != "CGK→DPS" {
		t.Errorf("segment 0 route = %q, want CGK→DPS", resp.Segments["0"].Route)
	}
	if resp.Segments["1"].Route != "DPS→SUB" {
		t.Errorf("segment 1 route = %q, want DPS→SUB", resp.Segments["1"].Route)
	}
	if resp.Segments["2"].Route != "SUB→CGK" {
		t.Errorf("segment 2 route = %q, want SUB→CGK", resp.Segments["2"].Route)
	}

	if len(resp.Metadata.Segments) != 3 {
		t.Fatalf("expected 3 segment metadata entries, got %d", len(resp.Metadata.Segments))
	}
	if resp.Metadata.TotalResults == 0 {
		t.Error("expected some flights total")
	}
	if resp.SearchCriteria.TripType != "multi_city" {
		t.Errorf("trip_type = %q, want multi_city", resp.SearchCriteria.TripType)
	}
}

func TestSearchEndpoint_MultiCity_TooFewSegments(t *testing.T) {
	r := setupRouter()
	w := doRequest(r, `{"tripType":"multi_city","passengers":1,"cabinClass":"economy","segments":[{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15"}]}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSearchEndpoint_MultiCity_TooManySegments(t *testing.T) {
	r := setupRouter()
	w := doRequest(r, `{
"tripType":"multi_city","passengers":1,"cabinClass":"economy",
"segments":[
{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15"},
{"origin":"DPS","destination":"SUB","departureDate":"2025-12-16"},
{"origin":"SUB","destination":"JOG","departureDate":"2025-12-17"},
{"origin":"JOG","destination":"BDO","departureDate":"2025-12-18"},
{"origin":"BDO","destination":"PLM","departureDate":"2025-12-19"},
{"origin":"PLM","destination":"PDG","departureDate":"2025-12-20"},
{"origin":"PDG","destination":"CGK","departureDate":"2025-12-21"}
]
}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for >6 segments, got %d", w.Code)
	}
}

func TestSearchEndpoint_MultiCity_NonChronological(t *testing.T) {
	r := setupRouter()
	w := doRequest(r, `{
"tripType":"multi_city","passengers":1,"cabinClass":"economy",
"segments":[
{"origin":"CGK","destination":"DPS","departureDate":"2025-12-18"},
{"origin":"DPS","destination":"SUB","departureDate":"2025-12-15"}
]
}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-chronological segments, got %d", w.Code)
	}
}

// --- Validation Tests ---

func TestSearchEndpoint_InvalidRequest(t *testing.T) {
	r := setupRouter()

	tests := []struct {
		name string
		body string
	}{
		{"empty body", `{}`},
		{"missing origin", `{"destination":"DPS","departureDate":"2025-12-15","passengers":1,"cabinClass":"economy"}`},
		{"same origin dest", `{"origin":"CGK","destination":"CGK","departureDate":"2025-12-15","passengers":1,"cabinClass":"economy"}`},
		{"invalid date", `{"origin":"CGK","destination":"DPS","departureDate":"invalid","passengers":1,"cabinClass":"economy"}`},
		{"invalid class", `{"origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"cabinClass":"supersonic"}`},
		{"invalid tripType", `{"tripType":"teleport","origin":"CGK","destination":"DPS","departureDate":"2025-12-15","passengers":1,"cabinClass":"economy"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doRequest(r, tt.body)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d for %s: %s", w.Code, tt.name, w.Body.String())
			}
		})
	}
}
