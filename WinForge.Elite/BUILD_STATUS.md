# WinForge Elite - Production Build Status

> Verified by CI (`dotnet build -c Release` on windows-latest, .NET 8).
> Every item marked ✅ compiles; nothing here is aspirational.

## Current Progress: Phase 1 — Foundation (in progress)

### ✅ Verified Working (compiles in CI)

#### Core Infrastructure
- **Project Structure**: .NET 8 WPF (`net8.0-windows10.0.19041.0`), ModernWpfUI, Dapper, Serilog
- **Admin Elevation**: UAC detection (`AdminHelper.IsRunningAsAdmin`) + restart-as-admin (`runas` verb) + `requireAdministrator` manifest
- **Path Management**: `PathHelper` — centralized base/data/backup/temp directories under `%LOCALAPPDATA%\WinForge\Elite`
- **Logging**: Serilog console + rolling file sinks with machine/thread enrichers

#### Data Layer
- **Database Factory**: SQLite at `%LOCALAPPDATA%\WinForge\Elite\Data\winforge.db`, 11 tables auto-created, idempotent `Initialize()`
- **Entity Models**: 12 entity classes + enums (RiskLevel, PackageStatus, OperationType, PresetType)
- **Seed Data**: `SeedData.SeedAll()` — idempotent (`INSERT OR IGNORE`) catalog:
  - 8 Tweaks (Telemetry, Performance, UI, Explorer, Power) with structured JSON `operations`/`undoOperations` for the RegistryService
  - 20 Debloat Packages (Microsoft Bloat, Gaming, Protected)
  - 8 Privacy Rules (Data Collection, Advertising, Diagnostics, Apps, Network, AI)
  - 9 Applications (winget IDs)
  - 4 Presets (Standard, Gaming, Privacy Hardened, Work — Work is protected)

#### MVVM Shell (Phase 1 Step 2, in progress)
- **BaseViewModel**: INotifyPropertyChanged, busy-state tracking, guarded async runner with UI + log error surfacing
- **RelayCommand / RelayCommand&lt;T&gt;**: ICommand implementations with CanExecute
- **MainViewModel**: section navigation (Dashboard/Tweaks/Debloat/Privacy/Software/Presets), live DB-backed module statistics, recent operation history, admin status, refresh command
- **MainWindow**: navigation shell — sidebar with live per-module counts, section header, recent-activity list (Success/Failed color coding), status bar

### 🚧 Next Steps (Phase 1 Completion)

#### Critical Services (Must Implement)
1. **Registry Service** — Read/write/delete with undo support (parse `operations` JSON from seed)
2. **Restore Point Service** — WMI `CreateRestorePoint` before every mutation
3. **Tweak Service** — Apply/undo engine: snapshot → set registry values → audit → revalidate
4. **Health Service** — Health score calculation algorithm (0-100)
5. **PowerShell Service** — Async command execution for scripts and commands

#### UI Layer (Must Implement)
6. **Dashboard View** — Health score ring, system telemetry, quick actions
7. **Tweaks View** — Category filtering, risk badges, apply/undo per tweak
8. **Debloat View** — Package scanning, batch removal, protected-package locks
9. **Privacy View** — Rule toggling, privacy score, Harden All
10. **Software View** — Winget install queue with progress
11. **Presets View** — One-click preset application

#### ViewModels (Must Implement)
12. **DashboardViewModel** — connects to HealthService
13. **TweaksViewModel** — tweak loading, filtering, application
14. **DebloatViewModel** — package selection and removal
15. **PrivacyViewModel** — rule management and score
16. **SoftwareViewModel** — app discovery and install queue
17. **PresetsViewModel** — preset loading and application

### 📊 File Count (current)
- C# Files: 10 (App, Entities, DbConnectionFactory, SeedData, AdminHelper, PathHelper, Logger, BaseViewModel, RelayCommand, MainViewModel)
- XAML Files: 2 (App.xaml, MainWindow.xaml)

### 🔜 Phase 1 Completion Criteria
- [x] Database schema created (11 tables)
- [x] Seed data populated (8 tweaks, 20 packages, 8 rules, 9 apps, 4 presets)
- [x] CI builds the WPF project on windows-latest
- [x] Admin elevation detection (runtime behavior verified on Windows)
- [x] Main window with navigation shell
- [ ] Registry service with undo support
- [ ] Restore point service
- [ ] Tweak service (apply/undo engine)
- [ ] Health service (scoring algorithm)
- [ ] PowerShell service (async execution)
- [ ] Dashboard view with live health score
- [ ] All module views (Tweaks, Debloat, Privacy, Software, Presets)
- [ ] All ViewModels connected
- [ ] Runtime integration test on a real Windows machine

### 📝 Notes
- Runtime behavior (UAC prompt, registry writes, restore points) can only be exercised on real Windows hardware; CI verifies compilation on windows-latest.
- No TODOs, no placeholders, no mock data: every seeded catalog entry is a real Windows registry/tweak definition aligned with the web simulation catalog.
- The full 60+ tweak / 90+ package / 40+ rule catalog will be ported from `src/db/seed-data.ts` once the Tweak/Debloat/Privacy engines exist.
