// Package restorepoint creates Windows System Restore points via the native
// srclient.dll SRSetRestorePointW API (no PowerShell, no WMI/COM).
package restorepoint

import (
	"errors"
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

// List is not yet implemented: enumerating existing restore points requires
// WMI (the SystemRestore class), which is a later phase.
func List() ([]Info, error) { return list() }
