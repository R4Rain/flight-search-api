package provider

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/service/flight-search/internal/model"
)

//go:embed mock/lion_air_search_response.json
var lionAirData embed.FS

type lionAirResponse struct {
	Success bool         `json:"success"`
	Data    lionAirData_ `json:"data"`
}

type lionAirData_ struct {
	AvailableFlights []lionAirFlight `json:"available_flights"`
}

type lionAirFlight struct {
	ID         string           `json:"id"`
	Carrier    lionAirCarrier   `json:"carrier"`
	Route      lionAirRoute     `json:"route"`
	Schedule   lionAirSchedule  `json:"schedule"`
	FlightTime int              `json:"flight_time"`
	IsDirect   bool             `json:"is_direct"`
	StopCount  int              `json:"stop_count"`
	Layovers   []lionAirLayover `json:"layovers,omitempty"`
	Pricing    lionAirPricing   `json:"pricing"`
	SeatsLeft  int              `json:"seats_left"`
	PlaneType  string           `json:"plane_type"`
	Services   lionAirServices  `json:"services"`
}

type lionAirCarrier struct {
	Name string `json:"name"`
	IATA string `json:"iata"`
}

type lionAirRoute struct {
	From lionAirAirport `json:"from"`
	To   lionAirAirport `json:"to"`
}

type lionAirAirport struct {
	Code string `json:"code"`
	Name string `json:"name"`
	City string `json:"city"`
}

type lionAirSchedule struct {
	Departure         string `json:"departure"`
	DepartureTimezone string `json:"departure_timezone"`
	Arrival           string `json:"arrival"`
	ArrivalTimezone   string `json:"arrival_timezone"`
}

type lionAirLayover struct {
	Airport         string `json:"airport"`
	DurationMinutes int    `json:"duration_minutes"`
}

type lionAirPricing struct {
	Total    int64  `json:"total"`
	Currency string `json:"currency"`
	FareType string `json:"fare_type"`
}

type lionAirServices struct {
	WifiAvailable    bool           `json:"wifi_available"`
	MealsIncluded    bool           `json:"meals_included"`
	BaggageAllowance lionAirBaggage `json:"baggage_allowance"`
}

type lionAirBaggage struct {
	Cabin string `json:"cabin"`
	Hold  string `json:"hold"`
}

type LionAirProvider struct{}

func NewLionAirProvider() *LionAirProvider {
	return &LionAirProvider{}
}

func (l *LionAirProvider) Name() string {
	return "Lion Air"
}

func (l *LionAirProvider) SearchFlights(ctx context.Context, req model.SearchRequest) ([]model.Flight, error) {
	// Simulate 100-200ms delay
	delay := time.Duration(100+rand.Intn(101)) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	data, err := lionAirData.ReadFile("mock/lion_air_search_response.json")
	if err != nil {
		return nil, fmt.Errorf("reading lion air mock data: %w", err)
	}

	var resp lionAirResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing lion air response: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("lion air API returned failure")
	}

	var flights []model.Flight
	for _, f := range resp.Data.AvailableFlights {
		flight, err := l.normalize(f, req)
		if err != nil {
			continue
		}
		flights = append(flights, flight)
	}
	return flights, nil
}

// parseWithTimezone parses a datetime string without offset and attaches the IANA timezone.
func parseWithTimezone(datetime, tz string) (time.Time, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, fmt.Errorf("loading timezone %s: %w", tz, err)
	}

	// Try with timezone offset first
	if t, err := time.Parse(time.RFC3339, datetime); err == nil {
		return t.In(loc), nil
	}

	// Parse without offset, assume the given timezone
	t, err := time.ParseInLocation("2006-01-02T15:04:05", datetime, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing datetime %s: %w", datetime, err)
	}
	return t, nil
}

func (l *LionAirProvider) normalize(f lionAirFlight, req model.SearchRequest) (model.Flight, error) {
	depParsed, err := parseWithTimezone(f.Schedule.Departure, f.Schedule.DepartureTimezone)
	if err != nil {
		return model.Flight{}, err
	}
	arrParsed, err := parseWithTimezone(f.Schedule.Arrival, f.Schedule.ArrivalTimezone)
	if err != nil {
		return model.Flight{}, err
	}

	if !arrParsed.After(depParsed) {
		return model.Flight{}, fmt.Errorf("arrival before departure for %s", f.ID)
	}

	depTime := depParsed.Format(time.RFC3339)
	arrTime := arrParsed.Format(time.RFC3339)

	stops := 0
	if !f.IsDirect {
		stops = f.StopCount
	}

	totalMinutes := int(arrParsed.Sub(depParsed).Minutes())

	var amenities []string
	if f.Services.WifiAvailable {
		amenities = append(amenities, "wifi")
	}
	if f.Services.MealsIncluded {
		amenities = append(amenities, "meal")
	}
	if amenities == nil {
		amenities = []string{}
	}

	cabinClass := strings.ToLower(f.Pricing.FareType)

	return model.Flight{
		ID:             fmt.Sprintf("%s_%s", f.ID, "LionAir"),
		Provider:       "Lion Air",
		Airline:        model.AirlineInfo{Name: f.Carrier.Name, Code: f.Carrier.IATA},
		FlightNumber:   f.ID,
		Departure:      model.AirportTime{Airport: f.Route.From.Code, City: f.Route.From.City, Datetime: depTime},
		Arrival:        model.AirportTime{Airport: f.Route.To.Code, City: f.Route.To.City, Datetime: arrTime},
		Duration:       model.Duration{TotalMinutes: totalMinutes, Formatted: model.FormatDuration(totalMinutes)},
		Stops:          stops,
		Price:          model.Price{Amount: f.Pricing.Total, Currency: f.Pricing.Currency, Display: model.FormatIDR(f.Pricing.Total)},
		AvailableSeats: f.SeatsLeft,
		CabinClass:     cabinClass,
		Aircraft:       &f.PlaneType,
		Amenities:      amenities,
		Baggage:        model.BaggageInfo{CarryOn: f.Services.BaggageAllowance.Cabin, Checked: f.Services.BaggageAllowance.Hold},
	}, nil
}
