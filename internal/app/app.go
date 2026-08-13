// Package app is the composition root: it wires configuration, the engine,
// the orchestrator, the package manager, and the audit log into one object
// used by both the CLI and the HTTP dashboard.
package app

import (
	"fmt"
	"os"
	"path/filepath"

	"winforge/internal/appmanager"
	"winforge/internal/audit"
	"winforge/internal/config"
	"winforge/internal/engine"
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
	ProtectedServices []string
	DataDir           string
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

	logger := audit.NewLogger(filepath.Join(dataDir, "logs"))
	exec := engine.NewExecutor(protected)
	orch := tweak.NewOrchestrator(exec, logger)

	return &App{
		Loader:            loader,
		Orchestrator:      orch,
		Packages:          appmanager.New(),
		Logger:            logger,
		Tweaks:            tweaksCfg.Tweaks,
		Apps:              appsCfg.Applications,
		ProtectedServices: protected,
		DataDir:           dataDir,
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

// Apply applies (or dry-runs) a tweak by id.
func (a *App) Apply(id string, dryRun bool) (tweak.Result, error) {
	t, ok := a.FindTweak(id)
	if !ok {
		return tweak.Result{}, fmt.Errorf("tweak %q not found", id)
	}
	return a.Orchestrator.Apply(*t, dryRun), nil
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
