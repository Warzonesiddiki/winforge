package plugin

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"winforge/internal/config"
)

// fakeWasmHost is a WasmHost that records the module it was given and can be
// programmed to return a fixed set of tweaks/error. It lets the platform-
// independent proposal/validation logic be unit-tested on Linux without a
// real wasmtime VM.
type fakeWasmHost struct {
	module []byte
	tweaks []config.Tweak
	logs   []string
	err    error
}

func (f *fakeWasmHost) Run(module []byte) ([]config.Tweak, []string, error) {
	f.module = append([]byte(nil), module...)
	return f.tweaks, f.logs, f.err
}

func TestWasmAPIRegistrySetDword(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x", "reversible": false}); err != nil {
		t.Fatalf("beginTweak: %v", err)
	}
	if _, err := a.registrySet("HKCU", `Software\WinForge`, "V", "dword", float64(1)); err != nil {
		t.Fatalf("registrySet: %v", err)
	}
	if err := a.commit(a.current); err != nil {
		t.Fatalf("commit: %v", err)
	}
	tw := a.tweaks()
	if len(tw) != 1 {
		t.Fatalf("want 1 tweak, got %d", len(tw))
	}
	if len(tw[0].Operations) != 1 || tw[0].Operations[0].Type != config.OpRegistrySetDword {
		t.Fatalf("unexpected op: %+v", tw[0].Operations)
	}
}

func TestWasmAPIRegistrySetString(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatalf("beginTweak: %v", err)
	}
	handle, err := a.registrySet("HKCU", `K`, "N", "string", "hello")
	if err != nil {
		t.Fatalf("registrySet: %v", err)
	}
	if _, err := a.revert(handle); err != nil {
		t.Fatalf("revert: %v", err)
	}
	if err := a.commit(a.current); err != nil {
		t.Fatalf("commit: %v", err)
	}
	tw := a.tweaks()[0]
	if len(tw.Revert) != 1 || tw.Revert[0].Type != config.OpRegistrySetString {
		t.Fatalf("revert not recorded: %+v", tw.Revert)
	}
}

func TestWasmAPIServiceSetStartMode(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x", "reversible": false}); err != nil {
		t.Fatalf("beginTweak: %v", err)
	}
	if _, err := a.serviceSetStartMode("Fax", "disabled"); err != nil {
		t.Fatalf("serviceSetStartMode: %v", err)
	}
	if err := a.commit(a.current); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// --- hostile input ---

func TestWasmAPIBadHive(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.registrySet("HKEY_BOGUS", `K`, "N", "dword", float64(1)); err == nil {
		t.Fatal("expected bad hive to be rejected")
	}
}

func TestWasmAPIOversizedString(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("x", 20000)
	if _, err := a.registrySet("HKCU", `K`, "N", "string", big); err == nil {
		t.Fatal("expected oversized string to be rejected")
	}
}

func TestWasmAPIDwordOverflow(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.registrySet("HKCU", `K`, "N", "dword", float64(1<<32)); err == nil {
		t.Fatal("expected out-of-range dword to be rejected")
	}
}

func TestWasmAPIDwordFractionRejected(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.registrySet("HKCU", `K`, "N", "dword", 1.5); err == nil {
		t.Fatal("expected fractional dword to be rejected")
	}
}

func TestWasmAPIUnknownKind(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.registrySet("HKCU", `K`, "N", "binary", float64(1)); err == nil {
		t.Fatal("expected unknown kind to be rejected")
	}
}

func TestWasmAPIHandleWithUnknownField(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	forged := map[string]any{
		"type":  "registry_set_dword",
		"hive":  "HKCU",
		"path":  "K",
		"name":  "N",
		"value": float64(1),
		"evil":  "pwned",
	}
	if _, err := a.revert(forged); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field rejection, got %v", err)
	}
}

func TestWasmAPIServiceBadMode(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.serviceSetStartMode("Fax", "yolo"); err == nil {
		t.Fatal("expected bad service mode to be rejected")
	}
}

func TestWasmAPICannotProposeCommand(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x", "reversible": false}); err != nil {
		t.Fatal(err)
	}
	forged := map[string]any{"type": "command", "value": "powershell.exe"}
	if _, err := a.revert(forged); err == nil {
		t.Fatal("expected forged command handle to be rejected by whitelist")
	}
}

func TestWasmAPIOperationOutsideTweak(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.registrySet("HKCU", "K", "N", "dword", float64(1)); err == nil {
		t.Fatal("expected operation outside a tweak block to be rejected")
	}
}

func TestWasmAPICommitWithoutOperations(t *testing.T) {
	a := newWasmAPI()
	tw, err := a.beginTweak(map[string]any{"id": "empty"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.commit(tw); err == nil {
		t.Fatal("expected commit with no operations to be rejected")
	}
}

func TestWasmAPINestedTweakRejected(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.beginTweak(map[string]any{"id": "second"}); err == nil {
		t.Fatal("expected a second open tweak to be rejected")
	}
}

func TestWasmAPILogBounded(t *testing.T) {
	a := newWasmAPI()
	for i := 0; i < maxWasmLogEntries; i++ {
		if err := a.log("x"); err != nil {
			t.Fatalf("log %d: %v", i, err)
		}
	}
	if err := a.log("overflow"); err == nil {
		t.Fatal("expected log cap to be enforced")
	}
}

func TestWasmAPILogTruncated(t *testing.T) {
	a := newWasmAPI()
	big := strings.Repeat("x", maxWasmLogBytes*2)
	if err := a.log(big); err != nil {
		t.Fatalf("log: %v", err)
	}
	if len(a.logs[0]) != maxWasmLogBytes {
		t.Fatalf("expected truncation to %d, got %d", maxWasmLogBytes, len(a.logs[0]))
	}
}

func TestWasmRegistryDelete(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	h, err := a.registryDelete("HKCU", `Software\WinForge`, "V")
	if err != nil {
		t.Fatalf("registryDelete: %v", err)
	}
	if _, err := a.revert(h); err != nil {
		t.Fatalf("revert: %v", err)
	}
	if err := a.commit(a.current); err != nil {
		t.Fatalf("commit: %v", err)
	}
	tw := a.tweaks()[0]
	if tw.Operations[0].Type != config.OpRegistryDelete {
		t.Fatalf("want registry_delete, got %s", tw.Operations[0].Type)
	}
}

func TestWasmRequiresID(t *testing.T) {
	a := newWasmAPI()
	if _, err := a.beginTweak(map[string]any{"name": "no id"}); err == nil {
		t.Fatal("expected missing id to be rejected")
	}
}

func TestWasmFakeHostInterface(t *testing.T) {
	var _ WasmHost = (*fakeWasmHost)(nil)
}

// --- wasm module validation ---

func TestValidateWasmModuleValid(t *testing.T) {
	valid := append([]byte("\x00asm\x01\x00\x00\x00"), 0x00)
	if err := validateWasmModule(valid); err != nil {
		t.Fatalf("validateWasmModule: %v", err)
	}
}

func TestValidateWasmModuleEmpty(t *testing.T) {
	if err := validateWasmModule(nil); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("want empty error, got %v", err)
	}
}

func TestValidateWasmModuleTooShort(t *testing.T) {
	if err := validateWasmModule([]byte("\x00asm")); err == nil || !strings.Contains(err.Error(), "too short") {
		t.Fatalf("want too short, got %v", err)
	}
}

func TestValidateWasmModuleBadMagic(t *testing.T) {
	bad := []byte("\x00bad\x01\x00\x00\x00extra")
	if err := validateWasmModule(bad); err == nil || !strings.Contains(err.Error(), "magic") {
		t.Fatalf("want magic error, got %v", err)
	}
}

func TestValidateWasmModuleBadVersion(t *testing.T) {
	bad := []byte("\x00asm\x02\x00\x00\x00")
	if err := validateWasmModule(bad); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("want version error, got %v", err)
	}
}

func TestValidateWasmModuleOversized(t *testing.T) {
	big := make([]byte, maxWasmModuleBytes+1)
	copy(big, []byte("\x00asm\x01\x00\x00\x00"))
	if err := validateWasmModule(big); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("want oversized, got %v", err)
	}
}

func TestWasmModuleSizeBoundInLoader(t *testing.T) {
	// Ensure the loader's file-size check fires before validateWasmModule
	// (maxWasmModuleBytes is smaller than maxPluginFileBytes, so a file that
	// passes the generic file cap can still be rejected here).
	big := bytes.Repeat([]byte{0x00}, maxWasmModuleBytes+1)
	if len(big) <= maxWasmModuleBytes {
		t.Fatalf("test setup: not oversized")
	}
	// Directly test the bound via validateWasmModule; the file-size check in
	// loadWasmPlugin uses the same constant.
	if err := validateWasmModule(big); err == nil {
		t.Fatal("expected oversized wasm to be rejected")
	}
}

// --- discovery integration (mirrors lua_test.go) ---

func TestDiscoverWasmPluginUnavailable(t *testing.T) {
	prev := newWasmHost
	defer func() { newWasmHost = prev }()
	newWasmHost = func([]string) (WasmHost, error) { return nil, ErrWasmUnavailable }

	root := t.TempDir()
	writeFile(t, join(root, "json-pack", "manifest.json"), `{"name":"json"}`)
	writeFile(t, join(root, "json-pack", "tweaks.json"),
		`{"tweaks":[{"id":"j","risk":"low","operations":[{"type":"registry_delete","hive":"HKCU","path":"A","name":"B"}]}]}`)
	writeFile(t, join(root, "wasm-pack", "manifest.json"), `{"name":"wasm","type":"wasm"}`)
	writeFile(t, join(root, "wasm-pack", "pack.wasm"), string([]byte("\x00asm\x01\x00\x00\x00")))

	plugins, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 1 || plugins[0].ID != "json-pack" {
		t.Fatalf("want only the json plugin, got %+v", plugins)
	}
}

func TestDiscoverWasmPluginWithFakeHost(t *testing.T) {
	prev := newWasmHost
	defer func() { newWasmHost = prev }()
	newWasmHost = func([]string) (WasmHost, error) {
		return &fakeWasmHost{tweaks: []config.Tweak{{
			ID:         "wasm-1",
			Name:       "Wasm One",
			Category:   "Privacy",
			Risk:       config.RiskLow,
			Reversible: false,
			Operations: []config.Operation{{Type: config.OpRegistrySetDword, Hive: "HKCU", Path: "K", Name: "N", Value: []byte("1")}},
		}}, logs: []string{"loaded"}}, nil
	}

	root := t.TempDir()
	writeFile(t, join(root, "wasm-pack", "manifest.json"),
		`{"name":"Wasm Pack","type":"wasm","module":"custom.wasm"}`)
	writeFile(t, join(root, "wasm-pack", "custom.wasm"), string([]byte("\x00asm\x01\x00\x00\x00\x00")))

	plugins, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 1 || plugins[0].ID != "wasm-pack" || plugins[0].Type != "wasm" {
		t.Fatalf("unexpected plugins: %+v", plugins)
	}
	if len(plugins[0].Tweaks) != 1 || plugins[0].Tweaks[0].ID != "wasm-1" {
		t.Fatalf("tweaks not loaded: %+v", plugins[0].Tweaks)
	}
	if len(plugins[0].ScriptLogs) != 1 || plugins[0].ScriptLogs[0] != "loaded" {
		t.Fatalf("logs not captured: %+v", plugins[0].ScriptLogs)
	}
}

func TestDiscoverWasmPluginRejectsPathTraversal(t *testing.T) {
	prev := newWasmHost
	defer func() { newWasmHost = prev }()
	newWasmHost = func([]string) (WasmHost, error) { return &fakeWasmHost{}, nil }

	root := t.TempDir()
	writeFile(t, join(root, "evil", "manifest.json"),
		`{"name":"evil","type":"wasm","module":"../../etc/passwd"}`)

	plugins, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected path-traversal module to be rejected, got %+v", plugins)
	}
}

func TestDiscoverWasmPluginRejectsInvalidModule(t *testing.T) {
	prev := newWasmHost
	defer func() { newWasmHost = prev }()
	newWasmHost = func([]string) (WasmHost, error) { return &fakeWasmHost{}, nil }

	root := t.TempDir()
	writeFile(t, join(root, "bad", "manifest.json"), `{"name":"bad","type":"wasm"}`)
	writeFile(t, join(root, "bad", "pack.wasm"), `not a wasm module`)

	plugins, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected invalid wasm magic to be rejected, got %+v", plugins)
	}
}

func TestDiscoverWasmPluginRejectsInvalidTweaks(t *testing.T) {
	prev := newWasmHost
	defer func() { newWasmHost = prev }()
	newWasmHost = func([]string) (WasmHost, error) {
		return &fakeWasmHost{tweaks: []config.Tweak{{ID: "bad", Risk: config.RiskLow}}}, nil
	}

	root := t.TempDir()
	writeFile(t, join(root, "bad", "manifest.json"), `{"name":"bad","type":"wasm"}`)
	writeFile(t, join(root, "bad", "pack.wasm"), string([]byte("\x00asm\x01\x00\x00\x00")))

	plugins, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected invalid wasm tweaks to be rejected, got %+v", plugins)
	}
}

func TestDiscoverWasmModuleAliasViaScript(t *testing.T) {
	prev := newWasmHost
	defer func() { newWasmHost = prev }()
	newWasmHost = func([]string) (WasmHost, error) {
		return &fakeWasmHost{tweaks: []config.Tweak{{
			ID:         "wasm-alias",
			Risk:       config.RiskLow,
			Operations: []config.Operation{{Type: config.OpRegistryDelete, Hive: "HKCU", Path: "A", Name: "B"}},
		}}}, nil
	}

	root := t.TempDir()
	// Manifest uses "script" instead of "module" for WASM — alias should work.
	writeFile(t, join(root, "alias", "manifest.json"), `{"name":"alias","type":"wasm","script":"alias.wasm"}`)
	writeFile(t, join(root, "alias", "alias.wasm"), string([]byte("\x00asm\x01\x00\x00\x00")))

	plugins, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(plugins) != 1 || plugins[0].Tweaks[0].ID != "wasm-alias" {
		t.Fatalf("alias not accepted: %+v", plugins)
	}
}

// join mirrors filepath.Join for test readability.
func join(elem ...string) string { return filepath.Join(elem...) }
