//go:build !windows

package scheduler

import "testing"

func TestNonWindowsStubsReturnUnsupported(t *testing.T) {
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
