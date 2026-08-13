//go:build windows

package updater

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"winforge/internal/winapi"
)

const (
	clsctxInprocServer  = 0x1
	coinitMultithreaded = 0x0

	// COM interface vtable indexes. Every interface below derives from
	// IDispatch, so indexes 0..2 are IUnknown and 3..6 are IDispatch; the
	// members follow. Indexes verified against mingw-w64 wuapi.h.
	idxCreateUpdateSearcher               = 12 // IUpdateSession::CreateUpdateSearcher
	idxCreateUpdateDownloader             = 13 // IUpdateSession::CreateUpdateDownloader
	idxCreateUpdateInstaller              = 14 // IUpdateSession::CreateUpdateInstaller
	idxSearch                             = 19 // IUpdateSearcher::Search
	idxSearchResultCode                   = 7  // ISearchResult::get_ResultCode
	idxGetUpdates                         = 9  // ISearchResult::get_Updates
	idxGetCount                           = 10 // IUpdateCollection::get_Count
	idxGetItem                            = 7  // IUpdateCollection::get_Item
	idxGetTitle                           = 7  // IUpdate::get_Title
	idxGetIsDownloaded                    = 23 // IUpdate::get_IsDownloaded
	idxGetIsHidden                        = 24 // IUpdate::get_IsHidden
	idxGetIsInstalled                     = 26 // IUpdate::get_IsInstalled
	idxDownloaderPutUpdates               = 14 // IUpdateDownloader::put_Updates
	idxDownload                           = 16 // IUpdateDownloader::Download
	idxDownloadResultCode                 = 8  // IDownloadResult::get_ResultCode
	idxDownloadGetUpdateResult            = 9  // IDownloadResult::GetUpdateResult
	idxUpdateDownloadResultHResult        = 7  // IUpdateDownloadResult::get_HResult
	idxUpdateDownloadResultCode           = 8  // IUpdateDownloadResult::get_ResultCode
	idxInstallerPutUpdates                = 16 // IUpdateInstaller::put_Updates
	idxInstall                            = 21 // IUpdateInstaller::Install
	idxGetRebootRequired                  = 8  // IInstallationResult::get_RebootRequired
	idxInstallationResultCode             = 9  // IInstallationResult::get_ResultCode
	idxInstallationGetUpdateResult        = 10 // IInstallationResult::GetUpdateResult
	idxUpdateInstallationResultHResult    = 7  // IUpdateInstallationResult::get_HResult
	idxUpdateInstallationResultResultCode = 9  // IUpdateInstallationResult::get_ResultCode
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

	ole32    = winapi.SystemDLL("ole32.dll")
	oleaut32 = winapi.SystemDLL("oleaut32.dll")

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
	method := vtable(p, i)
	callArgs := make([]uintptr, 1, len(args)+1)
	callArgs[0] = uintptr(unsafe.Pointer(p))
	callArgs = append(callArgs, args...)
	r1, _, _ := syscall.SyscallN(method, callArgs...)
	runtime.KeepAlive(p)
	return uint32(r1)
}

// comCallOut keeps a pointer to Go memory tracked across this helper call and
// appends it as the final COM argument only after allocating the argument list.
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

// comCallIface invokes a COM method with another interface pointer while both
// pointers remain typed until the final native dispatch.
func comCallIface(p *comIface, i int, arg *comIface) uint32 {
	method := vtable(p, i)
	r1, _, _ := syscall.SyscallN(
		method,
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(arg)),
	)
	runtime.KeepAlive(arg)
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
	if hr := coCreateInstance(&clsidUpdateSession, 0, clsctxInprocServer, &iidIUpdateSession, &session); hresultFailed(hr) {
		// S_OK and S_FALSE both require a balancing CoUninitialize.
		procCoUninitialize.Call()
		runtime.UnlockOSThread()
		return nil, nil, hresultError("CoCreateInstance(UpdateSession)", hr)
	}
	if session == nil {
		procCoUninitialize.Call()
		runtime.UnlockOSThread()
		return nil, nil, fmt.Errorf("CoCreateInstance(UpdateSession) returned nil")
	}

	cleanup = func() {
		release(session)
		procCoUninitialize.Call()
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
	if hr := comCallOut(session, idxCreateUpdateSearcher, unsafe.Pointer(&searcher)); hresultFailed(hr) {
		return nil, hresultError("CreateUpdateSearcher", hr)
	}
	if searcher == nil {
		return nil, fmt.Errorf("CreateUpdateSearcher returned nil")
	}
	defer release(searcher)

	criteria := sysAllocString(searchCriteria(installedOnly))
	if criteria == 0 {
		return nil, fmt.Errorf("allocate Windows Update search criteria")
	}
	defer sysFreeString(criteria)

	var result *comIface
	if hr := comCallOut(searcher, idxSearch, unsafe.Pointer(&result), criteria); hresultFailed(hr) {
		return nil, hresultError("IUpdateSearcher::Search", hr)
	}
	if result == nil {
		return nil, fmt.Errorf("IUpdateSearcher::Search returned nil")
	}
	defer release(result)

	searchCode, err := getOperationResultCode(result, idxSearchResultCode, "ISearchResult::get_ResultCode")
	if err != nil {
		return nil, err
	}
	if searchCode != ResultSucceeded {
		return nil, fmt.Errorf("Windows Update search completed with result %s", searchCode)
	}

	var collection *comIface
	if hr := comCallOut(result, idxGetUpdates, unsafe.Pointer(&collection)); hresultFailed(hr) {
		return nil, hresultError("ISearchResult::get_Updates", hr)
	}
	if collection == nil {
		return nil, fmt.Errorf("ISearchResult::get_Updates returned nil")
	}
	defer release(collection)

	var count int32
	if hr := comCallOut(collection, idxGetCount, unsafe.Pointer(&count)); hresultFailed(hr) {
		return nil, hresultError("IUpdateCollection::get_Count", hr)
	}

	if count < 0 || count > 100000 {
		return nil, fmt.Errorf("IUpdateCollection::get_Count returned invalid count %d", count)
	}
	updates := make([]Update, 0, count)
	for i := int32(0); i < count; i++ {
		var upd *comIface
		if hr := comCallOut(collection, idxGetItem, unsafe.Pointer(&upd), uintptr(i)); hresultFailed(hr) {
			return nil, hresultError("IUpdateCollection::get_Item", hr)
		}
		if upd == nil {
			return nil, fmt.Errorf("IUpdateCollection::get_Item(%d) returned nil", i)
		}
		u, err := readUpdate(upd)
		release(upd)
		if err != nil {
			return nil, fmt.Errorf("read update %d: %w", i, err)
		}
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
	if hr := comCallOut(session, idxCreateUpdateInstaller, unsafe.Pointer(&installer)); hresultFailed(hr) {
		return InstallResult{}, hresultError("CreateUpdateInstaller", hr)
	}
	if installer == nil {
		return InstallResult{}, fmt.Errorf("CreateUpdateInstaller returned nil")
	}
	defer release(installer)

	// Locate the not-installed updates to feed the installer.
	var searcher *comIface
	if hr := comCallOut(session, idxCreateUpdateSearcher, unsafe.Pointer(&searcher)); hresultFailed(hr) {
		return InstallResult{}, hresultError("CreateUpdateSearcher", hr)
	}
	if searcher == nil {
		return InstallResult{}, fmt.Errorf("CreateUpdateSearcher returned nil")
	}
	defer release(searcher)

	criteria := sysAllocString(searchCriteria(false))
	if criteria == 0 {
		return InstallResult{}, fmt.Errorf("allocate Windows Update search criteria")
	}
	defer sysFreeString(criteria)

	var result *comIface
	if hr := comCallOut(searcher, idxSearch, unsafe.Pointer(&result), criteria); hresultFailed(hr) {
		return InstallResult{}, hresultError("IUpdateSearcher::Search", hr)
	}
	if result == nil {
		return InstallResult{}, fmt.Errorf("IUpdateSearcher::Search returned nil")
	}
	defer release(result)

	searchCode, err := getOperationResultCode(result, idxSearchResultCode, "ISearchResult::get_ResultCode")
	if err != nil {
		return InstallResult{}, err
	}
	if searchCode != ResultSucceeded {
		return InstallResult{}, fmt.Errorf("Windows Update search completed with result %s", searchCode)
	}

	var collection *comIface
	if hr := comCallOut(result, idxGetUpdates, unsafe.Pointer(&collection)); hresultFailed(hr) {
		return InstallResult{}, hresultError("ISearchResult::get_Updates", hr)
	}
	if collection == nil {
		return InstallResult{}, fmt.Errorf("ISearchResult::get_Updates returned nil")
	}
	defer release(collection)

	var count int32
	if hr := comCallOut(collection, idxGetCount, unsafe.Pointer(&count)); hresultFailed(hr) {
		return InstallResult{}, hresultError("IUpdateCollection::get_Count", hr)
	}
	if count < 0 || count > 100000 {
		return InstallResult{}, fmt.Errorf("IUpdateCollection::get_Count returned invalid count %d", count)
	}
	if count == 0 {
		return InstallResult{ResultCode: ResultSucceeded}, nil
	}

	// Updates must be downloaded before they can be passed to the installer.
	var downloader *comIface
	if hr := comCallOut(session, idxCreateUpdateDownloader, unsafe.Pointer(&downloader)); hresultFailed(hr) {
		return InstallResult{}, hresultError("CreateUpdateDownloader", hr)
	}
	if downloader == nil {
		return InstallResult{}, fmt.Errorf("CreateUpdateDownloader returned nil")
	}
	defer release(downloader)

	if hr := comCallIface(downloader, idxDownloaderPutUpdates, collection); hresultFailed(hr) {
		return InstallResult{}, hresultError("IUpdateDownloader::put_Updates", hr)
	}

	var downloadResult *comIface
	if hr := comCallOut(downloader, idxDownload, unsafe.Pointer(&downloadResult)); hresultFailed(hr) {
		return InstallResult{}, hresultError("IUpdateDownloader::Download", hr)
	}
	if downloadResult == nil {
		return InstallResult{}, fmt.Errorf("IUpdateDownloader::Download returned nil")
	}
	defer release(downloadResult)

	downloadCode, err := getOperationResultCode(downloadResult, idxDownloadResultCode, "IDownloadResult::get_ResultCode")
	if err != nil {
		return InstallResult{}, err
	}
	downloadErr := collectUpdateResultErrors(
		downloadResult,
		idxDownloadGetUpdateResult,
		idxUpdateDownloadResultCode,
		idxUpdateDownloadResultHResult,
		collection,
		count,
		"download",
	)
	if downloadCode != ResultSucceeded {
		downloadErr = errors.Join(
			fmt.Errorf("Windows Update download completed with result %s", downloadCode),
			downloadErr,
		)
	}
	if downloadErr != nil {
		return InstallResult{}, downloadErr
	}

	if hr := comCallIface(installer, idxInstallerPutUpdates, collection); hresultFailed(hr) {
		return InstallResult{}, hresultError("IUpdateInstaller::put_Updates", hr)
	}

	var installResult *comIface
	if hr := comCallOut(installer, idxInstall, unsafe.Pointer(&installResult)); hresultFailed(hr) {
		return InstallResult{}, hresultError("IUpdateInstaller::Install", hr)
	}
	if installResult == nil {
		return InstallResult{}, fmt.Errorf("IUpdateInstaller::Install returned nil")
	}
	defer release(installResult)

	code, err := getOperationResultCode(installResult, idxInstallationResultCode, "IInstallationResult::get_ResultCode")
	if err != nil {
		return InstallResult{}, err
	}
	var reboot int16
	if hr := comCallOut(installResult, idxGetRebootRequired, unsafe.Pointer(&reboot)); hresultFailed(hr) {
		return InstallResult{}, hresultError("IInstallationResult::get_RebootRequired", hr)
	}

	result := InstallResult{ResultCode: code, RebootRequired: reboot != 0}
	installErr := collectUpdateResultErrors(
		installResult,
		idxInstallationGetUpdateResult,
		idxUpdateInstallationResultResultCode,
		idxUpdateInstallationResultHResult,
		collection,
		count,
		"installation",
	)
	if code != ResultSucceeded {
		installErr = errors.Join(
			fmt.Errorf("Windows Update installation completed with result %s", code),
			installErr,
		)
	}
	return result, installErr
}

func getOperationResultCode(p *comIface, idx int, op string) (ResultCode, error) {
	var code int32
	if hr := comCallOut(p, idx, unsafe.Pointer(&code)); hresultFailed(hr) {
		return ResultNotStarted, hresultError(op, hr)
	}
	result := ResultCode(code)
	if result < ResultNotStarted || result > ResultAborted {
		return ResultNotStarted, fmt.Errorf("%s returned invalid result code %d", op, code)
	}
	return result, nil
}

// collectUpdateResultErrors checks every update's result instead of trusting
// only the aggregate operation code. WUA can report partial success, and the
// per-update HRESULT is the only actionable failure detail in that case.
func collectUpdateResultErrors(
	operation *comIface,
	getUpdateResultIdx, resultCodeIdx, hresultIdx int,
	collection *comIface,
	count int32,
	operationName string,
) error {
	var errs []error
	for i := int32(0); i < count; i++ {
		var updateResult *comIface
		hr := comCallOut(operation, getUpdateResultIdx, unsafe.Pointer(&updateResult), uintptr(i))
		if hresultFailed(hr) {
			errs = append(errs, fmt.Errorf("%s: GetUpdateResult(%d): %w", operationName, i, hresultError("GetUpdateResult", hr)))
			continue
		}
		if updateResult == nil {
			errs = append(errs, fmt.Errorf("%s: GetUpdateResult(%d) returned nil", operationName, i))
			continue
		}

		code, codeErr := getOperationResultCode(updateResult, resultCodeIdx, "get_ResultCode")
		resultHRESULT, hresultErr := getResultHRESULT(updateResult, hresultIdx)
		release(updateResult)
		if codeErr != nil || hresultErr != nil {
			errs = append(errs, fmt.Errorf("%s for %s: %w", operationName, updateLabel(collection, i), errors.Join(codeErr, hresultErr)))
			continue
		}
		if code != ResultSucceeded || hresultFailed(resultHRESULT) {
			errs = append(errs, fmt.Errorf(
				"%s for %s: result %s, HRESULT 0x%08X",
				operationName,
				updateLabel(collection, i),
				code,
				resultHRESULT,
			))
		}
	}
	return errors.Join(errs...)
}

func getResultHRESULT(p *comIface, idx int) (uint32, error) {
	var result int32
	if hr := comCallOut(p, idx, unsafe.Pointer(&result)); hresultFailed(hr) {
		return 0, hresultError("get_HResult", hr)
	}
	return uint32(result), nil
}

func updateLabel(collection *comIface, index int32) string {
	fallback := fmt.Sprintf("update %d", index)
	var update *comIface
	if hr := comCallOut(collection, idxGetItem, unsafe.Pointer(&update), uintptr(index)); hresultFailed(hr) || update == nil {
		return fallback
	}
	defer release(update)
	title, err := getBSTR(update, idxGetTitle)
	if err != nil || title == "" {
		return fallback
	}
	return fmt.Sprintf("update %d (%q)", index, title)
}

// readUpdate reads the display fields of an IUpdate.
func readUpdate(p *comIface) (Update, error) {
	if p == nil {
		return Update{}, fmt.Errorf("nil IUpdate")
	}
	var u Update
	var err error
	if u.Title, err = getBSTR(p, idxGetTitle); err != nil {
		return Update{}, fmt.Errorf("IUpdate::get_Title: %w", err)
	}
	if u.IsDownloaded, err = getBool(p, idxGetIsDownloaded); err != nil {
		return Update{}, fmt.Errorf("IUpdate::get_IsDownloaded: %w", err)
	}
	if u.IsHidden, err = getBool(p, idxGetIsHidden); err != nil {
		return Update{}, fmt.Errorf("IUpdate::get_IsHidden: %w", err)
	}
	if u.IsInstalled, err = getBool(p, idxGetIsInstalled); err != nil {
		return Update{}, fmt.Errorf("IUpdate::get_IsInstalled: %w", err)
	}
	return u, nil
}

func getBSTR(p *comIface, idx int) (string, error) {
	var b *uint16
	if hr := comCallOut(p, idx, unsafe.Pointer(&b)); hresultFailed(hr) {
		return "", hresultError("property read", hr)
	}
	if b == nil {
		return "", nil
	}
	defer sysFreeString(uintptr(unsafe.Pointer(b)))
	return bstrToString(b)
}

func getBool(p *comIface, idx int) (bool, error) {
	var vb int16 // VARIANT_BOOL
	if hr := comCallOut(p, idx, unsafe.Pointer(&vb)); hresultFailed(hr) {
		return false, hresultError("property read", hr)
	}
	return vb != 0, nil
}
