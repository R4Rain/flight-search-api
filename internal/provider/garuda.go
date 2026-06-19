package provider

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/service/flight-search/internal/model"
)

//go:embed mock/garuda_indonesia_search_response.json
var garudaData embed.FS

// garudaResponse mirrors the Garuda Indonesia API JSON structure.
type garudaResponse struct {
	Status  string         `json:"status"`
	Flights []garudaFlight `json:"flights"`
}

type garudaFlight struct {
	FlightID     string          `json:"flight_id"`
	Airline      string          `json:"airline"`
	AirlineCode  string          `json:"airline_code"`
	Departure    garudaAirport   `json:"departure"`
	Arrival      garudaAirport   `json:"arrival"`
	DurationMins int             `json:"duration_minutes"`
	Stops        int             `json:"stops"`
	Aircraft     string          `json:"aircraft"`
	Price        garudaPrice     `json:"price"`
	AvailSeats   int             `json:"available_seats"`
	FareClass    string          `json:"fare_class"`
	Baggage      garudaBaggage   `json:"baggage"`
	Amenities    []string        `json:"amenities"`
	Segments     []garudaSegment `json:"segments,omitempty"`
}

type garudaAirport struct {
	Airport  string `json:"airport"`
	City     string `json:"city"`
	Time     string `json:"time"`
	Terminal string `json:"terminal"`
}

type garudaPrice struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type garudaBaggage struct {
	CarryOn int `json:"carry_on"`
	Checked int `json:"checked"`
}

type garudaSegment struct {
	FlightNumber string             `json:"flight_number"`
	Departure    garudaSegmentPoint `json:"departure"`
	Arrival      garudaSegmentPoint `json:"arrival"`
	DurationMins int                `json:"duration_minutes"`
	LayoverMins  int                `json:"layover_minutes,omitempty"`
}

type garudaSegmentPoint struct {
	Airport string `json:"airport"`
	Time    string `json:"time"`
}

type GarudaProvider struct{}

func NewGarudaProvider() *GarudaProvider {
	return &GarudaProvider{}
}

func (g *GarudaProvider) Name() string {
	return "Garuda Indonesia"
}

func (g *GarudaProvider) SearchFlights(ctx context.Context, req model.SearchRequest) ([]model.Flight, error) {
	// Simulate 50-100ms delay
	delay := time.Duration(50+rand.Intn(51)) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	data, err := garudaData.ReadFile("mock/garuda_indonesia_search_response.json")
	if err != nil {
		return nil, fmt.Errorf("reading garuda mock data: %w", err)
	}

	var resp garudaResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing garuda response: %w", err)
	}

	if resp.Status != "success" {
		return nil, fmt.Errorf("garuda API returned status: %s", resp.Status)
	}

	var flights []model.Flight
	for _, f := range resp.Flights {
		flight, err := g.normalize(f, req)
		if err != nil {
			continue // skip invalid flights
		}
		flights = append(flights, flight)
	}
	return flights, nil
}

func (g *GarudaProvider) normalize(f garudaFlight, req model.SearchRequest) (model.Flight, error) {
	var depAirport, depCity, depTime string
	var arrAirport, arrCity, arrTime string
	var totalMinutes int
	var stops int

	if len(f.Segments) > 1 {
		// Multi-segment: use first segment departure and last segment arrival
		first := f.Segments[0]
		last := f.Segments[len(f.Segments)-1]

		depAirport = first.Departure.Airport
		depTime = first.Departure.Time
		arrAirport = last.Arrival.Airport
		arrTime = last.Arrival.Time
		stops = len(f.Segments) - 1

		depParsed, err := time.Parse(time.RFC3339, depTime)
		if err != nil {
			return model.Flight{}, fmt.Errorf("parsing departure time: %w", err)
		}
		arrParsed, err := time.Parse(time.RFC3339, arrTime)
		if err != nil {
			return model.Flight{}, fmt.Errorf("parsing arrival time: %w", err)
		}
		totalMinutes = int(arrParsed.Sub(depParsed).Minutes())
	} else {
		depAirport = f.Departure.Airport
		depTime = f.Departure.Time
		arrAirport = f.Arrival.Airport
		arrTime = f.Arrival.Time
		stops = f.Stops
		totalMinutes = f.DurationMins
	}

	depCity = f.Departure.City
	if depCity == "" {
		depCity = CityForAirport(depAirport)
	}
	arrCity = f.Arrival.City
	if arrCity == "" || len(f.Segments) > 1 {
		arrCity = CityForAirport(arrAirport)
	}

	// Validate: arrival must be after departure
	depParsed, _ := time.Parse(time.RFC3339, depTime)
	arrParsed, _ := time.Parse(time.RFC3339, arrTime)
	if !arrParsed.After(depParsed) {
		return model.Flight{}, fmt.Errorf("arrival before departure for %s", f.FlightID)
	}

	aircraft := f.Aircraft
	bagCarryOn := fmt.Sprintf("%d pcs", f.Baggage.CarryOn)
	bagChecked := fmt.Sprintf("%d pcs", f.Baggage.Checked)

	amenities := f.Amenities
	if amenities == nil {
		amenities = []string{}
	}

	return model.Flight{
		ID:             fmt.Sprintf("%s_%s", f.FlightID, "GarudaIndonesia"),
		Provider:       "Garuda Indonesia",
		Airline:        model.AirlineInfo{Name: f.Airline, Code: f.AirlineCode},
		FlightNumber:   f.FlightID,
		Departure:      model.AirportTime{Airport: depAirport, City: depCity, Datetime: depTime},
		Arrival:        model.AirportTime{Airport: arrAirport, City: arrCity, Datetime: arrTime},
		Duration:       model.Duration{TotalMinutes: totalMinutes, Formatted: model.FormatDuration(totalMinutes)},
		Stops:          stops,
		Price:          model.Price{Amount: f.Price.Amount, Currency: f.Price.Currency, Display: model.FormatIDR(f.Price.Amount)},
		AvailableSeats: f.AvailSeats,
		CabinClass:     f.FareClass,
		Aircraft:       &aircraft,
		Amenities:      amenities,
		Baggage:        model.BaggageInfo{CarryOn: bagCarryOn, Checked: bagChecked},
	}, nil
}
