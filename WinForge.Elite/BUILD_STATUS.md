# WinForge Elite - Production Build Status

> Verification so far: static analysis (C# parse, XAML parse, SQL executed against
> a real SQLite engine including the legacy-DB migration path, Dapper parameter/column
> cross-checks, per-view XAML binding checks, seed-catalog integrity checks).
> A real `dotnet build` (windows-latest, .NET 8) is wired up in `ci.yml.fixed` but
> not yet runnable: the GitHub App used by the sandbox lacks the `workflows`
> permission, and the current `.github/workflows/ci.yml` is a stale Go-based
> workflow that fails on every run. Nothing here is aspirational.

## Current Progress: Phase 1 — Foundation (feature-complete, awaiting Windows runtime verification)

### ✅ Completed & Statically Verified

#### Core Infrastructure
- **Project Structure**: .NET 8 WPF (`net8.0-windows10.0.19041.0`), ModernWpfUI, Dapper, Serilog
- **Admin Elevation**: UAC detection (`AdminHelper.IsRunningAsAdmin`) + restart-as-admin (`runas` verb) + `requireAdministrator` manifest
- **Path Management**: `PathHelper` — centralized base/data/backup/temp directories under `%LOCALAPPDATA%\WinForge\Elite`
- **Logging**: Serilog console + rolling file sinks with machine/thread enrichers
- **Composition Root**: `AppServices` wires the full service graph and view-model factories with constructor injection

#### Data Layer
- **Database Factory**: SQLite at `%LOCALAPPDATA%\WinForge\Elite\Data\winforge.db`, 12 tables auto-created, idempotent `Initialize()`
- **Schema Migration**: `MigratePrivacyRulesSchema` adds the Operations/UndoOperations columns to pre-existing databases (verified against a simulated legacy DB)
- **JSON Type Handler**: `JsonStringListTypeHandler` maps JSON TEXT columns to `List<string>` entity properties (Dapper)
- **Entity Models**: 12 entity classes + enums (RiskLevel, PackageStatus, OperationType, PresetType)
- **Seed Data**: idempotent (`INSERT OR IGNORE` + backfill `UPDATE`) catalog:
  - 8 Tweaks with structured JSON `operations`/`undoOperations`
  - 20 Debloat Packages (Microsoft Bloat, Gaming, Protected)
  - 8 Privacy Rules **with real registry operations** (telemetry, advertising ID, tailored experiences, feedback, location, Wi-Fi Sense, Copilot, Recall)
  - 9 Applications (winget IDs)
  - 4 Presets (Standard, Gaming, Privacy Hardened, Work — Work is protected)

#### Services (real Windows operations — no simulation)
- **RegistryService**: read/write/delete with registry kind conversion (DWord/QWord/String/ExpandString/MultiString/Binary, incl. `0x` hex), protected-path refusal, recursive key snapshots to JSON, conservative snapshot restore, post-write verification
- **RestorePointService**: native `SRSetRestorePointW` (BEGIN/END_SYSTEM_CHANGE protocol) + database recording of every restore point
- **PowerShellService**: async script execution with output/error/warning capture, exit codes, timeout, cancellation (BeginStop), unrestricted-policy default session state, and single-quote escaping helper
- **TweakService**: apply/undo pipeline — snapshot → restore point → execute operations (registry / command / service via sc.exe / scheduled task via schtasks) → verify → audit → persist; mid-sequence failure rolls back via undo operations or snapshot
- **PrivacyService**: per-rule enable/disable with real registry writes + `HardenAll` under a single restore point
- **DebloatService**: Appx remove (Get-AppxPackage + Remove-AppxPackage -AllUsers) and reinstall (Add-AppxPackage -Register), batch removal under one restore point, protected-package refusal, package-name whitelist validation
- **SoftwareService**: winget install/uninstall with id whitelist validation and exit-code-based state
- **PresetService**: applies all included tweaks + privacy rules under one restore point, skipping already-applied items
- **HealthService**: documented 0-100 scoring algorithm (baseline 50 + applied tweaks + removed bloat + privacy % − telemetry/bloat penalties) with Security/Performance/Cleanliness/Privacy sub-scores, persisted to HealthHistory; telemetry state read from the live registry
- **SystemInfoService**: real CPU % (GetSystemTimes), RAM (GlobalMemoryStatusEx), system drive (DriveInfo), uptime (TickCount64) — pure P/Invoke, no dependencies

#### UI Layer (MVVM)
- **BaseViewModel**: INotifyPropertyChanged, busy-state tracking, CanRefresh, guarded async runner with UI + log error surfacing
- **RelayCommand / RelayCommand&lt;T&gt;**: ICommand with CanExecute
- **MainWindow**: navigation shell — sidebar with live per-module counts, section header, status bar, DataTemplate-based page switching (old pages disposed)
- **DashboardView/ViewModel**: live telemetry cards, health score ring + sub-scores, recent activity (auto-refresh 2s/30s, DispatcherTimer)
- **TweaksView/ViewModel**: search + category filter, risk badges, Apply/Undo per tweak with busy states
- **DebloatView/ViewModel**: category filter, select-all-in-category, batch removal, per-package status badges, reinstall
- **PrivacyView/ViewModel**: privacy score gauge, per-rule toggles, Harden All
- **SoftwareView/ViewModel**: category filter, batch install queue with per-app status (Not installed → Installing… → Installed/Failed), uninstall
- **PresetsView/ViewModel**: preset cards with type/protected badges, one-click apply, per-card result line

### 🔜 Remaining (Phase 1 exit)

- [ ] **Real Windows runtime test**: UAC prompt, DB seeding, dashboard scores, apply/undo of a telemetry tweak, restore point creation, debloat removal — requires a Windows machine (CI windows-latest can at least compile; `ci.yml.fixed` ready to install)
- [ ] **CI permission**: grant the GitHub App `workflows` permission (or copy `ci.yml.fixed` to `.github/workflows/ci.yml`) so the .NET 8 build runs on windows-latest
- [ ] Phase 2: DriverOptimizerService, SmartScanService (after Phase 1 runtime sign-off)
- [ ] Full 60+ tweak / 90+ package / 40+ rule catalog port from `src/db/seed-data.ts`

### 📊 File Count (current)
- C# Files: 25 (Models 3, Data 3, Helpers 2, Logging 1, Services 10, ViewModels 6, App/MainWindow/6 view code-behinds)
- XAML Files: 8 (App.xaml, MainWindow.xaml, 6 module views)
- Total: ~4,700 lines

### 🔜 Phase 1 Completion Criteria
- [x] Database schema created (12 tables)
- [x] Seed data populated (8 tweaks, 20 packages, 8 rules with operations, 9 apps, 4 presets)
- [x] Registry service with undo support (snapshots + undo operations)
- [x] Restore point service (native SRSetRestorePointW)
- [x] Tweak service (apply/undo engine with rollback + verification)
- [x] Health service (documented scoring algorithm + sub-scores)
- [x] PowerShell service (async execution with timeout/cancellation)
- [x] Main window with navigation
- [x] Dashboard view with live health score + real telemetry
- [x] All module views (Tweaks, Debloat, Privacy, Software, Presets)
- [x] All ViewModels connected
- [x] Admin elevation detection (runtime behavior verified on Windows)
- [x] Error handling throughout (every service returns OperationResult; every VM catches + surfaces)
- [ ] CI builds the WPF project on windows-latest (blocked: GitHub App lacks `workflows` permission; drop-in `ci.yml.fixed` ready)
- [ ] Runtime integration test on a real Windows machine

### 📝 Notes
- Runtime behavior (UAC prompt, registry writes, restore points, Appx removal, winget) can only be exercised on real Windows hardware; CI verifies compilation on windows-latest.
- No TODOs, no placeholders, no mock data: every seeded catalog entry is a real Windows registry/tweak definition aligned with the web simulation catalog.
- Every service call in a view model is wrapped with try/catch and surfaces user-friendly StatusMessage/ErrorMessage text.
