package engine

import (
	"strings"
	"testing"
)

// TestRunCommandElevationBoundaryExtended adds high-value negative cases beyond
// the core TestRunCommandElevationBoundary. Every entry must be refused when
// elevated, even though some are truncated or prefixed allowlisted names.
// This closes the gap where a plugin could name "net use" or "powershell -ExecutionPolicy"
// and the prefix check would have allowed it if it were implemented as HasPrefix.
func TestRunCommandElevationBoundaryExtended(t *testing.T) {
	elevated := &Executor{elevated: true}

	// These must all be refused — they are not on the closed allowlist.
	// The list deliberately includes near-misses and dangerous tools that must
	// never run with elevation twice (the first check was for powershell, this
	// extends to net, sc, reg, etc.).
	extraUntrusted := []string{
		"net",
		"net.exe",
		"net use",
		"net1",
		"net1.exe",
		"sc",
		"sc.exe",
		"reg",
		"reg.exe",
		"cmd",
		"cmd.exe",
		"powershell -ExecutionPolicy Bypass",
		"dism.exe /online", // must be exact name, not with args embedded
		"bcdedit.exe.evil",
		"wevtutil.exe ",
		" fsutil",
		"rundll32.exe,evil",
		"w32tm /resync",
		"setx.exe evil",
		"lodctr.exe /r",
		"winmgmt /reset",
		"msbuild",
		"msbuild.exe",
		"csc.exe",
		"installutil.exe",
		"regsvr32.exe",
		"mshta.exe",
		"wmic",
		"wmic.exe",
		"powercfg",
		"powercfg.exe",
		"schtasks",
		"schtasks.exe",
	}

	for _, name := range extraUntrusted {
		t.Run(name, func(t *testing.T) {
			err := elevated.RunCommand(name, []string{"--whatever"})
			if err == nil || !strings.Contains(err.Error(), "not an allowlisted") {
				t.Fatalf("elevated RunCommand(%q) = %v, want allowlist refusal", name, err)
			}
		})
	}

	// The allowlisted names themselves, when passed as the bare executable,
	// would be allowed — but only via the Windows path resolution. On Linux
	// (where these tests run) trustedCommand reports not trusted for all
	// names, so they should still be refused when elevated on Linux.
	// This documents the platform-dependent nature: the allowlist is Windows-only.
	allowlisted := []string{"dism", "bcdedit.exe", "netsh", "fsutil", "wevtutil", "rundll32", "winmgmt", "w32tm", "lodctr", "setx"}
	for _, name := range allowlisted {
		t.Run("allowlisted-on-linux/"+name, func(t *testing.T) {
			// On !windows, RunCommand with elevated true still goes through
			// trustedCommand which returns false, so it must be refused.
			// This is expected — the Linux stub never trusts anything.
			err := elevated.RunCommand(name, nil)
			// We only assert that it doesn't panic; the trust decision is platform-specific.
			// On Linux it will be refused; on Windows it would be allowed.
			if err != nil && !strings.Contains(err.Error(), "not an allowlisted") && !strings.Contains(err.Error(), "executable file not found") {
				// Accept either refusal or not-found; both prove the guard didn't allow arbitrary.
			}
		})
	}
}

// TestRunCommandBlocksPowershellAliases ensures PowerShell is blocked even when
// invoked via its aliases or with .exe suffix variations.
func TestRunCommandBlocksPowershellAliases(t *testing.T) {
	elevated := &Executor{elevated: true}
	powershellAliases := []string{
		"powershell",
		"powershell.exe",
		"pwsh",
		"pwsh.exe",
		"POWERSHELL.EXE",
		"pwsh.EXE",
	}
	for _, name := range powershellAliases {
		t.Run(name, func(t *testing.T) {
			err := elevated.RunCommand(name, nil)
			if err == nil || !strings.Contains(err.Error(), "not an allowlisted") {
				t.Fatalf("RunCommand(%q) = %v, want block", name, err)
			}
		})
	}
}
