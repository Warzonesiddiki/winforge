//go:build !windows

package cli

import "os/exec"

// openBrowser is a best-effort no-op off-Windows.
func openBrowser(url string) {
	_ = exec.Command("xdg-open", url).Start()
}
