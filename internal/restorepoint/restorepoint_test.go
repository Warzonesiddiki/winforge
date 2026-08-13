package restorepoint

import (
	"testing"
	"time"
	"unicode/utf8"
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

// TestTruncateUTF8 covers the aggregate description-bound helper. Descriptions
// come from WMI, so the helper must never split a multi-byte rune and must
// always respect the byte limit.
func TestTruncateUTF8(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"under limit is unchanged", "Install", 64, "Install"},
		{"exactly at limit is unchanged", "abcd", 4, "abcd"},
		{"ascii is truncated with ellipsis", "abcdefghij", 6, "abc…"},
		{"multibyte runes are not split", "Wiederherstellungspünkt", 10, "Wiederh…"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncateUTF8(c.in, c.limit)
			if got != c.want {
				t.Errorf("truncateUTF8(%q, %d) = %q, want %q", c.in, c.limit, got, c.want)
			}
			if len(got) > c.limit {
				t.Errorf("truncateUTF8(%q, %d) returned %d bytes, over the limit", c.in, c.limit, len(got))
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncateUTF8(%q, %d) = %q, which is not valid UTF-8", c.in, c.limit, got)
			}
		})
	}
}

// TestTruncateUTF8RespectsLimitForAllPrefixes guards the byte bound against
// off-by-one errors at every cut point of a multi-byte string.
func TestTruncateUTF8RespectsLimitForAllPrefixes(t *testing.T) {
	const input = "Sicherungspünkt vör dem Änderung 😀 der Systemkonfiguration"
	for limit := 4; limit <= len(input)+4; limit++ {
		got := truncateUTF8(input, limit)
		if len(got) > limit {
			t.Fatalf("truncateUTF8(input, %d) returned %d bytes", limit, len(got))
		}
		if !utf8.ValidString(got) {
			t.Fatalf("truncateUTF8(input, %d) = %q, which is not valid UTF-8", limit, got)
		}
	}
}

// TestEnumerationBoundsAreRealistic pins the aggregate enumeration budgets so a
// future change cannot quietly restore effectively unbounded retention.
func TestEnumerationBoundsAreRealistic(t *testing.T) {
	if maxRestorePoints > 4096 {
		t.Errorf("maxRestorePoints = %d, which is too permissive for real systems", maxRestorePoints)
	}
	if maxDescriptionBytes > maxDescriptionBudgetBytes {
		t.Errorf("per-description bound %d exceeds the aggregate budget %d", maxDescriptionBytes, maxDescriptionBudgetBytes)
	}
	// The worst case a single enumeration can retain must stay well bounded.
	if worst := maxDescriptionBudgetBytes + maxRowErrorBudgetBytes; worst > 8<<20 {
		t.Errorf("worst-case retained enumeration memory is %d bytes", worst)
	}
}
