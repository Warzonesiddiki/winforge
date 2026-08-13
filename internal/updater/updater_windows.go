//go:build windows

package updater

import (
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	clsctxInprocServer  = 0x1
	coinitMultithreaded = 0x0

	// COM interface vtable indexes. Every interface below derives from
	// IDispatch, so indexes 0..2 are IUnknown and 3..6 are IDispatch; the
	// members follow. Indexes verified against mingw-w64 wuapi.h.
	idxCreateUpdateSearcher  = 12 // IUpdateSession::CreateUpdateSearcher
	idxCreateUpdateInstaller = 14 // IUpdateSession::CreateUpdateInstaller
	idxSearch                = 19 // IUpdateSearcher::Search
	idxGetUpdates            = 9  // ISearchResult::get_Updates
	idxGetCount              = 10 // IUpdateCollection::get_Count
	idxGetItem               = 7  // IUpdateCollection::get_Item
	idxGetTitle              = 7  // IUpdate::get_Title
	idxGetIsDownloaded       = 23 // IUpdate::get_IsDownloaded
	idxGetIsHidden           = 24 // IUpdate::get_IsHidden
	idxGetIsInstalled        = 26 // IUpdate::get_IsInstalled
	idxPutUpdates            = 16 // IUpdateInstaller::put_Updates
	idxInstall               = 21 // IUpdateInstaller::Install
	idxGetRebootRequired     = 8  // IInstallationResult::get_RebootRequired
	idxGetResultCode         = 9  // IInstallationResult::get_ResultCode
)

// guid mirrors a Win32 GUID (little-endian fields).
type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	// CLSID_UpdateSession = 4cb43d7f-7eee-4906-8698-60da1c38f2fe
	clsidUpdateSession = guid{0x4cb43d7f, 0x7eee, 0x4906, [8]byte{0x86, 0x98, 0x60, 0xda, 0x1c, 0x38, 0xf2, 0xfe}}
	// IID_IUpdateSession = 816858a4-260d-4260-933a-2585f1abc76b
	iidIUpdateSession = guid{0x816858a4, 0x260d, 0x4260, [8]byte{0x93, 0x3a, 0x25, 0x85, 0xf1, 0xab, 0xc7, 0x6b}}

	ole32    = syscall.NewLazyDLL("ole32.dll")
	oleaut32 = syscall.NewLazyDLL("oleaut32.dll")

	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procSysAllocString   = oleaut32.NewProc("SysAllocString")
	procSysFreeString    = oleaut32.NewProc("SysFreeString")
)

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

// connect initializes COM (pinning the OS thread) and creates an
// IUpdateSession. The returned cleanup must be deferred; it releases the
// session and uninitializes COM on the same thread.
func connect() (session *comIface, cleanup func(), err error) {
	// COM apartment init/uninit must happen on the same OS thread.
	runtime.LockOSThread()

	r, _, _ := procCoInitializeEx.Call(0, uintptr(coinitMultithreaded))
	switch {
	case r == 0: // S_OK — we initialized COM
	case r == 1: // S_FALSE — already initialized on this thread
	default:
		runtime.UnlockOSThread()
		return nil, nil, hresultError("CoInitializeEx", uint32(r))
	}
	initialized := r == 0

	if hr := coCreateInstance(&clsidUpdateSession, 0, clsctxInprocServer, &iidIUpdateSession, &session); hr != 0 {
		if initialized {
			procCoUninitialize.Call()
		}
		runtime.UnlockOSThread()
		return nil, nil, hresultError("CoCreateInstance(UpdateSession)", hr)
	}

	cleanup = func() {
		release(session)
		if initialized {
			procCoUninitialize.Call()
		}
		runtime.UnlockOSThread()
	}
	return session, cleanup, nil
}

func search(installedOnly bool) ([]Update, error) {
	session, cleanup, err := connect()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	var searcher *comIface
	if hr := comCall(session, idxCreateUpdateSearcher, uintptr(unsafe.Pointer(&searcher))); hr != 0 {
		return nil, hresultError("CreateUpdateSearcher", hr)
	}
	defer release(searcher)

	criteria := sysAllocString(searchCriteria(installedOnly))
	defer sysFreeString(criteria)

	var result *comIface
	if hr := comCall(searcher, idxSearch, criteria, uintptr(unsafe.Pointer(&result))); hr != 0 {
		return nil, hresultError("IUpdateSearcher::Search", hr)
	}
	defer release(result)

	var collection *comIface
	if hr := comCall(result, idxGetUpdates, uintptr(unsafe.Pointer(&collection))); hr != 0 {
		return nil, hresultError("ISearchResult::get_Updates", hr)
	}
	defer release(collection)

	var count int32
	if hr := comCall(collection, idxGetCount, uintptr(unsafe.Pointer(&count))); hr != 0 {
		return nil, hresultError("IUpdateCollection::get_Count", hr)
	}

	updates := make([]Update, 0, count)
	for i := int32(0); i < count; i++ {
		var upd *comIface
		if hr := comCall(collection, idxGetItem, uintptr(i), uintptr(unsafe.Pointer(&upd))); hr != 0 {
			return nil, hresultError("IUpdateCollection::get_Item", hr)
		}
		u := readUpdate(upd)
		release(upd)
		updates = append(updates, u)
	}
	return updates, nil
}

func installAll() (InstallResult, error) {
	session, cleanup, err := connect()
	if err != nil {
		return InstallResult{}, err
	}
	defer cleanup()

	var installer *comIface
	if hr := comCall(session, idxCreateUpdateInstaller, uintptr(unsafe.Pointer(&installer))); hr != 0 {
		return InstallResult{}, hresultError("CreateUpdateInstaller", hr)
	}
	defer release(installer)

	// Locate the not-installed updates to feed the installer.
	var searcher *comIface
	if hr := comCall(session, idxCreateUpdateSearcher, uintptr(unsafe.Pointer(&searcher))); hr != 0 {
		return InstallResult{}, hresultError("CreateUpdateSearcher", hr)
	}
	defer release(searcher)

	criteria := sysAllocString(searchCriteria(false))
	defer sysFreeString(criteria)

	var result *comIface
	if hr := comCall(searcher, idxSearch, criteria, uintptr(unsafe.Pointer(&result))); hr != 0 {
		return InstallResult{}, hresultError("IUpdateSearcher::Search", hr)
	}
	defer release(result)

	var collection *comIface
	if hr := comCall(result, idxGetUpdates, uintptr(unsafe.Pointer(&collection))); hr != 0 {
		return InstallResult{}, hresultError("ISearchResult::get_Updates", hr)
	}
	defer release(collection)

	if hr := comCall(installer, idxPutUpdates, uintptr(unsafe.Pointer(collection))); hr != 0 {
		return InstallResult{}, hresultError("IUpdateInstaller::put_Updates", hr)
	}

	var installResult *comIface
	if hr := comCall(installer, idxInstall, uintptr(unsafe.Pointer(&installResult))); hr != 0 {
		return InstallResult{}, hresultError("IUpdateInstaller::Install", hr)
	}
	defer release(installResult)

	var code int32
	_ = comCall(installResult, idxGetResultCode, uintptr(unsafe.Pointer(&code)))
	var reboot int16
	_ = comCall(installResult, idxGetRebootRequired, uintptr(unsafe.Pointer(&reboot)))

	return InstallResult{ResultCode: ResultCode(code), RebootRequired: reboot != 0}, nil
}

// readUpdate reads the display fields of an IUpdate.
func readUpdate(p *comIface) Update {
	var u Update
	if b := getBSTR(p, idxGetTitle); b != "" {
		u.Title = b
	}
	u.IsDownloaded = getBool(p, idxGetIsDownloaded)
	u.IsHidden = getBool(p, idxGetIsHidden)
	u.IsInstalled = getBool(p, idxGetIsInstalled)
	return u
}

func getBSTR(p *comIface, idx int) string {
	var b *uint16
	if hr := comCall(p, idx, uintptr(unsafe.Pointer(&b))); hr != 0 || b == nil {
		return ""
	}
	defer sysFreeString(uintptr(unsafe.Pointer(b)))
	return bstrToString(b)
}

func getBool(p *comIface, idx int) bool {
	var vb int16 // VARIANT_BOOL
	if hr := comCall(p, idx, uintptr(unsafe.Pointer(&vb))); hr != 0 {
		return false
	}
	return vb != 0
}
