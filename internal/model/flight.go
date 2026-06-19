package model

import (
	"fmt"
	"strconv"
	"strings"
)

// Flight represents a normalized flight from any provider.
type Flight struct {
	ID             string      `json:"id"`
	Provider       string      `json:"provider"`
	Airline        AirlineInfo `json:"airline"`
	FlightNumber   string      `json:"flight_number"`
	Departure      AirportTime `json:"departure"`
	Arrival        AirportTime `json:"arrival"`
	Duration       Duration    `json:"duration"`
	Stops          int         `json:"stops"`
	Price          Price       `json:"price"`
	AvailableSeats int         `json:"available_seats"`
	CabinClass     string      `json:"cabin_class"`
	Aircraft       *string     `json:"aircraft"`
	Amenities      []string    `json:"amenities"`
	Baggage        BaggageInfo `json:"baggage"`
	Score          *float64    `json:"score,omitempty"`
}

type AirlineInfo struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type AirportTime struct {
	Airport  string `json:"airport"`
	City     string `json:"city"`
	Datetime string `json:"datetime"`
}

type Duration struct {
	TotalMinutes int    `json:"total_minutes"`
	Formatted    string `json:"formatted"`
}

type Price struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Display  string `json:"display"`
}

type BaggageInfo struct {
	CarryOn string `json:"carry_on"`
	Checked string `json:"checked"`
}

// FormatDuration converts total minutes into a human-readable string.
func FormatDuration(minutes int) string {
	h := minutes / 60
	m := minutes % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// FormatIDR formats an amount in Indonesian Rupiah.
func FormatIDR(amount int64) string {
	s := strconv.FormatInt(amount, 10)
	n := len(s)
	if n <= 3 {
		return "Rp " + s
	}

	var buf strings.Builder
	buf.WriteString("Rp ")
	remainder := n % 3
	if remainder > 0 {
		buf.WriteString(s[:remainder])
		if remainder < n {
			buf.WriteByte('.')
		}
	}
	for i := remainder; i < n; i += 3 {
		if i > remainder {
			buf.WriteByte('.')
		}
		buf.WriteString(s[i : i+3])
	}
	return buf.String()
}
