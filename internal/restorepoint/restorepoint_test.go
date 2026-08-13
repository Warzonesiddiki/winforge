package restorepoint

import (
	"testing"
	"time"
)

func TestParseWmiTime(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
	}{
		// No fractional part, no offset (UTC).
		{"20240102153045.000000+000", time.Date(2024, 1, 2, 15, 30, 45, 0, time.UTC)},
		// Real-world sample with fractional seconds and "-000" offset.
		{"20081229191404.400068-000", time.Date(2008, 12, 29, 19, 14, 4, 400068000, time.UTC)},
		// Positive offset (+480 min = UTC+8): local 12:00 -> 04:00 UTC.
		{"20240301120000.000000+480", time.Date(2024, 3, 1, 4, 0, 0, 0, time.UTC)},
		// Negative offset (-300 min = UTC-5): local 12:00 -> 17:00 UTC.
		{"20240301120000.000000-300", time.Date(2024, 3, 1, 17, 0, 0, 0, time.UTC)},
		// Fraction with fewer than 6 digits pads correctly.
		{"20240102153045.5+000", time.Date(2024, 1, 2, 15, 30, 45, 500000000, time.UTC)},
	}
	for _, c := range cases {
		got, err := parseWmiTime(c.in)
		if err != nil {
			t.Errorf("parseWmiTime(%q): %v", c.in, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("parseWmiTime(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseWmiTimeInvalid(t *testing.T) {
	for _, in := range []string{
		"",
		"20240102",
		"2024x102153045.000000+000",
		"20240102153045xx",
		"20240102153045.+000",
		"20240102153045.0000000+000",
		"20241302153045.000000+000",
		"20240230153045.000000+000",
		"20240102243045.000000+000",
		"20240102156045.000000+000",
		"20240102153060.000000+000",
	} {
		if _, err := parseWmiTime(in); err == nil {
			t.Errorf("parseWmiTime(%q): expected error", in)
		}
	}
}

func TestDigits(t *testing.T) {
	n, err := digits("0042")
	if err != nil {
		t.Fatal(err)
	}
	if n != 42 {
		t.Errorf("digits = %d, want 42", n)
	}
	if _, err := digits("12x"); err == nil {
		t.Error("digits(12x): expected error")
	}
}
