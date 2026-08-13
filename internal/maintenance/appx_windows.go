//go:build windows

package maintenance

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

const (
	coinitMultithreaded = 0x0

	// AsyncStatus values returned by IAsyncInfo::get_Status.
	asyncStatusStarted   = 1
	asyncStatusCompleted = 2
	asyncStatusCanceled  = 3
	asyncStatusError     = 4

	// COM vtable indices. Every interface derives from IInspectable, so
	// 0..2 are IUnknown (QueryInterface/AddRef/Release) and 3..5 are
	// IInspectable; members follow. IAsyncInfo: 6 get_Id, 7 get_Status,
	// 8 get_ErrorCode, 9 Cancel, 10 Close.
	idxGetStatus    = 7 // IAsyncInfo::get_Status
	idxGetErrorCode = 8 // IAsyncInfo::get_ErrorCode
	// IPackageManager (Windows.Management.Deployment) member layout,
	// verified against windows.management.deployment.idl:
	// 6 AddPackageAsync, 7 UpdatePackageAsync, 8 RemovePackageAsync,
	// 9 StagePackageAsync, 10 RegisterPackageAsync, …
	idxRemovePackageAsync = 8

	// removeTimeout bounds how long we poll the removal operation.
	removeTimeout = 10 * time.Minute

	// removePollInterval is how often the removal status is polled.
	removePollInterval = 250 * time.Millisecond
)

var (
	ole32    = syscall.NewLazyDLL("ole32.dll")
	combase  = syscall.NewLazyDLL("combase.dll")

	// IID_IPackageManager (Windows.Management.Deployment.IPackageManager),
	// verified against windows.management.deployment.idl:
	// uuid(9a7d4b65-5e8f-4fc7-a2e5-7f6925cb8b53).
	iidIPackageManager = guid{0x9a7d4b65, 0x5e8f, 0x4fc7, [8]byte{0xa2, 0xe5, 0x7f, 0x69, 0x25, 0xcb, 0x8b, 0x53}}

	procCoInitializeEx      = ole32.NewProc("CoInitializeEx")
	procCoUninitialize      = ole32.NewProc("CoUninitialize")
	procWindowsCreateString = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString = combase.NewProc("WindowsDeleteString")
	procRoActivateInstance  = combase.NewProc("RoActivateInstance")
)

// guid mirrors a Win32 GUID (little-endian fields).
type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// comIface is an opaque COM interface pointer. Concrete interfaces are
// interchangeable: method dispatch only reads the vtable.
type comIface struct{}

// vtable returns the address of the i-th virtual method of the COM object
// whose interface pointer is p.
func vtable(p *comIface, i int) uintptr {
	vtbl := *(**uintptr)(unsafe.Pointer(p))
	return *(*uintptr)(unsafe.Add(unsafe.Pointer(vtbl), uintptr(i)*unsafe.Sizeof(uintptr(0))))
}

// comCall invokes a COM virtual method and returns its HRESULT.
func comCall(p *comIface, i int, args ...uintptr) uint32 {
	r1, _, _ := syscall.SyscallN(vtable(p, i), args...)
	return uint32(r1)
}

// release decrements a COM object's reference count (IUnknown::Release).
func release(p *comIface) {
	if p != nil {
		_ = comCall(p, 2)
	}
}

// queryInterface performs IUnknown::QueryInterface on p.
func queryInterface(p *comIface, iid *guid, out **comIface) uint32 {
	return comCall(p, 0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(out)))
}

func hresultError(op string, hr uint32) error {
	return fmt.Errorf("%s failed: HRESULT 0x%08X", op, hr)
}

// windowsCreateString allocates an HSTRING from a Go string. The returned
// handle must be released with procWindowsDeleteString.
func windowsCreateString(s string) (uintptr, error) {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return 0, err
	}
	var hstr uintptr
	hr, _, _ := procWindowsCreateString.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(len(s)),
		uintptr(unsafe.Pointer(&hstr)),
	)
	if hr != 0 {
		return 0, hresultError("WindowsCreateString", uint32(hr))
	}
	return hstr, nil
}

// coInitialize wraps CoInitializeEx and runtime.LockOSThread, matching the
// pattern used in internal/updater/updater_windows.go. The returned cleanup
// is always non-nil; it only calls CoUninitialize when this call actually
// initialized the apartment, and always unlocks the OS thread.
func coInitialize() (func(), error) {
	runtime.LockOSThread()
	r, _, _ := procCoInitializeEx.Call(0, uintptr(coinitMultithreaded))
	switch {
	case r == 0: // S_OK — we initialized COM
	case r == 1: // S_FALSE — already initialized on this thread
	default:
		runtime.UnlockOSThread()
		return nil, hresultError("CoInitializeEx", uint32(r))
	}
	initialized := r == 0

	return func() {
		if initialized {
			procCoUninitialize.Call()
		}
		runtime.UnlockOSThread()
	}, nil
}

// removeAppxPackageByFullName removes an Appx package by its full name (e.g.
// "Microsoft.ZuneVideo_8wekyb3d8bbwe") for the current user using the WinRT
// Windows.Management.Deployment.PackageManager. The class is activated via
// RoActivateInstance (combase.dll) and its async RemovePackageAsync operation
// is polled to completion — the stdlib-only, CGO-free, PowerShell-free
// equivalent of Remove-AppxPackage.
//
// Note: the vtable index for RemovePackageAsync (8) and the IPackageManager
// IID are verified against windows.management.deployment.idl (the member
// order is AddPackageAsync 6, UpdatePackageAsync 7, RemovePackageAsync 8).
func removeAppxPackageByFullName(packageName string) error {
	finalizer, err := coInitialize()
	if err != nil {
		return err
	}
	defer finalizer()

	// RemovePackageAsync takes an HSTRING (WindowsStringHeader layout, header
	// 8 bytes before the data) — a SysAllocString BSTR (4-byte prefix) is NOT
	// interchangeable, so the name must be created with WindowsCreateString.
	pkgName, err := windowsCreateString(packageName)
	if err != nil {
		return err
	}
	defer procWindowsDeleteString.Call(pkgName)

	classID, err := windowsCreateString("Windows.Management.Deployment.PackageManager")
	if err != nil {
		return err
	}
	defer procWindowsDeleteString.Call(classID)

	var inspectable *comIface
	if hr := roActivateInstance(classID, &inspectable); hr != 0 {
		return hresultError("RoActivateInstance(PackageManager)", hr)
	}
	defer release(inspectable)

	var pm *comIface
	if hr := queryInterface(inspectable, &iidIPackageManager, &pm); hr != 0 {
		return hresultError("QueryInterface(IPackageManager)", hr)
	}
	defer release(pm)

	var op *comIface
	if hr := comCall(pm, idxRemovePackageAsync, pkgName, uintptr(unsafe.Pointer(&op))); hr != 0 {
		return hresultError("IPackageManager::RemovePackageAsync", hr)
	}
	if op == nil {
		return errors.New("RemovePackageAsync returned a nil operation")
	}
	defer release(op)

	return waitAsync(op)
}

// roActivateInstance activates a WinRT runtime class by its class name,
// returning an IInspectable pointer.
func roActivateInstance(classID uintptr, out **comIface) uint32 {
	r, _, _ := procRoActivateInstance.Call(classID, uintptr(unsafe.Pointer(out)))
	return uint32(r)
}

// waitAsync polls IAsyncInfo::get_Status until the operation completes,
// fails, or exceeds the timeout.
func waitAsync(op *comIface) error {
	deadline := time.Now().Add(removeTimeout)
	for {
		var status int32
		if hr := comCall(op, idxGetStatus, uintptr(unsafe.Pointer(&status))); hr != 0 {
			return hresultError("IAsyncInfo::get_Status", hr)
		}
		switch status {
		case asyncStatusCompleted:
			return nil
		case asyncStatusCanceled:
			return errors.New("appx removal canceled")
		case asyncStatusError:
			var code uint32
			if hr := comCall(op, idxGetErrorCode, uintptr(unsafe.Pointer(&code))); hr != 0 {
				return hresultError("IAsyncInfo::get_ErrorCode", hr)
			}
			return hresultError("RemovePackageAsync", code)
		}
		if time.Now().After(deadline) {
			return errors.New("appx removal timed out")
		}
		time.Sleep(removePollInterval)
	}
}