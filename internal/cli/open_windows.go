//go:build windows

package cli

import "os/exec"

// openBrowser opens the default browser without spawning a shell (no cmd.exe).
func openBrowser(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
