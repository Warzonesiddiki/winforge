//go:build windows

package cli

import (
	"os/exec"

	"winforge/internal/winapi"
)

// openBrowser opens the default browser without spawning a shell (no cmd.exe).
func openBrowser(url string) {
	cmd := exec.Command(
		winapi.SystemPath("rundll32.exe"),
		winapi.SystemPath("url.dll")+",FileProtocolHandler",
		url,
	)
	if err := cmd.Start(); err == nil {
		_ = cmd.Process.Release()
	}
}
