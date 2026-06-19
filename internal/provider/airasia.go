package provider

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/service/flight-search/internal/model"
)

//go:embed mock/airasia_search_response.json
var airasiaData embed.FS

type airasiaResponse struct {
	Status  string          `json:"status"`
	Flights []airasiaFlight `json:"flights"`
}

type airasiaFlight struct {
	FlightCode   string        `json:"flight_code"`
	Airline      string        `json:"airline"`
	FromAirport  string        `json:"from_airport"`
	ToAirport    string        `json:"to_airport"`
	DepartTime   string        `json:"depart_time"`
	ArriveTime   string        `json:"arrive_time"`
	DurationHrs  float64       `json:"duration_hours"`
	DirectFlight bool          `json:"direct_flight"`
	Stops        []airasiaStop `json:"stops,omitempty"`
	PriceIDR     int64         `json:"price_idr"`
	Seats        int           `json:"seats"`
	CabinClass   string        `json:"cabin_class"`
	BaggageNote  string        `json:"baggage_note"`
}

type airasiaStop struct {
	Airport         string `json:"airport"`
	WaitTimeMinutes int    `json:"wait_time_minutes"`
}

type AirAsiaProvider struct {
	maxRetries int
}

func NewAirAsiaProvider(maxRetries int) *AirAsiaProvider {
	return &AirAsiaProvider{maxRetries: maxRetries}
}

func (a *AirAsiaProvider) Name() string {
	return "AirAsia"
}

func (a *AirAsiaProvider) SearchFlights(ctx context.Context, req model.SearchRequest) ([]model.Flight, error) {
	var lastErr error

	for attempt := 0; attempt <= a.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 100ms, 200ms, 400ms...
			backoff := time.Duration(100*(1<<(attempt-1))) * time.Millisecond
			backoffTimer := time.NewTimer(backoff)
			select {
			case <-backoffTimer.C:
			case <-ctx.Done():
				backoffTimer.Stop()
				return nil, ctx.Err()
			}
		}

		flights, err := a.doSearch(ctx, req)
		if err == nil {
			return flights, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("airasia failed after %d retries: %w", a.maxRetries, lastErr)
}

func (a *AirAsiaProvider) doSearch(ctx context.Context, req model.SearchRequest) ([]model.Flight, error) {
	// Simulate 50-150ms delay
	delay := time.Duration(50+rand.Intn(101)) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Simulate 10% failure rate
	if rand.Float64() < 0.10 {
		return nil, fmt.Errorf("airasia API: simulated transient error")
	}

	data, err := airasiaData.ReadFile("mock/airasia_search_response.json")
	if err != nil {
		return nil, fmt.Errorf("reading airasia mock data: %w", err)
	}

	var resp airasiaResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing airasia response: %w", err)
	}

	if resp.Status != "ok" {
		return nil, fmt.Errorf("airasia API returned status: %s", resp.Status)
	}

	var flights []model.Flight
	for _, f := range resp.Flights {
		flight, err := a.normalize(f, req)
		if err != nil {
			continue
		}
		flights = append(flights, flight)
	}
	return flights, nil
}

func (a *AirAsiaProvider) normalize(f airasiaFlight, req model.SearchRequest) (model.Flight, error) {

	depParsed, err := time.Parse(time.RFC3339, f.DepartTime)
	if err != nil {
		return model.Flight{}, fmt.Errorf("parsing departure time: %w", err)
	}
	arrParsed, err := time.Parse(time.RFC3339, f.ArriveTime)
	if err != nil {
		return model.Flight{}, fmt.Errorf("parsing arrival time: %w", err)
	}

	if !arrParsed.After(depParsed) {
		return model.Flight{}, fmt.Errorf("arrival before departure for %s", f.FlightCode)
	}

	// Use actual timestamp difference for duration rather than the float field
	totalMinutes := int(math.Round(arrParsed.Sub(depParsed).Minutes()))

	depTime := depParsed.Format(time.RFC3339)
	arrTime := arrParsed.Format(time.RFC3339)

	stops := 0
	if !f.DirectFlight {
		stops = len(f.Stops)
		if stops == 0 {
			stops = 1
		}
	}

	carryOn, checked := parseAirAsiaBaggage(f.BaggageNote)

	return model.Flight{
		ID:             fmt.Sprintf("%s_%s", f.FlightCode, "AirAsia"),
		Provider:       "AirAsia",
		Airline:        model.AirlineInfo{Name: f.Airline, Code: "QZ"},
		FlightNumber:   f.FlightCode,
		Departure:      model.AirportTime{Airport: f.FromAirport, City: CityForAirport(f.FromAirport), Datetime: depTime},
		Arrival:        model.AirportTime{Airport: f.ToAirport, City: CityForAirport(f.ToAirport), Datetime: arrTime},
		Duration:       model.Duration{TotalMinutes: totalMinutes, Formatted: model.FormatDuration(totalMinutes)},
		Stops:          stops,
		Price:          model.Price{Amount: f.PriceIDR, Currency: "IDR", Display: model.FormatIDR(f.PriceIDR)},
		AvailableSeats: f.Seats,
		CabinClass:     strings.ToLower(f.CabinClass),
		Aircraft:       nil,
		Amenities:      []string{},
		Baggage:        model.BaggageInfo{CarryOn: carryOn, Checked: checked},
	}, nil
}

func parseAirAsiaBaggage(note string) (carryOn, checked string) {
	lower := strings.ToLower(note)
	if strings.Contains(lower, "cabin baggage only") {
		carryOn = "Cabin baggage only"
		checked = "Additional fee"
	} else {
		carryOn = note
		checked = ""
	}
	return
}
