package filter

import (
	"testing"

	"github.com/service/flight-search/internal/model"
)

func sampleFlights() []model.Flight {
	return []model.Flight{
		{
			ID: "f1", FlightNumber: "GA400",
			Airline:   model.AirlineInfo{Name: "Garuda Indonesia", Code: "GA"},
			Departure: model.AirportTime{Datetime: "2025-12-15T06:00:00+07:00"},
			Arrival:   model.AirportTime{Datetime: "2025-12-15T08:50:00+08:00"},
			Duration:  model.Duration{TotalMinutes: 110},
			Stops:     0, Price: model.Price{Amount: 1250000},
		},
		{
			ID: "f2", FlightNumber: "QZ520",
			Airline:   model.AirlineInfo{Name: "AirAsia", Code: "QZ"},
			Departure: model.AirportTime{Datetime: "2025-12-15T04:45:00+07:00"},
			Arrival:   model.AirportTime{Datetime: "2025-12-15T07:25:00+08:00"},
			Duration:  model.Duration{TotalMinutes: 100},
			Stops:     0, Price: model.Price{Amount: 650000},
		},
		{
			ID: "f3", FlightNumber: "JT650",
			Airline:   model.AirlineInfo{Name: "Lion Air", Code: "JT"},
			Departure: model.AirportTime{Datetime: "2025-12-15T16:20:00+07:00"},
			Arrival:   model.AirportTime{Datetime: "2025-12-15T21:10:00+08:00"},
			Duration:  model.Duration{TotalMinutes: 230},
			Stops:     1, Price: model.Price{Amount: 780000},
		},
		{
			ID: "f4", FlightNumber: "QZ532",
			Airline:   model.AirlineInfo{Name: "AirAsia", Code: "QZ"},
			Departure: model.AirportTime{Datetime: "2025-12-15T19:30:00+07:00"},
			Arrival:   model.AirportTime{Datetime: "2025-12-15T22:10:00+08:00"},
			Duration:  model.Duration{TotalMinutes: 100},
			Stops:     0, Price: model.Price{Amount: 595000},
		},
	}
}

func TestFilterByMaxPrice(t *testing.T) {
	maxPrice := int64(800000)
	req := model.SearchRequest{
		Filters: &model.SearchFilters{MaxPrice: &maxPrice},
	}
	result := Apply(sampleFlights(), req)

	for _, f := range result {
		if f.Price.Amount > maxPrice {
			t.Errorf("flight %s has price %d > maxPrice %d", f.FlightNumber, f.Price.Amount, maxPrice)
		}
	}
	if len(result) != 3 {
		t.Errorf("expected 3 flights within budget, got %d", len(result))
	}
}

func TestFilterByMaxStops(t *testing.T) {
	maxStops := 0
	req := model.SearchRequest{
		Filters: &model.SearchFilters{MaxStops: &maxStops},
	}
	result := Apply(sampleFlights(), req)

	for _, f := range result {
		if f.Stops > 0 {
			t.Errorf("flight %s has %d stops, expected direct only", f.FlightNumber, f.Stops)
		}
	}
	if len(result) != 3 {
		t.Errorf("expected 3 direct flights, got %d", len(result))
	}
}

func TestFilterByAirline(t *testing.T) {
	req := model.SearchRequest{
		Filters: &model.SearchFilters{Airlines: []string{"AirAsia"}},
	}
	result := Apply(sampleFlights(), req)

	if len(result) != 2 {
		t.Errorf("expected 2 AirAsia flights, got %d", len(result))
	}
	for _, f := range result {
		if f.Airline.Name != "AirAsia" {
			t.Errorf("expected AirAsia, got %s", f.Airline.Name)
		}
	}
}

func TestFilterByDepartureTime(t *testing.T) {
	after := "06:00"
	before := "17:00"
	req := model.SearchRequest{
		Filters: &model.SearchFilters{DepartureAfter: &after, DepartureBefore: &before},
	}
	result := Apply(sampleFlights(), req)

	// Only GA400 (06:00) and JT650 (16:20) should match
	if len(result) != 2 {
		t.Errorf("expected 2 flights between 06:00-17:00, got %d", len(result))
	}
}

func TestFilterByMaxDuration(t *testing.T) {
	maxDur := 120
	req := model.SearchRequest{
		Filters: &model.SearchFilters{MaxDuration: &maxDur},
	}
	result := Apply(sampleFlights(), req)

	for _, f := range result {
		if f.Duration.TotalMinutes > maxDur {
			t.Errorf("flight %s duration %dm exceeds max %dm", f.FlightNumber, f.Duration.TotalMinutes, maxDur)
		}
	}
}

func TestSortByPrice(t *testing.T) {
	req := model.SearchRequest{
		Sort: &model.SortOption{By: "price", Order: "asc"},
	}
	result := Apply(sampleFlights(), req)

	for i := 1; i < len(result); i++ {
		if result[i].Price.Amount < result[i-1].Price.Amount {
			t.Errorf("flights not sorted by price asc: %d < %d at index %d",
				result[i].Price.Amount, result[i-1].Price.Amount, i)
		}
	}
}

func TestSortByDuration(t *testing.T) {
	req := model.SearchRequest{
		Sort: &model.SortOption{By: "duration", Order: "asc"},
	}
	result := Apply(sampleFlights(), req)

	for i := 1; i < len(result); i++ {
		if result[i].Duration.TotalMinutes < result[i-1].Duration.TotalMinutes {
			t.Errorf("flights not sorted by duration asc at index %d", i)
		}
	}
}

func TestSortByBestValue(t *testing.T) {
	req := model.SearchRequest{
		Sort: &model.SortOption{By: "best_value", Order: "asc"},
	}
	result := Apply(sampleFlights(), req)

	// All flights should have a score
	for _, f := range result {
		if f.Score == nil {
			t.Errorf("flight %s has no best_value score", f.FlightNumber)
		}
	}

	// Verify ascending order
	for i := 1; i < len(result); i++ {
		if *result[i].Score < *result[i-1].Score {
			t.Errorf("flights not sorted by best_value asc at index %d: %f < %f",
				i, *result[i].Score, *result[i-1].Score)
		}
	}

	// QZ532 should be best value (cheapest + short duration + direct)
	if result[0].FlightNumber != "QZ532" {
		t.Errorf("expected QZ532 as best value, got %s (score: %f)", result[0].FlightNumber, *result[0].Score)
	}
}

func TestNoFilterNoSort(t *testing.T) {
	req := model.SearchRequest{}
	flights := sampleFlights()
	result := Apply(flights, req)

	if len(result) != len(flights) {
		t.Errorf("expected all %d flights, got %d", len(flights), len(result))
	}
}

func TestCombinedFiltersAndSort(t *testing.T) {
	maxPrice := int64(1000000)
	maxStops := 0
	req := model.SearchRequest{
		Filters: &model.SearchFilters{MaxPrice: &maxPrice, MaxStops: &maxStops},
		Sort:    &model.SortOption{By: "price", Order: "asc"},
	}
	result := Apply(sampleFlights(), req)

	// Should get QZ532 (595k, 0 stops) and QZ520 (650k, 0 stops); JT650 excluded (1 stop)
	if len(result) != 2 {
		t.Errorf("expected 2 flights, got %d", len(result))
	}
	if len(result) >= 2 && result[0].Price.Amount > result[1].Price.Amount {
		t.Error("flights not sorted by price ascending")
	}
}
