//go:build windows

package platform

import (
	"syscall"
	"unsafe"

	"winforge/internal/registry"
)

const (
	tokenQuery     = 0x0008
	tokenElevation = 20 // TOKEN_INFORMATION_CLASS TokenElevation
)

var (
	advapi32                = syscall.NewLazyDLL("advapi32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcessToken    = advapi32.NewProc("OpenProcessToken")
	procGetTokenInformation = advapi32.NewProc("GetTokenInformation")
	procCloseHandle         = kernel32.NewProc("CloseHandle")
)

type tokenElevationInfo struct {
	TokenIsElevated uint32
}

// isElevated checks whether the process token is elevated via raw P/Invoke,
// keeping the binary free of third-party modules. Detection failures return
// true deliberately: consumers use this result to decide whether user-writable
// configuration and audit snapshots are safe to trust, so failure must be
// conservative rather than silently weakening the privilege boundary.
func isElevated() bool {
	var token syscall.Handle
	proc, err := syscall.GetCurrentProcess()
	if err != nil {
		return true
	}
	r, _, _ := procOpenProcessToken.Call(
		uintptr(proc),
		uintptr(tokenQuery),
		uintptr(unsafe.Pointer(&token)),
	)
	if r == 0 {
		return true
	}
	defer procCloseHandle.Call(uintptr(token))

	var elev tokenElevationInfo
	var size uint32
	r, _, _ = procGetTokenInformation.Call(
		uintptr(token),
		uintptr(tokenElevation),
		uintptr(unsafe.Pointer(&elev)),
		uintptr(unsafe.Sizeof(elev)),
		uintptr(unsafe.Pointer(&size)),
	)
	if r == 0 {
		return true
	}
	return elev.TokenIsElevated != 0
}

// osInfo reads the Windows product name from the registry.
func osInfo() OSInfo {
	name := "Windows"
	if v, err := registry.String(registry.HKEY_LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName"); err == nil && v != "" {
		name = v
	}
	return OSInfo{OS: "windows", ProductName: name, Arch: Arch()}
}
