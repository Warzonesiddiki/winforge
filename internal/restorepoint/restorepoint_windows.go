//go:build windows

package restorepoint

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	beginSystemChange  = 100 // BEGIN_SYSTEM_CHANGE
	endSystemChange    = 101 // END_SYSTEM_CHANGE
	applicationInstall = 20  // APPLICATION_INSTALL
)

var (
	srclient               = syscall.NewLazyDLL("srclient.dll")
	procSRSetRestorePointW = srclient.NewProc("SRSetRestorePointW")
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
	SequenceNumber int64
}

func call(spec *restorePointInfo, status *stateMgrStatus) error {
	r, _, callErr := procSRSetRestorePointW.Call(
		uintptr(unsafe.Pointer(spec)),
		uintptr(unsafe.Pointer(status)),
	)
	if r == 0 {
		if callErr != nil {
			return callErr
		}
		if status.Status != 0 {
			return syscall.Errno(status.Status)
		}
		return errors.New("SRSetRestorePointW failed")
	}
	return nil
}

func create(description string) (Info, error) {
	desc, err := syscall.UTF16FromString(description)
	if err != nil {
		return Info{}, err
	}
	if len(desc) > 256 {
		desc = desc[:256]
	}

	status := stateMgrStatus{}

	begin := restorePointInfo{EventType: beginSystemChange, RestorePtType: applicationInstall}
	copy(begin.Description[:], desc)
	if err := call(&begin, &status); err != nil {
		return Info{}, err
	}

	// Finalize the restore point by ending the change.
	end := restorePointInfo{EventType: endSystemChange, RestorePtType: applicationInstall, SequenceNumber: status.SequenceNumber}
	copy(end.Description[:], desc)
	_ = call(&end, &status)

	return Info{
		SequenceNumber: status.SequenceNumber,
		Description:    description,
		CreatedAt:      time.Now(),
	}, nil
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
	wbemInfinite              = 0xFFFFFFFF

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

	ole32    = syscall.NewLazyDLL("ole32.dll")
	oleaut32 = syscall.NewLazyDLL("oleaut32.dll")

	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procSysAllocString   = oleaut32.NewProc("SysAllocString")
	procSysFreeString    = oleaut32.NewProc("SysFreeString")
	procVariantClear     = oleaut32.NewProc("VariantClear")
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

func hresultError(op string, hr uint32) error {
	return fmt.Errorf("%s failed: HRESULT 0x%08X", op, hr)
}

func coCreateInstance(clsid *guid, outer uintptr, ctx uint32, iid *guid, out **comIface) uint32 {
	r, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(clsid)),
		outer,
		uintptr(ctx),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(out)),
	)
	return uint32(r)
}

func sysAllocString(s string) uintptr {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return 0
	}
	r, _, _ := procSysAllocString.Call(uintptr(unsafe.Pointer(p)))
	return r
}

func sysFreeString(p uintptr) {
	if p != 0 {
		_, _, _ = procSysFreeString.Call(p)
	}
}

func variantClear(v *variant) {
	_, _, _ = procVariantClear.Call(uintptr(unsafe.Pointer(v)))
}

// bstrToString converts a BSTR (length-prefixed UTF-16) to a Go string.
func bstrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	byteLen := *(*uint32)(unsafe.Add(unsafe.Pointer(p), -4))
	if byteLen == 0 || byteLen > 1<<20 {
		return ""
	}
	units := unsafe.Slice(p, int(byteLen/2))
	return string(utf16.Decode(units))
}

// getProp fetches a named property into v.
func getProp(obj *comIface, name string, v *variant) uint32 {
	pname, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0x80070057 // E_INVALIDARG
	}
	return comCall(obj, iwbemClassObjectGet, uintptr(unsafe.Pointer(pname)), 0, uintptr(unsafe.Pointer(v)), 0, 0)
}

func readUint32Prop(obj *comIface, name string) (uint32, error) {
	var v variant
	if hr := getProp(obj, name, &v); hr != 0 {
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
	if hr := getProp(obj, name, &v); hr != 0 {
		return "", hresultError("IWbemClassObject::Get("+name+")", hr)
	}
	defer variantClear(&v)
	if v.vt != vtBstr {
		return "", fmt.Errorf("unexpected variant type %d for %s", v.vt, name)
	}
	return bstrToString(*(**uint16)(unsafe.Pointer(&v.data[0]))), nil
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
	// COM apartment init/uninit must happen on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	r, _, _ := procCoInitializeEx.Call(0, uintptr(coinitMultithreaded))
	switch {
	case r == 0: // S_OK
		defer procCoUninitialize.Call()
	case r == 1: // S_FALSE — COM already initialized on this thread
	default:
		return nil, hresultError("CoInitializeEx", uint32(r))
	}

	var locator *comIface
	if hr := coCreateInstance(&clsidWbemLocator, 0, clsctxInprocServer, &iidIWbemLocator, &locator); hr != 0 {
		return nil, hresultError("CoCreateInstance(WbemLocator)", hr)
	}
	defer release(locator)

	netRes := sysAllocString(`root\default`)
	defer sysFreeString(netRes)
	var services *comIface
	if hr := comCall(locator, iwbemLocatorConnectServer, netRes, 0, 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&services))); hr != 0 {
		return nil, hresultError("IWbemLocator::ConnectServer", hr)
	}
	defer release(services)

	lang := sysAllocString("WQL")
	defer sysFreeString(lang)
	query := sysAllocString("SELECT * FROM SystemRestore")
	defer sysFreeString(query)

	var enum *comIface
	flags := uintptr(wbemFlagForwardOnly | wbemFlagReturnImmediately)
	if hr := comCall(services, iwbemServicesExecQuery, lang, query, flags, 0, uintptr(unsafe.Pointer(&enum))); hr != 0 {
		return nil, hresultError("IWbemServices::ExecQuery", hr)
	}
	defer release(enum)

	var out []Info
	for {
		var obj *comIface
		var returned uint32
		if hr := comCall(enum, ienumWbemClassObjectNext, uintptr(wbemInfinite), 1, uintptr(unsafe.Pointer(&obj)), uintptr(unsafe.Pointer(&returned))); hr != 0 {
			return nil, hresultError("IEnumWbemClassObject::Next", hr)
		}
		if returned == 0 {
			break
		}
		info, err := readRestorePoint(obj)
		release(obj)
		if err != nil {
			continue // best-effort: skip malformed entries
		}
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
