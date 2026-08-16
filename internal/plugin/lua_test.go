package plugin

import (
	"strings"
	"testing"

	"winforge/internal/config"
)

// fakeHost is a ScriptHost that records the source it was given and can be
// programmed to return a fixed set of tweaks/error. It lets the platform-
// independent proposal/validation logic be unit-tested on Linux without a
// real Lua VM.
type fakeHost struct {
	source string
	tweaks []config.Tweak
	logs   []string
	err    error
}

func (f *fakeHost) Run(source string) ([]config.Tweak, []string, error) {
	f.source = source
	return f.tweaks, f.logs, f.err
}

// exerciseAPI drives an *luaAPI the same way the Windows host does for a
// small Lua program, so the proposal builders and hostile-input rejection
// are tested without a C interpreter. It understands a subset of Lua:
//
//	local t = winforge.tweak{id="...", ...}
//	local op = winforge.registry.set("HKCU","K","N","dword",1)
//	t:commit()
//
// This is intentionally not a Lua parser — it calls the Go API directly
// with the arguments the script would pass, which is exactly what the host
// callbacks do after decoding the stack.
func TestLuaAPIRegistrySetDword(t *testing.T) {
	a := newLuaAPI()
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

func TestLuaAPIRegistrySetString(t *testing.T) {
	a := newLuaAPI()
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

func TestLuaAPIServiceSetStartMode(t *testing.T) {
	a := newLuaAPI()
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

// --- hostile input: every case must be rejected by validation ---

func TestLuaAPIBadHive(t *testing.T) {
	a := newLuaAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.registrySet("HKEY_BOGUS", `K`, "N", "dword", float64(1)); err == nil {
		t.Fatal("expected bad hive to be rejected")
	}
}

func TestLuaAPIOversizedString(t *testing.T) {
	a := newLuaAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("x", 20000)
	if _, err := a.registrySet("HKCU", `K`, "N", "string", big); err == nil {
		t.Fatal("expected oversized string to be rejected")
	}
}

func TestLuaAPIDwordOverflow(t *testing.T) {
	a := newLuaAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	// Larger than uint32 max.
	if _, err := a.registrySet("HKCU", `K`, "N", "dword", float64(1<<32)); err == nil {
		t.Fatal("expected out-of-range dword to be rejected")
	}
}

func TestLuaAPIDwordFractionRejected(t *testing.T) {
	a := newLuaAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.registrySet("HKCU", `K`, "N", "dword", 1.5); err == nil {
		t.Fatal("expected fractional dword to be rejected")
	}
}

func TestLuaAPIUnknownKind(t *testing.T) {
	a := newLuaAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.registrySet("HKCU", `K`, "N", "binary", float64(1)); err == nil {
		t.Fatal("expected unknown kind to be rejected")
	}
}

func TestLuaAPIHandleWithUnknownField(t *testing.T) {
	a := newLuaAPI()
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

func TestLuaAPIServiceBadMode(t *testing.T) {
	a := newLuaAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.serviceSetStartMode("Fax", "yolo"); err == nil {
		t.Fatal("expected bad service mode to be rejected")
	}
}

func TestLuaAPIServiceMalformedMode(t *testing.T) {
	a := newLuaAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x", "reversible": false}); err != nil {
		t.Fatal(err)
	}
	// A bogus mode is rejected at proposal time. (Whitespace/protected-name
	// validation of the service name itself happens in the engine executor at
	// apply time, the same as for catalog tweaks — covered by the engine
	// package's guard tests.)
	if _, err := a.serviceSetStartMode("Fax", "automatic-delayed-???"); err == nil {
		t.Fatal("expected malformed service mode to be rejected")
	}
}

// TestLuaPluginCannotProposeCommand verifies the plugin whitelist blocks a
// script from forging a command op even if it constructs the handle manually.
// The elevated-executor allowlist is a second, independent gate at apply time.
func TestLuaPluginCannotProposeCommand(t *testing.T) {
	a := newLuaAPI()
	if _, err := a.beginTweak(map[string]any{"id": "x", "reversible": false}); err != nil {
		t.Fatal(err)
	}
	forged := map[string]any{"type": "command", "value": "powershell.exe"}
	if _, err := a.revert(forged); err == nil {
		t.Fatal("expected forged command handle to be rejected by the plugin whitelist")
	}
}

func TestLuaAPIOperationOutsideTweak(t *testing.T) {
	a := newLuaAPI()
	if _, err := a.registrySet("HKCU", "K", "N", "dword", float64(1)); err == nil {
		t.Fatal("expected operation outside a tweak block to be rejected")
	}
}

func TestLuaAPICommitWithoutOperations(t *testing.T) {
	a := newLuaAPI()
	tw, err := a.beginTweak(map[string]any{"id": "empty"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.commit(tw); err == nil {
		t.Fatal("expected commit with no operations to be rejected")
	}
}

func TestLuaAPINestedTweakRejected(t *testing.T) {
	a := newLuaAPI()
	if _, err := a.beginTweak(map[string]any{"id": "first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.beginTweak(map[string]any{"id": "second"}); err == nil {
		t.Fatal("expected a second open tweak to be rejected")
	}
}

func TestLuaAPILogBounded(t *testing.T) {
	a := newLuaAPI()
	for i := 0; i < maxLogEntries; i++ {
		if err := a.log("x"); err != nil {
			t.Fatalf("log %d: %v", i, err)
		}
	}
	if err := a.log("overflow"); err == nil {
		t.Fatal("expected log cap to be enforced")
	}
}

func TestLuaAPILogTruncated(t *testing.T) {
	a := newLuaAPI()
	big := strings.Repeat("x", maxLogBytes*2)
	if err := a.log(big); err != nil {
		t.Fatalf("log: %v", err)
	}
	if len(a.logs[0]) != maxLogBytes {
		t.Fatalf("expected truncation to %d, got %d", maxLogBytes, len(a.logs[0]))
	}
}

func TestLuaRegistryDelete(t *testing.T) {
	a := newLuaAPI()
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

// TestLuaRequiresID ensures a tweak table without id is rejected.
func TestLuaRequiresID(t *testing.T) {
	a := newLuaAPI()
	if _, err := a.beginTweak(map[string]any{"name": "no id"}); err == nil {
		t.Fatal("expected missing id to be rejected")
	}
}

// TestLuaFakeHostInterface ensures the fake satisfies ScriptHost.
func TestLuaFakeHostInterface(t *testing.T) {
	var _ ScriptHost = (*fakeHost)(nil)
}
