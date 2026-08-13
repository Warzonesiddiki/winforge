package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"winforge/internal/audit"
	"winforge/internal/config"
	"winforge/internal/tweak"
)

func TestNewLoadsEmbeddedConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "WinForge")
	a, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if len(a.Tweaks) == 0 {
		t.Fatal("expected embedded tweaks to load")
	}
	if len(a.Apps) == 0 {
		t.Fatal("expected embedded apps to load")
	}
	if a.DataDir != dir {
		t.Errorf("DataDir = %q, want %q", a.DataDir, dir)
	}

	first := a.Tweaks[0]
	if _, ok := a.FindTweak(first.ID); !ok {
		t.Errorf("FindTweak(%q) = false, want true", first.ID)
	}
	if got := len(a.AppliedMap()); got > len(a.Tweaks) {
		t.Errorf("AppliedMap returned %d entries for %d tweaks", got, len(a.Tweaks))
	}
}

func TestDefaultDataDirHonorsLocalAppData(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmp)
	if got := DefaultDataDir(); got != filepath.Join(tmp, "WinForge") {
		t.Errorf("DefaultDataDir() = %q, want under LOCALAPPDATA", got)
	}
}

func TestBloatwareEmptyOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bloatware detection scans the real registry on Windows")
	}
	a, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(a.Bloatware()) != 0 {
		t.Fatalf("Bloatware() = %v, want empty off-Windows", a.Bloatware())
	}
	if a.BloatwareCount() != 0 {
		t.Fatalf("BloatwareCount() = %d, want 0", a.BloatwareCount())
	}
}

func TestNewCreatesDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "WinForge")
	a, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.DataDir, "logs")); err != nil {
		t.Errorf("logs dir not created: %v", err)
	}
}

// stubExecutor is an in-memory tweak.Executor used to exercise RunMaintenance
// without touching real system state.
type stubExecutor struct {
	dwords map[string]uint32
}

func newStubExecutor() *stubExecutor {
	return &stubExecutor{dwords: map[string]uint32{}}
}

func (e *stubExecutor) key(hive, path, name string) string { return hive + "\\" + path + "\\" + name }

func (e *stubExecutor) RegistryGetDword(hive, path, name string) (uint32, bool, error) {
	v, ok := e.dwords[e.key(hive, path, name)]
	return v, ok, nil
}
func (e *stubExecutor) RegistryGetString(_, _, _ string) (string, bool, error) { return "", false, nil }
func (e *stubExecutor) RegistryGetQword(_, _, _ string) (uint64, bool, error) {
	return 0, false, nil
}
func (e *stubExecutor) RegistrySetDword(hive, path, name string, value uint32) error {
	e.dwords[e.key(hive, path, name)] = value
	return nil
}
func (e *stubExecutor) RegistrySetString(_, _, _ string, _ string) error { return nil }
func (e *stubExecutor) RegistrySetQword(_, _, _ string, _ uint64) error  { return nil }
func (e *stubExecutor) RegistryDeleteValue(hive, path, name string) error {
	delete(e.dwords, e.key(hive, path, name))
	return nil
}
func (e *stubExecutor) ServiceSetStartMode(_, _ string) error { return nil }
func (e *stubExecutor) ServiceGetStartMode(_ string) (string, error) {
	return "manual", nil
}
func (e *stubExecutor) ServiceStart(_ string) error { return nil }
func (e *stubExecutor) ServiceStop(_ string) error  { return nil }
func (e *stubExecutor) TaskDisable(_ string) error  { return nil }
func (e *stubExecutor) TaskEnable(_ string) error   { return nil }
func (e *stubExecutor) TaskDelete(_ string) error   { return nil }
func (e *stubExecutor) AppxRemove(_ string) error   { return nil }
func (e *stubExecutor) RunCommand(_ string, _ []string) error {
	return nil
}
func (e *stubExecutor) PowerGetActive() (string, error) { return "", nil }
func (e *stubExecutor) PowerSetActive(_ string) error   { return nil }

func dwordTweak(id string, val uint32) config.Tweak {
	return config.Tweak{
		ID:         id,
		Risk:       config.RiskLow,
		Reversible: true,
		Operations: []config.Operation{{
			Type:  config.OpRegistrySetDword,
			Hive:  "HKLM",
			Path:  "A",
			Name:  id,
			Value: json.RawMessage{byte('0' + val)},
		}},
	}
}

func TestRunMaintenanceSummary(t *testing.T) {
	logger := audit.NewLogger(t.TempDir())
	exec := newStubExecutor()
	a := &App{
		Orchestrator:     tweak.NewOrchestrator(exec, logger),
		Tweaks:           []config.Tweak{dwordTweak("t1", 1)},
		Packages:         nil, // nil → app-update phase reports "skipped"
		Logger:           logger,
		AutoRestorePoint: false,
	}

	sum := a.RunMaintenance(context.Background(), nil)
	if sum.RanAt.IsZero() {
		t.Error("RunMaintenance did not record RanAt")
	}
	if !sum.AppsSkipped {
		t.Error("AppsSkipped = false, want true (nil Packages)")
	}
	if len(sum.TweaksApplied) != 1 || sum.TweaksApplied[0] != "t1" {
		t.Errorf("TweaksApplied = %v, want [t1]", sum.TweaksApplied)
	}
	if len(sum.TweakErrors) != 0 {
		t.Errorf("TweakErrors = %v, want none", sum.TweakErrors)
	}

	entries, err := logger.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.OperationType == "maintenance" {
			found = true
		}
	}
	if !found {
		t.Error("RunMaintenance did not write an audit entry")
	}
}
