//go:build windows

package winapi

import "testing"

// On Windows, backslash is a path separator, so these names must be rejected.
// They cannot be asserted cross-platform because filepath.Base treats
// backslash as an ordinary character on Linux/macOS.
func TestValidateSystemFileNameRejectsBackslashPaths(t *testing.T) {
	bad := []string{
		`..\dism.exe`,
		`subdir\dism.exe`,
	}
	for _, name := range bad {
		t.Run(name, func(t *testing.T) {
			if err := validateSystemFileName(name); err == nil {
				t.Fatalf("validateSystemFileName(%q) = nil, want an error", name)
			}
		})
	}
}
