// Package plugin: WASM community packs (Phase 4 design, platform-independent tier).
//
// This file is platform-independent. It defines the whitelisted proposal API
// that a WASM guest uses to PROPOSE tweaks, and the strict validation of
// those proposals into []config.Tweak. A WASM module never executes at apply
// time: it runs once, during plugin discovery, and yields tweak definitions
// that flow through the exact same config validation and orchestrator paths
// (audit, undo, protected services, elevation boundary) as tweaks shipped in
// tweaks.json.
//
// The only platform-dependent piece is WasmHost — the thin layer that actually
// instantiates a module and links the host imports. On Windows it will be
// implemented with syscall.NewLazyDLL against a wasmtime.dll located next to
// the executable or in the WinForge data directory (see wasm_windows.go).
// On every other platform — or when wasmtime.dll cannot be found —
// loadWasmHost reports ErrWasmUnavailable and the plugin is skipped
// (see wasm_other.go), keeping the engine stdlib-only and CGO_ENABLED=0.
//
// Design is documented in docs/WASM_PLUGIN_SANDBOX.md; the decision to defer
// the Windows C binding until a real Windows runner is available is recorded
// in docs/WASM_REALSCOPE_2026-08-16.md.
// This file lands the platform-independent proposal/validation tier so the
// WASM design is testable on Linux via a fake WasmHost, mirroring the Lua
// pattern (W1). The Windows binding (wasm_windows.go) remains a stub until
// BLK-6 is satisfied.
//
// WASM guests see ONLY these host imports (each funnels through the same
// validation as Lua and the catalog):
//
//	winforge.health_score              () -> i32          (app.Health)
//	winforge.tweak_is_applied          (ptr,len) -> i32   (orchestrator.IsApplied)
//	winforge.propose_registry_set      (ptr,len) -> i32   (validate + record)
//	winforge.log                       (ptr,len)          (bounded log)
//
// Proposals are bounded by limits.go and validated via config.ValidateOperationForPlugin
// with a closed op-type whitelist (registry dword/qword/string, registry
// delete, service start_mode). Commands, Appx, tasks, power, netbios, delete_key
// are forbidden — same as Lua.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"winforge/internal/config"
)

// ErrWasmUnavailable is returned when no WASM runtime can be loaded on this
// platform/install. It is not fatal: a WASM plugin is skipped best-effort.
var ErrWasmUnavailable = errors.New("WASM runtime unavailable")

// newWasmHost is the factory used to obtain a WASM host. It is a variable so
// tests can substitute a fake without requiring the Windows-only wasmtime.dll.
var newWasmHost = loadWasmHost

// wasmInstructionBudget mirrors the Lua budget conceptually; wasmtime uses
// fuel metering (wasmtime_config_consume_fuel_set + wasmtime_context_set_fuel).
// 10M fuel is the guest CPU budget.
const wasmFuelBudget = 10_000_000

// maxWasmModuleBytes bounds a single .wasm file. The global plugin file cap
// is 8 MiB; a WASM module is capped tighter (4 MiB) because a well-formed
// module is denser than JSON and 4 MiB is far above any reasonable pack
// (the wasmtime spike wat compiled to <1 KiB).
const maxWasmModuleBytes = 4 << 20

// maxWasmLogEntries / maxWasmLogBytes bound WASM guest log output (same as Lua).
const (
	maxWasmLogEntries = 256
	maxWasmLogBytes   = 4096
)

// WasmHost evaluates one WASM module against the whitelisted WinForge imports.
// The Windows implementation owns the wasmtime engine/store/linker lifetime,
// fuel metering, and host-function bindings; tests substitute a fake.
type WasmHost interface {
	// Run instantiates module (a valid WASM binary) and returns the tweaks
	// the guest proposed plus any bounded log lines. It must never panic on
	// hostile input and must enforce fuel metering.
	Run(module []byte) (tweaks []config.Tweak, logs []string, err error)
}

// wasmAPI is the whitelisted set of proposal-building functions exposed to a
// WASM guest. The host (Windows or fake) calls these; they are plain Go so
// the logic is unit-tested on Linux. Each proposal is constructed as a
// config.Operation and run through the same validateOperation the strict
// loader uses, so a guest can never produce an operation the catalog could
// not itself contain.
//
// The shape mirrors luaAPI deliberately — the security model is identical —
// but the type is distinct so the two tiers can evolve independently.
type wasmAPI struct {
	current *wasmScriptTweak
	scripts []wasmScriptTweak
	logs    []string
}

func newWasmAPI() *wasmAPI { return &wasmAPI{} }

// registrySet implements winforge.registry.set(hive, path, name, kind, value)
// for WASM guests (host import winforge.propose_registry_set decodes to this
// internally, but the fake host calls it directly in tests).
func (a *wasmAPI) registrySet(hive, path, name, kind string, value any) (map[string]any, error) {
	var op config.Operation
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "dword":
		n, err := wasmUint32(value)
		if err != nil {
			return nil, fmt.Errorf("registry.set dword: %w", err)
		}
		op = config.Operation{Type: config.OpRegistrySetDword, Hive: hive, Path: path, Name: name, Value: json.RawMessage(fmt.Sprintf("%d", n))}
	case "qword":
		n, err := wasmUint64(value)
		if err != nil {
			return nil, fmt.Errorf("registry.set qword: %w", err)
		}
		op = config.Operation{Type: config.OpRegistrySetQword, Hive: hive, Path: path, Name: name, Value: json.RawMessage(fmt.Sprintf("%d", n))}
	case "string":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("registry.set string: value must be a string, got %T", value)
		}
		b, err := json.Marshal(s)
		if err != nil {
			return nil, fmt.Errorf("registry.set string: %w", err)
		}
		op = config.Operation{Type: config.OpRegistrySetString, Hive: hive, Path: path, Name: name, Value: json.RawMessage(b)}
	default:
		return nil, fmt.Errorf("registry.set: unknown kind %q (want dword, qword, or string)", kind)
	}
	return a.recordOp(op, false)
}

// registryDelete implements winforge.registry.delete(hive, path, name).
func (a *wasmAPI) registryDelete(hive, path, name string) (map[string]any, error) {
	op := config.Operation{Type: config.OpRegistryDelete, Hive: hive, Path: path, Name: name}
	return a.recordOp(op, false)
}

// serviceSetStartMode implements winforge.service.set_start_mode(name, mode).
func (a *wasmAPI) serviceSetStartMode(name, mode string) (map[string]any, error) {
	b, err := json.Marshal(mode)
	if err != nil {
		return nil, fmt.Errorf("service.set_start_mode: %w", err)
	}
	op := config.Operation{Type: config.OpServiceStartMode, Name: name, Value: json.RawMessage(b)}
	return a.recordOp(op, false)
}

// revert wraps an operation handle into the currently-open tweak's revert list.
func (a *wasmAPI) revert(handle map[string]any) (map[string]any, error) {
	if a.current == nil {
		return nil, errors.New("winforge.revert called outside a tweak")
	}
	op, err := wasmMapToOp(handle)
	if err != nil {
		return nil, err
	}
	a.current.revert = append(a.current.revert, op)
	return handle, nil
}

// log implements winforge.log(message), a bounded diagnostic line.
func (a *wasmAPI) log(message string) error {
	if len(a.logs) >= maxWasmLogEntries {
		return fmt.Errorf("script produced more than %d log entries", maxWasmLogEntries)
	}
	if len(message) > maxWasmLogBytes {
		message = message[:maxWasmLogBytes]
	}
	a.logs = append(a.logs, message)
	return nil
}

// recordOp validates an operation through the strict loader's validator,
// appends it to the currently-open tweak's apply list, and returns a
// canonical handle the guest can pass to winforge.revert.
func (a *wasmAPI) recordOp(op config.Operation, isRevert bool) (map[string]any, error) {
	if a.current == nil && !isRevert {
		return nil, errors.New("operation proposed outside a winforge.tweak block")
	}
	if _, allowed := allowedWasmOpTypes[op.Type]; !allowed {
		return nil, fmt.Errorf("operation type %q is not allowed in WASM plugins", op.Type)
	}
	if err := config.ValidateOperationForPlugin(op); err != nil {
		return nil, err
	}
	if a.current != nil && !isRevert {
		a.current.operations = append(a.current.operations, op)
	}
	return wasmOpToMap(op), nil
}

// wasmScriptTweak accumulates one tweak proposal between beginTweak and commit.
type wasmScriptTweak struct {
	id          string
	name        string
	category    string
	description string
	risk        string
	reversible  bool
	operations  []config.Operation
	revert      []config.Operation
}

// beginTweak opens a new tweak proposal from winforge.tweak{...}.
func (a *wasmAPI) beginTweak(fields map[string]any) (*wasmScriptTweak, error) {
	if a.current != nil {
		return nil, fmt.Errorf("tweak %q is still open; call commit before starting another", a.current.id)
	}
	t := &wasmScriptTweak{reversible: true}
	var err error
	if t.id, err = wasmStringField(fields, "id", true); err != nil {
		return nil, err
	}
	if t.name, err = wasmStringField(fields, "name", false); err != nil {
		return nil, err
	}
	if t.category, err = wasmStringField(fields, "category", false); err != nil {
		return nil, err
	}
	if t.description, err = wasmStringField(fields, "description", false); err != nil {
		return nil, err
	}
	if t.risk, err = wasmStringField(fields, "risk", false); err != nil {
		return nil, err
	}
	if rev, ok := fields["reversible"]; ok {
		b, ok := rev.(bool)
		if !ok {
			return nil, fmt.Errorf("tweak %q: reversible must be a boolean", t.id)
		}
		t.reversible = b
	}
	a.current = t
	return t, nil
}

// commit finalizes the currently-open tweak and stores it.
func (a *wasmAPI) commit(t *wasmScriptTweak) error {
	if a.current == nil || t != a.current {
		return errors.New("winforge.commit called without an open tweak")
	}
	defer func() { a.current = nil }()
	if len(t.operations) == 0 {
		return fmt.Errorf("tweak %q has no operations", t.id)
	}
	ct := config.Tweak{
		ID:          t.id,
		Name:        t.name,
		Category:    t.category,
		Description: t.description,
		Risk:        config.Risk(strings.ToLower(strings.TrimSpace(t.risk))),
		Reversible:  t.reversible,
		Operations:  t.operations,
		Revert:      t.revert,
	}
	cfg := config.TweakConfig{Tweaks: []config.Tweak{ct}}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("tweak %q: %w", t.id, err)
	}
	validated := cfg.Tweaks[0]
	a.scripts = append(a.scripts, wasmScriptTweak{
		id:          validated.ID,
		name:        validated.Name,
		category:    validated.Category,
		description: validated.Description,
		risk:        string(validated.Risk),
		reversible:  validated.Reversible,
		operations:  validated.Operations,
		revert:      validated.Revert,
	})
	return nil
}

// tweaks returns the finalized tweaks proposed by the guest.
func (a *wasmAPI) tweaks() []config.Tweak {
	out := make([]config.Tweak, 0, len(a.scripts))
	for _, s := range a.scripts {
		out = append(out, config.Tweak{
			ID:          s.id,
			Name:        s.name,
			Category:    s.category,
			Description: s.description,
			Risk:        config.Risk(s.risk),
			Reversible:  s.reversible,
			Operations:  s.operations,
			Revert:      s.revert,
		})
	}
	return out
}

// wasmOpToMap canonicalizes a validated Operation into the map handed to WASM
// as an operation handle (same encoding as Lua, so tests can share fixtures).
func wasmOpToMap(op config.Operation) map[string]any {
	b, _ := json.Marshal(struct {
		Type  string          `json:"type"`
		Hive  string          `json:"hive,omitempty"`
		Path  string          `json:"path,omitempty"`
		Name  string          `json:"name,omitempty"`
		Value json.RawMessage `json:"value,omitempty"`
		Args  []string        `json:"args,omitempty"`
	}{op.Type, op.Hive, op.Path, op.Name, op.Value, op.Args})
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// allowedWasmOpTypes is the closed set of operation kinds a WASM guest may
// propose (identical to Lua — the security boundary is tier-agnostic).
var allowedWasmOpTypes = map[string]struct{}{
	config.OpRegistrySetDword:  {},
	config.OpRegistrySetString: {},
	config.OpRegistrySetQword:  {},
	config.OpRegistryDelete:    {},
	config.OpServiceStartMode:  {},
}

// wasmMapToOp decodes an operation handle back into a config.Operation and
// re-validates it (rejects unknown fields and non-whitelisted types).
func wasmMapToOp(m map[string]any) (config.Operation, error) {
	if t, ok := m["type"].(string); ok {
		if _, allowed := allowedWasmOpTypes[t]; !allowed {
			return config.Operation{}, fmt.Errorf("operation type %q is not allowed in WASM plugins", t)
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return config.Operation{}, err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	var op config.Operation
	if err := dec.Decode(&op); err != nil {
		return config.Operation{}, fmt.Errorf("invalid operation: %w", err)
	}
	if _, allowed := allowedWasmOpTypes[op.Type]; !allowed {
		return config.Operation{}, fmt.Errorf("operation type %q is not allowed in WASM plugins", op.Type)
	}
	if err := config.ValidateOperationForPlugin(op); err != nil {
		return config.Operation{}, err
	}
	return op, nil
}

// --- numeric / field helpers (bounds-checked against hostile guest input) ---

func wasmUint32(v any) (uint32, error) {
	f, ok := wasmNumber(v)
	if !ok {
		return 0, fmt.Errorf("value must be a non-negative integer, got %T", v)
	}
	if f < 0 || f != float64(uint32(f)) {
		return 0, fmt.Errorf("value %v is out of DWORD range or not an integer", v)
	}
	return uint32(f), nil
}

func wasmUint64(v any) (uint64, error) {
	f, ok := wasmNumber(v)
	if !ok {
		return 0, fmt.Errorf("value must be a non-negative integer, got %T", v)
	}
	if f < 0 || f != float64(uint64(f)) {
		return 0, fmt.Errorf("value %v is out of QWORD range or not an integer", v)
	}
	return uint64(f), nil
}

func wasmNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func wasmStringField(fields map[string]any, key string, required bool) (string, error) {
	v, ok := fields[key]
	if !ok {
		if required {
			return "", fmt.Errorf("tweak field %q is required", key)
		}
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("tweak field %q must be a string, got %T", key, v)
	}
	return s, nil
}

// validateWasmModule performs the platform-independent structural checks on a
// raw .wasm byte slice before it is handed to a host. It is the WASM analog
// of the Lua source size / NUL checks: a hostile plugin cannot trigger a
// larger allocation or a deeper parse on the host side.
//
// Checks:
//   - non-empty and within maxWasmModuleBytes
//   - starts with the WASM magic `\x00asm`
//   - followed by version 1 (`\x01\x00\x00\x00` little-endian)
//
// A real wasmtime instantiation would perform far deeper validation (sections,
// imports, fuel). This pre-check fails fast on obviously malformed inputs and
// keeps the host from wasting fuel on them.
func validateWasmModule(b []byte) error {
	if len(b) == 0 {
		return errors.New("wasm module is empty")
	}
	if len(b) > maxWasmModuleBytes {
		return fmt.Errorf("wasm module exceeds %d-byte limit (%d bytes)", maxWasmModuleBytes, len(b))
	}
	if len(b) < 8 {
		return errors.New("wasm module too short to be valid")
	}
	if string(b[:4]) != "\x00asm" {
		return errors.New("wasm module missing magic \\x00asm")
	}
	// Version 1 = 0x01 0x00 0x00 0x00 (little-endian)
	if b[4] != 0x01 || b[5] != 0x00 || b[6] != 0x00 || b[7] != 0x00 {
		return fmt.Errorf("wasm module has unsupported version %d (want 1)", uint32(b[4])|uint32(b[5])<<8|uint32(b[6])<<16|uint32(b[7])<<24)
	}
	return nil
}
