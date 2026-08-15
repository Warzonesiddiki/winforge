// Package platform exposes small OS-dependent helpers (elevation check, OS
// identity) behind build tags so the rest of the app stays portable.
package platform

import "runtime"

// OSInfo describes the host operating system for the dashboard.
type OSInfo struct {
	OS          string `json:"os"`
	ProductName string `json:"productName"`
	Arch        string `json:"arch"`
}

// Arch returns the current GOARCH.
func Arch() string { return runtime.GOARCH }

// IsElevated reports whether the process is running with administrator rights.
// On Windows, an indeterminate token status is treated as elevated so callers
// do not cross a privilege boundary by trusting user-writable inputs.
func IsElevated() bool { return isElevated() }

// GetOSInfo returns OS identity information.
func GetOSInfo() OSInfo { return osInfo() }
