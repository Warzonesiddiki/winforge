// Package service controls Windows services through the Service Control
// Manager (advapi32), including changing start type — which Go's stdlib does
// not otherwise expose.
package service

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnsupported is returned on non-Windows platforms.
var ErrUnsupported = errors.New("service control is only supported on Windows")

// StartMode mirrors the Win32 SERVICE_START_TYPE values.
type StartMode int

const (
	StartAuto     StartMode = 2 // SERVICE_AUTO_START
	StartManual   StartMode = 3 // SERVICE_DEMAND_START
	StartDisabled StartMode = 4 // SERVICE_DISABLED
)

// String returns a human-readable start mode name.
func (m StartMode) String() string {
	switch m {
	case StartAuto:
		return "automatic"
	case StartManual:
		return "manual"
	case StartDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

// ParseStartMode converts "auto"/"automatic", "manual"/"demand", or
// "disabled" into a StartMode.
func ParseStartMode(s string) (StartMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "auto", "automatic":
		return StartAuto, nil
	case "manual", "demand":
		return StartManual, nil
	case "disabled", "disable":
		return StartDisabled, nil
	default:
		return 0, errors.New("unknown start mode: " + s)
	}
}

// GetStartMode returns the current start mode of a service.
func GetStartMode(name string) (StartMode, error) { return getStartMode(name) }

// SetStartMode changes the service start type.
func SetStartMode(name string, mode StartMode) error {
	switch mode {
	case StartAuto, StartManual, StartDisabled:
		return setStartMode(name, mode)
	default:
		return fmt.Errorf("invalid service start mode %d", mode)
	}
}

// Start starts a stopped service.
func Start(name string) error { return start(name) }

// Stop stops a running service.
func Stop(name string) error { return stop(name) }
