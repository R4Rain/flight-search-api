package handler

import (
	"fmt"
	"time"

	"github.com/service/flight-search/internal/model"
)

func validateRequest(req model.SearchRequest) error {
	tripType := req.EffectiveTripType()

	validTripTypes := map[string]bool{
		"one_way": true, "round_trip": true, "multi_city": true,
	}
	if !validTripTypes[tripType] {
		return fmt.Errorf("tripType must be one of: one_way, round_trip, multi_city")
	}

	if req.Passengers < 1 || req.Passengers > 9 {
		return fmt.Errorf("passengers must be between 1 and 9")
	}

	validClasses := map[string]bool{
		"economy": true, "business": true, "first": true, "premium_economy": true,
	}
	if !validClasses[req.CabinClass] {
		return fmt.Errorf("cabinClass must be one of: economy, business, first, premium_economy")
	}

	switch tripType {
	case "one_way":
		return validateOneWay(req)
	case "round_trip":
		return validateRoundTrip(req)
	case "multi_city":
		return validateMultiCity(req)
	default:
		return nil
	}
}

func validateOneWay(req model.SearchRequest) error {
	if err := validateRoute(req.Origin, req.Destination); err != nil {
		return err
	}
	if _, err := time.Parse("2006-01-02", req.DepartureDate); err != nil {
		return fmt.Errorf("departureDate must be in YYYY-MM-DD format")
	}
	return validateFilters(req.Filters)
}

func validateRoundTrip(req model.SearchRequest) error {
	if err := validateRoute(req.Origin, req.Destination); err != nil {
		return err
	}
	depDate, err := time.Parse("2006-01-02", req.DepartureDate)
	if err != nil {
		return fmt.Errorf("departureDate must be in YYYY-MM-DD format")
	}
	if req.ReturnDate == nil {
		return fmt.Errorf("returnDate is required for round_trip")
	}
	retDate, err := time.Parse("2006-01-02", *req.ReturnDate)
	if err != nil {
		return fmt.Errorf("returnDate must be in YYYY-MM-DD format")
	}
	if !retDate.After(depDate) {
		return fmt.Errorf("returnDate must be after departureDate")
	}
	return validateFilters(req.Filters)
}

func validateMultiCity(req model.SearchRequest) error {
	if len(req.Segments) < 2 {
		return fmt.Errorf("multi_city requires at least 2 segments")
	}
	if len(req.Segments) > 6 {
		return fmt.Errorf("multi_city allows at most 6 segments")
	}

	var prevDate time.Time
	for i, seg := range req.Segments {
		if err := validateRoute(seg.Origin, seg.Destination); err != nil {
			return fmt.Errorf("segment %d: %w", i, err)
		}
		segDate, err := time.Parse("2006-01-02", seg.DepartureDate)
		if err != nil {
			return fmt.Errorf("segment %d: departureDate must be in YYYY-MM-DD format", i)
		}
		if i > 0 && segDate.Before(prevDate) {
			return fmt.Errorf("segment %d: departureDate must be on or after segment %d (%s)", i, i-1, prevDate.Format("2006-01-02"))
		}
		prevDate = segDate
	}

	return validateFilters(req.Filters)
}

func validateRoute(origin, destination string) error {
	if len(origin) != 3 {
		return fmt.Errorf("origin must be a 3-letter airport code")
	}
	if len(destination) != 3 {
		return fmt.Errorf("destination must be a 3-letter airport code")
	}
	if origin == destination {
		return fmt.Errorf("origin and destination must be different")
	}
	return nil
}

func validateFilters(f *model.SearchFilters) error {
	if f == nil {
		return nil
	}

	if f.MinPrice != nil && *f.MinPrice < 0 {
		return fmt.Errorf("minPrice must be non-negative")
	}
	if f.MaxPrice != nil && *f.MaxPrice < 0 {
		return fmt.Errorf("maxPrice must be non-negative")
	}
	if f.MinPrice != nil && f.MaxPrice != nil && *f.MinPrice > *f.MaxPrice {
		return fmt.Errorf("minPrice must be less than or equal to maxPrice")
	}
	if f.MaxStops != nil && *f.MaxStops < 0 {
		return fmt.Errorf("maxStops must be non-negative")
	}
	if f.MaxDuration != nil && *f.MaxDuration < 0 {
		return fmt.Errorf("maxDuration must be non-negative")
	}

	timeFields := map[string]*string{
		"departureAfter":  f.DepartureAfter,
		"departureBefore": f.DepartureBefore,
		"arrivalAfter":    f.ArrivalAfter,
		"arrivalBefore":   f.ArrivalBefore,
	}
	for name, val := range timeFields {
		if val != nil {
			if _, err := time.Parse("15:04", *val); err != nil {
				return fmt.Errorf("%s must be in HH:MM format", name)
			}
		}
	}

	return nil
}
