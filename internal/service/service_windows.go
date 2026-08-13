//go:build windows

package service

import (
	"syscall"
	"unsafe"
)

const (
	scManagerConnect    = 0x0001
	serviceNoChange     = 0xFFFFFFFF
	serviceQueryConfig  = 0x0001
	serviceChangeConfig = 0x0002
	serviceStart        = 0x0010
	serviceStop         = 0x0020
	serviceControlStop  = 0x00000001
)

var (
	advapi32                  = syscall.NewLazyDLL("advapi32.dll")
	procOpenSCManagerW        = advapi32.NewProc("OpenSCManagerW")
	procOpenServiceW          = advapi32.NewProc("OpenServiceW")
	procChangeServiceConfigW  = advapi32.NewProc("ChangeServiceConfigW")
	procQueryServiceConfigW   = advapi32.NewProc("QueryServiceConfigW")
	procStartServiceW         = advapi32.NewProc("StartServiceW")
	procControlService        = advapi32.NewProc("ControlService")
	procCloseServiceHandle    = advapi32.NewProc("CloseServiceHandle")
)

func openSCManager() (syscall.Handle, error) {
	r, _, _ := procOpenSCManagerW.Call(0, 0, uintptr(scManagerConnect))
	if r == 0 {
		return 0, syscall.GetLastError()
	}
	return syscall.Handle(r), nil
}

func openService(name string, access uint32) (syscall.Handle, error) {
	scm, err := openSCManager()
	if err != nil {
		return 0, err
	}
	defer procCloseServiceHandle.Call(uintptr(scm))

	pname, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	r, _, _ := procOpenServiceW.Call(uintptr(scm), uintptr(unsafe.Pointer(pname)), uintptr(access))
	if r == 0 {
		return 0, syscall.GetLastError()
	}
	return syscall.Handle(r), nil
}

func getStartMode(name string) (StartMode, error) {
	h, err := openService(name, serviceQueryConfig)
	if err != nil {
		return 0, err
	}
	defer procCloseServiceHandle.Call(uintptr(h))

	// QUERY_SERVICE_CONFIGW: dwServiceType (4 bytes) then dwStartType (4 bytes).
	var buf [8]byte
	var needed uint32
	r, _, _ := procQueryServiceConfigW.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&needed)),
	)
	if r == 0 {
		return 0, syscall.GetLastError()
	}
	mode := StartMode(*(*uint32)(unsafe.Pointer(&buf[4])))
	return mode, nil
}

func setStartMode(name string, mode StartMode) error {
	h, err := openService(name, serviceChangeConfig)
	if err != nil {
		return err
	}
	defer procCloseServiceHandle.Call(uintptr(h))

	r, _, _ := procChangeServiceConfigW.Call(
		uintptr(h),
		uintptr(serviceNoChange), // service type: no change
		uintptr(uint32(mode)),    // start type
		uintptr(serviceNoChange), // error control: no change
		0, 0, 0, 0, 0, 0, 0,
	)
	if r == 0 {
		return syscall.GetLastError()
	}
	return nil
}

func start(name string) error {
	h, err := openService(name, serviceStart)
	if err != nil {
		return err
	}
	defer procCloseServiceHandle.Call(uintptr(h))

	r, _, _ := procStartServiceW.Call(uintptr(h), 0, 0)
	if r == 0 {
		return syscall.GetLastError()
	}
	return nil
}

func stop(name string) error {
	h, err := openService(name, serviceStop)
	if err != nil {
		return err
	}
	defer procCloseServiceHandle.Call(uintptr(h))

	var status [32]byte
	r, _, _ := procControlService.Call(
		uintptr(h),
		uintptr(serviceControlStop),
		uintptr(unsafe.Pointer(&status[0])),
	)
	if r == 0 {
		return syscall.GetLastError()
	}
	return nil
}
