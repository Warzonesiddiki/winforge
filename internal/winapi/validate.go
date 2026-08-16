package winapi

import (
	"fmt"
	"path/filepath"
	"strings"
)

// validateSystemFileName rejects names that are not a bare filename in the
// Windows system directory. It prevents path traversal, volume-qualified
// paths, ADS streams, and NUL injection before a value reaches a
// GetSystemDirectoryW-based path join. The check is platform-independent so
// it can be exercised on Linux CI without invoking Windows APIs.
func validateSystemFileName(name string) error {
	if name == "" || name == "." || name == ".." ||
		filepath.Base(name) != name ||
		filepath.VolumeName(name) != "" ||
		strings.ContainsAny(name, ":\x00") {
		return fmt.Errorf("invalid system file name %q", name)
	}
	return nil
}
