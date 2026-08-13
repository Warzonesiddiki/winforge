//go:build windows

package registry

import (
	"syscall"
	"unsafe"
)

// Local Win32 constants (stdlib does not expose registry access rights).
const (
	regSz    = 1
	regDword = 4

	rrfRtRegSz       = 0x00000002
	rrfRtRegExpandSz = 0x00000004
	rrfRtRegDword    = 0x00000010

	keySetValue         = 0x0002
	keyQueryValue       = 0x0001
	keyEnumerateSubKeys = 0x0008

	errorFileNotFound = 2
	errorNoMoreItems  = 259

	hkeyClassesRoot  = 0x80000000
	hkeyCurrentUser  = 0x80000001
	hkeyLocalMachine = 0x80000002
	hkeyUsers        = 0x80000003
)

var (
	advapi32            = syscall.NewLazyDLL("advapi32.dll")
	procRegGetValueW    = advapi32.NewProc("RegGetValueW")
	procRegSetValueExW  = advapi32.NewProc("RegSetValueExW")
	procRegCreateKeyExW = advapi32.NewProc("RegCreateKeyExW")
	procRegDeleteValueW = advapi32.NewProc("RegDeleteValueW")
	procRegOpenKeyExW   = advapi32.NewProc("RegOpenKeyExW")
	procRegEnumKeyExW   = advapi32.NewProc("RegEnumKeyExW")
	procRegCloseKey     = advapi32.NewProc("RegCloseKey")
)

func hiveRoot(h Hive) syscall.Handle {
	switch h {
	case HKEY_LOCAL_MACHINE:
		return hkeyLocalMachine
	case HKEY_CURRENT_USER:
		return hkeyCurrentUser
	case HKEY_CLASSES_ROOT:
		return hkeyClassesRoot
	case HKEY_USERS:
		return hkeyUsers
	default:
		return 0
	}
}

func errnoFrom(r uintptr) error {
	if r == errorFileNotFound {
		return ErrNotFound
	}
	return syscall.Errno(r)
}

// regGetValue reads a value directly by subkey path (no open handle needed).
func regGetValue(h Hive, path, name string, flags uint32) ([]byte, uint32, error) {
	ppath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, 0, err
	}
	pname, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, 0, err
	}

	var typ, size uint32
	r, _, _ := procRegGetValueW.Call(
		uintptr(hiveRoot(h)),
		uintptr(unsafe.Pointer(ppath)),
		uintptr(unsafe.Pointer(pname)),
		uintptr(flags),
		uintptr(unsafe.Pointer(&typ)),
		0,
		uintptr(unsafe.Pointer(&size)),
	)
	if r != 0 {
		return nil, 0, errnoFrom(r)
	}
	if size == 0 {
		return nil, typ, nil
	}
	buf := make([]byte, size)
	r, _, _ = procRegGetValueW.Call(
		uintptr(hiveRoot(h)),
		uintptr(unsafe.Pointer(ppath)),
		uintptr(unsafe.Pointer(pname)),
		uintptr(flags),
		uintptr(unsafe.Pointer(&typ)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r != 0 {
		return nil, 0, errnoFrom(r)
	}
	return buf, typ, nil
}

func openOrCreate(h Hive, path string) (syscall.Handle, error) {
	ppath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var k syscall.Handle
	var disp uint32
	r, _, _ := procRegCreateKeyExW.Call(
		uintptr(hiveRoot(h)),
		uintptr(unsafe.Pointer(ppath)),
		0, 0, 0,
		uintptr(keySetValue),
		0,
		uintptr(unsafe.Pointer(&k)),
		uintptr(unsafe.Pointer(&disp)),
	)
	if r != 0 {
		return 0, syscall.Errno(r)
	}
	return k, nil
}

func regSetValue(h Hive, path, name string, typ uint32, data []byte) error {
	k, err := openOrCreate(h, path)
	if err != nil {
		return err
	}
	defer procRegCloseKey.Call(uintptr(k))

	pname, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	var pdata unsafe.Pointer
	if len(data) > 0 {
		pdata = unsafe.Pointer(&data[0])
	}
	r, _, _ := procRegSetValueExW.Call(
		uintptr(k),
		uintptr(unsafe.Pointer(pname)),
		0,
		uintptr(typ),
		uintptr(pdata),
		uintptr(len(data)),
	)
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func dword(h Hive, path, name string) (uint32, error) {
	buf, _, err := regGetValue(h, path, name, rrfRtRegDword)
	if err != nil {
		return 0, err
	}
	if len(buf) < 4 {
		return 0, ErrNotFound
	}
	return *(*uint32)(unsafe.Pointer(&buf[0])), nil
}

func str(h Hive, path, name string) (string, error) {
	// Accept both REG_SZ and REG_EXPAND_SZ; both are UTF-16LE, null-terminated.
	buf, _, err := regGetValue(h, path, name, rrfRtRegSz|rrfRtRegExpandSz)
	if err != nil {
		return "", err
	}
	// buf is UTF-16LE, null-terminated.
	u := make([]uint16, len(buf)/2)
	for i := range u {
		u[i] = uint16(buf[i*2]) | uint16(buf[i*2+1])<<8
	}
	return syscall.UTF16ToString(u), nil
}

func setDword(h Hive, path, name string, value uint32) error {
	buf := (*[4]byte)(unsafe.Pointer(&value))[:]
	return regSetValue(h, path, name, regDword, buf)
}

func setString(h Hive, path, name, value string) error {
	u, err := syscall.UTF16FromString(value)
	if err != nil {
		return err
	}
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&u[0])), len(u)*2)
	return regSetValue(h, path, name, regSz, buf)
}

func deleteValue(h Hive, path, name string) error {
	k, err := openOrCreate(h, path)
	if err != nil {
		return err
	}
	defer procRegCloseKey.Call(uintptr(k))

	pname, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	r, _, _ := procRegDeleteValueW.Call(uintptr(k), uintptr(unsafe.Pointer(pname)))
	if r != 0 {
		return errnoFrom(r)
	}
	return nil
}

// enumSubkeys lists the names of the direct subkeys of path.
func enumSubkeys(h Hive, path string) ([]string, error) {
	ppath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var k syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(hiveRoot(h)),
		uintptr(unsafe.Pointer(ppath)),
		0,
		uintptr(keyQueryValue|keyEnumerateSubKeys),
		uintptr(unsafe.Pointer(&k)),
	)
	if r != 0 {
		return nil, errnoFrom(r)
	}
	defer procRegCloseKey.Call(uintptr(k))

	var names []string
	buf := make([]uint16, 256)
	for i := uint32(0); ; i++ {
		size := uint32(len(buf))
		r, _, _ := procRegEnumKeyExW.Call(
			uintptr(k),
			uintptr(i),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&size)),
			0, 0, 0, 0,
		)
		if r == errorNoMoreItems {
			break
		}
		if r != 0 {
			return nil, errnoFrom(r)
		}
		names = append(names, syscall.UTF16ToString(buf[:size]))
	}
	return names, nil
}
