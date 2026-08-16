// Package app is the composition root: it wires configuration, the engine,
// the orchestrator, the package manager, and the audit log into one object
// used by both the CLI and the HTTP dashboard.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"winforge/internal/appmanager"
	"winforge/internal/audit"
	"winforge/internal/bloatware"
	"winforge/internal/config"
	"winforge/internal/engine"
	"winforge/internal/platform"
	"winforge/internal/plugin"
	"winforge/internal/restorepoint"
	"winforge/internal/scheduler"
	"winforge/internal/tweak"
)

// Version is the application version, stamped at build time via -ldflags.
var Version = "0.1.0-dev"

// MaintenanceTaskName is the Task Scheduler task name used for the weekly
// scheduled maintenance pass.
const MaintenanceTaskName = "WinForge Maintenance"

// MaintenanceSummary reports the outcome of a scheduled maintenance run.
type MaintenanceSummary struct {
	TweaksApplied []string  `json:"tweaksApplied"`
	TweakErrors   []string  `json:"tweakErrors,omitempty"`
	AppsUpgraded  bool      `json:"appsUpgraded"`
	AppsSkipped   bool      `json:"appsSkipped"`
	AppError      string    `json:"appError,omitempty"`
	AuditError    string    `json:"auditError,omitempty"`
	RanAt         time.Time `json:"ranAt"`
}

// App bundles all runtime dependencies.
type App struct {
	Loader            *config.Loader
	Orchestrator      *tweak.Orchestrator
	packages          *appmanager.Manager
	Logger            *audit.Logger
	Tweaks            []config.Tweak
	Apps              []config.App
	Plugins           []plugin.Plugin
	ProtectedServices []string
	DnsPresets        []config.DnsEntry
	DataDir           string
	elevated          bool

	// AutoRestorePoint enables the safety-first policy: a system restore point
	// is created (best-effort, throttled) before the first mutation.
	AutoRestorePoint bool

	// mutationMu serializes every application-level system mutation, including
	// scheduler and CLI calls that do not pass through the HTTP server.
	mutationMu sync.Mutex

	mu               sync.Mutex
	lastRestorePoint time.Time

	// Restore-point seams keep throttle/error behavior testable without invoking
	// native Windows APIs. Production instances leave them nil and use the real
	// implementation and wall clock.
	restorePointCreator   func(string) (restorepoint.Info, error)
	restorePointAvailable func() bool
	clock                 func() time.Time

	// packageInstaller is the equivalent test seam for the external WinGet
	// process. Production instances dispatch through the private manager.
	packageInstaller func(context.Context, string, func(appmanager.Progress)) (*appmanager.Result, error)

	bloatwareOnce sync.Once
	bloatwareList []string
}

// New builds the application, reading embedded defaults plus standard-user
// overrides and preparing the standard-user data directory for logs.
func New(dataDir string) (*App, error) {
	return newApp(dataDir, platform.IsElevated())
}

func newApp(dataDir string, elevated bool) (*App, error) {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	// Files in the user's profile are writable without elevation. Never consume
	// those override/plugin definitions in an elevated process, where doing so
	// would turn a planted command or target into an administrator mutation.
	// The embedded catalogs remain available for explicit elevated maintenance.
	overrideDir := filepath.Join(dataDir, "config")
	if elevated {
		overrideDir = ""
	}
	loader := config.NewLoader(overrideDir)

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

	// Elevated processes also avoid all reads, appends, and retention deletes in
	// the user-controlled audit directory. File-level symlink checks cannot make
	// a writable parent directory a safe administrator trust boundary (junctions,
	// hard links, and directory replacement remain possible). History is thus an
	// explicitly standard-user feature. The same principle disables WinGet PATH
	// discovery while elevated; package management does not require WinForge's
	// administrator token and can be run after restarting normally.
	var logger *audit.Logger
	var packages *appmanager.Manager
	if !elevated {
		if err := os.MkdirAll(filepath.Join(dataDir, "logs"), 0o700); err != nil {
			return nil, err
		}
		logger = audit.NewLogger(filepath.Join(dataDir, "logs"))
		packages = appmanager.New()
	}
	exec := engine.NewExecutorWithElevation(protected, elevated)
	orch := tweak.NewOrchestrator(exec, logger)

	// Load plugins from <dataDir>/plugins only at standard privilege and merge
	// their tweaks after the built-in/user-override tweaks. Embedded definitions
	// win on id collisions. Elevated processes deliberately ignore this
	// user-writable extension point (see the security boundary above).
	//
	// Lua and WASM packs are supported only when their respective DLLs are
	// bundled next to the executable (the shipping location) or in the data
	// directory (a developer override). Each DLL is loaded by absolute path,
	// never the DLL search path. WASM is the strong-isolation tier; see
	// docs/WASM_REALSCOPE_2026-08-16.md (platform-independent validation now
	// lands, Windows binding deferred until BLK-6 hardware is available).
	var plugins []plugin.Plugin
	if !elevated {
		var dllDirs []string
		if exePath, exeErr := os.Executable(); exeErr == nil {
			if exeDir, absErr := filepath.Abs(filepath.Dir(exePath)); absErr == nil {
				dllDirs = append(dllDirs, exeDir)
			}
		}
		if dataAbs, absErr := filepath.Abs(dataDir); absErr == nil {
			dllDirs = append(dllDirs, dataAbs)
		}
		plugins, err = plugin.DiscoverWithOptions(filepath.Join(dataDir, "plugins"), plugin.Options{
			LuaDLLDirs:  dllDirs,
			WasmDLLDirs: dllDirs,
		})
		if err != nil {
			return nil, err
		}
	}
	tweaks := tweaksCfg.Tweaks
	for _, p := range plugins {
		tweaks = plugin.MergeTweaks(tweaks, p.Tweaks)
	}

	return &App{
		Loader:            loader,
		Orchestrator:      orch,
		packages:          packages,
		Logger:            logger,
		Tweaks:            tweaks,
		Apps:              appsCfg.Applications,
		Plugins:           plugins,
		ProtectedServices: protected,
		DnsPresets:        dnsCfg.Presets,
		DataDir:           dataDir,
		elevated:          elevated,
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

// Elevated reports the security mode captured when the application was built.
// It is intentionally stable for the lifetime of App so status surfaces agree
// with the trust decisions made while loading configuration.
func (a *App) Elevated() bool { return a.elevated }

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

// Health computes the dashboard health score from targets whose state can be
// verified. One-way commands and runtime actions are excluded rather than
// permanently penalizing a system for state WinForge cannot observe.
func (a *App) Health(bloatware int) tweak.Health {
	verifiable := make([]config.Tweak, 0, len(a.Tweaks))
	for _, t := range a.Tweaks {
		if tweak.CanVerify(t) {
			verifiable = append(verifiable, t)
		}
	}
	h := tweak.ComputeHealth(verifiable, a.AppliedMap(), bloatware)
	h.UnverifiableTweaks = len(a.Tweaks) - len(verifiable)
	return h
}

// Bloatware returns the display names of installed applications that WinForge
// recognizes as bloatware. The registry scan runs once per process and is
// memoized.
func (a *App) Bloatware() []string {
	a.bloatwareOnce.Do(func() {
		a.bloatwareList = bloatware.Detect(bloatware.Installed())
	})
	// Do not expose the memoized backing array: callers may sort or otherwise
	// modify their response without corrupting subsequent dashboard reads.
	return append([]string(nil), a.bloatwareList...)
}

// BloatwareCount returns the number of detected bloatware apps.
func (a *App) BloatwareCount() int { return len(a.Bloatware()) }

// Apply applies (or dry-runs) a tweak by id. Before a real (non-dry-run)
// mutation it ensures a system restore point exists (safety-first policy).
func (a *App) Apply(id string, dryRun bool) (tweak.Result, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()

	t, ok := a.FindTweak(id)
	if !ok {
		return tweak.Result{}, fmt.Errorf("tweak %q not found", id)
	}
	if !dryRun {
		a.EnsureRestorePoint("WinForge: apply " + id)
	}
	return a.Orchestrator.Apply(*t, dryRun), nil
}

func (a *App) now() time.Time {
	if a.clock != nil {
		return a.clock()
	}
	return time.Now()
}

func (a *App) restorePointsEnabled() bool {
	if a.restorePointAvailable != nil {
		return a.restorePointAvailable()
	}
	return restorepoint.IsEnabled()
}

// createRestorePoint creates and audits a restore point. The creation error is
// returned separately so callers can update throttling even if only the audit
// write failed. Caller must hold a.mu.
func (a *App) createRestorePoint(description string) (restorepoint.Info, error, error) {
	creator := a.restorePointCreator
	if creator == nil {
		creator = restorepoint.Create
	}
	info, createErr := creator(description)
	var auditErr error
	if a.Logger != nil {
		e := audit.Entry{
			OperationType: "restore_point",
			Target:        "system",
			NewValue:      description,
			Success:       createErr == nil,
			CanUndo:       false,
		}
		if createErr != nil {
			e.ErrorMessage = createErr.Error()
		}
		if err := a.Logger.Append(e); err != nil {
			auditErr = fmt.Errorf("record restore point: %w", err)
		}
	}
	return info, createErr, auditErr
}

// CreateRestorePoint creates a system restore point and records it in the
// audit log.
func (a *App) CreateRestorePoint(description string) (restorepoint.Info, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()

	a.mu.Lock()
	defer a.mu.Unlock()

	info, createErr, auditErr := a.createRestorePoint(description)
	if createErr == nil {
		a.lastRestorePoint = a.now()
	}
	return info, errors.Join(createErr, auditErr)
}

// ListRestorePoints enumerates existing restore points via WMI (newest first).
func (a *App) ListRestorePoints() ([]restorepoint.Info, error) {
	return restorepoint.List()
}

// EnsureRestorePoint creates a restore point at most once per hour. It is
// best-effort: failure never blocks the underlying operation.
func (a *App) EnsureRestorePoint(description string) {
	if !a.AutoRestorePoint || !a.restorePointsEnabled() {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	if !a.lastRestorePoint.IsZero() {
		age := now.Sub(a.lastRestorePoint)
		if age >= 0 && age < time.Hour {
			return
		}
	}
	_, createErr, _ := a.createRestorePoint(description)
	if createErr == nil {
		// Only successful creation consumes the throttle window; unsupported or
		// transient failures can be retried by the next mutation. Audit failures
		// do not trigger repeated creation of otherwise successful restore points.
		a.lastRestorePoint = now
	}
}

// SearchPackages searches the WinGet catalog without exposing the package
// manager's mutation methods to application front ends.
func (a *App) SearchPackages(ctx context.Context, query string) ([]string, error) {
	if a.elevated {
		return nil, errors.New("package management is disabled while WinForge is elevated; restart normally to use WinGet")
	}
	if a.packages == nil {
		return nil, errors.New("package manager unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return a.packages.Search(ctx, query)
}

// InstallPackage installs an exact WinGet package identity after requesting a
// best-effort restore point, then records the completed attempt in the audit
// log. Validation happens before any safety or mutation side effect.
func (a *App) InstallPackage(ctx context.Context, packageID string, progress func(appmanager.Progress)) (*appmanager.Result, error) {
	if a.elevated {
		err := errors.New("package management is disabled while WinForge is elevated; restart normally to use WinGet")
		return &appmanager.Result{Error: err}, err
	}
	if err := appmanager.ValidatePackageID(packageID); err != nil {
		return &appmanager.Result{Error: err}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	install := a.packageInstaller
	if install == nil {
		if a.packages == nil {
			err := errors.New("package manager unavailable")
			return &appmanager.Result{Error: err}, err
		}
		install = a.packages.Install
	}

	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()

	a.EnsureRestorePoint("WinForge: install package " + packageID)
	result, installErr := install(ctx, packageID, progress)
	if result == nil {
		if installErr == nil {
			installErr = errors.New("package installer returned no result")
		}
		result = &appmanager.Result{Error: installErr}
	}
	switch {
	case installErr == nil && result.Error != nil:
		installErr = result.Error
	case installErr != nil && result.Error == nil:
		result.Error = installErr
	case installErr != nil && result.Error != nil:
		// The process boundary and the result parser can independently explain a
		// failed install. Preserve both explanations unless one already wraps the
		// other, and keep Result.Error consistent with the returned install error.
		switch {
		case errors.Is(result.Error, installErr):
			installErr = result.Error
		case errors.Is(installErr, result.Error):
			// installErr already carries the result error.
		default:
			installErr = errors.Join(installErr, result.Error)
		}
		result.Error = installErr
	}
	if installErr == nil && !result.Success {
		installErr = errors.New("package installation did not report success")
		result.Error = installErr
	}

	var auditErr error
	if a.Logger != nil {
		entry := audit.Entry{
			OperationType: "package_install",
			Target:        packageID,
			NewValue:      "installed",
			Success:       installErr == nil && result.Success,
			CanUndo:       false,
		}
		if installErr != nil {
			entry.ErrorMessage = installErr.Error()
		}
		if err := a.Logger.Append(entry); err != nil {
			auditErr = fmt.Errorf("record package installation: %w", err)
		}
	}
	return result, errors.Join(installErr, auditErr)
}

// Undo reverses a tweak by id.
func (a *App) Undo(id string) (tweak.Result, error) {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()

	t, ok := a.FindTweak(id)
	if !ok {
		return tweak.Result{}, fmt.Errorf("tweak %q not found", id)
	}
	if t.Reversible {
		a.EnsureRestorePoint("WinForge: undo " + id)
	}
	return a.Orchestrator.Undo(*t), nil
}

// History returns all recorded audit entries, newest first.
func (a *App) History() ([]audit.Entry, error) {
	if a.Logger == nil {
		return []audit.Entry{}, nil
	}
	entries, readErr := a.Logger.ReadAll()
	undone := make(map[string]struct{})
	for _, e := range entries {
		if e.Success && e.UndoOf != "" {
			undone[e.UndoOf] = struct{}{}
		}
	}
	for i := range entries {
		if _, ok := undone[entries[i].ID]; ok {
			entries[i].CanUndo = false
		}
	}
	// Reverse for newest-first presentation.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	// Preserve valid entries for callers that can present partial history, but
	// still surface corruption so undo paths never make decisions from an
	// incomplete audit trail.
	return entries, readErr
}

// UndoEntry reverses a single recorded operation by id.
func (a *App) UndoEntry(id string) error {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()

	// Audit logs live in the user's profile and therefore cannot authorize an
	// administrator-level mutation: an unelevated process could rewrite a
	// snapshot before the user launches WinForge as administrator. Elevated
	// callers can still use catalog-backed tweak undo, whose targets and values
	// come from the embedded, validated configuration.
	if a.elevated {
		return errors.New("per-operation history undo is disabled while WinForge is elevated; use the tweak's validated Undo action instead")
	}
	if a.Logger == nil {
		return errors.New("audit history is unavailable")
	}

	entries, err := a.Logger.ReadAll()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Success && e.UndoOf == id {
			// Reject a consumed history row before creating another restore point.
			// The orchestrator performs the same check at its mutation boundary,
			// but the application owns this earlier safety side effect.
			return fmt.Errorf("operation %s has already been undone", id)
		}
	}
	var target *audit.Entry
	for i := range entries {
		if entries[i].ID != id {
			continue
		}
		if target != nil {
			return fmt.Errorf("operation id %q is ambiguous in the audit history", id)
		}
		target = &entries[i]
	}
	if target == nil {
		return fmt.Errorf("operation %q not found", id)
	}
	if target.Success && target.CanUndo {
		a.EnsureRestorePoint("WinForge: undo operation " + id)
	}
	return a.Orchestrator.UndoEntry(*target)
}

// RunMaintenance performs a full maintenance pass: it re-applies any tweak that
// is not in its target state and upgrades outdated winget apps. Progress lines
// are emitted to log (may be nil). Failures are captured in the returned
// summary rather than aborting the pass, and the pass is recorded in the audit
// log.
func (a *App) RunMaintenance(ctx context.Context, log func(string)) MaintenanceSummary {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	sum := MaintenanceSummary{RanAt: time.Now()}
	say := func(s string) {
		if log != nil {
			log(s)
		}
	}

	// Safety-first: best-effort restore point (throttled to 1/hr) before any
	// mutation the pass may perform.
	a.EnsureRestorePoint("WinForge: run-maintenance")

	say("Verifying tweak states…")
	applied, errs := a.Orchestrator.EnsureApplied(a.Tweaks)
	sum.TweaksApplied = applied
	for _, e := range errs {
		sum.TweakErrors = append(sum.TweakErrors, e.Error())
	}
	if len(applied) > 0 {
		say(fmt.Sprintf("Applied %d tweak(s).", len(applied)))
	}

	say("Checking for app updates…")
	if a.elevated {
		sum.AppsSkipped = true
		say("WinGet is disabled while WinForge is elevated; skipping app updates.")
	} else if a.packages == nil {
		sum.AppsSkipped = true
		say("app manager unavailable; skipping app updates.")
	} else {
		res, err := a.packages.UpgradeAll(ctx, func(p appmanager.Progress) {
			if p.Line != "" {
				say(p.Line)
			}
		})
		switch {
		case errors.Is(err, appmanager.ErrWingetMissing):
			sum.AppsSkipped = true
			say("winget not found; skipping app updates.")
		case err != nil:
			sum.AppError = err.Error()
			say("App update check failed: " + err.Error())
		default:
			sum.AppsUpgraded = res.Success
			say("App update check complete.")
		}
	}

	ok := len(errs) == 0 && sum.AppError == ""
	if a.Logger != nil {
		if err := a.Logger.Append(audit.Entry{
			OperationType: "maintenance",
			Target:        "system",
			Success:       ok,
			NewValue:      fmt.Sprintf("%d tweaks applied; appsUpgraded=%v", len(applied), sum.AppsUpgraded),
			CanUndo:       false,
		}); err != nil {
			sum.AuditError = err.Error()
			say("Recording maintenance result failed: " + err.Error())
		}
	}
	return sum
}

// ScheduleMaintenance registers the weekly Task Scheduler task that runs
// "<winforge.exe> run-maintenance".
func (a *App) ScheduleMaintenance() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()

	a.EnsureRestorePoint("WinForge: schedule maintenance")
	return scheduler.Register(MaintenanceTaskName, exe)
}

// UnscheduleMaintenance removes the weekly maintenance task.
func (a *App) UnscheduleMaintenance() error {
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()

	a.EnsureRestorePoint("WinForge: unschedule maintenance")
	return scheduler.Delete(MaintenanceTaskName)
}
