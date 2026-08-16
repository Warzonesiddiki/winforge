// Plugin runtime: Lua community packs.
//
// This file is platform-independent. It defines the whitelisted API that a
// Lua script uses to PROPOSE tweaks, and the strict decoding of those
// proposals into []config.Tweak. A Lua script never executes at apply time:
// it runs once, during plugin discovery, and yields tweak definitions that
// flow through the exact same config validation and orchestrator paths
// (audit, undo, protected services, elevation boundary) as tweaks shipped in
// tweaks.json.
//
// The only platform-dependent piece is ScriptHost — the thin layer that
// actually evaluates a script against the whitelisted API. On Windows it is
// implemented with syscall.NewLazyDLL against a copy of lua54.dll located
// next to the executable or in the WinForge data directory (see
// lua_windows.go). On every other platform — or when lua54.dll cannot be
// found — loadLuaScript reports ErrLuaUnavailable and the plugin is skipped
// (see lua_other.go), keeping the engine stdlib-only and CGO_ENABLED=0.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"winforge/internal/config"
)

// ErrLuaUnavailable is returned when no Lua runtime can be loaded on this
// platform/install. It is not fatal: a Lua plugin is skipped best-effort.
var ErrLuaUnavailable = errors.New("Lua runtime unavailable")

// newScriptHost is the factory used to obtain a Lua host. It is a variable so
// tests can substitute a fake without requiring the Windows-only lua54.dll.
var newScriptHost = loadLuaHost

// luaInstructionBudget is the maximum number of Lua VM instructions a single
// script may execute before its hook aborts it. It bounds runaway/hostile
// scripts (e.g. `while true do end`). Generous enough for real packs, small
// enough to fail fast.
const luaInstructionBudget = 10_000_000

// maxLogEntries / maxLogBytes bound the log output a script may produce.
const (
	maxLogEntries = 256
	maxLogBytes   = 4096
)

// ScriptHost evaluates one Lua source file against the whitelisted WinForge
// API. The Windows implementation owns the lua_State lifetime, the
// instruction hook, and the winforge.* function bindings; tests substitute
// a fake.
type ScriptHost interface {
	// Run evaluates source and returns the tweaks the script proposed plus
	// any bounded log lines. It must never panic on hostile input.
	Run(source string) (tweaks []config.Tweak, logs []string, err error)
}

// luaAPI is the whitelisted set of proposal-building functions exposed to a
// script. The host (Windows or fake) calls these; they are plain Go so the
// logic is unit-tested on Linux. Each proposal is constructed as a
// config.Operation and run through the same validateOperation the strict
// loader uses, so a script can never produce an operation the catalog could
// not itself contain.
type luaAPI struct {
	current *luaScriptTweak
	scripts []luaScriptTweak
	logs    []string
}

func newLuaAPI() *luaAPI { return &luaAPI{} }

// registrySet implements winforge.registry.set(hive, path, name, kind, value).
// kind is one of "dword", "qword", "string". value must match kind (number
// for dword/qword, string for string). The resulting operation is appended to
// the currently-open tweak's apply list and also returned as a handle so a
// script may add it to a revert list.
func (a *luaAPI) registrySet(hive, path, name, kind string, value any) (map[string]any, error) {
	var op config.Operation
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "dword":
		n, err := luaUint32(value)
		if err != nil {
			return nil, fmt.Errorf("registry.set dword: %w", err)
		}
		op = config.Operation{Type: config.OpRegistrySetDword, Hive: hive, Path: path, Name: name, Value: json.RawMessage(fmt.Sprintf("%d", n))}
	case "qword":
		n, err := luaUint64(value)
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
func (a *luaAPI) registryDelete(hive, path, name string) (map[string]any, error) {
	op := config.Operation{Type: config.OpRegistryDelete, Hive: hive, Path: path, Name: name}
	return a.recordOp(op, false)
}

// serviceSetStartMode implements winforge.service.set_start_mode(name, mode).
func (a *luaAPI) serviceSetStartMode(name, mode string) (map[string]any, error) {
	b, err := json.Marshal(mode)
	if err != nil {
		return nil, fmt.Errorf("service.set_start_mode: %w", err)
	}
	op := config.Operation{Type: config.OpServiceStartMode, Name: name, Value: json.RawMessage(b)}
	return a.recordOp(op, false)
}

// revert wraps an operation handle into the currently-open tweak's revert
// list. It is exposed to Lua as winforge.revert(handle) and returns the same
// handle so a script can use it inline.
func (a *luaAPI) revert(handle map[string]any) (map[string]any, error) {
	if a.current == nil {
		return nil, errors.New("winforge.revert called outside a tweak")
	}
	op, err := mapToOp(handle)
	if err != nil {
		return nil, err
	}
	a.current.revert = append(a.current.revert, op)
	return handle, nil
}

// log implements winforge.log(message), a bounded diagnostic line.
func (a *luaAPI) log(message string) error {
	if len(a.logs) >= maxLogEntries {
		return fmt.Errorf("script produced more than %d log entries", maxLogEntries)
	}
	if len(message) > maxLogBytes {
		message = message[:maxLogBytes]
	}
	a.logs = append(a.logs, message)
	return nil
}

// recordOp validates an operation through the strict loader's validator,
// appends it to the currently-open tweak's apply list, and returns a
// canonical handle the script can pass to winforge.revert. Validation is the
// security gate: it rejects bad hives, shallow delete-key paths, oversized
// strings, unknown op types, and malformed service modes identically for
// scripts and catalogs.
func (a *luaAPI) recordOp(op config.Operation, isRevert bool) (map[string]any, error) {
	if a.current == nil && !isRevert {
		return nil, errors.New("operation proposed outside a winforge.tweak block")
	}
	if _, allowed := allowedPluginOpTypes[op.Type]; !allowed {
		return nil, fmt.Errorf("operation type %q is not allowed in plugins", op.Type)
	}
	if err := config.ValidateOperationForPlugin(op); err != nil {
		return nil, err
	}
	if a.current != nil && !isRevert {
		a.current.operations = append(a.current.operations, op)
	}
	return opToMap(op), nil
}

// luaScriptTweak accumulates one tweak proposal between beginTweak and
// commit.
type luaScriptTweak struct {
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
func (a *luaAPI) beginTweak(fields map[string]any) (*luaScriptTweak, error) {
	if a.current != nil {
		return nil, fmt.Errorf("tweak %q is still open; call commit before starting another", a.current.id)
	}
	t := &luaScriptTweak{reversible: true}
	var err error
	if t.id, err = luaStringField(fields, "id", true); err != nil {
		return nil, err
	}
	if t.name, err = luaStringField(fields, "name", false); err != nil {
		return nil, err
	}
	if t.category, err = luaStringField(fields, "category", false); err != nil {
		return nil, err
	}
	if t.description, err = luaStringField(fields, "description", false); err != nil {
		return nil, err
	}
	if t.risk, err = luaStringField(fields, "risk", false); err != nil {
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
func (a *luaAPI) commit(t *luaScriptTweak) error {
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
	// Round-trip through the strict loader validator, which also normalizes
	// an empty risk to low and derives an empty name from the id.
	cfg := config.TweakConfig{Tweaks: []config.Tweak{ct}}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("tweak %q: %w", t.id, err)
	}
	validated := cfg.Tweaks[0]
	a.scripts = append(a.scripts, luaScriptTweak{
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

// tweaks returns the finalized tweaks proposed by the script.
func (a *luaAPI) tweaks() []config.Tweak {
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

// opToMap canonicalizes a validated Operation into the map handed to Lua as
// an operation handle. It is the inverse of mapToOp; going through JSON
// guarantees the two agree on field names and value encoding.
func opToMap(op config.Operation) map[string]any {
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

// allowedPluginOpTypes is the closed set of operation kinds a script may
// propose. A script must never be able to build a `command` (which reaches
// the elevated executor allowlist), an Appx removal, or a scheduled-task
// change: those are privileged actions reserved for the curated catalog.
var allowedPluginOpTypes = map[string]struct{}{
	config.OpRegistrySetDword:  {},
	config.OpRegistrySetString: {},
	config.OpRegistrySetQword:  {},
	config.OpRegistryDelete:    {},
	config.OpServiceStartMode:  {},
}

// mapToOp decodes an operation handle (possibly returned from a hostile
// script) back into a config.Operation and re-validates it. Unknown fields
// and operation types outside the plugin whitelist are rejected so a script
// cannot smuggle a privileged action or a field the engine ignores.
func mapToOp(m map[string]any) (config.Operation, error) {
	if t, ok := m["type"].(string); ok {
		if _, allowed := allowedPluginOpTypes[t]; !allowed {
			return config.Operation{}, fmt.Errorf("operation type %q is not allowed in plugins", t)
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
	if _, allowed := allowedPluginOpTypes[op.Type]; !allowed {
		return config.Operation{}, fmt.Errorf("operation type %q is not allowed in plugins", op.Type)
	}
	if err := config.ValidateOperationForPlugin(op); err != nil {
		return config.Operation{}, err
	}
	return op, nil
}

// --- numeric / field helpers (bounds-checked against hostile script input) ---

func luaUint32(v any) (uint32, error) {
	f, ok := luaNumber(v)
	if !ok {
		return 0, fmt.Errorf("value must be a non-negative integer, got %T", v)
	}
	if f < 0 || f != float64(uint32(f)) {
		return 0, fmt.Errorf("value %v is out of DWORD range or not an integer", v)
	}
	return uint32(f), nil
}

func luaUint64(v any) (uint64, error) {
	f, ok := luaNumber(v)
	if !ok {
		return 0, fmt.Errorf("value must be a non-negative integer, got %T", v)
	}
	if f < 0 || f != float64(uint64(f)) {
		return 0, fmt.Errorf("value %v is out of QWORD range or not an integer", v)
	}
	return uint64(f), nil
}

// luaNumber accepts a Lua number (float64 on the Go side) or an integer.
func luaNumber(v any) (float64, bool) {
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

func luaStringField(fields map[string]any, key string, required bool) (string, error) {
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
