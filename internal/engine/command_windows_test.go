//go:build windows

package engine

import (
	"path/filepath"
	"strings"
	"testing"

	"winforge/internal/winapi"
)

func TestResolveCommandUsesSystemPathsForInboxTools(t *testing.T) {
	for _, name := range []string{
		"dism", "w32tm.exe", "LODCTR", "rundll32.exe", "wevtutil",
		"fsutil.exe", "setx", "bcdedit.exe", "netsh",
	} {
		t.Run(name, func(t *testing.T) {
			got, trusted := trustedCommand(name)
			if !trusted {
				t.Fatalf("trustedCommand(%q) was not allowlisted", name)
			}
			if !filepath.IsAbs(got) || !strings.EqualFold(filepath.Dir(got), winapi.SystemDirectory()) {
				t.Fatalf("trustedCommand(%q) = %q, want path in %q", name, got, winapi.SystemDirectory())
			}
		})
	}
}

func TestResolveCommandUsesWbemWinmgmt(t *testing.T) {
	want := filepath.Join(winapi.SystemDirectory(), "wbem", "winmgmt.exe")
	if got, trusted := trustedCommand("WINMGMT.EXE"); !trusted || !strings.EqualFold(got, want) {
		t.Fatalf("trustedCommand(winmgmt) = %q, want %q", got, want)
	}
}

func TestResolveCommandPreservesUnknownTool(t *testing.T) {
	const name = "vendor-tool.exe"
	if _, trusted := trustedCommand(name); trusted {
		t.Fatalf("trustedCommand(%q) unexpectedly allowed an unknown tool", name)
	}
	if got := resolveCommand(name); got != name {
		t.Fatalf("resolveCommand(%q) = %q", name, got)
	}
}
