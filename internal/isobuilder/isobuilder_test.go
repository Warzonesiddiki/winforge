package isobuilder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeSource(t *testing.T, imageName string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sources"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sources", imageName), []byte("fake"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return dir
}

func TestValidateOptions(t *testing.T) {
	dir := makeSource(t, "install.wim")

	t.Run("normalizes", func(t *testing.T) {
		o := &Options{SourceDir: dir, OutputISO: filepath.Join(dir, "out"), Label: "win 11 pro"}
		if err := ValidateOptions(o); err != nil {
			t.Fatalf("ValidateOptions: %v", err)
		}
		if !strings.HasSuffix(o.OutputISO, ".iso") {
			t.Errorf("OutputISO = %q, want .iso suffix", o.OutputISO)
		}
		if o.Label != "WIN_11_PRO" {
			t.Errorf("Label = %q, want WIN_11_PRO", o.Label)
		}
	})

	t.Run("default label", func(t *testing.T) {
		o := &Options{SourceDir: dir, OutputISO: filepath.Join(dir, "x.iso")}
		if err := ValidateOptions(o); err != nil {
			t.Fatalf("ValidateOptions: %v", err)
		}
		if o.Label != "WINFORGE" {
			t.Errorf("Label = %q, want WINFORGE", o.Label)
		}
	})

	t.Run("missing source", func(t *testing.T) {
		if err := ValidateOptions(&Options{OutputISO: "x.iso"}); err == nil {
			t.Error("expected error for empty source")
		}
	})

	t.Run("missing image", func(t *testing.T) {
		empty := t.TempDir()
		if err := ValidateOptions(&Options{SourceDir: empty, OutputISO: "x.iso"}); err == nil {
			t.Error("expected error for missing install.wim")
		}
	})

	t.Run("missing output", func(t *testing.T) {
		if err := ValidateOptions(&Options{SourceDir: dir}); err == nil {
			t.Error("expected error for empty output")
		}
	})
}

func TestSanitizeLabel(t *testing.T) {
	cases := map[string]string{
		"":                      "WINFORGE",
		"Win 11 Pro x64!":       "WIN_11_PRO_X64_",
		"WINDOWS_SETUP":         "WINDOWS_SETUP",
		"   ":                   "WINFORGE",
		strings.Repeat("a", 40): strings.Repeat("A", 32),
	}
	for in, want := range cases {
		if got := SanitizeLabel(in); got != want {
			t.Errorf("SanitizeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestImageFile(t *testing.T) {
	if _, err := imageFile(makeSource(t, "install.wim")); err != nil {
		t.Errorf("imageFile(wim): %v", err)
	}
	if _, err := imageFile(makeSource(t, "install.esd")); err != nil {
		t.Errorf("imageFile(esd): %v", err)
	}
	if _, err := imageFile(t.TempDir()); err == nil {
		t.Error("imageFile(empty): expected error")
	}
}

const wimInfoEnglish = `
Deployment Image Servicing and Management tool
Version: 10.0.22621.1

Details for image : C:\win\sources\install.wim

Index : 1
Name : Windows 11 Home
Description : Windows 11 Home
Size : 15,677,216,106 bytes

Index : 2
Name : Windows 11 Pro
Description : Windows 11 Pro
Size : 16,123,456,789 bytes

The operation completed successfully.
`

const wimInfoChinese = `
部署映像服务和管理工具
版本: 10.0.22621.1

映像详细信息: C:\win\sources\install.wim

索引: 1
名称: Windows 11 Home
说明: Windows 11 Home
大小: 15,677,216,106 字节

索引: 2
名称: Windows 11 Pro
说明: Windows 11 Pro
大小: 16,123,456,789 字节

操作成功完成。
`

func TestParseWimInfo(t *testing.T) {
	for name, out := range map[string]string{"english": wimInfoEnglish, "chinese": wimInfoChinese} {
		got := parseWimInfo(out)
		if len(got) != 2 {
			t.Fatalf("%s: want 2 editions, got %d: %+v", name, len(got), got)
		}
		if got[0].Index != 1 || got[0].Name != "Windows 11 Home" {
			t.Errorf("%s: got[0] = %+v", name, got[0])
		}
		if got[1].Index != 2 || got[1].Name != "Windows 11 Pro" {
			t.Errorf("%s: got[1] = %+v", name, got[1])
		}
	}
}

func TestSelectIndexes(t *testing.T) {
	all := []Edition{{Index: 1, Name: "Windows 11 Home"}, {Index: 2, Name: "Windows 11 Pro"}}

	got, err := selectIndexes(all, []string{"Windows 11 Pro"})
	if err != nil {
		t.Fatalf("selectIndexes: %v", err)
	}
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("want [2], got %v", got)
	}

	got, err = selectIndexes(all, nil)
	if err != nil {
		t.Fatalf("selectIndexes(all): %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want [1 2], got %v", got)
	}

	if _, err := selectIndexes(all, []string{"Windows 11 Enterprise"}); err == nil {
		t.Error("expected error for unknown edition")
	}

	if _, err := selectIndexes(nil, []string{"X"}); err == nil {
		t.Error("expected error for empty edition list")
	}
}
