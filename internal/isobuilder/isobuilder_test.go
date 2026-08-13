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
		o := &Options{SourceDir: dir, OutputISO: filepath.Join(filepath.Dir(dir), "out"), Label: "win 11 pro"}
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
		o := &Options{SourceDir: dir, OutputISO: filepath.Join(filepath.Dir(dir), "x.iso")}
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

	t.Run("output inside source", func(t *testing.T) {
		o := &Options{SourceDir: dir, OutputISO: filepath.Join(dir, "nested", "x.iso")}
		if err := ValidateOptions(o); err == nil {
			t.Error("expected error for output ISO inside source tree")
		}
	})

	t.Run("output resolves inside source", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "source-link")
		if err := os.Symlink(dir, link); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		o := &Options{SourceDir: dir, OutputISO: filepath.Join(link, "nested", "x.iso")}
		if err := ValidateOptions(o); err == nil {
			t.Error("expected error for output ISO resolving inside source tree")
		}
	})

	t.Run("nested symlink", func(t *testing.T) {
		source := makeSource(t, "install.wim")
		if err := os.Symlink(filepath.Join(source, "sources", "install.wim"), filepath.Join(source, "payload-link")); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		o := &Options{SourceDir: source, OutputISO: filepath.Join(t.TempDir(), "x.iso")}
		if err := ValidateOptions(o); err == nil {
			t.Error("expected error for a symbolic link in the source tree")
		}
	})

	t.Run("empty edition", func(t *testing.T) {
		o := &Options{SourceDir: dir, OutputISO: filepath.Join(t.TempDir(), "x.iso"), Editions: []string{" "}}
		if err := ValidateOptions(o); err == nil {
			t.Error("expected error for an empty requested edition")
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

func TestCopyTreeCopiesRegularFilesAndSkipsImages(t *testing.T) {
	src := makeSource(t, "install.wim")
	if err := os.WriteFile(filepath.Join(src, "sources", "INSTALL.ESD"), []byte("image"), 0o644); err != nil {
		t.Fatalf("write uppercase image: %v", err)
	}
	regular := filepath.Join(src, "boot", "nested", "payload.bin")
	if err := os.MkdirAll(filepath.Dir(regular), 0o755); err != nil {
		t.Fatalf("mkdir regular file: %v", err)
	}
	if err := os.WriteFile(regular, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write regular file: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "copy")

	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "boot", "nested", "payload.bin"))
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(got) != "payload" {
		t.Fatalf("copied content = %q, want payload", got)
	}
	for _, name := range []string{"install.wim", "INSTALL.ESD"} {
		if _, err := os.Stat(filepath.Join(dst, "sources", name)); !os.IsNotExist(err) {
			t.Fatalf("image %q was copied or stat failed unexpectedly: %v", name, err)
		}
	}
}

func TestCopyTreeRejectsSymlinks(t *testing.T) {
	src := t.TempDir()
	target := filepath.Join(src, "target")
	if err := os.WriteFile(target, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(src, "link")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}

	if err := copyTree(src, filepath.Join(t.TempDir(), "copy")); err == nil {
		t.Fatal("copyTree accepted a symbolic link")
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

func TestParseWimInfo(t *testing.T) {
	got := parseWimInfo(wimInfoEnglish)
	if len(got) != 2 {
		t.Fatalf("want 2 editions, got %d: %+v", len(got), got)
	}
	if got[0].Index != 1 || got[0].Name != "Windows 11 Home" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Index != 2 || got[1].Name != "Windows 11 Pro" {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestParseWimInfoIgnoresNumericMetadata(t *testing.T) {
	out := "ServicePack Build : 1\nServicePack Level : 0\n" + wimInfoEnglish
	got := parseWimInfo(out)
	if len(got) != 2 || got[0].Name != "Windows 11 Home" || got[1].Name != "Windows 11 Pro" {
		t.Fatalf("numeric metadata was parsed as an edition: %+v", got)
	}
}

func TestParseWimInfoRequiresNameAndAllowsReorderedMetadata(t *testing.T) {
	out := "Index : 0\nName : invalid zero index\n\nIndex : 1\nDescription : not an edition name\nSize : 123\n\nIndex : 2\nDescription : Windows 11 Pro\nArchitecture : x64\nName : Windows 11 Pro\n"
	got := parseWimInfo(out)
	if len(got) != 1 {
		t.Fatalf("parseWimInfo returned %d editions: %+v", len(got), got)
	}
	if got[0].Index != 2 || got[0].Name != "Windows 11 Pro" {
		t.Fatalf("parseWimInfo returned %+v", got[0])
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
