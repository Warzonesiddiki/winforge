package scheduler

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validateRegister checks the scheduled executable path before any schtasks
// call. It is platform-independent so the input-validation contract can be
// tested on Linux CI without invoking the Windows-only schtasks.exe.
func validateRegister(exePath string) error {
	if !filepath.IsAbs(exePath) {
		return fmt.Errorf("scheduled executable path must be absolute: %q", exePath)
	}
	if strings.ContainsAny(exePath, "\x00\"\r\n") {
		return errors.New("scheduled executable path contains an unsafe character")
	}
	info, err := os.Stat(exePath)
	if err != nil {
		return fmt.Errorf("inspect scheduled executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("scheduled executable must be a regular file")
	}
	return nil
}
