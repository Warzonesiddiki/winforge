//go:build windows

package scheduler

import (
	"fmt"
	"os/exec"
	"strings"

	"winforge/internal/procout"
	"winforge/internal/winapi"
)

// run executes schtasks.exe, returning a decorated error on failure.
func run(args ...string) error {
	cmd := exec.Command(winapi.SystemPath("schtasks.exe"), args...)
	out, err := procout.CombinedOutput(cmd, 1<<20)
	if err != nil {
		return fmt.Errorf("schtasks %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func enable(path string) error  { return run("/change", "/tn", path, "/enable") }
func disable(path string) error { return run("/change", "/tn", path, "/disable") }
func deleteTask(path string) error {
	return run("/delete", "/tn", path, "/f")
}

func register(name, exePath string) error {
	if err := validateRegister(exePath); err != nil {
		return err
	}

	task := fmt.Sprintf(`"%s" run-maintenance`, exePath)
	// Portable WinForge builds are commonly run from user-writable locations.
	// HIGHEST would let a later executable replacement (or a user-controlled
	// config/winget path) become silent elevation. LIMITED deliberately keeps
	// the weekly task at the user's standard privilege; administrators can run
	// maintenance interactively when system-wide tweaks need elevation.
	return run("/create", "/tn", name, "/tr", task, "/sc", "weekly", "/d", "MON", "/st", "03:00", "/rl", "LIMITED", "/f")
}
