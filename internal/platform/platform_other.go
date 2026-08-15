//go:build !windows

package platform

import "runtime"

// isElevated always returns true off-Windows so local development and tests
// are not blocked by an elevation check.
func isElevated() bool { return true }

// osInfo returns a generic identity off-Windows.
func osInfo() OSInfo {
	return OSInfo{OS: runtime.GOOS, ProductName: runtime.GOOS, Arch: Arch()}
}
