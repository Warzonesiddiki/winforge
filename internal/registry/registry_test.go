package registry

import (
	"strings"
	"testing"
)

func TestValidateHive(t *testing.T) {
	for _, hive := range []Hive{HKEY_LOCAL_MACHINE, HKEY_CURRENT_USER, HKEY_CLASSES_ROOT, HKEY_USERS} {
		if err := validateHive(hive); err != nil {
			t.Errorf("validateHive(%q) returned %v", hive, err)
		}
	}
	for _, hive := range []Hive{"", "HKCC", "invalid"} {
		if err := validateHive(hive); err == nil {
			t.Errorf("validateHive(%q) succeeded, want error", hive)
		}
	}
}

func TestPublicOperationsRejectUnsupportedHive(t *testing.T) {
	const invalid Hive = "invalid"
	operations := []struct {
		name string
		call func() error
	}{
		{name: "Dword", call: func() error { _, err := Dword(invalid, `Software\WinForge`, "Value"); return err }},
		{name: "Qword", call: func() error { _, err := Qword(invalid, `Software\WinForge`, "Value"); return err }},
		{name: "String", call: func() error { _, err := String(invalid, `Software\WinForge`, "Value"); return err }},
		{name: "ExpandString", call: func() error { _, err := ExpandString(invalid, `Software\WinForge`, "Value"); return err }},
		{name: "SetDword", call: func() error { return SetDword(invalid, `Software\WinForge`, "Value", 1) }},
		{name: "SetQword", call: func() error { return SetQword(invalid, `Software\WinForge`, "Value", 1) }},
		{name: "SetString", call: func() error { return SetString(invalid, `Software\WinForge`, "Value", "value") }},
		{name: "SetExpandString", call: func() error { return SetExpandString(invalid, `Software\WinForge`, "Value", "value") }},
		{name: "DeleteValue", call: func() error { return DeleteValue(invalid, `Software\WinForge`, "Value") }},
		{name: "EnumSubkeys", call: func() error { _, err := EnumSubkeys(invalid, `Software\WinForge`); return err }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			err := operation.call()
			if err == nil || !strings.Contains(err.Error(), "unsupported registry hive") {
				t.Fatalf("operation error = %v, want unsupported-hive error", err)
			}
		})
	}
}
