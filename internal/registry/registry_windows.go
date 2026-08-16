//go:build windows

package registry

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

// Local Win32 constants (stdlib does not expose registry access rights).
const (
	regSz       = 1
	regExpandSz = 2
	regDword    = 4
	regQword    = 11

	rrfRtRegSz       = 0x00000002
	rrfRtRegExpandSz = 0x00000004
	rrfRtRegDword    = 0x00000010
	rrfRtRegQword    = 0x00000040
	rrfNoExpand      = 0x10000000

	// Registry values used by WinForge are tiny. Bound a value that races with
	// the sizing call so a corrupt or hostile key cannot force unbounded memory.
	maxRegistryValueBytes = 16 << 20
	maxRegistryReadTries  = 8
	maxRegistrySubkeys    = 10_000
	maxRegistryKeyUnits   = 16_384

	keySetValue         = 0x0002
	keyEnumerateSubKeys = 0x0008
	keyRead             = 0x20019 // STANDARD_RIGHTS_READ | KEY_QUERY_VALUE | KEY_ENUMERATE_SUB_KEYS | KEY_NOTIFY

	errorFileNotFound = 2
	errorMoreData     = 234
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
	procRegDeleteTreeW  = advapi32.NewProc("RegDeleteTreeW")
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

	for attempt := 0; attempt < maxRegistryReadTries; attempt++ {
		if size > maxRegistryValueBytes {
			return nil, 0, fmt.Errorf("registry value is too large: %d bytes", size)
		}
		buf := make([]byte, size)
		actual := size
		r, _, _ = procRegGetValueW.Call(
			uintptr(hiveRoot(h)),
			uintptr(unsafe.Pointer(ppath)),
			uintptr(unsafe.Pointer(pname)),
			uintptr(flags),
			uintptr(unsafe.Pointer(&typ)),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&actual)),
		)
		if r == errorMoreData {
			// RegGetValueW normally reports the newly required size. Its
			// documentation permits an unpredictable size in special cases, so
			// make forward progress even when it does not.
			if actual <= size {
				if size > maxRegistryValueBytes/2 {
					return nil, 0, fmt.Errorf("registry value grew beyond %d bytes while reading", maxRegistryValueBytes)
				}
				size *= 2
			} else {
				size = actual
			}
			continue
		}
		if r != 0 {
			return nil, 0, errnoFrom(r)
		}
		if actual > uint32(len(buf)) {
			return nil, 0, fmt.Errorf("RegGetValueW returned invalid size %d for %d-byte buffer", actual, len(buf))
		}
		return buf[:actual], typ, nil
	}
	return nil, 0, fmt.Errorf("registry value changed repeatedly while reading")
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
	if len(buf) != 4 {
		return 0, fmt.Errorf("REG_DWORD has invalid length %d", len(buf))
	}
	return binary.LittleEndian.Uint32(buf), nil
}

func qword(h Hive, path, name string) (uint64, error) {
	buf, _, err := regGetValue(h, path, name, rrfRtRegQword)
	if err != nil {
		return 0, err
	}
	if len(buf) != 8 {
		return 0, fmt.Errorf("REG_QWORD has invalid length %d", len(buf))
	}
	return binary.LittleEndian.Uint64(buf), nil
}

func str(h Hive, path, name string) (string, error) {
	buf, _, err := regGetValue(h, path, name, rrfRtRegSz)
	if err != nil {
		return "", err
	}
	return decodeString(buf)
}

func expandString(h Hive, path, name string) (string, error) {
	// Preserve the unexpanded representation so a snapshot can restore both
	// the exact data and REG_EXPAND_SZ semantics.
	buf, _, err := regGetValue(h, path, name, rrfRtRegExpandSz|rrfNoExpand)
	if err != nil {
		return "", err
	}
	return decodeString(buf)
}

func decodeString(buf []byte) (string, error) {
	if len(buf)%2 != 0 {
		return "", fmt.Errorf("registry string has odd byte length %d", len(buf))
	}
	u := make([]uint16, len(buf)/2)
	for i := range u {
		u[i] = uint16(buf[i*2]) | uint16(buf[i*2+1])<<8
	}
	if len(u) == 0 || u[len(u)-1] != 0 {
		return "", fmt.Errorf("registry string is not NUL-terminated")
	}
	for i := 0; i < len(u)-1; i++ {
		switch {
		case u[i] == 0:
			return "", fmt.Errorf("registry string contains an embedded NUL")
		case u[i] >= 0xD800 && u[i] <= 0xDBFF:
			if i+1 >= len(u)-1 || u[i+1] < 0xDC00 || u[i+1] > 0xDFFF {
				return "", fmt.Errorf("registry string contains invalid UTF-16")
			}
			i++
		case u[i] >= 0xDC00 && u[i] <= 0xDFFF:
			return "", fmt.Errorf("registry string contains invalid UTF-16")
		}
	}
	return syscall.UTF16ToString(u), nil
}

func setDword(h Hive, path, name string, value uint32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], value)
	return regSetValue(h, path, name, regDword, buf[:])
}

func setQword(h Hive, path, name string, value uint64) error {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], value)
	return regSetValue(h, path, name, regQword, buf[:])
}

func setString(h Hive, path, name, value string) error {
	return setUTF16String(h, path, name, value, regSz)
}

func setExpandString(h Hive, path, name, value string) error {
	return setUTF16String(h, path, name, value, regExpandSz)
}

func setUTF16String(h Hive, path, name, value string, typ uint32) error {
	u, err := syscall.UTF16FromString(value)
	if err != nil {
		return err
	}
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&u[0])), len(u)*2)
	return regSetValue(h, path, name, typ, buf)
}

func deleteValue(h Hive, path, name string) error {
	ppath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var k syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(hiveRoot(h)),
		uintptr(unsafe.Pointer(ppath)),
		0,
		uintptr(keySetValue),
		uintptr(unsafe.Pointer(&k)),
	)
	if r != 0 {
		return errnoFrom(r)
	}
	defer procRegCloseKey.Call(uintptr(k))

	pname, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	r, _, _ = procRegDeleteValueW.Call(uintptr(k), uintptr(unsafe.Pointer(pname)))
	if r != 0 {
		return errnoFrom(r)
	}
	return nil
}

// keyExists reports whether the key at path can be opened for reading.
func keyExists(h Hive, path string) (bool, error) {
	ppath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	var k syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(hiveRoot(h)),
		uintptr(unsafe.Pointer(ppath)),
		0,
		uintptr(keyRead),
		uintptr(unsafe.Pointer(&k)),
	)
	if r == errorFileNotFound {
		return false, nil
	}
	if r != 0 {
		return false, syscall.Errno(r)
	}
	procRegCloseKey.Call(uintptr(k))
	return true, nil
}

// deleteKeyTree removes path and everything under it via RegDeleteTreeW,
// which deletes the subkeys and values of the specified key recursively.
func deleteKeyTree(h Hive, path string) error {
	ppath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	r, _, _ := procRegDeleteTreeW.Call(
		uintptr(hiveRoot(h)),
		uintptr(unsafe.Pointer(ppath)),
	)
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
		uintptr(keyEnumerateSubKeys),
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
		if r == errorMoreData {
			if len(buf) >= maxRegistryKeyUnits {
				return nil, fmt.Errorf("registry subkey name exceeds %d UTF-16 code units", maxRegistryKeyUnits)
			}
			buf = make([]uint16, min(len(buf)*2, maxRegistryKeyUnits))
			i-- // retry the same index with the larger buffer
			continue
		}
		if r == errorNoMoreItems {
			break
		}
		if r != 0 {
			return nil, errnoFrom(r)
		}
		if len(names) >= maxRegistrySubkeys {
			return nil, fmt.Errorf("registry key contains more than %d subkeys", maxRegistrySubkeys)
		}
		if size > uint32(len(buf)) {
			return nil, fmt.Errorf("RegEnumKeyExW returned invalid name length %d for %d-unit buffer", size, len(buf))
		}
		names = append(names, syscall.UTF16ToString(buf[:size]))
	}
	return names, nil
}
