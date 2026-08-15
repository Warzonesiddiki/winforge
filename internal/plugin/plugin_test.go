package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"winforge/internal/config"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDiscoverMissingRoot(t *testing.T) {
	plugins, err := Discover(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("want 0 plugins, got %d", len(plugins))
	}
}

func TestPluginFileReadIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, maxPluginFileBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := readPluginFile(path); err == nil {
		t.Fatal("readPluginFile accepted an oversized plugin file")
	}
}

func TestPluginFileRejectsSymbolicLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	link := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if _, err := readPluginFile(link); err == nil {
		t.Fatal("readPluginFile accepted a symbolic link")
	}
}

func TestDiscoverRejectsExcessiveRootEntries(t *testing.T) {
	root := t.TempDir()
	for i := 0; i <= maxPluginRootEntries; i++ {
		path := filepath.Join(root, fmt.Sprintf("entry-%05d", i))
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("create root entry %d: %v", i, err)
		}
	}
	if _, err := Discover(root); err == nil || !strings.Contains(err.Error(), "entry scan limit") {
		t.Fatalf("Discover error = %v, want entry scan limit", err)
	}
}

func TestDiscoverRejectsExcessiveCandidateDirectories(t *testing.T) {
	root := t.TempDir()
	for i := 0; i <= maxPluginDirectories; i++ {
		path := filepath.Join(root, fmt.Sprintf("candidate-%04d", i))
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatalf("create candidate directory %d: %v", i, err)
		}
	}
	if _, err := Discover(root); err == nil || !strings.Contains(err.Error(), "directory limit") {
		t.Fatalf("Discover error = %v, want directory limit", err)
	}
}

func TestDiscoverRejectsExcessiveValidPlugins(t *testing.T) {
	root := t.TempDir()
	for i := 0; i <= maxDiscoveredPlugins; i++ {
		writeFile(t, filepath.Join(root, fmt.Sprintf("plugin-%03d", i), "manifest.json"), `{}`)
	}
	if _, err := Discover(root); err == nil || !strings.Contains(err.Error(), "valid-plugin limit") {
		t.Fatalf("Discover error = %v, want valid-plugin limit", err)
	}
}

func TestDiscoverRejectsExcessiveAggregateTweaks(t *testing.T) {
	root := t.TempDir()

	// The aggregate limit spans plugins, so the tweaks are spread across
	// several directories. Packing them all into one file would instead trip
	// the config package's per-file tweak-count limit, which makes load() skip
	// the plugin as broken and never exercises the aggregate check at all.
	const tweaksPerPlugin = 2000
	total := maxDiscoveredPluginTweaks + 1
	for start, p := 0, 0; start < total; start, p = start+tweaksPerPlugin, p+1 {
		count := tweaksPerPlugin
		if remaining := total - start; remaining < count {
			count = remaining
		}
		dir := filepath.Join(root, fmt.Sprintf("bulk-%03d", p))
		writeFile(t, filepath.Join(dir, "manifest.json"), `{}`)

		var contents strings.Builder
		contents.WriteString(`{"tweaks":[`)
		for i := 0; i < count; i++ {
			if i > 0 {
				contents.WriteByte(',')
			}
			fmt.Fprintf(&contents, `{"id":"t-%d","risk":"low","operations":[{"type":"registry_delete","hive":"HKCU","path":"A","name":"B"}]}`, start+i)
		}
		contents.WriteString(`]}`)
		writeFile(t, filepath.Join(dir, "tweaks.json"), contents.String())
	}

	if _, err := Discover(root); err == nil || !strings.Contains(err.Error(), "tweak aggregate limit") {
		t.Fatalf("Discover error = %v, want aggregate tweak limit", err)
	}
}

// TestDiscoverSkipsPluginExceedingPerFileTweakLimit pins the other half of the
// interaction above: a single plugin whose tweaks.json exceeds the config
// package's per-file limit is skipped as broken rather than aborting discovery,
// so one malformed plugin cannot deny service to every other plugin.
func TestDiscoverSkipsPluginExceedingPerFileTweakLimit(t *testing.T) {
	root := t.TempDir()

	good := filepath.Join(root, "aaa-good")
	writeFile(t, filepath.Join(good, "manifest.json"), `{"name":"Good"}`)
	writeFile(t, filepath.Join(good, "tweaks.json"),
		`{"tweaks":[{"id":"ok","risk":"low","operations":[{"type":"registry_delete","hive":"HKCU","path":"A","name":"B"}]}]}`)

	huge := filepath.Join(root, "zzz-huge")
	writeFile(t, filepath.Join(huge, "manifest.json"), `{"name":"Huge"}`)
	var contents strings.Builder
	contents.WriteString(`{"tweaks":[`)
	for i := 0; i < maxDiscoveredPluginTweaks; i++ {
		if i > 0 {
			contents.WriteByte(',')
		}
		fmt.Fprintf(&contents, `{"id":"t-%d","risk":"low","operations":[{"type":"registry_delete","hive":"HKCU","path":"A","name":"B"}]}`, i)
	}
	contents.WriteString(`]}`)
	writeFile(t, filepath.Join(huge, "tweaks.json"), contents.String())

	plugins, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 1 || plugins[0].ID != "aaa-good" {
		t.Fatalf("want only the valid plugin, got %+v", plugins)
	}
}

func TestDiscoverValidPlugin(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "my-plugin")
	writeFile(t, filepath.Join(dir, "manifest.json"),
		`{"name":"My Tweaks","version":"1.2.0","description":"nice","author":"jane"}`)
	writeFile(t, filepath.Join(dir, "tweaks.json"), `{"tweaks":[
		{"id":"p1","name":"One","category":"x","description":"d","risk":"low","reversible":true,
		 "operations":[{"type":"registry_set_dword","hive":"HKCU","path":"A","name":"B","value":1}],
		 "revert":[{"type":"registry_delete","hive":"HKCU","path":"A","name":"B"}]}
	]}`)

	plugins, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("want 1 plugin, got %d", len(plugins))
	}
	p := plugins[0]
	if p.ID != "my-plugin" || p.Name != "My Tweaks" || p.Version != "1.2.0" || p.Author != "jane" {
		t.Errorf("unexpected plugin: %+v", p)
	}
	if len(p.Tweaks) != 1 || p.Tweaks[0].ID != "p1" {
		t.Errorf("unexpected tweaks: %+v", p.Tweaks)
	}
}

func TestDiscoverSkipsNonPluginsAndBroken(t *testing.T) {
	root := t.TempDir()

	// A directory with no manifest is not a plugin.
	writeFile(t, filepath.Join(root, "not-a-plugin", "readme.txt"), "hi")

	// A hidden directory is ignored.
	writeFile(t, filepath.Join(root, ".hidden", "manifest.json"), `{"name":"hidden"}`)

	// A regular file at root is ignored.
	writeFile(t, filepath.Join(root, "loose.json"), `{}`)

	// Broken manifest -> skipped.
	writeFile(t, filepath.Join(root, "bad-manifest", "manifest.json"), `{not json`)

	// Broken tweaks (invalid risk) -> skipped.
	writeFile(t, filepath.Join(root, "bad-tweaks", "manifest.json"), `{"name":"bad"}`)
	writeFile(t, filepath.Join(root, "bad-tweaks", "tweaks.json"), `{"tweaks":[
		{"id":"x","name":"X","operations":[{"type":"command","value":"echo"}],"risk":"yolo"}
	]}`)

	// A misspelled operation field must not silently decode to an empty name.
	writeFile(t, filepath.Join(root, "unknown-field", "manifest.json"), `{"name":"bad fields"}`)
	writeFile(t, filepath.Join(root, "unknown-field", "tweaks.json"), `{"tweaks":[
		{"id":"x","name":"X","operations":[{"type":"service_stop","nmae":"Svc"}]}
	]}`)

	plugins, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("want 0 plugins, got %+v", plugins)
	}
}

func TestDiscoverPluginWithoutTweaks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "meta-only", "manifest.json"), `{"name":"Meta Only"}`)

	plugins, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("want 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name != "Meta Only" {
		t.Errorf("want name fallback to manifest, got %q", plugins[0].Name)
	}
	if len(plugins[0].Tweaks) != 0 {
		t.Errorf("want 0 tweaks, got %d", len(plugins[0].Tweaks))
	}
}

func TestMergeTweaks(t *testing.T) {
	mkTweak := func(id string) config.Tweak {
		return config.Tweak{ID: id, Name: id}
	}
	base := []config.Tweak{mkTweak("a"), mkTweak("b")}
	extra := []config.Tweak{mkTweak("b"), mkTweak("c"), mkTweak("c")}

	merged := MergeTweaks(base, extra)

	want := []string{"a", "b", "c"}
	if len(merged) != len(want) {
		t.Fatalf("want %d tweaks, got %d: %+v", len(want), len(merged), merged)
	}
	for i, id := range want {
		if merged[i].ID != id {
			t.Errorf("merged[%d] = %q, want %q", i, merged[i].ID, id)
		}
	}

	// base must not be mutated.
	if len(base) != 2 {
		t.Fatalf("base was mutated: %+v", base)
	}
}
