// Package app is the composition root: it wires configuration, the engine,
// the orchestrator, the package manager, and the audit log into one object
// used by both the CLI and the HTTP dashboard.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"winforge/internal/appmanager"
	"winforge/internal/audit"
	"winforge/internal/config"
	"winforge/internal/engine"
	"winforge/internal/plugin"
	"winforge/internal/restorepoint"
	"winforge/internal/tweak"
)

// Version is the application version, stamped at build time via -ldflags.
var Version = "0.1.0-dev"

// App bundles all runtime dependencies.
type App struct {
	Loader            *config.Loader
	Orchestrator      *tweak.Orchestrator
	Packages          *appmanager.Manager
	Logger            *audit.Logger
	Tweaks            []config.Tweak
	Apps              []config.App
	Plugins           []plugin.Plugin
	ProtectedServices []string
	DnsPresets        []config.DnsEntry
	DataDir           string

	// AutoRestorePoint enables the safety-first policy: a system restore point
	// is created (best-effort, throttled) before the first mutation.
	AutoRestorePoint bool

	mu               sync.Mutex
	lastRestorePoint time.Time
}

// New builds the application, reading config (overrides then embedded defaults)
// and preparing the data directory for logs.
func New(dataDir string) (*App, error) {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "logs"), 0o700); err != nil {
		return nil, err
	}

	loader := config.NewLoader(filepath.Join(dataDir, "config"))

	tweaksCfg, err := loader.LoadTweaks()
	if err != nil {
		return nil, err
	}
	appsCfg, err := loader.LoadApps()
	if err != nil {
		return nil, err
	}
	protected, err := loader.LoadProtectedServices()
	if err != nil {
		return nil, err
	}
	dnsCfg, err := loader.LoadDns()
	if err != nil {
		return nil, err
	}

	logger := audit.NewLogger(filepath.Join(dataDir, "logs"))
	exec := engine.NewExecutor(protected)
	orch := tweak.NewOrchestrator(exec, logger)

	// Load plugins from <dataDir>/plugins and merge their tweaks after the
	// built-in (and user-override) tweaks. Built-ins win on id collisions.
	plugins, err := plugin.Discover(filepath.Join(dataDir, "plugins"))
	if err != nil {
		return nil, err
	}
	tweaks := tweaksCfg.Tweaks
	for _, p := range plugins {
		tweaks = plugin.MergeTweaks(tweaks, p.Tweaks)
	}

	return &App{
		Loader:            loader,
		Orchestrator:      orch,
		Packages:          appmanager.New(),
		Logger:            logger,
		Tweaks:            tweaks,
		Apps:              appsCfg.Applications,
		Plugins:           plugins,
		ProtectedServices: protected,
		DnsPresets:        dnsCfg.Presets,
		DataDir:           dataDir,
		AutoRestorePoint:  os.Getenv("WINFORGE_NO_RESTORE_POINT") == "",
	}, nil
}

// DefaultDataDir returns %LOCALAPPDATA%\WinForge on Windows and the user config
// directory elsewhere.
func DefaultDataDir() string {
	// Windows: %LOCALAPPDATA% (os.UserConfigDir returns Roaming AppData instead).
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "WinForge")
	}
	if base, err := os.UserConfigDir(); err == nil && base != "" {
		return filepath.Join(base, "WinForge")
	}
	return "."
}

// FindTweak looks up a tweak by id.
func (a *App) FindTweak(id string) (*config.Tweak, bool) {
	for i := range a.Tweaks {
		if a.Tweaks[i].ID == id {
			return &a.Tweaks[i], true
		}
	}
	return nil, false
}

// AppliedMap returns the set of tweak ids currently applied on the system.
func (a *App) AppliedMap() map[string]bool {
	m := make(map[string]bool, len(a.Tweaks))
	for i := range a.Tweaks {
		if a.Orchestrator.IsApplied(a.Tweaks[i]) {
			m[a.Tweaks[i].ID] = true
		}
	}
	return m
}

// Health computes the dashboard health score. bloatware is the number of
// detected bloatware apps (0 until the bloatware scanner lands in a later phase).
func (a *App) Health(bloatware int) tweak.Health {
	return tweak.ComputeHealth(a.Tweaks, a.AppliedMap(), bloatware)
}

// Apply applies (or dry-runs) a tweak by id. Before a real (non-dry-run)
// mutation it ensures a system restore point exists (safety-first policy).
func (a *App) Apply(id string, dryRun bool) (tweak.Result, error) {
	t, ok := a.FindTweak(id)
	if !ok {
		return tweak.Result{}, fmt.Errorf("tweak %q not found", id)
	}
	if !dryRun {
		a.EnsureRestorePoint("WinForge: apply " + id)
	}
	return a.Orchestrator.Apply(*t, dryRun), nil
}

// CreateRestorePoint creates a system restore point and records it in the
// audit log.
func (a *App) CreateRestorePoint(description string) (restorepoint.Info, error) {
	info, err := restorepoint.Create(description)
	if a.Logger != nil {
		e := audit.Entry{
			OperationType: "restore_point",
			Target:        "system",
			NewValue:      description,
			Success:       err == nil,
			CanUndo:       false,
		}
		if err != nil {
			e.ErrorMessage = err.Error()
		}
		_ = a.Logger.Append(e)
	}
	return info, err
}

// ListRestorePoints enumerates existing restore points via WMI (newest first).
func (a *App) ListRestorePoints() ([]restorepoint.Info, error) {
	return restorepoint.List()
}

// EnsureRestorePoint creates a restore point at most once per hour. It is
// best-effort: failure never blocks the underlying operation.
func (a *App) EnsureRestorePoint(description string) {
	if !a.AutoRestorePoint {
		return
	}
	a.mu.Lock()
	if time.Since(a.lastRestorePoint) < time.Hour {
		a.mu.Unlock()
		return
	}
	a.lastRestorePoint = time.Now()
	a.mu.Unlock()

	if !restorepoint.IsEnabled() {
		return
	}
	_, _ = a.CreateRestorePoint(description)
}

// Undo reverses a tweak by id.
func (a *App) Undo(id string) (tweak.Result, error) {
	t, ok := a.FindTweak(id)
	if !ok {
		return tweak.Result{}, fmt.Errorf("tweak %q not found", id)
	}
	return a.Orchestrator.Undo(*t), nil
}

// History returns all recorded audit entries, newest first.
func (a *App) History() ([]audit.Entry, error) {
	entries, err := a.Logger.ReadAll()
	if err != nil {
		return nil, err
	}
	// Reverse for newest-first presentation.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries, nil
}

// UndoEntry reverses a single recorded operation by id.
func (a *App) UndoEntry(id string) error {
	entries, err := a.Logger.ReadAll()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.ID == id {
			return a.Orchestrator.UndoEntry(e)
		}
	}
	return fmt.Errorf("operation %q not found", id)
}
