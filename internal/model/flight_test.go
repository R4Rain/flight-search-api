package model

import "testing"

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		minutes int
		want    string
	}{
		{0, "0m"},
		{30, "30m"},
		{60, "1h"},
		{90, "1h 30m"},
		{105, "1h 45m"},
		{120, "2h"},
		{225, "3h 45m"},
		{260, "4h 20m"},
	}

	for _, tt := range tests {
		got := FormatDuration(tt.minutes)
		if got != tt.want {
			t.Errorf("FormatDuration(%d) = %q, want %q", tt.minutes, got, tt.want)
		}
	}
}

func TestFormatIDR(t *testing.T) {
	tests := []struct {
		amount int64
		want   string
	}{
		{0, "Rp 0"},
		{500, "Rp 500"},
		{1000, "Rp 1.000"},
		{50000, "Rp 50.000"},
		{485000, "Rp 485.000"},
		{650000, "Rp 650.000"},
		{950000, "Rp 950.000"},
		{1100000, "Rp 1.100.000"},
		{1250000, "Rp 1.250.000"},
		{1850000, "Rp 1.850.000"},
		{10000000, "Rp 10.000.000"},
	}

	for _, tt := range tests {
		got := FormatIDR(tt.amount)
		if got != tt.want {
			t.Errorf("FormatIDR(%d) = %q, want %q", tt.amount, got, tt.want)
		}
	}
}
