//go:build !windows

package engine

import "testing"

// TestTrustedCommandOffWindows pins the non-Windows stub. The allowlist is a
// Windows concept (it resolves inbox tools under System32), so off Windows
// nothing is trusted — which is why the elevated path refuses everything here.
// The Windows-side behaviour is pinned by command_windows_test.go.
func TestTrustedCommandOffWindows(t *testing.T) {
	for _, name := range []string{"dism", "dism.exe", "netsh", "anything"} {
		if _, trusted := trustedCommand(name); trusted {
			t.Fatalf("trustedCommand(%q) reported trusted off Windows", name)
		}
	}
}
