//go:build windows

package maintenance

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"winforge/internal/winapi"
)

const (
	roinitMultithreaded = 0x1

	// Windows.Foundation.AsyncStatus values returned by IAsyncInfo::get_Status.
	asyncStatusStarted   = 0
	asyncStatusCompleted = 1
	asyncStatusCanceled  = 2
	asyncStatusError     = 3

	// COM vtable indices. Every interface derives from IInspectable, so
	// 0..2 are IUnknown (QueryInterface/AddRef/Release) and 3..5 are
	// IInspectable; members follow. IAsyncInfo: 6 get_Id, 7 get_Status,
	// 8 get_ErrorCode, 9 Cancel, 10 Close.
	idxGetStatus    = 7  // IAsyncInfo::get_Status
	idxGetErrorCode = 8  // IAsyncInfo::get_ErrorCode
	idxAsyncCancel  = 9  // IAsyncInfo::Cancel
	idxAsyncClose   = 10 // IAsyncInfo::Close

	// IAsyncOperationWithProgress<DeploymentResult, DeploymentProgress> adds
	// put/get Progress, put/get Completed, and GetResults after IInspectable.
	idxAsyncGetResults = 10

	// IDeploymentResult members after IInspectable.
	idxDeploymentErrorText         = 6
	idxDeploymentExtendedErrorCode = 8

	// IPackageManager (Windows.Management.Deployment) member layout,
	// verified against windows.management.deployment.idl:
	// 6 AddPackageAsync, 7 UpdatePackageAsync, 8 RemovePackageAsync,
	// 9 StagePackageAsync, 10 RegisterPackageAsync, 11 FindPackages,
	// 12 FindPackagesByUserSecurityId, …
	idxRemovePackageAsync                  = 8
	idxFindPackagesByUserSecurityID        = 12
	idxIterableFirst                       = 6
	idxIteratorCurrent                     = 6
	idxIteratorHasCurrent                  = 7
	idxIteratorMoveNext                    = 8
	idxPackageID                           = 6
	idxPackageIDName                       = 6
	idxPackageIDResourceID                 = 9
	idxPackageIDFullName                   = 12
	idxPackageIDFamilyName                 = 13
	hresultPackageNotFound          uint32 = 0x80073CF1

	// removeTimeout bounds how long we poll the removal operation.
	removeTimeout = 10 * time.Minute

	// removePollInterval is how often the removal status is polled.
	removePollInterval = 250 * time.Millisecond

	// cancellationGrace bounds how long a timed-out operation is given to
	// acknowledge cancellation. IAsyncInfo::Close is illegal before the
	// operation reaches a terminal state.
	cancellationGrace = 30 * time.Second
)

var (
	combase = winapi.SystemDLL("combase.dll")

	// IID_IPackageManager (Windows.Management.Deployment.IPackageManager),
	// verified against windows.management.deployment.idl:
	// uuid(9a7d4b65-5e8f-4fc7-a2e5-7f6925cb8b53).
	iidIPackageManager = guid{0x9a7d4b65, 0x5e8f, 0x4fc7, [8]byte{0xa2, 0xe5, 0x7f, 0x69, 0x25, 0xcb, 0x8b, 0x53}}
	// IID_IAsyncInfo = 00000036-0000-0000-C000-000000000046.
	iidIAsyncInfo = guid{0x00000036, 0x0000, 0x0000, [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}

	procRoInitialize              = combase.NewProc("RoInitialize")
	procRoUninitialize            = combase.NewProc("RoUninitialize")
	procWindowsCreateString       = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString       = combase.NewProc("WindowsDeleteString")
	procWindowsGetStringRawBuffer = combase.NewProc("WindowsGetStringRawBuffer")
	procRoActivateInstance        = combase.NewProc("RoActivateInstance")
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
	method := vtable(p, i)
	callArgs := make([]uintptr, 1, len(args)+1)
	callArgs[0] = uintptr(unsafe.Pointer(p))
	callArgs = append(callArgs, args...)
	r1, _, _ := syscall.SyscallN(method, callArgs...)
	runtime.KeepAlive(p)
	return uint32(r1)
}

// comCallOut invokes a COM method whose final argument is a pointer to Go
// memory. Keeping that argument typed as unsafe.Pointer until this function
// prevents a stack move from invalidating a uintptr created by the caller.
func comCallOut(p *comIface, i int, out unsafe.Pointer, args ...uintptr) uint32 {
	method := vtable(p, i)
	callArgs := make([]uintptr, 1, len(args)+2)
	callArgs[0] = uintptr(unsafe.Pointer(p))
	callArgs = append(callArgs, args...)
	callArgs = append(callArgs, uintptr(out))
	r1, _, _ := syscall.SyscallN(method, callArgs...)
	runtime.KeepAlive(out)
	runtime.KeepAlive(p)
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
	method := vtable(p, 0)
	r1, _, _ := syscall.SyscallN(
		method,
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(out)),
	)
	runtime.KeepAlive(out)
	runtime.KeepAlive(iid)
	runtime.KeepAlive(p)
	return uint32(r1)
}

func hresultError(op string, hr uint32) error {
	return fmt.Errorf("%s failed: HRESULT 0x%08X", op, hr)
}

func hresultFailed(hr uint32) bool {
	return hr&0x80000000 != 0
}

// windowsCreateString allocates an HSTRING from a Go string. The returned
// handle must be released with procWindowsDeleteString.
func windowsCreateString(s string) (uintptr, error) {
	units, err := syscall.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var hstr uintptr
	addr := procWindowsCreateString.Addr()
	hr, _, _ := syscall.SyscallN(
		addr,
		uintptr(unsafe.Pointer(&units[0])),
		uintptr(len(units)-1), // WindowsCreateString expects UTF-16 code units, excluding NUL.
		uintptr(unsafe.Pointer(&hstr)),
	)
	runtime.KeepAlive(units)
	if hresultFailed(uint32(hr)) {
		return 0, hresultError("WindowsCreateString", uint32(hr))
	}
	return hstr, nil
}

// nativeStringPointer reinterprets the address of an OS-owned string buffer as
// a pointer. It must only be called with addresses returned by Windows APIs
// that own the underlying memory and keep it alive for the caller; passing a
// Go-managed address here would hide the reference from the garbage collector.
//
// The address is reinterpreted rather than converted directly because neither
// `go vet`'s unsafeptr analysis nor the runtime checkptr instrumentation can
// tell that this memory lies outside the Go heap, and both would otherwise
// report a false positive on a conversion that is required here.
//
//go:nocheckptr
func nativeStringPointer(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}

func hstringToString(hstr uintptr) (string, error) {
	if hstr == 0 {
		return "", nil
	}
	var length uint32
	addr := procWindowsGetStringRawBuffer.Addr()
	p, _, _ := syscall.SyscallN(addr, hstr, uintptr(unsafe.Pointer(&length)))
	runtime.KeepAlive(&length)
	if length == 0 {
		return "", nil
	}
	if p == 0 || length > maxHSTRINGChars {
		return "", fmt.Errorf("invalid HSTRING buffer (length %d)", length)
	}
	// p addresses the Windows Runtime's own HSTRING backing buffer, which is
	// owned by the OS rather than the Go heap and stays valid until the caller
	// releases the HSTRING. Converting this uintptr to unsafe.Pointer is
	// therefore safe, but `go vet`'s unsafeptr check cannot see that the memory
	// is not Go-managed and reports a possible misuse, so the conversion is
	// isolated in a documented helper.
	buffer := unsafe.Slice((*uint16)(nativeStringPointer(p)), int(length))
	return string(utf16.Decode(buffer)), nil
}

func getHSTRING(p *comIface, idx int, op string) (string, error) {
	var hstr uintptr
	if hr := comCallOut(p, idx, unsafe.Pointer(&hstr)); hresultFailed(hr) {
		return "", hresultError(op, hr)
	}
	if hstr == 0 {
		return "", nil
	}
	defer procWindowsDeleteString.Call(hstr)
	return hstringToString(hstr)
}

// initializeRuntime initializes the Windows Runtime and pins the goroutine to
// the current OS thread. Every successful RoInitialize call, including
// S_FALSE, must be balanced by RoUninitialize on that same thread.
func initializeRuntime() (func(), error) {
	runtime.LockOSThread()
	r, _, _ := procRoInitialize.Call(uintptr(roinitMultithreaded))
	hr := uint32(r)
	if hresultFailed(hr) {
		runtime.UnlockOSThread()
		return nil, hresultError("RoInitialize", hr)
	}
	return func() {
		procRoUninitialize.Call()
		runtime.UnlockOSThread()
	}, nil
}

// removeAppxPackageByFullName accepts a stable package identity name, package
// family name, or package full name. It resolves every matching current-user
// package to the version-specific full name required by RemovePackageAsync.
// Missing packages are treated as success, making removal idempotent.
func removeAppxPackageByFullName(packageName string) error {
	finalizer, err := initializeRuntime()
	if err != nil {
		return err
	}
	defer finalizer()

	classID, err := windowsCreateString("Windows.Management.Deployment.PackageManager")
	if err != nil {
		return err
	}
	defer procWindowsDeleteString.Call(classID)

	var inspectable *comIface
	if hr := roActivateInstance(classID, &inspectable); hresultFailed(hr) {
		return hresultError("RoActivateInstance(PackageManager)", hr)
	}
	if inspectable == nil {
		return errors.New("RoActivateInstance(PackageManager) returned nil")
	}
	defer release(inspectable)

	var pm *comIface
	if hr := queryInterface(inspectable, &iidIPackageManager, &pm); hresultFailed(hr) {
		return hresultError("QueryInterface(IPackageManager)", hr)
	}
	if pm == nil {
		return errors.New("QueryInterface(IPackageManager) returned nil")
	}
	defer release(pm)

	fullNames, err := findPackageFullNames(pm, packageName)
	if err != nil {
		return err
	}
	var errs []error
	for _, fullName := range fullNames {
		if err := removePackageByFullName(pm, fullName); err != nil {
			errs = append(errs, fmt.Errorf("remove %q: %w", fullName, err))
		}
	}
	return errors.Join(errs...)
}

func findPackageFullNames(pm *comIface, wanted string) ([]string, error) {
	currentUser, err := windowsCreateString("")
	if err != nil {
		return nil, err
	}
	defer procWindowsDeleteString.Call(currentUser)

	var iterable *comIface
	if hr := comCallOut(pm, idxFindPackagesByUserSecurityID, unsafe.Pointer(&iterable), currentUser); hresultFailed(hr) {
		return nil, hresultError("IPackageManager::FindPackagesForUser", hr)
	}
	if iterable == nil {
		return nil, errors.New("FindPackagesForUser returned a nil collection")
	}
	defer release(iterable)

	var iterator *comIface
	if hr := comCallOut(iterable, idxIterableFirst, unsafe.Pointer(&iterator)); hresultFailed(hr) {
		return nil, hresultError("IIterable<Package>::First", hr)
	}
	if iterator == nil {
		return nil, errors.New("IIterable<Package>::First returned nil")
	}
	defer release(iterator)

	var fullNames []string
	retainedBytes := 0
	seen := make(map[string]struct{})
	for n := 0; n < maxPackageCount; n++ {
		var hasCurrent uint8
		if hr := comCallOut(iterator, idxIteratorHasCurrent, unsafe.Pointer(&hasCurrent)); hresultFailed(hr) {
			return nil, hresultError("IIterator<Package>::get_HasCurrent", hr)
		}
		if hasCurrent == 0 {
			return fullNames, nil
		}

		var pkg *comIface
		if hr := comCallOut(iterator, idxIteratorCurrent, unsafe.Pointer(&pkg)); hresultFailed(hr) {
			return nil, hresultError("IIterator<Package>::get_Current", hr)
		}
		if pkg == nil {
			return nil, errors.New("IIterator<Package>::get_Current returned nil")
		}
		name, familyName, fullName, resourceID, readErr := readPackageIdentity(pkg)
		release(pkg)
		if readErr != nil {
			return nil, readErr
		}
		if strings.EqualFold(wanted, name) || strings.EqualFold(wanted, familyName) || strings.EqualFold(wanted, fullName) {
			key := strings.ToLower(fullName)
			if _, ok := seen[key]; !ok {
				// Bound the aggregate matched set, not just each name. A single
				// removal request should never accumulate an unbounded list.
				if len(fullNames) >= maxMatchedPackages || retainedBytes+len(fullName) > maxMatchedPackageBytes {
					return nil, fmt.Errorf(
						"package %q matched more than %d packages or %d bytes of package names",
						wanted, maxMatchedPackages, maxMatchedPackageBytes,
					)
				}
				retainedBytes += len(fullName)
				// Put the main package before resource packages. Removing the main
				// package often removes its resources as part of the same operation.
				if resourceID == "" {
					fullNames = append([]string{fullName}, fullNames...)
				} else {
					fullNames = append(fullNames, fullName)
				}
				seen[key] = struct{}{}
			}
		}

		var advanced uint8
		if hr := comCallOut(iterator, idxIteratorMoveNext, unsafe.Pointer(&advanced)); hresultFailed(hr) {
			return nil, hresultError("IIterator<Package>::MoveNext", hr)
		}
	}
	return nil, fmt.Errorf("package enumeration exceeded the %d package safety limit", maxPackageCount)
}

func readPackageIdentity(pkg *comIface) (name, familyName, fullName, resourceID string, err error) {
	var id *comIface
	if hr := comCallOut(pkg, idxPackageID, unsafe.Pointer(&id)); hresultFailed(hr) {
		return "", "", "", "", hresultError("IPackage::get_Id", hr)
	}
	if id == nil {
		return "", "", "", "", errors.New("IPackage::get_Id returned nil")
	}
	defer release(id)

	if name, err = getHSTRING(id, idxPackageIDName, "IPackageId::get_Name"); err != nil {
		return "", "", "", "", err
	}
	if familyName, err = getHSTRING(id, idxPackageIDFamilyName, "IPackageId::get_FamilyName"); err != nil {
		return "", "", "", "", err
	}
	if fullName, err = getHSTRING(id, idxPackageIDFullName, "IPackageId::get_FullName"); err != nil {
		return "", "", "", "", err
	}
	if resourceID, err = getHSTRING(id, idxPackageIDResourceID, "IPackageId::get_ResourceId"); err != nil {
		return "", "", "", "", err
	}
	if fullName == "" {
		return "", "", "", "", errors.New("IPackageId::get_FullName returned an empty string")
	}
	// Identity fields are compared and retained, so reject implausibly long
	// values rather than truncating them: a truncated identity could otherwise
	// be made to compare equal to an unrelated package.
	for _, field := range []struct {
		op    string
		value string
	}{
		{"IPackageId::get_Name", name},
		{"IPackageId::get_FamilyName", familyName},
		{"IPackageId::get_FullName", fullName},
		{"IPackageId::get_ResourceId", resourceID},
	} {
		if len(field.value) > maxPackageIdentityBytes {
			return "", "", "", "", fmt.Errorf(
				"%s returned %d bytes, exceeding the %d byte limit",
				field.op, len(field.value), maxPackageIdentityBytes,
			)
		}
	}
	return name, familyName, fullName, resourceID, nil
}

func removePackageByFullName(pm *comIface, fullName string) (err error) {
	// RemovePackageAsync takes an HSTRING — a SysAllocString BSTR is not
	// interchangeable, so the name must be created with WindowsCreateString.
	pkgName, err := windowsCreateString(fullName)
	if err != nil {
		return err
	}
	defer procWindowsDeleteString.Call(pkgName)

	var op *comIface
	if hr := comCallOut(pm, idxRemovePackageAsync, unsafe.Pointer(&op), pkgName); hresultFailed(hr) {
		if hr == hresultPackageNotFound {
			return nil
		}
		return hresultError("IPackageManager::RemovePackageAsync", hr)
	}
	if op == nil {
		return errors.New("RemovePackageAsync returned a nil operation")
	}
	defer release(op)

	// The returned IAsyncOperationWithProgress has its own vtable; status and
	// error members belong to the separate IAsyncInfo interface.
	var info *comIface
	if hr := queryInterface(op, &iidIAsyncInfo, &info); hresultFailed(hr) {
		return hresultError("QueryInterface(IAsyncInfo)", hr)
	}
	if info == nil {
		return errors.New("QueryInterface(IAsyncInfo) returned nil")
	}
	defer release(info)

	missing, terminal, waitErr := waitAsync(info)
	if terminal {
		// Close is only valid after completion, cancellation, or failure.
		defer func() {
			if hr := comCall(info, idxAsyncClose); hresultFailed(hr) {
				err = errors.Join(err, hresultError("IAsyncInfo::Close", hr))
			}
		}()
	}
	if waitErr != nil {
		return waitErr
	}
	if missing {
		return nil
	}
	return readDeploymentResult(op)
}

// readDeploymentResult calls the typed async operation's GetResults method.
// Package deployment operations can have AsyncStatus::Completed while carrying
// a failed ExtendedErrorCode, so status alone is not a success indication.
func readDeploymentResult(op *comIface) error {
	var result *comIface
	hr := comCallOut(op, idxAsyncGetResults, unsafe.Pointer(&result))
	if hr == hresultPackageNotFound {
		return nil
	}
	if hresultFailed(hr) {
		return hresultError("RemovePackageAsync::GetResults", hr)
	}
	if result == nil {
		return errors.New("RemovePackageAsync::GetResults returned nil")
	}
	defer release(result)

	var extended uint32
	if hr := comCallOut(result, idxDeploymentExtendedErrorCode, unsafe.Pointer(&extended)); hresultFailed(hr) {
		return hresultError("IDeploymentResult::get_ExtendedErrorCode", hr)
	}
	if extended == hresultPackageNotFound {
		return nil
	}
	if !hresultFailed(extended) {
		return nil
	}

	text, textErr := getHSTRING(result, idxDeploymentErrorText, "IDeploymentResult::get_ErrorText")
	failure := hresultError("RemovePackageAsync", extended)
	if text != "" {
		failure = fmt.Errorf("%w: %s", failure, text)
	}
	return errors.Join(failure, textErr)
}

// roActivateInstance activates a WinRT runtime class by its class name,
// returning an IInspectable pointer.
func roActivateInstance(classID uintptr, out **comIface) uint32 {
	addr := procRoActivateInstance.Addr()
	r, _, _ := syscall.SyscallN(addr, classID, uintptr(unsafe.Pointer(out)))
	runtime.KeepAlive(out)
	return uint32(r)
}

// waitAsync polls IAsyncInfo::get_Status until the operation completes,
// fails, or exceeds the timeout. terminal reports whether Close may legally be
// called. Cancellation is asynchronous, so a timeout requests cancellation and
// waits briefly for a terminal status rather than closing a running operation.
func waitAsync(op *comIface) (packageMissing, terminal bool, err error) {
	deadline := time.Now().Add(removeTimeout)
	var timeoutErr error
	var cancelDeadline time.Time
	for {
		var status int32
		if hr := comCallOut(op, idxGetStatus, unsafe.Pointer(&status)); hresultFailed(hr) {
			return false, false, errors.Join(timeoutErr, hresultError("IAsyncInfo::get_Status", hr))
		}
		switch status {
		case asyncStatusStarted:
			// Keep polling.
		case asyncStatusCompleted:
			return false, true, timeoutErr
		case asyncStatusCanceled:
			if timeoutErr != nil {
				return false, true, timeoutErr
			}
			return false, true, errors.New("appx removal canceled")
		case asyncStatusError:
			var code uint32
			if hr := comCallOut(op, idxGetErrorCode, unsafe.Pointer(&code)); hresultFailed(hr) {
				return false, true, errors.Join(timeoutErr, hresultError("IAsyncInfo::get_ErrorCode", hr))
			}
			if code == hresultPackageNotFound {
				return true, true, timeoutErr
			}
			return false, true, errors.Join(timeoutErr, hresultError("RemovePackageAsync", code))
		default:
			return false, false, errors.Join(timeoutErr, fmt.Errorf("IAsyncInfo::get_Status returned invalid status %d", status))
		}

		now := time.Now()
		if timeoutErr == nil && now.After(deadline) {
			timeoutErr = errors.New("appx removal timed out")
			if hr := comCall(op, idxAsyncCancel); hresultFailed(hr) {
				return false, false, errors.Join(timeoutErr, hresultError("IAsyncInfo::Cancel", hr))
			}
			cancelDeadline = now.Add(cancellationGrace)
		} else if timeoutErr != nil && now.After(cancelDeadline) {
			return false, false, errors.Join(timeoutErr, errors.New("appx removal did not acknowledge cancellation"))
		}
		time.Sleep(removePollInterval)
	}
}
