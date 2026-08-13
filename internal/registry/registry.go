// Package registry provides a minimal, stdlib-only Windows registry client.
//
// Reads and writes are implemented via direct advapi32 P/Invoke (syscall),
// keeping the binary free of third-party modules.
package registry

import "errors"

// ErrNotFound is returned when a key or value does not exist.
var ErrNotFound = errors.New("registry value not found")

// ErrUnsupported is returned on non-Windows platforms.
var ErrUnsupported = errors.New("registry access is only supported on Windows")

// Hive is a registry root hive identifier.
type Hive string

// Well-known root hives.
const (
	HKEY_LOCAL_MACHINE Hive = "HKLM"
	HKEY_CURRENT_USER  Hive = "HKCU"
	HKEY_CLASSES_ROOT  Hive = "HKCR"
	HKEY_USERS         Hive = "HKU"
)

// Dword returns the uint32 value at path\name, or ErrNotFound.
func Dword(h Hive, path, name string) (uint32, error) {
	return dword(h, path, name)
}

// Qword returns the uint64 value at path\name, or ErrNotFound.
func Qword(h Hive, path, name string) (uint64, error) {
	return qword(h, path, name)
}

// String returns the string value at path\name, or ErrNotFound.
func String(h Hive, path, name string) (string, error) {
	return str(h, path, name)
}

// SetDword writes a REG_DWORD value, creating the key if necessary.
func SetDword(h Hive, path, name string, value uint32) error {
	return setDword(h, path, name, value)
}

// SetQword writes a REG_QWORD value, creating the key if necessary.
func SetQword(h Hive, path, name string, value uint64) error {
	return setQword(h, path, name, value)
}

// SetString writes a REG_SZ value, creating the key if necessary.
func SetString(h Hive, path, name, value string) error {
	return setString(h, path, name, value)
}

// DeleteValue removes a value from a key, returning ErrNotFound if absent.
func DeleteValue(h Hive, path, name string) error {
	return deleteValue(h, path, name)
}

// EnumSubkeys returns the names of the direct subkeys under path. It is used
// by the bloatware scanner to enumerate installed-application uninstall keys.
func EnumSubkeys(h Hive, path string) ([]string, error) {
	return enumSubkeys(h, path)
}
