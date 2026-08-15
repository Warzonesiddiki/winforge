//go:build !windows

package bloatware

// Installed returns no results off-Windows; the bloatware scanner is a
// Windows-only capability.
func Installed() []string { return nil }
