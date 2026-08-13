package tweak

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"winforge/internal/audit"
	"winforge/internal/config"
	"winforge/internal/power"
)

// mockExecutor is an in-memory Executor used to verify orchestration logic.
type mockExecutor struct {
	dwords        map[string]uint32
	qwords        map[string]uint64
	strings       map[string]string
	expandStrings map[string]string
	startModes    map[string]string
	commands      []string
	activePlan    string
	mutationErr   error
}

func newMock() *mockExecutor {
	return &mockExecutor{
		dwords:        map[string]uint32{},
		qwords:        map[string]uint64{},
		strings:       map[string]string{},
		expandStrings: map[string]string{},
		startModes:    map[string]string{},
	}
}

func key(hive, path, name string) string { return hive + "\\" + path + "\\" + name }

func (m *mockExecutor) RegistryGetDword(hive, path, name string) (uint32, bool, error) {
	v, ok := m.dwords[key(hive, path, name)]
	return v, ok, nil
}
func (m *mockExecutor) RegistryGetString(hive, path, name string) (string, bool, error) {
	v, ok := m.strings[key(hive, path, name)]
	return v, ok, nil
}
func (m *mockExecutor) RegistryGetExpandString(hive, path, name string) (string, bool, error) {
	v, ok := m.expandStrings[key(hive, path, name)]
	return v, ok, nil
}
func (m *mockExecutor) RegistryGetQword(hive, path, name string) (uint64, bool, error) {
	v, ok := m.qwords[key(hive, path, name)]
	return v, ok, nil
}
func (m *mockExecutor) RegistrySetDword(hive, path, name string, value uint32) error {
	if m.mutationErr != nil {
		return m.mutationErr
	}
	k := key(hive, path, name)
	delete(m.strings, k)
	delete(m.expandStrings, k)
	delete(m.qwords, k)
	m.dwords[k] = value
	return nil
}
func (m *mockExecutor) RegistrySetString(hive, path, name string, value string) error {
	if m.mutationErr != nil {
		return m.mutationErr
	}
	k := key(hive, path, name)
	delete(m.dwords, k)
	delete(m.expandStrings, k)
	delete(m.qwords, k)
	m.strings[k] = value
	return nil
}
func (m *mockExecutor) RegistrySetExpandString(hive, path, name string, value string) error {
	if m.mutationErr != nil {
		return m.mutationErr
	}
	k := key(hive, path, name)
	delete(m.dwords, k)
	delete(m.qwords, k)
	delete(m.strings, k)
	m.expandStrings[k] = value
	return nil
}
func (m *mockExecutor) RegistrySetQword(hive, path, name string, value uint64) error {
	if m.mutationErr != nil {
		return m.mutationErr
	}
	k := key(hive, path, name)
	delete(m.dwords, k)
	delete(m.strings, k)
	delete(m.expandStrings, k)
	m.qwords[k] = value
	return nil
}
func (m *mockExecutor) RegistryDeleteValue(hive, path, name string) error {
	if m.mutationErr != nil {
		return m.mutationErr
	}
	delete(m.strings, key(hive, path, name))
	delete(m.expandStrings, key(hive, path, name))
	delete(m.dwords, key(hive, path, name))
	delete(m.qwords, key(hive, path, name))
	return nil
}
func (m *mockExecutor) ServiceSetStartMode(name, mode string) error {
	if m.mutationErr != nil {
		return m.mutationErr
	}
	m.startModes[name] = mode
	return nil
}
func (m *mockExecutor) ServiceGetStartMode(name string) (string, error) {
	if v, ok := m.startModes[name]; ok {
		return v, nil
	}
	return "manual", nil
}
func (m *mockExecutor) ServiceStart(string) error { return nil }
func (m *mockExecutor) ServiceStop(string) error  { return nil }
func (m *mockExecutor) TaskDisable(string) error  { return nil }
func (m *mockExecutor) TaskEnable(string) error   { return nil }
func (m *mockExecutor) TaskDelete(string) error   { return nil }
func (m *mockExecutor) AppxRemove(string) error   { return nil }
func (m *mockExecutor) RunCommand(c string, _ []string) error {
	if m.mutationErr != nil {
		return m.mutationErr
	}
	m.commands = append(m.commands, c)
	return nil
}
func (m *mockExecutor) PowerGetActive() (string, error) {
	return m.activePlan, nil
}
func (m *mockExecutor) PowerSetActive(guid string) error {
	if m.mutationErr != nil {
		return m.mutationErr
	}
	m.activePlan = guid
	return nil
}

func dwordOp(hive, path, name string, val int) config.Operation {
	return config.Operation{
		Type: config.OpRegistrySetDword,
		Hive: hive, Path: path, Name: name,
		Value: json.RawMessage([]byte{byte('0' + val)}),
	}
}

func TestDryRunDoesNotMutate(t *testing.T) {
	exec := newMock()
	o := NewOrchestrator(exec, nil)

	tw := config.Tweak{
		ID:         "t1",
		Reversible: true,
		Risk:       config.RiskLow,
		Operations: []config.Operation{dwordOp("HKLM", "A", "B", 1)},
	}

	res := o.Apply(tw, true)
	if res.Failed != 0 {
		t.Fatalf("dry run failed: %+v", res.Effects)
	}
	if res.Changed != 1 {
		t.Errorf("dry run should report 1 changed, got %d", res.Changed)
	}
	if _, exists, err := exec.RegistryGetDword("HKLM", "A", "B"); err != nil || exists {
		t.Fatal("dry run must not mutate state")
	}
}

func TestApplyMutatesAndIsApplied(t *testing.T) {
	exec := newMock()
	o := NewOrchestrator(exec, nil)

	tw := config.Tweak{
		ID:         "t1",
		Reversible: true,
		Risk:       config.RiskLow,
		Operations: []config.Operation{dwordOp("HKLM", "A", "B", 1)},
	}

	if o.IsApplied(tw) {
		t.Fatal("tweak should not be applied yet")
	}

	res := o.Apply(tw, false)
	if res.Failed != 0 || res.Succeeded != 1 {
		t.Fatalf("apply failed: %+v", res.Effects)
	}
	if !o.IsApplied(tw) {
		t.Fatal("tweak should be applied after Apply")
	}

	// Applying again should report no change (idempotent).
	res2 := o.Apply(tw, false)
	if res2.Changed != 0 {
		t.Errorf("second apply should change nothing, got %d changed", res2.Changed)
	}
}

func TestServiceStartModeAliasesAreIdempotent(t *testing.T) {
	exec := newMock()
	exec.startModes["Svc"] = "automatic"
	exec.mutationErr = errors.New("mutation should not be attempted")
	o := NewOrchestrator(exec, nil)

	tw := config.Tweak{
		ID: "service-alias",
		Operations: []config.Operation{{
			Type:  config.OpServiceStartMode,
			Name:  "Svc",
			Value: json.RawMessage(`" AUTO "`),
		}},
	}

	if !o.IsApplied(tw) {
		t.Fatal("automatic service should satisfy the auto alias")
	}
	result := o.Apply(tw, false)
	if result.Failed != 0 || result.Changed != 0 {
		t.Fatalf("alias-equivalent mode should not trigger a mutation: %+v", result)
	}
}

func TestUndoWithRevertList(t *testing.T) {
	exec := newMock()
	o := NewOrchestrator(exec, nil)

	tw := config.Tweak{
		ID:         "t1",
		Reversible: true,
		Risk:       config.RiskLow,
		Operations: []config.Operation{dwordOp("HKLM", "A", "B", 0)},
		Revert:     []config.Operation{dwordOp("HKLM", "A", "B", 1)},
	}

	o.Apply(tw, false)
	if v, _, _ := exec.RegistryGetDword("HKLM", "A", "B"); v != 0 {
		t.Fatalf("after apply, want 0, got %d", v)
	}
	o.Undo(tw)
	if v, _, _ := exec.RegistryGetDword("HKLM", "A", "B"); v != 1 {
		t.Fatalf("after undo, want 1, got %d", v)
	}
}

func TestEnsureApplied(t *testing.T) {
	exec := newMock()
	o := NewOrchestrator(exec, nil)

	// a is already at its target value; b is not.
	exec.dwords[key("HKLM", "A", "X")] = 1
	tweaks := []config.Tweak{
		{ID: "a", Risk: config.RiskLow, Operations: []config.Operation{dwordOp("HKLM", "A", "X", 1)}},
		{ID: "b", Risk: config.RiskLow, Operations: []config.Operation{dwordOp("HKLM", "A", "Y", 2)}},
	}

	applied, errs := o.EnsureApplied(tweaks)
	if len(errs) != 0 {
		t.Fatalf("EnsureApplied errors: %v", errs)
	}
	if len(applied) != 1 || applied[0] != "b" {
		t.Fatalf("want [b] applied, got %v", applied)
	}
	if v, _, _ := exec.RegistryGetDword("HKLM", "A", "Y"); v != 2 {
		t.Errorf("tweak b not applied: value=%d", v)
	}

	// A second run must be a no-op.
	applied, errs = o.EnsureApplied(tweaks)
	if len(errs) != 0 || len(applied) != 0 {
		t.Fatalf("second EnsureApplied should be a no-op, got applied=%v errs=%v", applied, errs)
	}
}

func TestPowerSchemeOp(t *testing.T) {
	exec := newMock()
	exec.activePlan = power.Balanced
	o := NewOrchestrator(exec, nil)

	tw := config.Tweak{
		ID:         "t-power",
		Reversible: true,
		Risk:       config.RiskLow,
		Operations: []config.Operation{{
			Type:  config.OpPowerScheme,
			Value: json.RawMessage(`"ultimate"`),
		}},
	}

	if o.IsApplied(tw) {
		t.Fatal("ultimate must not be applied while balanced is active")
	}

	res := o.Apply(tw, false)
	if res.Failed != 0 || res.Succeeded != 1 {
		t.Fatalf("apply failed: %+v", res.Effects)
	}
	if exec.activePlan != power.UltimateClone {
		t.Errorf("want active plan %s, got %s", power.UltimateClone, exec.activePlan)
	}
	if !o.IsApplied(tw) {
		t.Fatal("tweak should be applied after PowerSetActive")
	}

	// Re-applying must be a no-op.
	res2 := o.Apply(tw, false)
	if res2.Changed != 0 {
		t.Errorf("second apply should change nothing, got %d changed", res2.Changed)
	}
}

func TestUndoEntryRestoresAbsentZeroEmptyAndQword(t *testing.T) {
	tests := []struct {
		name      string
		operation config.Operation
		seed      func(*mockExecutor)
		assert    func(*testing.T, *mockExecutor)
	}{
		{
			name:      "absent dword",
			operation: dwordOp("HKLM", "A", "absent", 1),
			assert: func(t *testing.T, exec *mockExecutor) {
				if _, exists, _ := exec.RegistryGetDword("HKLM", "A", "absent"); exists {
					t.Fatal("undo did not restore value absence")
				}
			},
		},
		{
			name:      "zero dword",
			operation: dwordOp("HKLM", "A", "zero", 1),
			seed: func(exec *mockExecutor) {
				exec.dwords[key("HKLM", "A", "zero")] = 0
			},
			assert: func(t *testing.T, exec *mockExecutor) {
				if value, exists, _ := exec.RegistryGetDword("HKLM", "A", "zero"); !exists || value != 0 {
					t.Fatalf("undo restored DWORD as value=%d exists=%v, want zero and present", value, exists)
				}
			},
		},
		{
			name: "empty string",
			operation: config.Operation{Type: config.OpRegistrySetString, Hive: "HKCU", Path: "A", Name: "empty",
				Value: json.RawMessage(`"new"`)},
			seed: func(exec *mockExecutor) {
				exec.strings[key("HKCU", "A", "empty")] = ""
			},
			assert: func(t *testing.T, exec *mockExecutor) {
				if value, exists, _ := exec.RegistryGetString("HKCU", "A", "empty"); !exists || value != "" {
					t.Fatalf("undo restored string as value=%q exists=%v, want empty and present", value, exists)
				}
			},
		},
		{
			name: "expand string",
			operation: config.Operation{Type: config.OpRegistrySetString, Hive: "HKCU", Path: "A", Name: "expand",
				Value: json.RawMessage(`"replacement"`)},
			seed: func(exec *mockExecutor) {
				exec.expandStrings[key("HKCU", "A", "expand")] = `%SystemRoot%\\Temp`
			},
			assert: func(t *testing.T, exec *mockExecutor) {
				value, exists, _ := exec.RegistryGetExpandString("HKCU", "A", "expand")
				if !exists || value != `%SystemRoot%\\Temp` {
					t.Fatalf("undo restored expand string as value=%q exists=%v", value, exists)
				}
				if _, exists, _ := exec.RegistryGetString("HKCU", "A", "expand"); exists {
					t.Fatal("undo restored REG_EXPAND_SZ as REG_SZ")
				}
			},
		},
		{
			name: "qword",
			operation: config.Operation{Type: config.OpRegistrySetQword, Hive: "HKLM", Path: "A", Name: "qword",
				Value: json.RawMessage(`18446744073709551615`)},
			seed: func(exec *mockExecutor) {
				exec.qwords[key("HKLM", "A", "qword")] = 42
			},
			assert: func(t *testing.T, exec *mockExecutor) {
				if value, exists, _ := exec.RegistryGetQword("HKLM", "A", "qword"); !exists || value != 42 {
					t.Fatalf("undo restored QWORD as value=%d exists=%v, want 42 and present", value, exists)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := newMock()
			if tt.seed != nil {
				tt.seed(exec)
			}
			logger := audit.NewLogger(t.TempDir())
			o := NewOrchestrator(exec, logger)
			res := o.Apply(config.Tweak{ID: "undo-test", Reversible: true, Operations: []config.Operation{tt.operation}}, false)
			if err := res.Failure(); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			entries, err := logger.ReadAll()
			if err != nil || len(entries) != 1 {
				t.Fatalf("ReadAll: entries=%d err=%v", len(entries), err)
			}
			entry := entries[0]
			if !entry.PreviousValueCaptured || !entry.CanUndo {
				t.Fatalf("entry did not capture undo state: %+v", entry)
			}
			if err := o.UndoEntry(entry); err != nil {
				t.Fatalf("UndoEntry: %v", err)
			}
			tt.assert(t, exec)
		})
	}
}

func TestUndoEntryRestoresServiceAndPowerState(t *testing.T) {
	exec := newMock()
	exec.startModes["example"] = "auto"
	exec.activePlan = power.Balanced
	logger := audit.NewLogger(t.TempDir())
	o := NewOrchestrator(exec, logger)

	tw := config.Tweak{ID: "state", Reversible: true, Operations: []config.Operation{
		{Type: config.OpServiceStartMode, Name: "example", Value: json.RawMessage(`"disabled"`)},
		{Type: config.OpPowerScheme, Value: json.RawMessage(`"ultimate"`)},
	}}
	if err := o.Apply(tw, false).Failure(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	entries, err := logger.ReadAll()
	if err != nil || len(entries) != 2 {
		t.Fatalf("ReadAll: entries=%d err=%v", len(entries), err)
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if err := o.UndoEntry(entries[i]); err != nil {
			t.Fatalf("UndoEntry(%s): %v", entries[i].OperationType, err)
		}
	}
	if exec.startModes["example"] != "auto" {
		t.Errorf("service mode = %q, want auto", exec.startModes["example"])
	}
	if exec.activePlan != power.Balanced {
		t.Errorf("power plan = %q, want balanced", exec.activePlan)
	}
}

func TestUndoEntryRejectsInconsistentSnapshotMetadata(t *testing.T) {
	exec := newMock()
	o := NewOrchestrator(exec, nil)
	entry := audit.Entry{
		ID:                    "corrupt",
		OperationType:         config.OpServiceStartMode,
		Success:               true,
		CanUndo:               true,
		PreviousValueCaptured: true,
		PreviousValueExists:   true,
		PreviousValueType:     "dword",
		PreviousValue:         "1",
		RegistryHive:          "HKLM",
		RegistryPath:          "A",
		RegistryName:          "B",
	}
	if err := o.UndoEntry(entry); err == nil {
		t.Fatal("UndoEntry accepted cross-type snapshot metadata")
	}
	if _, exists, _ := exec.RegistryGetDword("HKLM", "A", "B"); exists {
		t.Fatal("inconsistent snapshot metadata caused a mutation")
	}
}

func TestUndoEntrySupportsConsistentLegacyRegistrySnapshot(t *testing.T) {
	exec := newMock()
	exec.dwords[regKey("HKLM", "A", "B")] = 9
	o := NewOrchestrator(exec, nil)
	entry := audit.Entry{
		ID:            "legacy",
		OperationType: config.OpRegistrySetDword,
		Target:        `HKLM\A\B`,
		PreviousValue: "5",
		Success:       true,
		CanUndo:       true,
		RegistryHive:  "HKLM",
		RegistryPath:  "A",
		RegistryName:  "B",
	}
	if err := o.UndoEntry(entry); err != nil {
		t.Fatalf("UndoEntry rejected a consistent legacy snapshot: %v", err)
	}
	if value := exec.dwords[regKey("HKLM", "A", "B")]; value != 5 {
		t.Fatalf("legacy snapshot restored %d, want 5", value)
	}
}

func TestUndoEntryRejectsMismatchedRegistryTarget(t *testing.T) {
	exec := newMock()
	exec.dwords[regKey("HKLM", "A", "B")] = 7
	o := NewOrchestrator(exec, nil)
	entry := audit.Entry{
		ID:                    "mismatch",
		OperationType:         config.OpRegistrySetDword,
		Target:                `HKLM\A\DifferentValue`,
		Success:               true,
		CanUndo:               true,
		PreviousValueCaptured: true,
		PreviousValueExists:   false,
		PreviousValueType:     "dword",
		RegistryHive:          "HKLM",
		RegistryPath:          "A",
		RegistryName:          "B",
	}
	if err := o.UndoEntry(entry); err == nil {
		t.Fatal("UndoEntry accepted a target that disagreed with structured registry metadata")
	}
	if value := exec.dwords[regKey("HKLM", "A", "B")]; value != 7 {
		t.Fatalf("invalid registry snapshot caused a mutation: %d", value)
	}
}

func TestUndoEntryRejectsInvalidRequestedPowerScheme(t *testing.T) {
	exec := newMock()
	exec.activePlan = power.UltimateClone
	o := NewOrchestrator(exec, nil)
	entry := audit.Entry{
		ID:                    "bad-power-target",
		OperationType:         config.OpPowerScheme,
		Target:                "power:not-a-power-scheme",
		Success:               true,
		CanUndo:               true,
		PreviousValueCaptured: true,
		PreviousValueExists:   true,
		PreviousValueType:     "power_scheme",
		PreviousValue:         power.Balanced,
	}
	if err := o.UndoEntry(entry); err == nil {
		t.Fatal("UndoEntry accepted an invalid requested power scheme")
	}
	if exec.activePlan != power.UltimateClone {
		t.Fatalf("invalid power snapshot caused a mutation: %q", exec.activePlan)
	}
}

func TestUndoEntryRejectsDuplicateUndo(t *testing.T) {
	exec := newMock()
	logger := audit.NewLogger(t.TempDir())
	o := NewOrchestrator(exec, logger)
	res := o.Apply(config.Tweak{
		ID: "duplicate", Reversible: true,
		Operations: []config.Operation{dwordOp("HKLM", "A", "B", 1)},
	}, false)
	if err := res.Failure(); err != nil {
		t.Fatal(err)
	}
	entries, err := logger.ReadAll()
	if err != nil || len(entries) != 1 {
		t.Fatalf("ReadAll: entries=%d err=%v", len(entries), err)
	}
	if err := o.UndoEntry(entries[0]); err != nil {
		t.Fatalf("first UndoEntry: %v", err)
	}
	if err := o.UndoEntry(entries[0]); err == nil || !strings.Contains(err.Error(), "already been undone") {
		t.Fatalf("duplicate UndoEntry error = %v", err)
	}
}

func TestFailedMutationIsAuditedAndPropagated(t *testing.T) {
	exec := newMock()
	exec.mutationErr = errors.New("access denied")
	logger := audit.NewLogger(t.TempDir())
	o := NewOrchestrator(exec, logger)
	tw := config.Tweak{ID: "failure", Reversible: true, Operations: []config.Operation{dwordOp("HKLM", "A", "B", 1)}}

	res := o.Apply(tw, false)
	if res.Failed != 1 || res.Failure() == nil {
		t.Fatalf("failure was not propagated: %+v", res)
	}
	entries, err := logger.ReadAll()
	if err != nil || len(entries) != 1 {
		t.Fatalf("ReadAll: entries=%d err=%v", len(entries), err)
	}
	if entries[0].Success || entries[0].CanUndo || entries[0].ErrorMessage == "" {
		t.Fatalf("failed audit entry is incorrect: %+v", entries[0])
	}
}

func TestEnsureAppliedSkipsUnverifiableOperations(t *testing.T) {
	exec := newMock()
	o := NewOrchestrator(exec, nil)
	tw := config.Tweak{ID: "command", Operations: []config.Operation{{
		Type: config.OpCommand, Value: json.RawMessage(`"example.exe"`),
	}}}
	applied, errs := o.EnsureApplied([]config.Tweak{tw})
	if len(applied) != 0 || len(errs) != 0 || len(exec.commands) != 0 {
		t.Fatalf("unverifiable command was rerun: applied=%v errs=%v commands=%v", applied, errs, exec.commands)
	}
}

func TestResultFailureIncludesEffectErrors(t *testing.T) {
	res := Result{TweakID: "nested", Failed: 2, Effects: []Effect{
		{OperationType: config.OpCommand, Target: "first.exe", Err: errors.New("top")},
		{OperationType: config.OpCommand, Target: "second.exe", Err: errors.New("nested")},
	}}
	err := res.Failure()
	if err == nil || !strings.Contains(err.Error(), "top") || !strings.Contains(err.Error(), "nested") {
		t.Fatalf("Failure() = %v, want both effect errors", err)
	}
}

func TestUndoRejectsMissingExplicitRevert(t *testing.T) {
	exec := newStubExecutor()
	exec.dwords[regKey("HKLM", "A", "B")] = 42
	o := NewOrchestrator(exec, nil)

	res := o.Undo(config.Tweak{
		ID:         "unsafe-derived-revert",
		Reversible: true,
		Operations: []config.Operation{dwordOp("HKLM", "A", "B", 1)},
	})
	if res.Failed != 1 || res.Failure() == nil {
		t.Fatalf("Undo result = %+v, want explicit-revert failure", res)
	}
	if got := exec.dwords[regKey("HKLM", "A", "B")]; got != 42 {
		t.Fatalf("pre-existing registry value was changed to %d", got)
	}
}

func TestUndoRejectsNonReversibleTweak(t *testing.T) {
	o := NewOrchestrator(newMock(), nil)
	res := o.Undo(config.Tweak{ID: "one-way", Reversible: false, Revert: []config.Operation{dwordOp("HKLM", "A", "B", 1)}})
	if res.Failed != 1 || res.Failure() == nil {
		t.Fatalf("non-reversible tweak undo was not rejected: %+v", res)
	}
}

func TestComputeHealth(t *testing.T) {
	tweaks := []config.Tweak{
		{ID: "a", Risk: config.RiskLow},
		{ID: "b", Risk: config.RiskMedium},
		{ID: "c", Risk: config.RiskHigh},
	}
	applied := map[string]bool{"a": true}
	h := ComputeHealth(tweaks, applied, 2)

	// 100 - 0*2 (a applied) - 1*5 (b) - 1*10 (c) - 2*3 (bloatware) = 79
	if h.Score != 79 {
		t.Errorf("want 79, got %d", h.Score)
	}
	if h.UnappliedMedium != 1 || h.UnappliedHigh != 1 {
		t.Errorf("unexpected unapplied counts: %+v", h)
	}
}
