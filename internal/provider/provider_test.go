package provider

import (
	"context"
	"testing"
	"time"

	"github.com/service/flight-search/internal/model"
)

var testReq = model.SearchRequest{
	Origin:        "CGK",
	Destination:   "DPS",
	DepartureDate: "2025-12-15",
	Passengers:    1,
	CabinClass:    "economy",
}

func TestGarudaProvider(t *testing.T) {
	p := NewGarudaProvider()
	if p.Name() != "Garuda Indonesia" {
		t.Errorf("Name() = %q, want %q", p.Name(), "Garuda Indonesia")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	flights, err := p.SearchFlights(ctx, testReq)
	if err != nil {
		t.Fatalf("SearchFlights() error: %v", err)
	}

	// GA400, GA410 are direct CGK→DPS; GA315 is connecting via SUB→DPS
	if len(flights) != 3 {
		t.Fatalf("expected 3 flights, got %d", len(flights))
	}

	// Check GA400 (direct flight)
	ga400 := findFlight(flights, "GA400")
	if ga400 == nil {
		t.Fatal("GA400 not found")
	}
	if ga400.Stops != 0 {
		t.Errorf("GA400 stops = %d, want 0", ga400.Stops)
	}
	if ga400.Price.Amount != 1250000 {
		t.Errorf("GA400 price = %d, want 1250000", ga400.Price.Amount)
	}
	if ga400.Departure.Airport != "CGK" || ga400.Arrival.Airport != "DPS" {
		t.Errorf("GA400 route = %s→%s, want CGK→DPS", ga400.Departure.Airport, ga400.Arrival.Airport)
	}

	// Check GA315 (connecting flight: CGK→SUB→DPS)
	ga315 := findFlight(flights, "GA315")
	if ga315 == nil {
		t.Fatal("GA315 not found")
	}
	if ga315.Stops != 1 {
		t.Errorf("GA315 stops = %d, want 1 (inferred from segments)", ga315.Stops)
	}
	if ga315.Arrival.Airport != "DPS" {
		t.Errorf("GA315 arrival = %s, want DPS (last segment)", ga315.Arrival.Airport)
	}
	if ga315.Duration.TotalMinutes <= 0 {
		t.Errorf("GA315 duration = %d, want > 0", ga315.Duration.TotalMinutes)
	}
}

func TestLionAirProvider(t *testing.T) {
	p := NewLionAirProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	flights, err := p.SearchFlights(ctx, testReq)
	if err != nil {
		t.Fatalf("SearchFlights() error: %v", err)
	}

	if len(flights) != 3 {
		t.Fatalf("expected 3 flights, got %d", len(flights))
	}

	// Check JT740 (direct)
	jt740 := findFlight(flights, "JT740")
	if jt740 == nil {
		t.Fatal("JT740 not found")
	}
	if jt740.Stops != 0 {
		t.Errorf("JT740 stops = %d, want 0", jt740.Stops)
	}
	if jt740.Price.Amount != 950000 {
		t.Errorf("JT740 price = %d, want 950000", jt740.Price.Amount)
	}
	if jt740.CabinClass != "economy" {
		t.Errorf("JT740 cabin = %q, want economy", jt740.CabinClass)
	}

	// Check JT650 (1 stop)
	jt650 := findFlight(flights, "JT650")
	if jt650 == nil {
		t.Fatal("JT650 not found")
	}
	if jt650.Stops != 1 {
		t.Errorf("JT650 stops = %d, want 1", jt650.Stops)
	}
}

func TestBatikAirProvider(t *testing.T) {
	p := NewBatikAirProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	flights, err := p.SearchFlights(ctx, testReq)
	if err != nil {
		t.Fatalf("SearchFlights() error: %v", err)
	}

	if len(flights) != 3 {
		t.Fatalf("expected 3 flights, got %d", len(flights))
	}

	// Check ID6514 - uses totalPrice (base + taxes)
	id6514 := findFlight(flights, "ID6514")
	if id6514 == nil {
		t.Fatal("ID6514 not found")
	}
	if id6514.Price.Amount != 1100000 {
		t.Errorf("ID6514 price = %d, want 1100000 (totalPrice)", id6514.Price.Amount)
	}
	if id6514.CabinClass != "economy" {
		t.Errorf("ID6514 cabin = %q, want economy (normalized from Y)", id6514.CabinClass)
	}
	// Verify datetime offset was properly reformatted
	if id6514.Departure.Datetime == "" {
		t.Error("ID6514 departure datetime is empty")
	}
}

func TestAirAsiaProvider(t *testing.T) {
	p := NewAirAsiaProvider(3)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// May need multiple attempts due to 10% failure rate
	var flights []model.Flight
	var err error
	flights, err = p.SearchFlights(ctx, testReq)
	if err != nil {
		t.Fatalf("SearchFlights() error after retries: %v", err)
	}

	if len(flights) != 4 {
		t.Fatalf("expected 4 flights, got %d", len(flights))
	}

	// Check QZ520 (direct, cheapest)
	qz520 := findFlight(flights, "QZ520")
	if qz520 == nil {
		t.Fatal("QZ520 not found")
	}
	if qz520.Price.Amount != 650000 {
		t.Errorf("QZ520 price = %d, want 650000", qz520.Price.Amount)
	}
	if qz520.Airline.Code != "QZ" {
		t.Errorf("QZ520 airline code = %q, want QZ", qz520.Airline.Code)
	}

	// Check QZ7250 (1 stop)
	qz7250 := findFlight(flights, "QZ7250")
	if qz7250 == nil {
		t.Fatal("QZ7250 not found")
	}
	if qz7250.Stops != 1 {
		t.Errorf("QZ7250 stops = %d, want 1", qz7250.Stops)
	}
}

func TestAirportCityLookup(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"CGK", "Jakarta"},
		{"DPS", "Denpasar"},
		{"SUB", "Surabaya"},
		{"UPG", "Makassar"},
		{"XXX", "XXX"}, // unknown returns code
	}

	for _, tt := range tests {
		got := CityForAirport(tt.code)
		if got != tt.want {
			t.Errorf("CityForAirport(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func findFlight(flights []model.Flight, flightNumber string) *model.Flight {
	for i := range flights {
		if flights[i].FlightNumber == flightNumber {
			return &flights[i]
		}
	}
	return nil
}
