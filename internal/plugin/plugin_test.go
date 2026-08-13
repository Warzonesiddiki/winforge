package plugin

import (
	"os"
	"path/filepath"
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

func TestDiscoverValidPlugin(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "my-plugin")
	writeFile(t, filepath.Join(dir, "manifest.json"),
		`{"name":"My Tweaks","version":"1.2.0","description":"nice","author":"jane"}`)
	writeFile(t, filepath.Join(dir, "tweaks.json"), `{"tweaks":[
		{"id":"p1","name":"One","category":"x","description":"d","risk":"low","reversible":true,
		 "operations":[{"type":"registry_set_dword","hive":"HKCU","path":"A","name":"B","value":1}]}
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
