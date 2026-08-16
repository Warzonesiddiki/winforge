//go:build !windows

package platform

import (
	"runtime"
	"testing"
)

func TestArchReturnsGOARCH(t *testing.T) {
	if got := Arch(); got != runtime.GOARCH {
		t.Errorf("Arch() = %q, want %q", got, runtime.GOARCH)
	}
}

func TestIsElevatedOnNonWindows(t *testing.T) {
	// Off-Windows the elevation check is intentionally true so local
	// development and tests are not blocked by an elevation boundary that
	// cannot be crossed. Pin that contract.
	if !IsElevated() {
		t.Fatal("IsElevated() = false on non-Windows, want true (conservative default)")
	}
}

func TestGetOSInfoPopulated(t *testing.T) {
	info := GetOSInfo()
	if info.OS != runtime.GOOS {
		t.Errorf("OSInfo.OS = %q, want %q", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("OSInfo.Arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
	if info.ProductName == "" {
		t.Error("OSInfo.ProductName is empty off-Windows")
	}
}
