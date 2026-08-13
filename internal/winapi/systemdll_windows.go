//go:build windows

// Package winapi contains small shared helpers for direct Windows API use.
package winapi

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	// kernel32.dll is registered as a system DLL by Go's syscall package and
	// is therefore loaded from System32 rather than through the normal DLL
	// search path.
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemDirectory = kernel32.NewProc("GetSystemDirectoryW")
	systemDirectoryOnce    sync.Once
	systemDirectory        string
	systemDirectoryErr     error
)

func systemPath(name string) string {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name ||
		filepath.VolumeName(name) != "" || strings.ContainsAny(name, ":\x00") {
		panic(fmt.Sprintf("invalid system file name %q", name))
	}

	systemDirectoryOnce.Do(func() {
		// Windows paths can be up to 32,767 UTF-16 code units with the extended
		// path prefix. GetSystemDirectoryW normally returns a much shorter path,
		// but using the architectural limit avoids a truncation fallback.
		buf := make([]uint16, 32768)
		n, _, callErr := procGetSystemDirectory.Call(
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
		)
		if n == 0 {
			systemDirectoryErr = fmt.Errorf("GetSystemDirectoryW failed: %v", callErr)
			return
		}
		if n >= uintptr(len(buf)) {
			systemDirectoryErr = fmt.Errorf("GetSystemDirectoryW returned invalid length %d", n)
			return
		}
		systemDirectory = syscall.UTF16ToString(buf[:n])
		runtime.KeepAlive(buf)
		if !filepath.IsAbs(systemDirectory) {
			systemDirectoryErr = fmt.Errorf("GetSystemDirectoryW returned non-absolute path %q", systemDirectory)
			systemDirectory = ""
		}
	})
	if systemDirectoryErr != nil {
		panic(systemDirectoryErr)
	}
	return filepath.Join(systemDirectory, name)
}

// SystemDirectory returns the real Windows system directory.
func SystemDirectory() string { return filepath.Dir(systemPath("kernel32.dll")) }

// SystemPath returns an absolute path to a file in the real Windows system
// directory. It avoids PATH-based executable and DLL search hijacking.
func SystemPath(name string) string { return systemPath(name) }

// SystemDLL returns a lazy DLL whose absolute path is rooted in the real
// Windows system directory. syscall.NewLazyDLL's normal search behavior can
// otherwise load an attacker-controlled DLL from the application directory.
func SystemDLL(name string) *syscall.LazyDLL {
	return syscall.NewLazyDLL(systemPath(name))
}
