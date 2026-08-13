//go:build windows

package engine

import (
	"path/filepath"
	"strings"

	"winforge/internal/winapi"
)

func trustedCommand(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "dism", "dism.exe":
		return winapi.SystemPath("dism.exe"), true
	case "w32tm", "w32tm.exe":
		return winapi.SystemPath("w32tm.exe"), true
	case "lodctr", "lodctr.exe":
		return winapi.SystemPath("lodctr.exe"), true
	case "winmgmt", "winmgmt.exe":
		return filepath.Join(winapi.SystemDirectory(), "wbem", "winmgmt.exe"), true
	case "rundll32", "rundll32.exe":
		return winapi.SystemPath("rundll32.exe"), true
	case "wevtutil", "wevtutil.exe":
		return winapi.SystemPath("wevtutil.exe"), true
	case "fsutil", "fsutil.exe":
		return winapi.SystemPath("fsutil.exe"), true
	case "setx", "setx.exe":
		return winapi.SystemPath("setx.exe"), true
	case "bcdedit", "bcdedit.exe":
		return winapi.SystemPath("bcdedit.exe"), true
	case "netsh", "netsh.exe":
		return winapi.SystemPath("netsh.exe"), true
	default:
		return "", false
	}
}

func resolveCommand(name string) string {
	if command, ok := trustedCommand(name); ok {
		return command
	}
	return name
}
