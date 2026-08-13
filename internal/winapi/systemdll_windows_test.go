//go:build windows

package winapi

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemPathIsAbsoluteAndContained(t *testing.T) {
	dir := SystemDirectory()
	path := SystemPath("dism.exe")
	if !filepath.IsAbs(path) {
		t.Fatalf("SystemPath() = %q, want absolute path", path)
	}
	if !strings.EqualFold(filepath.Dir(path), dir) {
		t.Fatalf("SystemPath() directory = %q, want %q", filepath.Dir(path), dir)
	}
}

func TestSystemPathRejectsNonFileNames(t *testing.T) {
	for _, name := range []string{"", ".", "..", `..\\dism.exe`, `C:\\Windows\\dism.exe`, "dism.exe:stream"} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("SystemPath(%q) did not panic", name)
				}
			}()
			_ = SystemPath(name)
		})
	}
}
