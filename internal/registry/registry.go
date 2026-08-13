// Package registry provides a minimal, stdlib-only Windows registry client.
//
// Reads and writes are implemented via direct advapi32 P/Invoke (syscall),
// keeping the binary free of third-party modules.
package registry

import (
	"errors"
	"fmt"
)

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

func validateHive(h Hive) error {
	switch h {
	case HKEY_LOCAL_MACHINE, HKEY_CURRENT_USER, HKEY_CLASSES_ROOT, HKEY_USERS:
		return nil
	default:
		return fmt.Errorf("unsupported registry hive %q", h)
	}
}

// Dword returns the uint32 value at path\name, or ErrNotFound.
func Dword(h Hive, path, name string) (uint32, error) {
	if err := validateHive(h); err != nil {
		return 0, err
	}
	return dword(h, path, name)
}

// Qword returns the uint64 value at path\name, or ErrNotFound.
func Qword(h Hive, path, name string) (uint64, error) {
	if err := validateHive(h); err != nil {
		return 0, err
	}
	return qword(h, path, name)
}

// String returns the REG_SZ value at path\name, or ErrNotFound. A value of a
// different registry type is rejected rather than converted.
func String(h Hive, path, name string) (string, error) {
	if err := validateHive(h); err != nil {
		return "", err
	}
	return str(h, path, name)
}

// ExpandString returns the unexpanded REG_EXPAND_SZ value at path\name, or
// ErrNotFound. A value of a different registry type is rejected.
func ExpandString(h Hive, path, name string) (string, error) {
	if err := validateHive(h); err != nil {
		return "", err
	}
	return expandString(h, path, name)
}

// SetDword writes a REG_DWORD value, creating the key if necessary.
func SetDword(h Hive, path, name string, value uint32) error {
	if err := validateHive(h); err != nil {
		return err
	}
	return setDword(h, path, name, value)
}

// SetQword writes a REG_QWORD value, creating the key if necessary.
func SetQword(h Hive, path, name string, value uint64) error {
	if err := validateHive(h); err != nil {
		return err
	}
	return setQword(h, path, name, value)
}

// SetString writes a REG_SZ value, creating the key if necessary.
func SetString(h Hive, path, name, value string) error {
	if err := validateHive(h); err != nil {
		return err
	}
	return setString(h, path, name, value)
}

// SetExpandString writes a REG_EXPAND_SZ value without expanding its data,
// creating the key if necessary.
func SetExpandString(h Hive, path, name, value string) error {
	if err := validateHive(h); err != nil {
		return err
	}
	return setExpandString(h, path, name, value)
}

// DeleteValue removes a value from a key, returning ErrNotFound if absent.
func DeleteValue(h Hive, path, name string) error {
	if err := validateHive(h); err != nil {
		return err
	}
	return deleteValue(h, path, name)
}

// EnumSubkeys returns the names of the direct subkeys under path. It is used
// by the bloatware scanner to enumerate installed-application uninstall keys.
func EnumSubkeys(h Hive, path string) ([]string, error) {
	if err := validateHive(h); err != nil {
		return nil, err
	}
	return enumSubkeys(h, path)
}
