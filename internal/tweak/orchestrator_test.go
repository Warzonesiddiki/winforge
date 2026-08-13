package tweak

import (
	"encoding/json"
	"testing"

	"winforge/internal/config"
)

// mockExecutor is an in-memory Executor used to verify orchestration logic.
type mockExecutor struct {
	dwords  map[string]uint32
	strings map[string]string
	startModes map[string]string
	commands []string
}

func newMock() *mockExecutor {
	return &mockExecutor{
		dwords:     map[string]uint32{},
		strings:    map[string]string{},
		startModes: map[string]string{},
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
func (m *mockExecutor) RegistrySetDword(hive, path, name string, value uint32) error {
	m.dwords[key(hive, path, name)] = value
	return nil
}
func (m *mockExecutor) RegistrySetString(hive, path, name string, value string) error {
	m.strings[key(hive, path, name)] = value
	return nil
}
func (m *mockExecutor) RegistryDeleteValue(hive, path, name string) error {
	delete(m.strings, key(hive, path, name))
	delete(m.dwords, key(hive, path, name))
	return nil
}
func (m *mockExecutor) ServiceSetStartMode(name, mode string) error {
	m.startModes[name] = mode
	return nil
}
func (m *mockExecutor) ServiceGetStartMode(name string) (string, error) {
	if v, ok := m.startModes[name]; ok {
		return v, nil
	}
	return "manual", nil
}
func (m *mockExecutor) ServiceStart(string) error   { return nil }
func (m *mockExecutor) ServiceStop(string) error    { return nil }
func (m *mockExecutor) TaskDisable(string) error    { return nil }
func (m *mockExecutor) TaskEnable(string) error     { return nil }
func (m *mockExecutor) TaskDelete(string) error     { return nil }
func (m *mockExecutor) AppxRemove(string) error     { return nil }
func (m *mockExecutor) RunCommand(c string, _ []string) error {
	m.commands = append(m.commands, c)
	return nil
}

func dwordOp(hive, path, name string, val int) config.Operation {
	return config.Operation{
		Type:  config.OpRegistrySetDword,
		Hive:  hive, Path: path, Name: name,
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
	if _, exists := exec.RegistryGetDword("HKLM", "A", "B"); exists {
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
	if v, _ := exec.RegistryGetDword("HKLM", "A", "B"); v != 0 {
		t.Fatalf("after apply, want 0, got %d", v)
	}
	o.Undo(tw)
	if v, _ := exec.RegistryGetDword("HKLM", "A", "B"); v != 1 {
		t.Fatalf("after undo, want 1, got %d", v)
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
