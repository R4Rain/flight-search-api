package filter

import (
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/service/flight-search/internal/model"
)

// Applies filters and sorting to a list of flights.
func Apply(flights []model.Flight, req model.SearchRequest) []model.Flight {
	filtered := applyFilters(flights, req.Filters)
	sorted := applySort(filtered, req.Sort)
	return sorted
}

func applyFilters(flights []model.Flight, filters *model.SearchFilters) []model.Flight {
	if filters == nil {
		return flights
	}

	var result []model.Flight
	for _, f := range flights {
		if !matchesFilters(f, filters) {
			continue
		}
		result = append(result, f)
	}
	return result
}

func matchesFilters(f model.Flight, filters *model.SearchFilters) bool {
	if filters.MinPrice != nil && f.Price.Amount < *filters.MinPrice {
		return false
	}
	if filters.MaxPrice != nil && f.Price.Amount > *filters.MaxPrice {
		return false
	}
	if filters.MaxStops != nil && f.Stops > *filters.MaxStops {
		return false
	}
	if filters.MaxDuration != nil && f.Duration.TotalMinutes > *filters.MaxDuration {
		return false
	}
	if len(filters.Airlines) > 0 {
		matched := false
		for _, airline := range filters.Airlines {
			if strings.EqualFold(f.Airline.Name, airline) || strings.EqualFold(f.Airline.Code, airline) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if filters.DepartureAfter != nil {
		if t, err := parseTimeOfDay(*filters.DepartureAfter); err == nil {
			depTime := extractTimeOfDay(f.Departure.Datetime)
			if depTime.Before(t) {
				return false
			}
		}
	}
	if filters.DepartureBefore != nil {
		if t, err := parseTimeOfDay(*filters.DepartureBefore); err == nil {
			depTime := extractTimeOfDay(f.Departure.Datetime)
			if depTime.After(t) {
				return false
			}
		}
	}
	if filters.ArrivalAfter != nil {
		if t, err := parseTimeOfDay(*filters.ArrivalAfter); err == nil {
			arrTime := extractTimeOfDay(f.Arrival.Datetime)
			if arrTime.Before(t) {
				return false
			}
		}
	}
	if filters.ArrivalBefore != nil {
		if t, err := parseTimeOfDay(*filters.ArrivalBefore); err == nil {
			arrTime := extractTimeOfDay(f.Arrival.Datetime)
			if arrTime.After(t) {
				return false
			}
		}
	}
	return true
}

// parseTimeOfDay parses "HH:MM" into a reference time for comparison.
func parseTimeOfDay(s string) (time.Time, error) {
	return time.Parse("15:04", s)
}

// extractTimeOfDay extracts the HH:MM portion from an RFC3339 datetime for comparison.
func extractTimeOfDay(datetime string) time.Time {
	t, err := time.Parse(time.RFC3339, datetime)
	if err != nil {
		return time.Time{}
	}
	ref, _ := time.Parse("15:04", t.Format("15:04"))
	return ref
}

// mustParseFlightTime parses an RFC3339 datetime, logging a warning on failure.
// Returns zero time on error so SliceStable preserves original ordering.
func mustParseFlightTime(datetime, flightID, field string) time.Time {
	t, err := time.Parse(time.RFC3339, datetime)
	if err != nil {
		slog.Warn("failed to parse flight datetime for sorting",
			"flight_id", flightID,
			"field", field,
			"datetime", datetime,
			"error", err.Error(),
		)
		return time.Time{}
	}
	return t
}

func applySort(flights []model.Flight, sortOpt *model.SortOption) []model.Flight {
	if sortOpt == nil {
		return flights
	}

	ascending := sortOpt.Order == "asc"

	switch sortOpt.By {
	case "price":
		sort.SliceStable(flights, func(i, j int) bool {
			if ascending {
				return flights[i].Price.Amount < flights[j].Price.Amount
			}
			return flights[i].Price.Amount > flights[j].Price.Amount
		})
	case "duration":
		sort.SliceStable(flights, func(i, j int) bool {
			if ascending {
				return flights[i].Duration.TotalMinutes < flights[j].Duration.TotalMinutes
			}
			return flights[i].Duration.TotalMinutes > flights[j].Duration.TotalMinutes
		})
	case "departure":
		sort.SliceStable(flights, func(i, j int) bool {
			ti := mustParseFlightTime(flights[i].Departure.Datetime, flights[i].ID, "departure")
			tj := mustParseFlightTime(flights[j].Departure.Datetime, flights[j].ID, "departure")
			if ascending {
				return ti.Before(tj)
			}
			return ti.After(tj)
		})
	case "arrival":
		sort.SliceStable(flights, func(i, j int) bool {
			ti := mustParseFlightTime(flights[i].Arrival.Datetime, flights[i].ID, "arrival")
			tj := mustParseFlightTime(flights[j].Arrival.Datetime, flights[j].ID, "arrival")
			if ascending {
				return ti.Before(tj)
			}
			return ti.After(tj)
		})
	case "best_value":
		computeBestValue(flights)
		sort.SliceStable(flights, func(i, j int) bool {
			si := safeScore(flights[i].Score)
			sj := safeScore(flights[j].Score)
			if ascending {
				return si < sj
			}
			return si > sj
		})
	}
	return flights
}

// computeBestValue calculates a best-value score for each flight.
// Score = 0.6 * priceScore + 0.3 * durationScore + 0.1 * stopsScore
// Lower score = better value.
func computeBestValue(flights []model.Flight) {
	if len(flights) == 0 {
		return
	}

	var minPrice, maxPrice int64
	var minDur, maxDur int
	var maxStops int

	minPrice = math.MaxInt64
	minDur = math.MaxInt32

	for _, f := range flights {
		if f.Price.Amount < minPrice {
			minPrice = f.Price.Amount
		}
		if f.Price.Amount > maxPrice {
			maxPrice = f.Price.Amount
		}
		if f.Duration.TotalMinutes < minDur {
			minDur = f.Duration.TotalMinutes
		}
		if f.Duration.TotalMinutes > maxDur {
			maxDur = f.Duration.TotalMinutes
		}
		if f.Stops > maxStops {
			maxStops = f.Stops
		}
	}

	for i := range flights {
		priceScore := normalize(float64(flights[i].Price.Amount), float64(minPrice), float64(maxPrice))
		durScore := normalize(float64(flights[i].Duration.TotalMinutes), float64(minDur), float64(maxDur))
		stopsScore := normalize(float64(flights[i].Stops), 0, float64(maxStops))

		score := 0.6*priceScore + 0.3*durScore + 0.1*stopsScore
		// Round to 4 decimal places
		score = math.Round(score*10000) / 10000
		flights[i].Score = &score
	}
}

func normalize(val, min, max float64) float64 {
	if max == min {
		return 0
	}
	return (val - min) / (max - min)
}

func safeScore(s *float64) float64 {
	if s == nil {
		return 0
	}
	return *s
}
