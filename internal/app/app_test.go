package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"winforge/internal/appmanager"
	"winforge/internal/audit"
	"winforge/internal/config"
	"winforge/internal/restorepoint"
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

func TestElevatedNewIgnoresUserWritableOverridesAndPlugins(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "tweaks.json"), []byte(`{broken`), 0o600); err != nil {
		t.Fatal(err)
	}
	pluginDir := filepath.Join(dir, "plugins", "untrusted")
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.json"), []byte(`{"name":"Untrusted"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	a, err := newApp(dir, true)
	if err != nil {
		t.Fatalf("elevated newApp consumed untrusted configuration: %v", err)
	}
	if !a.elevated {
		t.Fatal("newApp did not retain elevated security mode")
	}
	if len(a.Tweaks) == 0 {
		t.Fatal("embedded tweaks were not loaded")
	}
	if len(a.Plugins) != 0 {
		t.Fatalf("elevated newApp loaded user plugins: %+v", a.Plugins)
	}
	if a.Logger != nil {
		t.Fatal("elevated newApp enabled user-profile audit I/O")
	}
	if a.packages != nil {
		t.Fatal("elevated newApp enabled PATH-discovered package execution")
	}
	if _, err := os.Stat(filepath.Join(dir, "logs")); !os.IsNotExist(err) {
		t.Fatalf("elevated newApp touched user-profile log directory: %v", err)
	}
}

func TestElevatedNewDoesNotRequireAccessibleDataDirectory(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("user-controlled"), 0o600); err != nil {
		t.Fatalf("create blocked data path: %v", err)
	}

	a, err := newApp(blocked, true)
	if err != nil {
		t.Fatalf("elevated newApp accessed the blocked data path: %v", err)
	}
	if a.Logger != nil || a.packages != nil || len(a.Plugins) != 0 {
		t.Fatalf("elevated dependencies = logger %v, packages %v, plugins %v", a.Logger, a.packages, a.Plugins)
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

func TestBloatwareReturnsCopyOfMemoizedList(t *testing.T) {
	a := &App{bloatwareList: []string{"Example"}}
	a.bloatwareOnce.Do(func() {}) // mark the test fixture as already scanned
	got := a.Bloatware()
	got[0] = "mutated"
	if again := a.Bloatware(); len(again) != 1 || again[0] != "Example" {
		t.Fatalf("caller mutation corrupted memoized list: %v", again)
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
func (e *stubExecutor) RegistryGetString(_, _, _ string) (string, bool, error) {
	return "", false, nil
}
func (e *stubExecutor) RegistryGetExpandString(_, _, _ string) (string, bool, error) {
	return "", false, nil
}
func (e *stubExecutor) RegistryGetQword(_, _, _ string) (uint64, bool, error) {
	return 0, false, nil
}
func (e *stubExecutor) RegistrySetDword(hive, path, name string, value uint32) error {
	e.dwords[e.key(hive, path, name)] = value
	return nil
}
func (e *stubExecutor) RegistrySetString(_, _, _ string, _ string) error { return nil }
func (e *stubExecutor) RegistrySetExpandString(_, _, _ string, _ string) error {
	return nil
}
func (e *stubExecutor) RegistrySetQword(_, _, _ string, _ uint64) error { return nil }
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

func TestHistoryMarksSuccessfullyUndoneEntryConsumed(t *testing.T) {
	logger := audit.NewLogger(t.TempDir())
	now := time.Now()
	original := audit.Entry{
		ID: "original", Timestamp: now, OperationType: config.OpRegistrySetDword,
		Success: true, CanUndo: true, PreviousValueCaptured: true,
	}
	if err := logger.Append(original); err != nil {
		t.Fatal(err)
	}
	if err := logger.Append(audit.Entry{
		ID: "undo", Timestamp: now.Add(time.Second), OperationType: "undo",
		Success: true, UndoOf: original.ID,
	}); err != nil {
		t.Fatal(err)
	}

	a := &App{Logger: logger}
	entries, err := a.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("History returned %d entries, want 2", len(entries))
	}
	for _, entry := range entries {
		if entry.ID == original.ID && entry.CanUndo {
			t.Fatalf("consumed entry remains undoable: %+v", entry)
		}
	}
}

func TestHistoryIgnoresFailedUndoLink(t *testing.T) {
	logger := audit.NewLogger(t.TempDir())
	now := time.Now()
	if err := logger.Append(audit.Entry{
		ID: "original", Timestamp: now, OperationType: config.OpRegistrySetDword,
		Success: true, CanUndo: true, PreviousValueCaptured: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Append(audit.Entry{
		ID: "failed-undo", Timestamp: now.Add(time.Second), OperationType: "undo",
		Success: false, UndoOf: "original",
	}); err != nil {
		t.Fatal(err)
	}

	entries, err := (&App{Logger: logger}).History()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.ID == "original" && !entry.CanUndo {
			t.Fatalf("failed undo consumed original entry: %+v", entry)
		}
	}
}

func TestUndoEntryRejectsConsumedEntryBeforeRestorePoint(t *testing.T) {
	logger := audit.NewLogger(t.TempDir())
	now := time.Now()
	if err := logger.Append(audit.Entry{
		ID: "original", Timestamp: now, OperationType: config.OpRegistrySetDword,
		Success: true, CanUndo: true, PreviousValueCaptured: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Append(audit.Entry{
		ID: "undo", Timestamp: now.Add(time.Second), OperationType: "undo",
		Success: true, UndoOf: "original",
	}); err != nil {
		t.Fatal(err)
	}

	restoreCalls := 0
	a := &App{
		Logger:                logger,
		AutoRestorePoint:      true,
		restorePointAvailable: func() bool { restoreCalls++; return true },
	}
	if err := a.UndoEntry("original"); err == nil {
		t.Fatal("UndoEntry accepted an already-consumed entry")
	}
	if restoreCalls != 0 {
		t.Fatalf("already-consumed undo caused %d restore-point side effect(s)", restoreCalls)
	}
}

func TestUndoEntryRejectsDuplicateAuditIDBeforeSideEffects(t *testing.T) {
	logger := audit.NewLogger(t.TempDir())
	now := time.Now()
	entry := audit.Entry{
		ID:                    "duplicate",
		Timestamp:             now,
		OperationType:         config.OpRegistrySetDword,
		Target:                `HKLM\A\B`,
		PreviousValueCaptured: true,
		PreviousValueExists:   false,
		PreviousValueType:     "dword",
		Success:               true,
		CanUndo:               true,
		RegistryHive:          "HKLM",
		RegistryPath:          "A",
		RegistryName:          "B",
	}
	if err := logger.Append(entry); err != nil {
		t.Fatal(err)
	}
	entry.Timestamp = now.Add(time.Second)
	if err := logger.Append(entry); err != nil {
		t.Fatal(err)
	}

	exec := newStubExecutor()
	key := exec.key("HKLM", "A", "B")
	exec.dwords[key] = 9
	restoreCalls := 0
	a := &App{
		Logger:                logger,
		Orchestrator:          tweak.NewOrchestrator(exec, logger),
		AutoRestorePoint:      true,
		restorePointAvailable: func() bool { restoreCalls++; return true },
	}
	if err := a.UndoEntry(entry.ID); err == nil {
		t.Fatal("UndoEntry accepted an ambiguous duplicate audit id")
	}
	if restoreCalls != 0 {
		t.Fatalf("duplicate-id undo caused %d restore-point side effect(s)", restoreCalls)
	}
	if value, exists := exec.dwords[key]; !exists || value != 9 {
		t.Fatalf("duplicate-id undo mutated registry fixture: exists=%v value=%d", exists, value)
	}
}

func TestUndoEntryIsDisabledForElevatedApplication(t *testing.T) {
	restoreCalls := 0
	a := &App{
		elevated:              true,
		AutoRestorePoint:      true,
		restorePointAvailable: func() bool { restoreCalls++; return true },
	}
	if err := a.UndoEntry("untrusted"); err == nil {
		t.Fatal("elevated application accepted per-operation history undo")
	}
	if restoreCalls != 0 {
		t.Fatalf("rejected elevated undo caused %d restore-point side effect(s)", restoreCalls)
	}
}

func TestEnsureRestorePointThrottlesNativeSuccessDespiteAuditFailure(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatalf("create audit blocker: %v", err)
	}
	now := time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)
	calls := 0
	a := &App{
		AutoRestorePoint:      true,
		Logger:                audit.NewLogger(blocked),
		restorePointAvailable: func() bool { return true },
		restorePointCreator: func(string) (restorepoint.Info, error) {
			calls++
			return restorepoint.Info{Description: "created"}, nil
		},
		clock: func() time.Time { return now },
	}

	a.EnsureRestorePoint("first")
	a.EnsureRestorePoint("second")
	if calls != 1 {
		t.Fatalf("native restore-point calls = %d, want 1", calls)
	}
	if !a.lastRestorePoint.Equal(now) {
		t.Fatalf("lastRestorePoint = %v, want %v", a.lastRestorePoint, now)
	}
}

func TestEnsureRestorePointRetriesNativeFailure(t *testing.T) {
	createErr := errors.New("native failure")
	calls := 0
	a := &App{
		AutoRestorePoint:      true,
		restorePointAvailable: func() bool { return true },
		restorePointCreator: func(string) (restorepoint.Info, error) {
			calls++
			return restorepoint.Info{}, createErr
		},
		clock: func() time.Time { return time.Now() },
	}

	a.EnsureRestorePoint("first")
	a.EnsureRestorePoint("second")
	if calls != 2 {
		t.Fatalf("native restore-point calls = %d, want 2", calls)
	}
	if !a.lastRestorePoint.IsZero() {
		t.Fatalf("failed creation consumed throttle window: %v", a.lastRestorePoint)
	}
}

func TestEnsureRestorePointRecoversFromFutureThrottleTimestamp(t *testing.T) {
	now := time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)
	calls := 0
	a := &App{
		AutoRestorePoint:      true,
		lastRestorePoint:      now.Add(time.Minute),
		restorePointAvailable: func() bool { return true },
		restorePointCreator: func(string) (restorepoint.Info, error) {
			calls++
			return restorepoint.Info{}, nil
		},
		clock: func() time.Time { return now },
	}

	a.EnsureRestorePoint("clock corrected")
	if calls != 1 {
		t.Fatalf("native restore-point calls = %d, want 1", calls)
	}
}

func TestCreateRestorePointReturnsAuditFailureAfterNativeSuccess(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatalf("create audit blocker: %v", err)
	}
	now := time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)
	a := &App{
		Logger: audit.NewLogger(blocked),
		restorePointCreator: func(string) (restorepoint.Info, error) {
			return restorepoint.Info{Description: "created"}, nil
		},
		clock: func() time.Time { return now },
	}

	info, err := a.CreateRestorePoint("created")
	if err == nil {
		t.Fatal("CreateRestorePoint returned nil error for audit failure")
	}
	if info.Description != "created" {
		t.Fatalf("CreateRestorePoint info = %+v", info)
	}
	if !a.lastRestorePoint.Equal(now) {
		t.Fatalf("lastRestorePoint = %v, want %v", a.lastRestorePoint, now)
	}
}

func TestInstallPackageRestoresThenInstallsAndAudits(t *testing.T) {
	logger := audit.NewLogger(t.TempDir())
	var events []string
	a := &App{
		AutoRestorePoint:      true,
		Logger:                logger,
		restorePointAvailable: func() bool { return true },
		restorePointCreator: func(description string) (restorepoint.Info, error) {
			events = append(events, "restore:"+description)
			return restorepoint.Info{Description: description}, nil
		},
		packageInstaller: func(_ context.Context, id string, _ func(appmanager.Progress)) (*appmanager.Result, error) {
			events = append(events, "install:"+id)
			return &appmanager.Result{Success: true}, nil
		},
	}

	result, err := a.InstallPackage(nil, "Microsoft.VisualStudioCode", nil)
	if err != nil {
		t.Fatalf("InstallPackage: %v", err)
	}
	if !result.Success {
		t.Fatalf("InstallPackage result = %+v", result)
	}
	if len(events) != 2 || events[0] != "restore:WinForge: install package Microsoft.VisualStudioCode" || events[1] != "install:Microsoft.VisualStudioCode" {
		t.Fatalf("operation order = %v", events)
	}

	entries, err := logger.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 2 || entries[0].OperationType != "restore_point" || entries[1].OperationType != "package_install" {
		t.Fatalf("audit entries = %+v", entries)
	}
	if !entries[1].Success || entries[1].Target != "Microsoft.VisualStudioCode" || entries[1].CanUndo {
		t.Fatalf("package audit entry = %+v", entries[1])
	}
}

func TestInstallPackageUsesResultErrorAndAuditsFailure(t *testing.T) {
	logger := audit.NewLogger(t.TempDir())
	installErr := errors.New("winget reported a deployment failure")
	a := &App{
		Logger: logger,
		packageInstaller: func(context.Context, string, func(appmanager.Progress)) (*appmanager.Result, error) {
			return &appmanager.Result{Error: installErr}, nil
		},
	}

	result, err := a.InstallPackage(context.Background(), "Microsoft.VisualStudioCode", nil)
	if !errors.Is(err, installErr) {
		t.Fatalf("InstallPackage error = %v, want %v", err, installErr)
	}
	if result.Error != installErr {
		t.Fatalf("result error = %v, want original error %v", result.Error, installErr)
	}
	entries, readErr := logger.ReadAll()
	if readErr != nil {
		t.Fatalf("ReadAll: %v", readErr)
	}
	if len(entries) != 1 || entries[0].Success || entries[0].ErrorMessage != installErr.Error() {
		t.Fatalf("package audit entry = %+v", entries)
	}
}

func TestInstallPackagePreservesDistinctReturnedAndResultErrors(t *testing.T) {
	returnedErr := errors.New("winget process exited unsuccessfully")
	resultErr := errors.New("winget output reported a deployment failure")
	a := &App{
		packageInstaller: func(context.Context, string, func(appmanager.Progress)) (*appmanager.Result, error) {
			return &appmanager.Result{Error: resultErr}, returnedErr
		},
	}

	result, err := a.InstallPackage(context.Background(), "Microsoft.VisualStudioCode", nil)
	if !errors.Is(err, returnedErr) || !errors.Is(err, resultErr) {
		t.Fatalf("InstallPackage error = %v, want both %v and %v", err, returnedErr, resultErr)
	}
	if !errors.Is(result.Error, returnedErr) || !errors.Is(result.Error, resultErr) {
		t.Fatalf("result error = %v, want both %v and %v", result.Error, returnedErr, resultErr)
	}
}

func TestInstallPackageRejectsInvalidIDBeforeSideEffects(t *testing.T) {
	called := false
	a := &App{
		AutoRestorePoint:      true,
		restorePointAvailable: func() bool { called = true; return true },
		packageInstaller: func(context.Context, string, func(appmanager.Progress)) (*appmanager.Result, error) {
			called = true
			return &appmanager.Result{Success: true}, nil
		},
	}

	if _, err := a.InstallPackage(context.Background(), "--source.Evil", nil); err == nil {
		t.Fatal("InstallPackage accepted an option-like package id")
	}
	if called {
		t.Fatal("invalid package id caused a restore-point or install side effect")
	}
}

func TestInstallPackageSurfacesAuditFailureAfterSuccessfulInstall(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatalf("create audit blocker: %v", err)
	}
	a := &App{
		Logger: audit.NewLogger(blocked),
		packageInstaller: func(context.Context, string, func(appmanager.Progress)) (*appmanager.Result, error) {
			return &appmanager.Result{Success: true}, nil
		},
	}

	result, err := a.InstallPackage(context.Background(), "Microsoft.VisualStudioCode", nil)
	if err == nil {
		t.Fatal("InstallPackage returned nil error for audit failure")
	}
	if !result.Success {
		t.Fatalf("successful install result was lost: %+v", result)
	}
}

func TestRunMaintenanceReportsAuditFailure(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatalf("create audit blocker: %v", err)
	}
	exec := newStubExecutor()
	a := &App{
		Orchestrator: tweak.NewOrchestrator(exec, nil),
		Logger:       audit.NewLogger(blocked),
	}

	sum := a.RunMaintenance(context.Background(), nil)
	if sum.AuditError == "" {
		t.Fatal("RunMaintenance() AuditError is empty, want append failure")
	}
}

func TestRunMaintenanceSummary(t *testing.T) {
	logger := audit.NewLogger(t.TempDir())
	exec := newStubExecutor()
	a := &App{
		Orchestrator:     tweak.NewOrchestrator(exec, logger),
		Tweaks:           []config.Tweak{dwordTweak("t1", 1)},
		packages:         nil, // nil → app-update phase reports "skipped"
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
			if !e.Success {
				t.Error("maintenance audit entry marked a graceful app-update skip as failure")
			}
		}
	}
	if !found {
		t.Error("RunMaintenance did not write an audit entry")
	}
}
