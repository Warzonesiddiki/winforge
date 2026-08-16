package winapi

import "testing"

func TestValidateSystemFileNameAcceptsBareNames(t *testing.T) {
	for _, name := range []string{"dism.exe", "kernel32.dll", "netsh.exe", "w32tm.exe", "bcdedit"} {
		if err := validateSystemFileName(name); err != nil {
			t.Errorf("validateSystemFileName(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateSystemFileNameRejectsTraversalAndQualifiers(t *testing.T) {
	bad := []string{
		"",
		".",
		"..",
		`../dism.exe`,
		`C:\Windows\dism.exe`,
		`C:`,
		`dism.exe:stream`, // NTFS alternate data stream
		"subdir/dism.exe",
		"dism\x00.exe",
	}
	for _, name := range bad {
		t.Run(name, func(t *testing.T) {
			if err := validateSystemFileName(name); err == nil {
				t.Fatalf("validateSystemFileName(%q) = nil, want an error", name)
			}
		})
	}
}
