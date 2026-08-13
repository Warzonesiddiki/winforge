//go:build windows

package scheduler

import (
	"fmt"
	"os/exec"
	"strings"
)

// run executes schtasks.exe, returning a decorated error on failure.
func run(args ...string) error {
	cmd := exec.Command("schtasks", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func enable(path string) error { return run("/change", "/tn", path, "/enable") }

func disable(path string) error { return run("/change", "/tn", path, "/disable") }

func deleteTask(path string) error { return run("/delete", "/tn", path, "/f") }

func register(name, exePath string) error {
	task := fmt.Sprintf(`"%s" run-maintenance`, exePath)
	return run("/create", "/tn", name, "/tr", task, "/sc", "weekly", "/d", "MON", "/st", "03:00", "/rl", "HIGHEST", "/f")
}
