package provider

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/service/flight-search/internal/model"
)

//go:embed mock/batik_air_search_response.json
var batikAirData embed.FS

type batikAirResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Results []batikAirFlight `json:"results"`
}

type batikAirFlight struct {
	FlightNumber      string               `json:"flightNumber"`
	AirlineName       string               `json:"airlineName"`
	AirlineIATA       string               `json:"airlineIATA"`
	Origin            string               `json:"origin"`
	Destination       string               `json:"destination"`
	DepartureDateTime string               `json:"departureDateTime"`
	ArrivalDateTime   string               `json:"arrivalDateTime"`
	TravelTime        string               `json:"travelTime"`
	NumberOfStops     int                  `json:"numberOfStops"`
	Connections       []batikAirConnection `json:"connections,omitempty"`
	Fare              batikAirFare         `json:"fare"`
	SeatsAvailable    int                  `json:"seatsAvailable"`
	AircraftModel     string               `json:"aircraftModel"`
	BaggageInfo       string               `json:"baggageInfo"`
	OnboardServices   []string             `json:"onboardServices"`
}

type batikAirConnection struct {
	StopAirport  string `json:"stopAirport"`
	StopDuration string `json:"stopDuration"`
}

type batikAirFare struct {
	BasePrice    int64  `json:"basePrice"`
	Taxes        int64  `json:"taxes"`
	TotalPrice   int64  `json:"totalPrice"`
	CurrencyCode string `json:"currencyCode"`
	Class        string `json:"class"`
}

type BatikAirProvider struct{}

func NewBatikAirProvider() *BatikAirProvider {
	return &BatikAirProvider{}
}

func (b *BatikAirProvider) Name() string {
	return "Batik Air"
}

func (b *BatikAirProvider) SearchFlights(ctx context.Context, req model.SearchRequest) ([]model.Flight, error) {
	// Simulate 200-400ms delay
	delay := time.Duration(200+rand.Intn(201)) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	data, err := batikAirData.ReadFile("mock/batik_air_search_response.json")
	if err != nil {
		return nil, fmt.Errorf("reading batik air mock data: %w", err)
	}

	var resp batikAirResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing batik air response: %w", err)
	}

	if resp.Code != 200 {
		return nil, fmt.Errorf("batik air API returned code: %d", resp.Code)
	}

	var flights []model.Flight
	for _, f := range resp.Results {
		flight, err := b.normalize(f, req)
		if err != nil {
			continue
		}
		flights = append(flights, flight)
	}
	return flights, nil
}

// parseBatikTime handles Batik Air's offset format (+0700 without colon).
func parseBatikTime(dt string) (time.Time, error) {
	// Try standard RFC3339 first
	if t, err := time.Parse(time.RFC3339, dt); err == nil {
		return t, nil
	}
	// Try without colon in offset: 2025-12-15T07:15:00+0700
	t, err := time.Parse("2006-01-02T15:04:05-0700", dt)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing batik air time %s: %w", dt, err)
	}
	return t, nil
}

// parseTravelTime parses strings like "1h 45m" or "3h 5m" into minutes.
func parseTravelTime(s string) (int, error) {
	re := regexp.MustCompile(`(?:(\d+)h)?\s*(?:(\d+)m)?`)
	matches := re.FindStringSubmatch(s)
	if matches == nil || (matches[1] == "" && matches[2] == "") {
		return 0, fmt.Errorf("cannot parse travel time: %q", s)
	}
	hours, minutes := 0, 0
	if matches[1] != "" {
		hours, _ = strconv.Atoi(matches[1])
	}
	if matches[2] != "" {
		minutes, _ = strconv.Atoi(matches[2])
	}
	return hours*60 + minutes, nil
}

// normalizeFareClass converts Batik Air's fare class codes to readable strings.
func normalizeFareClass(class string) string {
	switch strings.ToUpper(class) {
	case "Y":
		return "economy"
	case "C":
		return "business"
	case "F":
		return "first"
	default:
		return strings.ToLower(class)
	}
}

func (b *BatikAirProvider) normalize(f batikAirFlight, req model.SearchRequest) (model.Flight, error) {

	depParsed, err := parseBatikTime(f.DepartureDateTime)
	if err != nil {
		return model.Flight{}, err
	}
	arrParsed, err := parseBatikTime(f.ArrivalDateTime)
	if err != nil {
		return model.Flight{}, err
	}

	if !arrParsed.After(depParsed) {
		return model.Flight{}, fmt.Errorf("arrival before departure for %s", f.FlightNumber)
	}

	totalMinutes := int(arrParsed.Sub(depParsed).Minutes())

	// Also try parsing from the travelTime string as a fallback
	if tt, err := parseTravelTime(f.TravelTime); err == nil && totalMinutes == 0 {
		totalMinutes = tt
	}

	depTime := depParsed.Format(time.RFC3339)
	arrTime := arrParsed.Format(time.RFC3339)

	// Parse baggage info like "7kg cabin, 20kg checked"
	carryOn, checked := parseBaggageInfo(f.BaggageInfo)

	amenities := make([]string, 0, len(f.OnboardServices))
	for _, svc := range f.OnboardServices {
		amenities = append(amenities, strings.ToLower(svc))
	}

	return model.Flight{
		ID:             fmt.Sprintf("%s_%s", f.FlightNumber, "BatikAir"),
		Provider:       "Batik Air",
		Airline:        model.AirlineInfo{Name: f.AirlineName, Code: f.AirlineIATA},
		FlightNumber:   f.FlightNumber,
		Departure:      model.AirportTime{Airport: f.Origin, City: CityForAirport(f.Origin), Datetime: depTime},
		Arrival:        model.AirportTime{Airport: f.Destination, City: CityForAirport(f.Destination), Datetime: arrTime},
		Duration:       model.Duration{TotalMinutes: totalMinutes, Formatted: model.FormatDuration(totalMinutes)},
		Stops:          f.NumberOfStops,
		Price:          model.Price{Amount: f.Fare.TotalPrice, Currency: f.Fare.CurrencyCode, Display: model.FormatIDR(f.Fare.TotalPrice)},
		AvailableSeats: f.SeatsAvailable,
		CabinClass:     normalizeFareClass(f.Fare.Class),
		Aircraft:       &f.AircraftModel,
		Amenities:      amenities,
		Baggage:        model.BaggageInfo{CarryOn: carryOn, Checked: checked},
	}, nil
}

// parseBaggageInfo parses strings like "7kg cabin, 20kg checked".
func parseBaggageInfo(info string) (carryOn, checked string) {
	parts := strings.Split(info, ",")
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if strings.Contains(part, "cabin") {
			carryOn = strings.TrimSpace(part)
		} else if strings.Contains(part, "checked") {
			checked = strings.TrimSpace(part)
		}
	}
	if carryOn == "" {
		carryOn = info
	}
	return
}
