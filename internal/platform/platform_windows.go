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

// isElevated checks the process token for the Administrators group via raw
// P/Invoke, keeping the binary free of third-party modules.
func isElevated() bool {
	var token syscall.Handle
	proc, err := syscall.GetCurrentProcess()
	if err != nil {
		return false
	}
	r, _, _ := procOpenProcessToken.Call(
		uintptr(proc),
		uintptr(tokenQuery),
		uintptr(unsafe.Pointer(&token)),
	)
	if r == 0 {
		return false
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
		return false
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
