package scheduler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRegisterRejectsRelativePath(t *testing.T) {
	err := validateRegister("winforge.exe")
	if err == nil {
		t.Fatal("validateRegister accepted a relative path")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("error = %v, want it to mention an absolute-path requirement", err)
	}
}

func TestValidateRegisterRejectsUnsafeCharacters(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "winforge.exe")
	if err := os.WriteFile(exe, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"NUL":          exe + "\x00",
		"double-quote": filepath.Join(dir, `wi"n.exe`),
		"CR":           filepath.Join(dir, "wi\n.exe"),
		"LF":           filepath.Join(dir, "wi\r.exe"),
	} {
		t.Run(name, func(t *testing.T) {
			// Write the oddly-named file where needed (except NUL which cannot
			// be created and must be rejected before os.Stat).
			if name != "NUL" {
				if err := os.WriteFile(path, nil, 0o700); err != nil {
					t.Skipf("cannot create fixture path: %v", err)
				}
			}
			err := validateRegister(path)
			if err == nil {
				t.Fatalf("validateRegister accepted unsafe path %q", path)
			}
		})
	}
}

func TestValidateRegisterRejectsMissingFile(t *testing.T) {
	err := validateRegister(filepath.Join(t.TempDir(), "does-not-exist.exe"))
	if err == nil {
		t.Fatal("validateRegister accepted a missing executable")
	}
}

func TestValidateRegisterRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	err := validateRegister(dir)
	if err == nil {
		t.Fatal("validateRegister accepted a directory as the executable")
	}
	if !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("error = %v, want a regular-file complaint", err)
	}
}

func TestValidateRegisterAcceptsRegularFile(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "winforge.exe")
	if err := os.WriteFile(exe, []byte("MZ"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := validateRegister(exe); err != nil {
		t.Fatalf("validateRegister(%q) = %v, want nil", exe, err)
	}
}

func TestNonWindowsStubsReturnUnsupported(t *testing.T) {
	// On Linux CI the platform-specific functions return ErrUnsupported.
	// This pins the contract without exercising schtasks.exe.
	if err := Enable("\\Foo\\Bar"); err != ErrUnsupported {
		t.Errorf("Enable = %v, want ErrUnsupported off-Windows", err)
	}
	if err := Disable("\\Foo\\Bar"); err != ErrUnsupported {
		t.Errorf("Disable = %v, want ErrUnsupported off-Windows", err)
	}
	if err := Delete("\\Foo\\Bar"); err != ErrUnsupported {
		t.Errorf("Delete = %v, want ErrUnsupported off-Windows", err)
	}
}
