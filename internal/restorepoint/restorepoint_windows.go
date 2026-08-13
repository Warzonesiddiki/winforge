//go:build windows

package restorepoint

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"winforge/internal/winapi"
)

const (
	beginSystemChange  = 100 // BEGIN_SYSTEM_CHANGE
	endSystemChange    = 101 // END_SYSTEM_CHANGE
	applicationInstall = 20  // APPLICATION_INSTALL

	securityDescriptorRevision  = 1
	winSelfSid                  = 10
	winLocalSystemSid           = 22
	winLocalServiceSid          = 23
	winNetworkServiceSid        = 24
	winBuiltinAdministratorsSid = 26
	setAccess                   = 2
	trusteeIsSID                = 0
	trusteeIsGroup              = 2
	comRightsExecuteLocal       = 0x1 | 0x2
	rpcAuthnWinNT               = 10
	rpcAuthzNone                = 0
	rpcAuthnLevelPacketPrivacy  = 6
	rpcImpersonationIdentify    = 2
	rpcImpersonationImpersonate = 3
	eoacNone                    = 0
	eoacDisableAAA              = 0x1000
	eoacNoCustomMarshal         = 0x2000
	lmemFixedZeroinit           = 0x0040
	securityMaxSIDSize          = 68
	rpcETooLate                 = 0x80010119
	errorServiceDisabled        = 1058
)

var (
	srclient               = winapi.SystemDLL("srclient.dll")
	advapi32               = syscall.NewLazyDLL("advapi32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procSRSetRestorePointW = srclient.NewProc("SRSetRestorePointW")

	procInitializeSecurityDescriptor = advapi32.NewProc("InitializeSecurityDescriptor")
	procCreateWellKnownSid           = advapi32.NewProc("CreateWellKnownSid")
	procSetEntriesInAclW             = advapi32.NewProc("SetEntriesInAclW")
	procSetSecurityDescriptorOwner   = advapi32.NewProc("SetSecurityDescriptorOwner")
	procSetSecurityDescriptorGroup   = advapi32.NewProc("SetSecurityDescriptorGroup")
	procSetSecurityDescriptorDacl    = advapi32.NewProc("SetSecurityDescriptorDacl")
	procLocalAlloc                   = kernel32.NewProc("LocalAlloc")
	procLocalFree                    = kernel32.NewProc("LocalFree")

	comSecurityOnce sync.Once
	comSecurityErr  error
)

// restorePointInfo mirrors the Win32 RESTOREPOINTINFOW structure.
type restorePointInfo struct {
	EventType      uint32
	RestorePtType  uint32
	SequenceNumber int64
	Description    [256]uint16
}

// stateMgrStatus mirrors the Win32 STATEMGRSTATUS structure.
type stateMgrStatus struct {
	Status         uint32
	_              uint32 // STATEMGRSTATUS aligns the following INT64 to 8 bytes.
	SequenceNumber int64
}

// These structures mirror SECURITY_DESCRIPTOR, TRUSTEE_W, and
// EXPLICIT_ACCESS_W. Pointer-sized fields preserve the native layout on both
// 32- and 64-bit Windows.
type securityDescriptor struct {
	Revision byte
	Sbz1     byte
	Control  uint16
	Owner    uintptr
	Group    uintptr
	Sacl     uintptr
	Dacl     uintptr
}

type trustee struct {
	MultipleTrustee          uintptr
	MultipleTrusteeOperation int32
	TrusteeForm              int32
	TrusteeType              int32
	Name                     uintptr
}

type explicitAccess struct {
	AccessPermissions uint32
	AccessMode        int32
	Inheritance       uint32
	Trustee           trustee
}

// nativeArg preserves Go pointers across helper calls. Converting a stack
// pointer to uintptr in the caller can leave a stale address if the stack grows
// before the syscall; conversion is therefore delayed until the final helper.
type nativeArg struct {
	value   uintptr
	pointer unsafe.Pointer
}

func valueArg(value uintptr) nativeArg            { return nativeArg{value: value} }
func pointerArg(pointer unsafe.Pointer) nativeArg { return nativeArg{pointer: pointer} }

func call(spec *restorePointInfo, status *stateMgrStatus) error {
	addr := procSRSetRestorePointW.Addr()
	r, _, callErr := syscall.SyscallN(
		addr,
		uintptr(unsafe.Pointer(spec)),
		uintptr(unsafe.Pointer(status)),
	)
	runtime.KeepAlive(spec)
	runtime.KeepAlive(status)
	if status.Status != 0 {
		return restorePointWin32Error(status.Status)
	}
	if r == 0 {
		if errno, ok := callErr.(syscall.Errno); ok {
			if errno != 0 {
				return restorePointWin32Error(uint32(errno))
			}
		} else if callErr != nil {
			return callErr
		}
		return errors.New("SRSetRestorePointW failed")
	}
	return nil
}

func restorePointWin32Error(code uint32) error {
	if code == errorServiceDisabled {
		return fmt.Errorf("%w (Win32 error %d)", ErrDisabled, code)
	}
	return syscall.Errno(code)
}

// enterCOM initializes a COM apartment on a pinned thread and ensures the
// process-wide COM access policy required by SRSetRestorePointW is installed.
// Its cleanup must run on the same goroutine.
func enterCOM() (func(), error) {
	runtime.LockOSThread()
	r, _, _ := procCoInitializeEx.Call(0, uintptr(coinitMultithreaded))
	if r != 0 && r != 1 { // S_OK and S_FALSE both require CoUninitialize.
		runtime.UnlockOSThread()
		return nil, hresultError("CoInitializeEx", uint32(r))
	}
	if err := initializeCOMSecurity(); err != nil {
		procCoUninitialize.Call()
		runtime.UnlockOSThread()
		return nil, err
	}
	return func() {
		procCoUninitialize.Call()
		runtime.UnlockOSThread()
	}, nil
}

func initializeCOMSecurity() error {
	comSecurityOnce.Do(func() {
		// sync.Once considers a panicking function complete. Native procedure
		// resolution can panic, so recover inside the once body and persist the
		// failure instead of making every later caller observe a false success.
		defer func() {
			if recovered := recover(); recovered != nil {
				comSecurityErr = fmt.Errorf("initialize COM security: %v", recovered)
			}
		}()
		comSecurityErr = configureCOMSecurity()
	})
	return comSecurityErr
}

// configureCOMSecurity follows Microsoft's SRSetRestorePoint guidance: permit
// local COM callbacks from Administrators, LocalService, NetworkService, Self,
// and LocalSystem. CoInitializeSecurity requires an absolute (not self-relative)
// security descriptor, so it is assembled with the Win32 ACL APIs.
func configureCOMSecurity() error {
	var descriptor securityDescriptor
	if err := requireWin32Success(
		"InitializeSecurityDescriptor",
		procInitializeSecurityDescriptor,
		pointerArg(unsafe.Pointer(&descriptor)),
		valueArg(securityDescriptorRevision),
	); err != nil {
		return err
	}

	sidTypes := [...]uint32{
		winBuiltinAdministratorsSid,
		winLocalServiceSid,
		winNetworkServiceSid,
		winSelfSid,
		winLocalSystemSid,
	}
	// SECURITY_DESCRIPTOR and TRUSTEE_W retain SID pointers across several
	// API calls. Allocate those buffers from the Windows local heap rather than
	// embedding Go stack pointers in uintptr fields.
	sidBuffers := make([]uintptr, len(sidTypes))
	entries := make([]explicitAccess, len(sidTypes))
	for i, sidType := range sidTypes {
		buffer, _, allocErr := procLocalAlloc.Call(lmemFixedZeroinit, securityMaxSIDSize)
		if buffer == 0 {
			if allocErr != nil && allocErr != syscall.Errno(0) {
				return fmt.Errorf("allocate COM security SID %d: %w", sidType, allocErr)
			}
			return fmt.Errorf("allocate COM security SID %d", sidType)
		}
		sidBuffers[i] = buffer
		defer procLocalFree.Call(buffer)

		size := uint32(securityMaxSIDSize)
		if err := requireWin32Success(
			"CreateWellKnownSid",
			procCreateWellKnownSid,
			valueArg(uintptr(sidType)),
			valueArg(0),
			valueArg(buffer),
			pointerArg(unsafe.Pointer(&size)),
		); err != nil {
			return fmt.Errorf("create COM security SID %d: %w", sidType, err)
		}
		entries[i] = explicitAccess{
			AccessPermissions: comRightsExecuteLocal,
			AccessMode:        setAccess,
			Trustee: trustee{
				TrusteeForm: trusteeIsSID,
				TrusteeType: trusteeIsGroup,
				Name:        buffer,
			},
		}
	}

	var acl uintptr
	setEntriesInACL := procSetEntriesInAclW.Addr()
	status, _, _ := syscall.SyscallN(
		setEntriesInACL,
		uintptr(len(entries)),
		uintptr(unsafe.Pointer(&entries[0])),
		0,
		uintptr(unsafe.Pointer(&acl)),
	)
	runtime.KeepAlive(entries)
	runtime.KeepAlive(sidBuffers)
	if status != 0 {
		return fmt.Errorf("SetEntriesInAclW: %w", syscall.Errno(status))
	}
	if acl == 0 {
		return errors.New("SetEntriesInAclW returned a nil ACL")
	}
	defer procLocalFree.Call(acl)

	adminSID := sidBuffers[0]
	if err := requireWin32Success(
		"SetSecurityDescriptorOwner",
		procSetSecurityDescriptorOwner,
		pointerArg(unsafe.Pointer(&descriptor)), valueArg(adminSID), valueArg(0),
	); err != nil {
		return err
	}
	if err := requireWin32Success(
		"SetSecurityDescriptorGroup",
		procSetSecurityDescriptorGroup,
		pointerArg(unsafe.Pointer(&descriptor)), valueArg(adminSID), valueArg(0),
	); err != nil {
		return err
	}
	if err := requireWin32Success(
		"SetSecurityDescriptorDacl",
		procSetSecurityDescriptorDacl,
		pointerArg(unsafe.Pointer(&descriptor)), valueArg(1), valueArg(acl), valueArg(0),
	); err != nil {
		return err
	}

	coInitializeSecurity := procCoInitializeSecurity.Addr()
	hr, _, _ := syscall.SyscallN(
		coInitializeSecurity,
		uintptr(unsafe.Pointer(&descriptor)),
		uintptr(^uint32(0)), // let COM choose authentication services
		0,
		0,
		rpcAuthnLevelPacketPrivacy,
		rpcImpersonationIdentify,
		0,
		eoacDisableAAA|eoacNoCustomMarshal,
		0,
	)
	runtime.KeepAlive(sidBuffers)
	runtime.KeepAlive(descriptor)
	result := uint32(hr)
	if result == rpcETooLate {
		// Another COM client initialized process security first. It cannot be
		// replaced, so continue under that existing policy and let the restore
		// point API report any actual callback-access failure.
		return nil
	}
	if hresultFailed(result) {
		return hresultError("CoInitializeSecurity", result)
	}
	return nil
}

func requireWin32Success(op string, proc *syscall.LazyProc, args ...nativeArg) error {
	addr := proc.Addr() // Resolve the lazy procedure before converting pointers.
	values := make([]uintptr, len(args))
	for i, arg := range args {
		if arg.pointer != nil {
			values[i] = uintptr(arg.pointer)
		} else {
			values[i] = arg.value
		}
	}
	r1, _, callErr := syscall.SyscallN(addr, values...)
	runtime.KeepAlive(args)
	if r1 != 0 {
		return nil
	}
	if callErr != nil && callErr != syscall.Errno(0) {
		return fmt.Errorf("%s: %w", op, callErr)
	}
	return fmt.Errorf("%s failed", op)
}

func create(description string) (Info, error) {
	desc, err := syscall.UTF16FromString(description)
	if err != nil {
		return Info{}, err
	}
	if len(desc) > 256 {
		// Leave room for NUL and do not split a surrogate pair at the fixed
		// RESTOREPOINTINFOW description boundary.
		desc = desc[:255]
		if n := len(desc); n > 0 && desc[n-1] >= 0xD800 && desc[n-1] <= 0xDBFF {
			desc = desc[:n-1]
		}
		desc = append(desc, 0)
		description = string(utf16.Decode(desc[:len(desc)-1]))
	}

	cleanup, err := enterCOM()
	if err != nil {
		return Info{}, err
	}
	defer cleanup()

	status := stateMgrStatus{}

	begin := restorePointInfo{EventType: beginSystemChange, RestorePtType: applicationInstall}
	copy(begin.Description[:], desc)
	if err := call(&begin, &status); err != nil {
		return Info{}, err
	}
	sequence := status.SequenceNumber
	info := Info{SequenceNumber: sequence, Description: description, CreatedAt: time.Now()}

	// Finalize the restore point by ending the change.
	end := restorePointInfo{EventType: endSystemChange, RestorePtType: applicationInstall, SequenceNumber: sequence}
	copy(end.Description[:], desc)
	status = stateMgrStatus{}
	if err := call(&end, &status); err != nil {
		return info, fmt.Errorf("finalize restore point: %w", err)
	}
	return info, nil
}

func isEnabled() bool {
	return srclient.Load() == nil
}

// ---------------------------------------------------------------------------
// Enumerating existing restore points via WMI (root\default:SystemRestore).
//
// Implemented with raw COM P/Invoke against ole32/oleaut32 — no PowerShell,
// no third-party modules, no WMI/COM wrapper libraries. The interface vtable
// indexes and GUIDs below match wbemcli.h.
// ---------------------------------------------------------------------------

const (
	// GUIDs
	// CLSID_WbemLocator = 4590f811-1d3a-11d0-891f-00aa004b2e24
	// IID_IWbemLocator  = dc12a687-737f-11cf-884d-00aa004b2e24

	clsctxInprocServer        = 0x1
	coinitMultithreaded       = 0x0
	wbemFlagForwardOnly       = 0x20
	wbemFlagReturnImmediately = 0x10
	wbemNextTimeoutMillis     = 30000
	wbemSFalse                = 0x00000001
	wbemSTimedout             = 0x00040004

	// VARIANT types.
	vtI4   = 3
	vtBstr = 8
	vtUI4  = 19

	// COM interface vtable indexes (index 0..2 are IUnknown).
	iwbemLocatorConnectServer = 3
	iwbemServicesExecQuery    = 20
	ienumWbemClassObjectNext  = 4
	iwbemClassObjectGet       = 4
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	clsidWbemLocator = guid{0x4590f811, 0x1d3a, 0x11d0, [8]byte{0x89, 0x1f, 0x00, 0xaa, 0x00, 0x4b, 0x2e, 0x24}}
	iidIWbemLocator  = guid{0xdc12a687, 0x737f, 0x11cf, [8]byte{0x88, 0x4d, 0x00, 0xaa, 0x00, 0x4b, 0x2e, 0x24}}

	ole32    = winapi.SystemDLL("ole32.dll")
	oleaut32 = winapi.SystemDLL("oleaut32.dll")

	procCoInitializeEx       = ole32.NewProc("CoInitializeEx")
	procCoInitializeSecurity = ole32.NewProc("CoInitializeSecurity")
	procCoSetProxyBlanket    = ole32.NewProc("CoSetProxyBlanket")
	procCoUninitialize       = ole32.NewProc("CoUninitialize")
	procCoCreateInstance     = ole32.NewProc("CoCreateInstance")
	procSysAllocString       = oleaut32.NewProc("SysAllocString")
	procSysFreeString        = oleaut32.NewProc("SysFreeString")
	procVariantClear         = oleaut32.NewProc("VariantClear")
)

// variant mirrors the Win32 VARIANT structure (24 bytes on x64).
type variant struct {
	vt        uint16
	reserved1 uint16
	reserved2 uint16
	reserved3 uint16
	data      [16]byte // union; BSTR/uint32 values are read from the low bytes
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

// comCall invokes a COM virtual method and returns its HRESULT. Pointer
// arguments remain GC-visible until all allocation and vtable lookup work is
// complete, then are converted immediately before SyscallN.
func comCall(p *comIface, i int, args ...nativeArg) uint32 {
	method := vtable(p, i)
	callArgs := make([]uintptr, len(args)+1)
	callArgs[0] = uintptr(unsafe.Pointer(p))
	for i, arg := range args {
		if arg.pointer != nil {
			callArgs[i+1] = uintptr(arg.pointer)
		} else {
			callArgs[i+1] = arg.value
		}
	}
	r1, _, _ := syscall.SyscallN(method, callArgs...)
	runtime.KeepAlive(args)
	runtime.KeepAlive(p)
	return uint32(r1)
}

// release decrements a COM object's reference count (IUnknown::Release).
func release(p *comIface) {
	if p != nil {
		_ = comCall(p, 2)
	}
}

func hresultError(op string, hr uint32) error {
	return fmt.Errorf("%s failed: HRESULT 0x%08X", op, hr)
}

func hresultFailed(hr uint32) bool {
	return hr&0x80000000 != 0
}

func coCreateInstance(clsid *guid, outer uintptr, ctx uint32, iid *guid, out **comIface) uint32 {
	addr := procCoCreateInstance.Addr()
	r, _, _ := syscall.SyscallN(
		addr,
		uintptr(unsafe.Pointer(clsid)),
		outer,
		uintptr(ctx),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(out)),
	)
	runtime.KeepAlive(out)
	runtime.KeepAlive(iid)
	runtime.KeepAlive(clsid)
	return uint32(r)
}

func setProxyBlanket(p *comIface) error {
	addr := procCoSetProxyBlanket.Addr()
	hr, _, _ := syscall.SyscallN(
		addr,
		uintptr(unsafe.Pointer(p)),
		rpcAuthnWinNT,
		rpcAuthzNone,
		0,
		rpcAuthnLevelPacketPrivacy,
		rpcImpersonationImpersonate,
		0,
		eoacNone,
	)
	runtime.KeepAlive(p)
	if result := uint32(hr); hresultFailed(result) {
		return hresultError("CoSetProxyBlanket(IWbemServices)", result)
	}
	return nil
}

func sysAllocString(s string) uintptr {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return 0
	}
	addr := procSysAllocString.Addr()
	r, _, _ := syscall.SyscallN(addr, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	return r
}

func sysFreeString(p uintptr) {
	if p != 0 {
		_, _, _ = procSysFreeString.Call(p)
	}
}

func variantClear(v *variant) {
	addr := procVariantClear.Addr()
	_, _, _ = syscall.SyscallN(addr, uintptr(unsafe.Pointer(v)))
	runtime.KeepAlive(v)
}

// bstrToString converts a BSTR (length-prefixed UTF-16) to a Go string.
func bstrToString(p *uint16) (string, error) {
	if p == nil {
		return "", nil
	}
	byteLen := *(*uint32)(unsafe.Add(unsafe.Pointer(p), -4))
	if byteLen == 0 {
		return "", nil
	}
	if byteLen > 1<<20 || byteLen%2 != 0 {
		return "", fmt.Errorf("invalid BSTR byte length %d", byteLen)
	}
	units := unsafe.Slice(p, int(byteLen/2))
	return string(utf16.Decode(units)), nil
}

// getProp fetches a named property into v.
func getProp(obj *comIface, name string, v *variant) uint32 {
	pname, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0x80070057 // E_INVALIDARG
	}
	hr := comCall(
		obj,
		iwbemClassObjectGet,
		pointerArg(unsafe.Pointer(pname)),
		valueArg(0),
		pointerArg(unsafe.Pointer(v)),
		valueArg(0),
		valueArg(0),
	)
	runtime.KeepAlive(pname)
	return hr
}

func readUint32Prop(obj *comIface, name string) (uint32, error) {
	var v variant
	hr := getProp(obj, name, &v)
	defer variantClear(&v)
	if hresultFailed(hr) {
		return 0, hresultError("IWbemClassObject::Get("+name+")", hr)
	}
	switch v.vt {
	case vtUI4:
		return *(*uint32)(unsafe.Pointer(&v.data[0])), nil
	case vtI4:
		return uint32(*(*int32)(unsafe.Pointer(&v.data[0]))), nil
	default:
		return 0, fmt.Errorf("unexpected variant type %d for %s", v.vt, name)
	}
}

func readStringProp(obj *comIface, name string) (string, error) {
	var v variant
	hr := getProp(obj, name, &v)
	defer variantClear(&v)
	if hresultFailed(hr) {
		return "", hresultError("IWbemClassObject::Get("+name+")", hr)
	}
	if v.vt != vtBstr {
		return "", fmt.Errorf("unexpected variant type %d for %s", v.vt, name)
	}
	return bstrToString(*(**uint16)(unsafe.Pointer(&v.data[0])))
}

func readRestorePoint(obj *comIface) (Info, error) {
	var info Info

	seq, err := readUint32Prop(obj, "SequenceNumber")
	if err != nil {
		return Info{}, err
	}
	info.SequenceNumber = int64(seq)

	if info.Description, err = readStringProp(obj, "Description"); err != nil {
		return Info{}, err
	}

	created, err := readStringProp(obj, "CreationTime")
	if err != nil {
		return Info{}, err
	}
	if info.CreatedAt, err = parseWmiTime(created); err != nil {
		return Info{}, err
	}
	return info, nil
}

// list enumerates existing system restore points via WMI, newest first.
func list() ([]Info, error) {
	cleanup, err := enterCOM()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	var locator *comIface
	if hr := coCreateInstance(&clsidWbemLocator, 0, clsctxInprocServer, &iidIWbemLocator, &locator); hresultFailed(hr) {
		return nil, hresultError("CoCreateInstance(WbemLocator)", hr)
	}
	if locator == nil {
		return nil, errors.New("CoCreateInstance(WbemLocator) returned nil")
	}
	defer release(locator)

	netRes := sysAllocString(`root\default`)
	if netRes == 0 {
		return nil, errors.New("allocate WMI namespace")
	}
	defer sysFreeString(netRes)
	var services *comIface
	if hr := comCall(
		locator,
		iwbemLocatorConnectServer,
		valueArg(netRes),
		valueArg(0),
		valueArg(0),
		valueArg(0),
		valueArg(0),
		valueArg(0),
		valueArg(0),
		pointerArg(unsafe.Pointer(&services)),
	); hresultFailed(hr) {
		return nil, hresultError("IWbemLocator::ConnectServer", hr)
	}
	if services == nil {
		return nil, errors.New("IWbemLocator::ConnectServer returned nil")
	}
	defer release(services)
	// Process-wide COM security supplies defaults, but WMI also requires an
	// impersonating blanket on the connected IWbemServices proxy—even for a
	// local namespace—before providers are queried.
	if err := setProxyBlanket(services); err != nil {
		return nil, err
	}

	lang := sysAllocString("WQL")
	if lang == 0 {
		return nil, errors.New("allocate WMI query language")
	}
	defer sysFreeString(lang)
	query := sysAllocString("SELECT * FROM SystemRestore")
	if query == 0 {
		return nil, errors.New("allocate WMI query")
	}
	defer sysFreeString(query)

	var enum *comIface
	flags := uintptr(wbemFlagForwardOnly | wbemFlagReturnImmediately)
	if hr := comCall(
		services,
		iwbemServicesExecQuery,
		valueArg(lang),
		valueArg(query),
		valueArg(flags),
		valueArg(0),
		pointerArg(unsafe.Pointer(&enum)),
	); hresultFailed(hr) {
		return nil, hresultError("IWbemServices::ExecQuery", hr)
	}
	if enum == nil {
		return nil, errors.New("IWbemServices::ExecQuery returned nil")
	}
	defer release(enum)

	var out []Info
	var readErrs []error
	for n := 0; ; n++ {
		if n >= 100000 {
			return nil, errors.New("WMI restore-point enumeration exceeded safety limit")
		}
		var obj *comIface
		var returned uint32
		hr := comCall(
			enum,
			ienumWbemClassObjectNext,
			valueArg(wbemNextTimeoutMillis),
			valueArg(1),
			pointerArg(unsafe.Pointer(&obj)),
			pointerArg(unsafe.Pointer(&returned)),
		)
		if hresultFailed(hr) {
			release(obj)
			return nil, hresultError("IEnumWbemClassObject::Next", hr)
		}
		if hr == wbemSTimedout {
			release(obj)
			return nil, errors.New("IEnumWbemClassObject::Next timed out")
		}
		if returned == 0 {
			release(obj)
			if hr != wbemSFalse {
				return nil, fmt.Errorf("IEnumWbemClassObject::Next returned no object with HRESULT 0x%08X", hr)
			}
			break
		}
		if returned != 1 || obj == nil || hr != 0 {
			release(obj)
			return nil, fmt.Errorf("IEnumWbemClassObject::Next returned count %d, object %p, HRESULT 0x%08X", returned, obj, hr)
		}
		info, err := readRestorePoint(obj)
		release(obj)
		if err != nil {
			readErrs = append(readErrs, fmt.Errorf("restore point %d: %w", n, err))
			continue // retain valid rows, but report that the result is incomplete
		}
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(readErrs) > 0 {
		return out, fmt.Errorf("read WMI restore points: %w", errors.Join(readErrs...))
	}
	return out, nil
}
