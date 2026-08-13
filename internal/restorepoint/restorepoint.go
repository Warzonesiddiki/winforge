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
)

// ErrUnsupported is returned on non-Windows platforms.
var ErrUnsupported = errors.New("system restore points are only supported on Windows")

// ErrDisabled indicates System Restore is disabled on the target machine.
var ErrDisabled = errors.New("system restore is disabled on this system")

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
		if i > 0 {
			frac := rest[:i]
			for len(frac) < 6 {
				frac += "0"
			}
			if micros, err = digits(frac[:6]); err != nil {
				return time.Time{}, fmt.Errorf("invalid WMI datetime %q: %w", s, err)
			}
			rest = rest[i:]
		}
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

	// CIM datetimes are wall-clock time with a UTC offset; convert to UTC.
	t := time.Date(year, time.Month(month), day, hour, minute, sec, micros*1000, time.UTC)
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
