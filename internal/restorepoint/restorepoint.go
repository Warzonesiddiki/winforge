// Package restorepoint creates and lists Windows System Restore points using
// native APIs only: srclient.dll SRSetRestorePointW for creation and raw WMI
// COM (root\default:SystemRestore) for enumeration. No PowerShell, no
// third-party modules.
package restorepoint

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrUnsupported is returned on non-Windows platforms.
var ErrUnsupported = errors.New("system restore points are only supported on Windows")

// ErrDisabled indicates System Restore is disabled on the target machine.
var ErrDisabled = errors.New("system restore is disabled on this system")

const (
	// Aggregate WMI enumeration bounds. Per-row and per-BSTR checks alone
	// still allow a corrupt SystemRestore provider to force WinForge to retain
	// 100,000 rows, each with a 1 MiB description, plus one error string per
	// malformed row. These budgets bound the total instead.
	//
	// maxRestorePoints is well above what System Restore itself retains (disk
	// quota caps real systems at tens of points) while ending a runaway
	// enumeration.
	maxRestorePoints = 1024
	// maxBSTRBytes bounds a single BSTR conversion. Restore-point descriptions
	// and WMI datetimes are short strings.
	maxBSTRBytes = 64 << 10
	// maxDescriptionBytes bounds one retained restore-point description.
	maxDescriptionBytes = 1024
	// maxDescriptionBudgetBytes bounds all descriptions retained by one list.
	maxDescriptionBudgetBytes = 1 << 20
	// maxRowErrors and maxRowErrorBudgetBytes bound the malformed-row detail
	// reported by a single enumeration.
	maxRowErrors           = 64
	maxRowErrorBudgetBytes = 64 << 10
)

// Info describes a created restore point.
type Info struct {
	SequenceNumber int64     `json:"sequenceNumber"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Create creates a system restore point with the given description.
func Create(description string) (Info, error) { return create(description) }

// IsEnabled reports whether the restore-point API is available (srclient.dll
// is loadable). It does not guarantee System Restore is configured.
func IsEnabled() bool { return isEnabled() }

// List enumerates existing system restore points (WMI SystemRestore class,
// root\default namespace), newest first.
func List() ([]Info, error) { return list() }

// parseWmiTime parses a WMI CIM datetime string (yyyymmddHHMMSS.mmmmmmsUUU)
// into a time.Time in UTC. The format is wall-clock time with an optional
// fractional-second suffix and an optional signed UTC offset in minutes.
func parseWmiTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if len(s) < 14 {
		return time.Time{}, fmt.Errorf("invalid WMI datetime %q", s)
	}

	year, err := digits(s[0:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid WMI datetime %q: %w", s, err)
	}
	month, err := digits(s[4:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid WMI datetime %q: %w", s, err)
	}
	day, err := digits(s[6:8])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid WMI datetime %q: %w", s, err)
	}
	hour, err := digits(s[8:10])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid WMI datetime %q: %w", s, err)
	}
	minute, err := digits(s[10:12])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid WMI datetime %q: %w", s, err)
	}
	sec, err := digits(s[12:14])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid WMI datetime %q: %w", s, err)
	}

	var micros int
	rest := s[14:]
	if strings.HasPrefix(rest, ".") {
		rest = rest[1:]
		i := 0
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			i++
		}
		if i == 0 {
			return time.Time{}, fmt.Errorf("invalid WMI datetime %q: empty fractional seconds", s)
		}
		if i > 6 {
			return time.Time{}, fmt.Errorf("invalid WMI datetime %q: fractional seconds exceed six digits", s)
		}
		frac := rest[:i]
		for len(frac) < 6 {
			frac += "0"
		}
		if micros, err = digits(frac); err != nil {
			return time.Time{}, fmt.Errorf("invalid WMI datetime %q: %w", s, err)
		}
		rest = rest[i:]
	}

	// Optional signed UTC offset in minutes (e.g. "+480" or "-000").
	var offsetMin int
	if strings.HasPrefix(rest, "+") || strings.HasPrefix(rest, "-") {
		if len(rest) < 4 {
			return time.Time{}, fmt.Errorf("invalid WMI datetime %q: truncated UTC offset", s)
		}
		sign := 1
		if rest[0] == '-' {
			sign = -1
		}
		if offsetMin, err = digits(rest[1:4]); err != nil {
			return time.Time{}, fmt.Errorf("invalid WMI datetime %q: %w", s, err)
		}
		offsetMin *= sign
		rest = rest[4:]
	}
	if rest != "" {
		return time.Time{}, fmt.Errorf("invalid WMI datetime %q: trailing %q", s, rest)
	}

	// time.Date normalizes invalid fields, so reject malformed WMI values before
	// constructing the timestamp rather than silently changing their date.
	if month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || minute > 59 || sec > 59 {
		return time.Time{}, fmt.Errorf("invalid WMI datetime %q: component out of range", s)
	}

	// CIM datetimes are wall-clock time with a UTC offset; convert to UTC.
	t := time.Date(year, time.Month(month), day, hour, minute, sec, micros*1000, time.UTC)
	if t.Year() != year || int(t.Month()) != month || t.Day() != day {
		return time.Time{}, fmt.Errorf("invalid WMI datetime %q: nonexistent calendar date", s)
	}
	return t.Add(-time.Duration(offsetMin) * time.Minute), nil
}

func digits(s string) (int, error) {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-digit in %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// truncateUTF8 bounds text to limit bytes without splitting a rune.
// Restore-point descriptions come from WMI and are only displayed, so trimming
// an oversized one is preferable to retaining it in full.
func truncateUTF8(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	const ellipsis = "…"
	if limit < len(ellipsis) {
		// Too small to hold even the marker; return nothing rather than slicing
		// with a negative index.
		return ""
	}
	text = text[:limit-len(ellipsis)]
	for len(text) > 0 && !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text + ellipsis
}
