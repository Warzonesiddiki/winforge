# WinForge Elite — Control Center

> The definitive all-in-one Windows optimization, debloat, privacy hardening, repair, and power-user configuration suite.

## The Native Engine (Go) 🆕

The **native Windows utility** now lives in this repository as a self-contained Go
program (`cmd/winforge`, `internal/*`, `config/*`, `web/*`): 15,800 lines,
**stdlib-only** (zero third-party modules), builds a **6.24 MB static `winforge.exe`**.

- Full CLI: `winforge apply --id <tweak>`, `undo`, `scan`, `list`, `history`,
  `restore-point`, `install --id <winget-id>`, `build-iso`, `updates`, `set-dns`,
  `enable-feature`, `run-maintenance`, `plugins`, plus a **dashboard server**
  (`winforge serve`) with a bundled web UI on `localhost:8696`.
- Hardened native Win32: registry via raw syscalls with bounded reads, system
  restore points, Appx debloat engine, winget app manager, DISM/ISO builder,
  Windows Update, DNS, audit + undo, plugin system, weekly maintenance scheduler.
- Verified in CI-less sandboxes too: **builds, vets, and tests (18/18 packages,
  incl. race detector) on Linux, and cross-compiles to Windows** — see
  [docs/GO_TOOLCHAIN_BOOTSTRAP.md](docs/GO_TOOLCHAIN_BOOTSTRAP.md).
- Language strategy: Go-primary 10-language hybrid —
  [docs/LANGUAGE_SELECTION.md](docs/LANGUAGE_SELECTION.md);
  engine README: [docs/GO_ENGINE_README.md](docs/GO_ENGINE_README.md);
  open blockers: [docs/BLOCKED_ITEMS.md](docs/BLOCKED_ITEMS.md).

## The Web Control Center (Next.js)

This repo also hosts a fullstack Next.js web application that models the complete
WinForge Elite specification. Since this environment runs on Linux rather than
Windows, all system operations are **safely simulated** against a PostgreSQL
database, with full audit logging and reversibility — exactly mirroring the safety
principles of the native app.

## Live Demo & Vision

- **Current state**: Web simulation running on Linux (functional, demo-ready)
- **Goal**: Migrate to native WPF/.NET 8 Windows utility (self-contained EXE with real Registry/WMI/Appx/DISM operations)
- **Inspiration**: Chris Titus Tech's Windows Utility (30M+ runs, 6 years, 200+ contributors)
- **Distribution model**: Signed EXE (`irm domain.com/win | iex`) or Inno Setup installer

## Features Dashboard

### Dashboard
- Live system health score (0-100) with color-coded gauge
- Real-time CPU/RAM/Disk/Network sparkline telemetry
- Quick wins recommendations based on scan
- One-click preset buttons (Standard, Gaming, Privacy, Work)
- System information panel (simulated Windows 11 info)
- **Quick System Scan** — live scan using real database state with deep links to fix modules

### Debloat
- 90+ bloatware packages across 8 categories
- Catalog aligned with **AtlasOS** and **Chris Titus Tech (CTT)** debloat lists
- Per-package remove/reinstall with protected package locking
- Bulk selection and batch removal
- Startup manager with enable/disable toggles
- Real-time status tracking

### Tweaks
- 60+ granular system tweaks across 9 categories
- Registry operations sourced from **AtlasOS**, **ReviOS**, and **CTT** tweak catalogs
- Three-panel layout with category filtering
- Risk-level badges (Low/Medium/High/Expert)
- Dry-run preview mode showing exact operations + undo operations
- "May affect" warnings listing broken features (e.g., disabling WSearch breaks search)
- Export/Import custom `.winforge` preset files
- Live toggle state with undo buttons

### Privacy Hardening
- 40+ privacy rules across 7 categories
- Privacy score gauge (0-100)
- One-click "Harden All" with confirmation
- Exportable HTML privacy audit report
- Includes AI-era rules: Copilot disable, Recall disable (aligned with ReviOS privacy posture)

### Software Installer
- 60+ curated applications across 7 categories
- Batch install queue with progress tracking
- Live installation log console
- Installed detection with status badges

### System Repair
- **Full System Check** — sequential SFC + DISM Scan + DISM Restore + CHKDSK pass
- SFC /scannow (System File Checker)
- Windows Update reset (stop services, rename SoftwareDistribution)
- DISM Scan/Restore Health
- DNS flush and network stack reset
- Real temp file size calculation with cleanup
- DNS preset configuration (Cloudflare, Google, Quad9, AdGuard, NextDNS, etc.)
- Windows Features manager (enable/disable optional features)

### Windows Updates
- Available/History/Hidden tabs
- Selective install with severity indicators
- Hide unwanted updates
- Pause/Resume update controls

### ISO Builder
- MicroWin-style custom image configuration
- Bloatware removal options
- Privacy tweaks injection
- Edge/OneDrive/Recall removal options
- TPM/Secure Boot bypass option
- SHA-256 checksum display

### History & Undo
- Full audit trail of all operations
- Per-operation undo buttons
- Bulk "Undo All Today" feature
- Export to CSV
- Filter by category and status

### Settings
- Theme: Light/Dark/System
- Backdrop: Mica/Acrylic/None
- Language selector (5 languages)
- Safety toggles (restore point, expert tweaks)
- Restore point history viewer

## Technology Stack

- **Framework**: Next.js 16 (App Router, Server Components, Server Actions)
- **Database**: PostgreSQL via Drizzle ORM
- **Styling**: Tailwind CSS 4
- **Runtime**: Node.js (real CPU/memory/disk metrics via `os` module)

## Safety Principles

1. **Restore Point Before Mutation**: Every system change creates a restore point first (configurable)
2. **Full Undo Support**: Every operation logs its undo payload for complete reversibility
3. **Risk Classification**: All tweaks have risk levels (Low/Medium/High/Expert)
4. **Protected Resources**: Critical system components are locked and cannot be modified
5. **Audit Trail**: Every operation is logged with timestamp, change details, and undo data

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/health` | Health check |
| `GET /api/metrics` | Live system telemetry (real data) |
| `GET /api/privacy/audit` | HTML privacy audit report |
| `GET /api/history/export` | CSV operation history export |
| `GET /api/cli` | CLI documentation |

## Database Schema (11 Tables)

| Table | Purpose |
|-------|---------|
| `tweaks` | 60+ system tweaks with operations & undo operations |
| `debloat_packages` | 90+ bloatware packages (AtlasOS/CTT-aligned) |
| `privacy_rules` | 40+ privacy rules |
| `applications` | 64 installable apps |
| `presets` | 4 preset profiles |
| `startup_items` | 10 startup programs |
| `windows_updates` | 10 simulated updates |
| `operation_history` | Full audit log with undo data |
| `restore_points` | Restore point records |
| `iso_jobs` | ISO build jobs |
| `app_settings` | Application settings (theme, language, safety toggles) |

## Quick Start

```bash
# Install dependencies
npm install

# Push database schema (if PostgreSQL is available)
npx drizzle-kit push

# Run development server
npm run dev

# Build for production
npm run build

# Lint code
npm run lint

# Typecheck
npm run typecheck
```

## Development

### Adding New Catalog Entries

**Tweaks**: Edit `src/db/seed-data.ts` → add a new `t("id", ...)` entry → run `npx drizzle-kit push`

**Debloat Packages**: Edit `src/db/seed-data.ts` → add to `debloatRaw` array → run seed

**Privacy Rules**: Edit `src/db/seed-data.ts` → add to `privacySeed` array → run seed

**Presets**: Edit `src/db/seed-data.ts` → update `presetsSeed` arrays → run seed

### Health Score Algorithm

The algorithm in `src/lib/health.ts` balances penalties (bloatware, unapplied tweaks) with bonuses (applied tweaks, removed bloat, privacy hardening) to give a fair score:

1. Start at 50 (neutral baseline)
2. **+ Bonuses** (max +50):
   - Applied tweaks: `min(20, appliedTweaks.length * 2)` — up to +20
   - Removed bloat: `min(15, removedBloatCount * 0.5)` — up to +15
   - Privacy score: `round(privacyScore * 0.15)` — up to +15
3. **- Penalties** (max -50):
   - Security updates: `min(10, pendingSecurityUpdates * 5)` — max -10
   - Optional updates: `min(5, pendingOptionalUpdates * 1)` — max -5
   - Telemetry enabled: `-5` if still enabled
   - Heavy bloatware: `-5` if >50 packages, `-3` if >30 packages
4. **+ Bonus**: +5 if all default-enabled tweaks are applied
5. **Clamp**: `max(0, min(100, round(score)))`

### Catalog Data Highlights

- **60+ Tweaks**: Registry operations with `operations[]` + `undoOperations[]`, `risk` levels, `breaksFeatures[]` warnings, `tags[]`
- **90+ Debloat Packages**: Microsoft bloat, OEM apps, advertising, advertising, gaming, social, widgets, AI/Copilot, protected (never removable)
- **40+ Privacy Rules**: Data collection, app permissions, advertising, Microsoft account, browser privacy, network privacy, extended (AtlasOS/CTT-aligned)
- **4 Presets**: Standard (safe optimizations), Gaming (performance-focused), Privacy Hardened (aggressive lockdown), Work/Corporate (conservative changes)

### Project Structure

```
winforge/
├── src/
│   ├── app/              # Next.js App Router pages
│   ├── components/       # React UI components (40+ components)
│   ├── db/               # Drizzle ORM schema + seed data
│   │   ├── schema.ts     # 11 database tables (tweaks, debloat, privacy, etc.)
│   │   ├── seed-data.ts  # Catalog: 60 tweaks, 90 packages, 40 rules
│   │   ├── seed.ts       # Idempotent seeding (INSERT ... ON CONFLICT DO NOTHING)
│   │   └── dns-presets.ts # DNS configuration presets
│   ├── lib/              # Business logic
│   │   ├── health.ts     # Health score algorithm (0-100)
│   │   └── actions.ts    # All Server Actions (tweaks, debloat, privacy, undo, ISO, repair, etc.)
│   └── db/               # Database types + connections
├── public/               # Static assets
├── postcss.config.mjs    # Tailwind CSS 4 config
├── next.config.ts        # Next.js config (minimal, default)
├── eslint.config.mjs     # ESLint 9 config
├── tsconfig.json         # TypeScript 5.9 config
├── drizzle.config.json   # Drizzle ORM config
├── package.json          # Dependencies (Next.js 16, React 19, Tailwind 4, Drizzle ORM, pg)
└── README.md             # This file
```

## Handover for Another AI

### Project Context

This project began as a **Next.js simulation** of a Windows optimization suite (WinForge Elite). The catalog data (60+ tweaks, 90+ debloat packages, 40+ privacy rules) was informed by real-world projects: **AtlasOS**, **ReviOS**, and **Chris Titus Tech Windows Utility**. The safety model (restore points before mutation, full undo payloads, risk classification) is the core differentiator.

The **goal** is to migrate from the web simulation to a **native Windows WPF/.NET 8 executable** that directly manipulates Windows (registry, services, appx packages, DISM, Windows Update COM, ISO building). The current app runs on Linux and simulates all operations against PostgreSQL; the target runs on Windows and calls real APIs.

### Key Files for Continuing Work

| File | Purpose | Lines | Priority |
|------|---------|-------|----------|
| `src/db/schema.ts` | 11 database table definitions | 249 | **Critical** — defines all catalog data and audit structure |
| `src/db/seed-data.ts` | 60 tweaks + 90 packages + 40 privacy rules + 4 presets + 64 apps | 936 | **Critical** — source of truth for all catalog data |
| `src/lib/health.ts` | Health score algorithm (0-100) | 153 | **High** — port to C# for native version |
| `src/lib/actions.ts` | All Server Actions (20+ functions) | 1108 | **High** — model as C# services for native version |
| `src/db/seed.ts` | Idempotent seeding logic | 129 | **Medium** — adapt for SQLite in native version |
| `src/app/dashboard/page.tsx` | Dashboard UI with health panel + quick wins + presets | 103 | **Medium** — refactor to WPF XAML |
| `src/components/HealthPanel.tsx` | Health gauge + bloat + applied/tweaks display | ~80 | **Medium** |
| `src/components/PresetButtons.tsx` | 4 preset buttons (Standard/Gaming/Privacy/Work) | ~40 | **Low** |

### Adding a New Tweak

1. Edit `src/db/seed-data.ts` → add entry to `tweaksSeed` array:
   ```typescript
   t("new-tweak-id", "Name", "Description", "Category", "risk", true, ["tags"], 
     ["registry\\path\\operation"], ["registry\\path\\undo-operation"])
   ```
2. Run `npx drizzle-kit push` (if using PostgreSQL) or manually update SQLite
3. The tweak auto-appears in the UI via the preset system

### Preset System

4 presets defined in `src/db/seed-data.ts` `presetsSeed`:

- **Standard**: `tweaksSeed.filter(x => x.defaultEnabled)` + low-risk debloat (first 20) + default-enabled privacy rules
- **Gaming**: Performance/Gaming/Power tweaks (excluding expert) + advertising/widgets debloat (first 15) + no privacy rules
- **Privacy Hardened**: Telemetry/privacy tweaks (excluding expert) + advertising/AI/Copilot/widgets debloat + high/expert-risk privacy excluded
- **Work/Corporate**: Low-risk tweaks only (Telemetry/Explorer/UI categories) + Microsoft Bloat/Advertising/Social debloat (low risk) + Data Collection/Advertising privacy rules

Each preset has: `id`, `name`, `description`, `tweakIds[]`, `debloatPackages[]`, `privacyRuleIds[]`.

### Health Score Porting

The C# version in the native app should replicate `computeHealthReport()` from `src/lib/health.ts`. Key formulas:

```csharp
// Baseline
int score = 50;

// Tweak bonus: up to +20
int tweakBonus = Math.Min(20, appliedTweaks.Count * 2);

// Debloat bonus: up to +15  
int debloatBonus = Math.Min(15, removedBloatCount * 5 / 10); // 0.5 per package → integer math

// Privacy bonus: up to +15
int enabledPrivacy = privacyRules.Count(p => p.Enabled);
int privacyScore = privacyRules.Count == 0 ? 100 : (int)Math.Round((double)enabledPrivacy / privacyRules.Count * 100);
int privacyBonus = (int)Math.Round(privacyScore * 0.15);

// Apply bonuses
score += tweakBonus + debloatBonus + privacyBonus;

// Penalties
int pendingSecurity = updates.Where(u => u.Severity == "Critical" || u.Severity == "Important").Count();
int pendingOptional = updates.Where(u => u.Severity == "Optional" || u.Severity == "Feature").Count();
score -= Math.Min(10, pendingSecurity * 5);      // max -10
score -= Math.Min(5, pendingOptional * 1);      // max -5
score -= telemetryEnabled ? 5 : 0;              // -5 if telemetry enabled

// Heavy bloatware penalty
if (bloatwareCount > 50) score -= 5;
else if (bloatwareCount > 30) score -= 3;

// Default-enabled tweak bonus
int defaultApplied = allTweaks.Count(t => t.DefaultEnabled && t.Applied);
if (defaultEnabledTweaks.Count > 0 && defaultApplied == defaultEnabledTweaks.Count)
    score += 5;

// Clamp
score = Math.Max(0, Math.Min(100, (int)Math.Round(score)));
```

### Migration Path to Native WPF

**Phase 1 (Foundation)**: 
- Scaffold `dotnet new wpf -n WinForge.Wpf`
- Migrate `seed-data.ts` → C# models + embedded JSON resources
- Implement `AdminChecker.cs` (auto-elevate via `process.StartInfo.Verb = "runas"`)
- Build `RegistryService.cs` (HKLM/HKCU read/write)
- Build `RestorePointService.cs` (WMI `SRSetRestorePoint`)
- Load embedded JSON and display 60 tweaks in ListBox

**Phase 2 (Core Operations)**:
- Implement `TweakEngine.Apply()` + `Undo()` pipeline (snapshot → execute → audit → log)
- Debloat engine: `Get-AppxPackage` / `Remove-AppxPackage -AllUsers`
- Privacy rules: registry policy toggles under `HKLM\SOFTWARE\Policies\Microsoft\Windows\`
- SQLite audit DB: `OperationHistory(Id, Timestamp, Kind, Target, PreviousValue, NewValue, Risk, CanUndo, Undone, UndoData JSON)`
- "Undo All Today" functionality

**Phase 3 (Advanced)**:
- DISM `Microsoft.Dism` NuGet: ScanHealth, RestoreHealth
- SFC: `Process.Start("sfc", "/scannow")` with output capture
- Windows Update COM: `IUpdateSession` search/download/install/hide
- DNS: `Set-DnsClientServerAddress` or `netsh interface ip set dns`
- ISO Builder: Mount WIM → modify Appx packages → inject registry hive → unmount → `Oscdimg` → SHA-256

**Phase 4 (Polish)**:
- EV code signing + SmartScreen "Verified Publisher"
- Inno Setup installer (Start Menu, desktop shortcuts, uninstall)
- Auto-update: check GitHub Releases API, download newer EXE
- Safety disclaimer on first run
- 5-language localization (en-US, es-ES, fr-FR, de-DE, zh-CN) via XAML `Language` binding + `.resx` resource files

### Code Style Conventions (Next.js portion)

- **TypeScript**: `strict: true` in `tsconfig.json` — no `any` types unless absolutely necessary
- **ESLint**: Next.js core rules + import sorting + no `console.log` in production
- **Tailwind CSS 4**: Utility-first, `dark:` variants for themes, `safari:` vendor prefixes disabled
- **Next.js**: App Router, `dynamic "force-fynamic"` where data fetching must be fresh, `export const metadata` for OG tags
- **Drizzle ORM**: `sql` tag for raw SQL, `pgEnum` for enums, `onConflictDoNothing()` for idempotent seeding
- **Server Actions**: `"use server"` at top, `export async function` returning `{ success: boolean; message: string }`

### Testing

Currently no test suite exists. To add:

- **Unit tests**: `vitest` or `jest` for `health.ts` algorithm, `seed-data.ts` parsing, API route handlers
- **Integration**: Test Server Actions via `supertest` against `localhost:3000/api/...`
- **Snapshot tests**: UI component rendering (HealthPanel, PresetButtons, Health gauge)

Run: `npm test` (would need to add test script to `package.json`)

### Known Issues / TODOs

1. **No test suite** — add `vitest`/`jest` for health algorithm and Server Actions
2. **Locale strings hardcoded** — extract to `.json` or `.mdx` for 5-language support
3. **PostgreSQL dependency** — migrate to SQLite for native WPF version (no server needed)
4. **SmartScreen compatibility** — not applicable until code-signed EXE built
5. **ISO Builder** — requires Windows ADK installed; simulate with mock data until ADK available
6. **PowerThrottling tweak** (`game-disable-powerthrottling`) has undefined undo operations — needs attention
7. **LargeSystemCache tweak** (`perf-large-system-cache`) has undefined undo operations — needs attention
8. **VerboseStatus tweak** (`perf-verbose-status`) has undefined undo operations — needs attention

### Getting Help

- **Issue tracker**: GitHub Issues
- **Roadmap**: See `PROJECT_ROADMAP.md` and `PROJECT_BLUEPRINT.md` in repo root
- **Catalog data**: Edit `src/db/seed-data.ts` — all 60 tweaks, 90 packages, 40 rules, 4 presets, 64 apps
- **Safety model**: Review `src/lib/actions.ts` — every mutating function follows: `maybeCreateRestorePoint()` → `db.update()` → `log()` → `revalidateAll()`

---
*This README is generated from the WinForge Elite project. For the full migration blueprint, see PROJECT_BLUEPRINT.md. For the phased roadmap, see PROJECT_ROADMAP.md.*