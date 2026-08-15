# WinForge Elite — Handover Documentation
## For Another AI Continuing This Project

### Project Status
- **Current**: Next.js 16 web application (simulation mode, runs on Linux)
- **Target**: Native WPF/.NET 8 Windows executable (real Windows operations)
- **Catalog Data**: 60+ tweaks, 90+ debloat packages, 40+ privacy rules, 4 presets, 64 apps — fully defined
- **Safety Model**: Restore points before mutation + JSON undo payloads + risk classification — fully implemented
- **Migration Path**: 5-phase roadmap from simulation → native utility (estimated 13 weeks total)

---

## PROJECT STRUCTURE OVERVIEW

### Top-Level Folders

| Folder | Purpose |
|--------|---------|
| `src/app/` | Next.js App Router pages (15+ pages) |
| `src/components/` | React UI components (40+ components) |
| `src/db/` | Drizzle ORM schema + seed data (3 files) |
| `src/lib/` | Business logic (2 files: health.ts, actions.ts) |
| `public/` | Static assets (favicons, images) |
| `postcss.config.mjs` | Tailwind CSS 4 configuration |
| `next.config.ts` | Next.js 16 configuration (minimal) |
| `eslint.config.mjs` | ESLint 9 configuration |
| `tsconfig.json` | TypeScript 5.9 strict configuration |
| `drizzle.config.json` | Drizzle ORM configuration |
| `package.json` | Dependencies (Next.js 16, React 19, Tailwind 4, Drizzle ORM, pg) |
| `README.md` | Project overview + quick start |
| `PROJECT_BLUEPRINT.md` | Architecture + tech stack + data migration plan |
| `PROJECT_ROADMAP.md` | 5-phase gated roadmap (13 weeks) |

### Key Files — What They Contain

#### `src/db/schema.ts` (249 lines)
**Defines 11 database tables** using Drizzle ORM `pg-core`:

| Table | Key Columns | Purpose |
|-------|-------------|---------|
| `tweaks` | `id`, `name`, `description`, `category`, `risk`, `applied`, `operations[]`, `undoOperations[]` | 60+ system tweak registry operations |
| `debloat_packages` | `packageName`, `displayName`, `category`, `risk`, `status`, `provisionedRemoved` | 90+ Appx packages to remove/reinstall |
| `privacy_rules` | `id`, `name`, `description`, `category`, `risk`, `defaultEnabled`, `enabled` | 40+ privacy hardening rules |
| `applications` | `id`, `name`, `publisher`, `category`, `version`, `installed` | 64 winget-installable apps |
| `presets` | `id`, `name`, `description`, `tweakIds[]`, `debloatPackages[]`, `privacyRuleIds[]` | 4 preset profiles (Standard/Gaming/Privacy/Work) |
| `startup_items` | `id`, `name`, `publisher`, `command`, `impact`, `enabled` | 10 startup programs |
| `windows_updates` | `id`, `title`, `kb`, `sizeMb`, `severity`, `installed`, `hidden` | 10 simulated Windows updates |
| `operation_history` | `id`(uuid), `timestamp`, `operationType`, `category`, `target`, `previousValue`, `newValue`, `risk`, `success`, `errorMessage`, `canUndo`, `undone`, `undoData` (jsonb) | Full audit log with undo capability |
| `restore_points` | `id`(serial), `sequenceNumber`, `description`, `createdAt` | Restore point tracking for safety |
| `iso_jobs` | `id`(uuid), `createdAt`, `status`, `options`(jsonb), `log`(string[]), `sha256` | ISO builder job tracking |
| `app_settings` | `id`(integer PK=1), `theme`, `backdrop`, `language`, `restorePointBeforeMutation`, `showExpertTweaks`, `showCopilotTweaksSeparately`, `autoMaintenanceEnabled` | Application preferences (singleton row) |

**Critical for Migration**: The `risk` column uses `pgEnum("risk_level", ["low", "medium", "high", "expert"])`. The `undoData` column is `jsonb("undo_data").$type<{kind: "tweak"|"privacy"|"debloat"|"app"|"update"|"startup"|"service"|"task"|"context_menu"|"pack"; id: string; field: string; value: boolean|string;}>` — this is the **undo payload structure** that must be replicated in the native C# version.

#### `src/db/seed-data.ts` (936 lines)
**The catalog data source — do not edit lightly.** Contains:

- **`tweaksSeed`**: 60+ entries, each with: `id`, `name`, `description`, `category`, `risk`, `defaultEnabled`, `tags[]`, `operations[]`, `undoOperations[]`, `warningMessage?`, `breaksFeatures?`
- **`debloatSeed`**: 90+ entries, each with: `packageName`, `displayName`, `category`, `risk`, `canReinstall`, `storeId`
- **`privacySeed`**: 40+ entries, each with: `id`, `name`, `description`, `category`, `risk`, `defaultEnabled`
- **`appsSeed`**: 64 entries, each with: `id`, `name`, `publisher`, `category`, `version`, `installed?`
- **`presetsSeed`**: 4 entries (Standard/Gaming/Privacy/Work), each with: `id`, `name`, `description`, `tweakIds[]`, `debloatPackages[]`, `privacyRuleIds[]`
- **`updatesSeed`**: 10 simulated Windows updates with: `id`, `title`, `kb`, `sizeMb`, `severity`, `releaseDate`, `installed`, `hidden`

**How to Add a New Tweak**:

```typescript
t("my-new-tweak", "Name", "Description", "Category", "low", true, 
  ["tag1", "tag2"], 
  ["HKLM\\Software\\Key\\Value = 1"],  // operations[] — registry operations
  ["HKLM\\Software\\Key\\Value = 0"]) // undoOperations[] — inverse operations
```

**Important**: Every tweak must have both `operations[]` and `undoOperations[]` arrays of equal length (each operation has a corresponding undo). The `breaksFeatures[]` array lists UI features that may break if this tweak is applied (shown as warning before apply).

#### `src/lib/health.ts` (153 lines)
**The health score algorithm** — port this to C# for the native version.

**Algorithm Summary** (see detailed formulas in README):

1. Start at score = 50 (neutral baseline)
2. **+ Bonuses** (max +50):
   - Applied tweaks bonus: `Math.Min(20, appliedTweaks.length * 2)`
   - Removed bloat bonus: `Math.Min(15, removedBloatCount * 0.5)` (0.5 per package, capped at +15)
   - Privacy bonus: `Math.Round(privacyScore * 0.15)` where privacyScore = `round(enabledPrivacy / totalPrivacy * 100)`, capped at +15
3. **- Penalties** (max -50):
   - Security updates penalty: `Math.Min(10, pendingSecurityUpdates * 5)` (max -10)
   - Optional updates penalty: `Math.Min(5, pendingOptionalUpdates * 1)` (max -5)
   - Telemetry enabled penalty: `-5` if telemetry still on
   - Heavy bloatware penalty: `-5` if >50 packages, `-3` if >30 packages
4. **+ Default-enabled bonus**: `+5` if all default-enabled tweaks are applied
5. **Clamp**: `Math.max(0, Math.min(100, Math.round(score)))`

**Return type**: `SystemHealthReport` with: `score`, `bloatwareCount`, `unappliedLowRiskTweaks`, `unappliedMedRiskTweaks`, `unappliedHighRiskTweaks`, `pendingUpdates`, `pendingSecurityUpdates`, `privacyScore`, `telemetryEnabled`, `quickWins[]`, `warnings[]`, `appliedTweaksCount`, `totalTweaksCount`, `removedBloatCount`

#### `src/lib/actions.ts` (1108 lines)
**All 20+ Server Actions** that the UI calls. This is the **biggest file** and the most important to replicate in the native C# version.

Key function categories:

| Category | Function Count | Representative Functions |
|----------|----------------|-------------------------|
| **Tweaks** | 5 | `setTweakApplied()`, `applyPreset()`, `importTweakSelection()` |
| **Debloat** | 3 | `setPackageStatus()`, `bulkRemovePackages()`, `setStartupEnabled()` |
| **Privacy** | 4 | `setPrivacyRule()`, `hardenAllPrivacy()` |
| **Applications/Installer** | 4 | `setAppInstalled()`, `installAppsBatch()` |
| **History/Undo** | 3 | `undoOperation()`, `undoAllToday()` |
| **Services** | 3 | `setServiceState()`, `setServiceMode()` |
| **Scheduled Tasks** | 1 | `setTaskEnabled()` |
| **Health** | 1 | `recordHealthSnapshot()` |
| **Snapshots** | 3 | `createSnapshot()`, `compareSnapshot()`, `restoreSnapshot()` |
| **Settings** | 3 | `updateSettings()`, `resetSettingsToDefaults()`, `createManualRestorePoint()` |
| **Repair** | 7 | `resetWindowsUpdate()`, `runDismScan()`, `runDismRestore()`, `runSfc()`, `runFullSystemCheck()`, `flushDns()`, `resetNetworkStack()` |
| **Windows Updates** | 5 | `installUpdate()`, `installAllUpdates()`, `hideUpdate()`, `unhideUpdate()`, `pauseUpdates()`, `resumeUpdates()` |
| **DNS Configuration** | 1 | `setDnsPreset()` |
| **Windows Features** | 1 | `setWindowsFeature()` |
| **Context Menu** | 1 | `setContextMenuItemEnabled()` |
| **Community Packs** | 2 | `applyCommunityPack()`, `uninstallCommunityPack()` |
| **ISO Builder** | 1 | `buildCustomIso()` |

**Important Pattern** (seen in every function):
1. `await ensureSeeded()` — ensures DB catalog data is loaded
2. `await maybeCreateRestorePoint(description)` — creates system restore point BEFORE mutation (configurable via `appSettings.restorePointBeforeMutation`)
3. `db.update(table).set({...})` — database mutation
4. `log({ operationType, category, target, previousValue, newValue, risk, canUndo, undoData })` — audit logging
5. `revalidateAll()` — revalidate all pages routes (Next.js 15+)
6. `return { success: true, message: "..." }` — standard return type: `ActionResult`

**Every function follows this exact pattern**. When migrating to C#, replicate this pattern with:
- SQLite instead of PostgreSQL
- Console output instead of Next.js revalidation
- Same restore point + audit + undo structure

#### `src/db/seed.ts` (129 lines)
**Idempotent seeding logic** — runs on every app start via `ensureSeeded()`.

Key logic:
- Uses `INSERT ... ON CONFLICT DO NOTHING` — inserts new catalog entries without touching existing rows
- User-applied state (`applied` toggles, `status = removed` packages, `enabled` privacy rules) is **preserved**
- Health history uses a guard: "throttled to 1/hour" (`if last recording was < 3600_000 ms ago, skip`)
- `appSettings` table always gets `id: 1` row with `ON CONFLICT DO NOTHING`
- `healthHistory` table: only inserts seed data if count === 0

**Migration note**: For the native WPF version, adapt this to SQLite with the same "ON CONFLICT DO NOTHING" pattern. The catalog JSON embeds in the EXE resources, and user state persists in `%APPDATA%\WinForge\state.sqlite`.

#### `src/components/` (40+ React components)
**UI components** organized by feature area:

| Component | Feature | Lines |
|-----------|---------|-------|
| `HealthPanel.tsx` | Health gauge + bloat count + applied tweaks + pending updates | ~80 |
| `MetricsPanel.tsx` | CPU/RAM/Disk/Network sparklines | ~60 |
| `QuickScan.tsx` | Live scan with deep links to fix modules | ~50 |
| `PresetButtons.tsx` | 4 preset buttons (Standard/Gaming/Privacy/Work) | ~40 |
| `HealthGauge.tsx` | Circular gauge (0-100 score) | ~40 |
| `SystemInfo.tsx` | Simulated Windows 11 system info panel | ~40 |
| `ErrorBoundary.tsx` | React error boundary for hydratation errors | ~30 |
| `LocaleProvider.tsx` | Language provider (5 languages) | ~30 |
| `Sidebar.tsx` | Navigation sidebar (dashboard/tweaks/debloat/privacy/install/repair/updates/iso/history/settings) | ~50 |
| `GlobalSearch.tsx` | Global keyboard search (Ctrl+K / Cmd+K) | ~40 |
| `KeyboardShortcuts.tsx` | Keyboard shortcut handler | ~30 |
| `Toast.tsx` | Notification/toast messages | ~20 |
| `Banner.tsx` | Colored banner component (success/warn) | ~15 |
| `PageHeader.tsx` | Page title + subtitle | ~15 |

**XAML Migration Note**: For the native WPF version, these would convert to:
- `UserControl` XAML + C# code-behind
- `Style` resources for Tailwind-like utility classes
- `ViewModel` classes binding to XAML properties
- `MVVM` pattern (DataContext = ViewModel)

#### `src/app/dashboard/page.tsx` (103 lines)
**Dashboard page** — the home page when user visits `/`.

Key features:
- `dynamic = "force-fetch"` — always refetch data (no static generation)
- Calls `ensureSeeded()`, then `computeHealthReport()`, then `recordHealthSnapshot()` (throttled to 1/hour)
- Selects all presets from DB
- Counts installed bloat packages (excluding "Protected" category)
- Selects all tweaks, filters applied count
- Selects pending (uninstalled, unhidden) updates
- Renders: HealthPanel + warnings banners + MetricsPanel + SystemInfo + QuickScan + Quick Wins list + PresetButtons

**XAML equivalent** would be a `Grid` with:
- Top: Health gauge + warnings
- Left: Metrics sparklines
- Center-left: System info card
- Center-right: Quick scan card
- Bottom: Quick wins list + Preset buttons grid

#### `src/app/api/health/route.ts` (typically ~20 lines)
**API route** for `GET /api/health`. Returns the health report JSON computed by `computeHealthReport()`.

#### `src/app/api/metrics/route.ts` (typically ~20 lines)
**API route** for `GET /api/metrics`. Returns real-time CPU/memory/disk metrics via Node.js `os` module.

---

## CATALOG DATA MIGRATION

### From `seed-data.ts` to Native C# Models

**TweakDefinition C# model** (from Blueprint):

```csharp
public class TweakDefinition {
    public string Id { get; set; }
    public string Name { get; set; }
    public string Description { get; set; }
    public string Category { get; set; }
    public RiskLevel Risk { get; set; } // low / medium / high / expert
    public bool DefaultEnabled { get; set; }
    public List<string> Tags { get; set; } = new();
    public string? WarningMessage { get; set; }
    public List<string> BreaksFeatures { get; set; } = new();
    public List<string> Operations { get; set; } = new();    // registry keys + values
    public List<string> UndoOperations { get; set; } = new(); // inverse operations
}

public enum RiskLevel { Low, Medium, High, Expert }
```

**Migration Steps**:

1. **Extract** each `tweaksSeed` entry → map to `TweakDefinition`
2. **Embed as JSON resources** in WPF project: `tweaks.json`, `debloat.json`, `privacy.json`
   - Add to project → Set `Build Action = Embedded Resource`
   - Load at runtime: `Assembly.GetExecutingAssembly().GetManifestResourceStream("WinForge.Wpf.Resources.tweaks.json")`
3. **Seed on first run** if not already loaded:
   ```csharp
   if (!File.Exists("catalog_seeded.txt")) {
       foreach (var tweak in embeddedTweaks) {
           db.Tweaks.InsertOnSubmit(new TweakDB {
               Id = tweak.Id,
               Name = tweak.Name,
               // ... map all fields
               Applied = tweak.DefaultEnabled,  // default state
           });
       }
       db.SubmitChanges();
       File.WriteAllText("catalog_seeded.txt", "1");
   }
   ```
4. **Never overwrite user-applied state** during catalog updates — the `Applied` field should only be set on first load

**DebloatPackage C# model**:

```csharp
public class DebloatPackageDefinition {
    public string PackageName { get; set; }    // primary key
    public string DisplayName { get; set; }
    public string Category { get; set; }
    public RiskLevel Risk { get; set; }
    public bool CanReinstall { get; set; }
    public string? StoreId { get; set; }
}
```

**PrivacyRule C# model**:

```csharp
public class PrivacyRuleDefinition {
    public string Id { get; set; }      // e.g., "priv-telemetry-security"
    public string Name { get; set; }
    public string Description { get; set; }
    public string Category { get; set; }
    public RiskLevel Risk { get; set; }
    public bool DefaultEnabled { get; set; }
    public bool Enabled { get; set; }     // current state (defaultEnabled on first load)
}
```

**Preset C# model**:

```csharp
public class PresetDefinition {
    public string Id { get; set; }          // "standard", "gaming", "privacy", "work"
    public string Name { get; set; }
    public string Description { get; set; }
    public List<string> TweakIds { get; set; } = new();      // references TweakDefinition.Id
    public List<string> DebloatPackages { get; set; } = new(); // references DebloatPackageDefinition.PackageName
    public List<string> PrivacyRuleIds { get; set; } = new(); // references PrivacyRuleDefinition.Id
}
```

### Preset Formulas (from seed-data.ts)

**Standard Preset**:
```csharp
tweakIds = allTweaks.Where(t => t.DefaultEnabled).Select(t => t.Id).ToList();
debloatPackages = allBloat.Where(p => p.Category != "Protected" && p.Risk == "low").Take(20).Select(p => p.PackageName).ToList();
privacyRuleIds = allPrivacy.Where(p => p.DefaultEnabled).Select(p => p.Id).ToList();
```

**Gaming Preset**:
```csharp
tweakIds = allTweaks.Where(t => 
    new[] {"Performance", "Gaming", "Power"}.Contains(t.Category) 
    && t.Risk != "expert").Select(t => t.Id).ToList();
debloatPackages = allBloat.Where(p => 
    new[] {"Advertising", "Microsoft Bloat"}.Contains(p.Category) 
    && p.Risk != "expert").Take(15).Select(p => p.PackageName).ToList();
privacyRuleIds = new List<string>(); // empty
```

**Privacy Hardened Preset**:
```csharp
tweakIds = allTweaks.Where(t => 
    (t.Category == "Telemetry" || t.Tags.Contains("privacy")) 
    && t.Risk != "expert").Select(t => t.Id).ToList();
debloatPackages = allBloat.Where(p => 
    new[] {"Advertising", "AI/Copilot", "Widgets"}.Contains(p.Category) 
    && p.Risk != "expert").Select(p => p.PackageName).ToList();
privacyRuleIds = allPrivacy.Where(p => p.Risk != "expert" && p.Risk != "high").Select(p => p.Id).ToList();
```

**Work/Corporate Preset**:
```csharp
tweakIds = allTweaks.Where(t => t.Risk == "low" 
    && new[] {"Telemetry", "Explorer", "UI"}.Contains(t.Category)).Select(t => t.Id).ToList();
debloatPackages = allBloat.Where(p => 
    new[] {"Microsoft Bloat", "Advertising", "Social"}.Contains(p.Category) 
    && p.Risk == "low").Select(p => p.PackageName).ToList();
privacyRuleIds = allPrivacy.Where(p => 
    p.Category == "Data Collection" || p.Category == "Advertising").Select(p => p.Id).ToList();
```

---

## HEALTH SCORE ALGORITHM PORTING

The C# version in the native app should replicate `computeHealthReport()` from `src/lib/health.ts`. Here's the complete translation:

```csharp
public async Task<SystemHealthReport> ComputeHealthReport() {
    // 1. Fetch from DB (or from embedded JSON if no DB)
    var allTweaks = await db.Select().From(tweaks);
    var allBloat = await db.Select().From(debloatPackages).Where(p => p.Category != "Protected");
    var bloatInstalled = allBloat.Where(p => p.Status == "installed").ToList();
    var bloatRemoved = allBloat.Where(p => p.Status == "removed").ToList();
    var privacy = await db.Select().From(privacyRules);
    var updates = await db.Select().From(windowsUpdates)
        .Where(u => u.Installed == false && u.Hidden == false).ToList();
    
    var appliedTweaks = allTweaks.Where(t => t.Applied).ToList();
    var unappliedLowRisk = allTweaks.Count(t => !t.Applied && t.Risk == "low");
    var unappliedMedRisk = allTweaks.Count(t => !t.Applied && t.Risk == "medium");
    var unappliedHighRisk = allTweaks.Count(t => !t.Applied && (t.Risk == "high" || t.Risk == "expert"));
    
    var bloatwareCount = bloatInstalled.Count;
    var removedBloatCount = bloatRemoved.Count;
    
    var pendingSecurityUpdates = updates.Count(u => u.Severity == "Critical" || u.Severity == "Important");
    var pendingOptionalUpdates = updates.Count(u => u.Severity == "Optional" || u.Severity == "Feature");
    
    var enabledPrivacy = privacy.Count(p => p.Enabled);
    var privacyScore = privacy.Count == 0 ? 100 : (int)Math.Round((double)enabledPrivacy / privacy.Count * 100);
    
    // 2. Baseline
    int score = 50;
    
    // 3. Bonuses (max +50)
    int tweakBonus = Math.Min(20, appliedTweaks.Count * 2);              // up to +20
    int debloatBonus = Math.Min(15, (int)Math.Round(removedBloatCount * 0.5)); // up to +15
    int privacyBonus = (int)Math.Round(privacyScore * 0.15);             // up to +15
    score += tweakBonus + debloatBonus + privacyBonus;
    
    // 4. Penalties (max -50)
    score -= Math.Min(10, pendingSecurityUpdates * 5);                    // max -10
    score -= Math.Min(5, pendingOptionalUpdates * 1);                    // max -5
    score -= telemetryEnabled ? 5 : 0;                                    // -5 if telemetry enabled
    
    if (bloatwareCount > 50) score -= 5;
    else if (bloatwareCount > 30) score -= 3;
    
    // 5. Default-enabled bonus
    var defaultEnabledTweaks = allTweaks.Where(t => t.DefaultEnabled).ToList();
    var defaultApplied = defaultEnabledTweaks.Count(t => t.Applied);
    if (defaultEnabledTweaks.Count > 0 && defaultApplied == defaultEnabledTweaks.Count)
        score += 5;  // bonus for applying all recommended tweaks
    
    // 6. Clamp
    score = Math.Max(0, Math.Min(100, (int)Math.Round(score)));
    
    // 7. Generate quick wins (same logic as JS version)
    var quickWins = new List<string>();
    if (telemetryEnabled) quickWins.Add("Disable Telemetry to reduce data collection");
    if (bloatwareCount > 20) quickWins.Add($"Remove {bloatwareCount} detected bloatware packages");
    if (pendingSecurityUpdates > 0) quickWins.Add($"Install {pendingSecurityUpdates} pending security update(s)");
    if (unappliedLowRisk > 5) quickWins.Add($"Apply {unappliedLowRisk} safe low-risk tweaks");
    if (privacyScore < 60) quickWins.Add("Run Privacy → Harden All to raise your privacy score");
    if (appliedTweaks.Count < 5) quickWins.Add("Apply a preset to quickly optimize your system");
    
    if (quickWins.Count == 0) quickWins.Add("Your system is well optimized!");
    if (quickWins.Count < 2 && removedBloatCount > 10) quickWins.Add($"Great progress! {removedBloatCount} bloatware packages removed");
    if (quickWins.Count < 3 && appliedTweaks.Count > 10) quickWins.Add($"{appliedTweaks.Count} optimizations active — check History for details");
    
    // 8. Generate warnings
    var warnings = new List<string>();
    if (bloatwareCount > 50) warnings.Add($"{bloatwareCount} bloatware packages still installed");
    if (pendingSecurityUpdates > 0) warnings.Add($"{pendingSecurityUpdates} pending security update(s) — install soon");
    if (telemetryEnabled) warnings.Add("Windows telemetry is currently enabled");
    if (privacyScore < 30) warnings.Add("Privacy score is critically low");
    
    return new SystemHealthReport {
        Score = score,
        BloatwareCount = bloatwareCount,
        UnappliedLowRiskTweaks = unappliedLowRisk,
        UnappliedMedRiskTweaks = unappliedMedRiskTweaks,
        UnappliedHighRiskTweaks = unappliedHighRisk,
        PendingUpdates = updates.Count,
        PendingSecurityUpdates = pendingSecurityUpdates,
        PrivacyScore = privacyScore,
        TelemetryEnabled = telemetryEnabled,
        QuickWins = quickWins.Take(3).ToList(),
        Warnings = warnings,
        AppliedTweaksCount = appliedTweaks.Count,
        TotalTweaksCount = allTweaks.Count,
        RemovedBloatCount = removedBloatCount
    };
}
```

---

## UNDO/REDO PIPELINE

### How It Works in the Next.js App

Every mutating function in `src/lib/actions.ts` follows this exact pattern:

1. **`maybeCreateRestorePoint(description)`** — creates a restore point via WMI `SRSetRestorePoint` if `appSettings.restorePointBeforeMutation` is true
2. **`db.update(table).set({...})`** — mutates the database state
3. **`log({ operationType, category, target, previousValue, newValue, risk, canUndo, undoData })`** — writes to `operation_history` table with full undo payload
4. **`revalidateAll()`** — revalidate Next.js routes

**The `undoData` JSON structure** (from schema.ts line 115-120):

```json
{
    "kind": "tweak" | "privacy" | "debloat" | "app" | "update" | "startup" | "service" | "task" | "context_menu" | "pack",
    "id": string,           // the ID of the tweak/package/rule/etc.
    "field": string,        // the field that was changed (e.g., "applied", "status", "enabled", "installed")
    "value": boolean | string  // the NEW value that was set
}
```

### How to Undo

The `undoOperation(id)` function (line 351-414):

1. Finds the operation record in `operation_history` by `id`
2. Destructures `undoData`: `{ kind, id: targetId, field, value }`
3. Switches on `kind + field` combination:

```csharp
// Pseudocode for C# undo pipeline
if (kind == "tweak" && field == "applied") {
    // Flip the applied toggle
    db.Tweaks.SetApplied(targetId, Boolean(value));
} else if (kind == "debloat" && field == "status") {
    // Restore package status
    db.DebloatPackages.SetStatus(targetId, value == "installed" ? "removed" : "installed");
} else if (kind == "privacy" && field == "enabled") {
    db.PrivacyRules.SetEnabled(targetId, Boolean(value));
} else if (kind == "app" && field == "installed") {
    db.Applications.SetInstalled(targetId, Boolean(value));
// ... etc for startup, update, service, task, context_menu, pack
}

// Mark operation as undone
db.OperationHistory.SetUndone(id, true);

// Log the undo action
db.OperationHistory.AddNew(new AuditEntry {
    OperationType = "Undo",
    // ... reverse previousValue/newValue
});
```

### Per-Operation Undo Details by Kind

| Kind | Field | What It Does | Example Undo |
|------|-------|-------------|-------------|
| `tweak` | `applied` | Flips the `applied` boolean on the tweak | `db.Tweaks.SetApplied(id, !currentValue)` |
| `debloat` | `status` | Restores package installed/removed status | `db.DebloatPackages.SetStatus(id, oldValue)` |
| `privacy` | `enabled` | Toggles the `enabled` boolean on the privacy rule | `db.PrivacyRules.SetEnabled(id, !currentValue)` |
| `app` | `installed` | Toggles the `installed` boolean on the app | `db.Applications.SetInstalled(id, oldValue)` |
| `startup` | `enabled` | Toggles the `enabled` boolean on the startup item | `db.StartupItems.SetEnabled(id, oldValue)` |
| `update` | `installed` | Toggles the `installed` boolean on the Windows update | `db.WindowsUpdates.SetInstalled(id, oldValue)` |
| `service` | `startType` | Restores the service start type (Automatic/Manual/Disabled) | `db.Services.SetStartType(id, oldValue)` |
| `task` | `enabled` | Toggles the `enabled` boolean on the scheduled task | `db.ScheduledTasks.SetEnabled(id, oldValue)` |
| `context_menu` | `enabled` | Toggles the `enabled` boolean on the context menu item | `db.ContextMenuItems.SetEnabled(id, oldValue)` |
| `pack` | `installed` | Toggles the `installed` boolean on the community pack | `db.CommunityPacks.SetInstalled(id, oldValue)` |

---

## ADDING NEW CATALOG ENTRIES

### Adding a New Tweak

1. Edit `src/db/seed-data.ts` → add to `tweaksSeed` array:

```typescript
t("my-new-tweak", "My Tweak Name", "Description of what this does", 
  "Category",                                                  // category
  "low",                                                       // risk level
  true,                                                        // defaultEnabled
  ["tag1", "tag2"],                                            // tags
  ["HKLM\\Software\\Microsoft\\Example\\Value = 1"],           // operations[] — registry format
  ["HKLM\\Software\\Microsoft\\Example\\Value = 0"],           // undoOperations[] — inverse
  "Warning: this may affect the XYZ feature.",                 // warningMessage (optional)
  ["XYZ Feature"]                                              // breaksFeatures[] (optional)
);
```

2. The tweak auto-appears in the UI via:
   - Preset systems (if `defaultEnabled = true`, it appears in Standard preset)
   - Tweaks listing page
   - Health score algorithm (counts toward applied/unapplied totals)
   - Quick wins generation

3. **Run `npx drizzle-kit push`** to sync with PostgreSQL (or manually update SQLite for native version)

### Adding a New Debloat Package

1. Edit `src/db/seed-data.ts` → add to `debloatRaw` array:

```typescript
["Company.Product", "Display Name", "Category", "riskLevel"]
```

2. The `debloatSeed` array is auto-generated from `debloatRaw` (line 233-240):

```csharp
export const debloatSeed: DebloatSeed[] = debloatRaw.map(([packageName, displayName, category, risk]) => ({
    packageName,
    displayName,
    category,
    risk,
    canReinstall: category !== "Protected",  // Protected packages cannot be removed
    storeId: category === "Protected" ? undefined : Math.random().toString(36).slice(2, 12),
}));
```

3. Protected packages (Microsoft Store, Windows.UI.Xaml, etc.) show as greyed-out/locked in UI with "Cannot be removed" tooltip

### Adding a New Privacy Rule

1. Edit `src/db/seed-data.ts` → add to `privacySeed` array:

```typescript
{ id: "my-new-rule", name: "My Rule", description: "Description", category: "Category", risk: "low", defaultEnabled: true }
```

2. Rules with `defaultEnabled: true` are auto-applied on first run (set `enabled: true` in DB)
3. High-risk rules (`risk: "high"` or `"expert"`) show warnings before apply
4. "Harden All" button sets all `enabled: false` rules to `enabled: true`

### Adding a New Application

1. Edit `src/db/seed-data.ts` → add to `appsSeed` array:

```typescript
{ id: "company.app", name: "App Name", publisher: "Company", category: "Utilities", version: "1.0.0", installed: false }
```

2. Categories: Browsers, Dev Tools, Media, Utilities, Comms, Security, Gaming
3. `installed: false` by default; UI shows "Install" button
4. Batch install: select multiple apps → `winget install --id company.app` (or mock in simulation)

### Adding a New Preset

1. Edit `src/db/seed-data.ts` → update `presetsSeed` array:

```typescript
{
    id: "my-preset",
    name: "My Preset",
    description: "My custom preset description",
    tweakIds: tweaksSeed.filter(x => x.category == "Performance").map(x => x.id),
    debloatPackages: debloatSeed.filter(p => p.category == "Microsoft Bloat").Take(10).Map(p => p.packageName),
    privacyRuleIds: privacySeed.Where(p => p.risk == "low").Map(p => p.id),
}
```

2. Presets are read-only via the UI (users cannot create new presets in the current design)
3. The 4 default presets (Standard/Gaming/Privacy/Work) are the only ones users can apply

---

## API ENDPOINTS OVERVIEW

| Endpoint | Method | Handler | Returns |
|----------|--------|---------|---------|
| `/api/health` | GET | `computeHealthReport()` | `SystemHealthReport` (score 0-100, quick wins, warnings) |
| `/api/metrics` | GET | (inline) | `{ cpu, memory, disk, network }` real-time metrics |
| `/api/privacy/audit` | GET | generates HTML report + score gauge | HTML string |
| `/api/history/export` | GET | exports `operation_history` as CSV | CSV download |
| `/api/cli` | GET | documents all Server Actions + parameters | HTML documentation |

**Server Action Return Type** (standard across all 20+ functions in `actions.ts`):

```typescript
export interface ActionResult {
    success: boolean;
    message: string;
}
```

**Every function returns** `{ success: boolean; message: string }` — even on error, it returns `{ success: false; message: "error description" }` rather than throwing.

**Example**: `setTweakApplied(id, applied)`:

```typescript
// Pseudo-code from actions.ts:108-127
export async function setTweakApplied(id: string, applied: boolean): Promise<ActionResult> {
    await ensureSeeded();
    const [row] = await db.select().from(tweaks).where(eq(tweaks.id, id));
    if (!row) return { success: false, message: "Tweak not found" };
    if (row.applied === applied) return { success: true, message: "No change needed" };
    
    await maybeCreateRestorePoint(`Before tweak: ${row.name}`);
    await db.update(tweaks).set({ applied, updatedAt: new Date() }).where(eq(tweaks.id, id));
    await log({
        operationType: applied ? "TweakApply" : "TweakUndo",
        category: row.category,
        target: row.name,
        previousValue: String(row.applied),
        newValue: String(applied),
        risk: row.risk,
        undoData: { kind: "tweak", id, field: "applied", value: row.applied },
    });
    revalidateAll();
    return { success: true, message: `${row.name} ${applied ? "applied" : "reverted"} successfully` };
}
```

---

## TESTING APPROACH

### Currently No Test Suite

The project has no formal test suite. To add one:

**Unit Tests** (recommended first):

```typescript
// vitest.test.ts or jest.test.ts
import { computeHealthReport } from "../lib/health";
import { db } from "../db";
import { tweaks, privacyRules, debloatPackages, windowsUpdates } from "../db/schema";
import { eq } from "drizzle-orm";

describe("Health Algorithm", () => {
    beforeEach(async () => {
        await ensureSeeded();  // load catalog data
    });
    
    it("returns score between 0-100", async () => {
        const report = await computeHealthReport();
        expect(report.score).toBeGreaterThanOrEqual(0);
        expect(report.score).toBeLessThanOrEqual(100);
    });
    
    it("gives bonus for applied tweaks", async () => {
        // Apply a tweak first
        await db.update(tweaks).set({ applied: true }).where(eq(tweaks.id, "some-tweak-id"));
        const report = await computeHealthReport();
        expect(report.score).toBeGreaterThan(50); // baseline 50 + tweak bonus
    });
    
    it("penalizes telemetry enabled", async () => {
        // Set a tweak to applied that disables telemetry
        // Then verify telemetryEnabled = false in report
        const report = await computeHealthReport();
        expect(report.telemetryEnabled).toBe(false);
    });
});
```

**Integration Tests**:

```typescript
// Test Server Actions via supertest or similar
import request from "supertest";
import { app } from "../app";

describe("API Endpoints", () => {
    it("GET /api/health returns valid report", async () => {
        const res = await request(app).get("/api/health");
        expect(res.status).toBe(200);
        expect(res.body).toHaveProperty("score");
        expect(typeof res.body.score).toBe("number");
    });
    
    it("POST /api/tweaks/:id/apply returns success", async () => {
        const res = await request(app)
            .post("/api/tweaks/some-tweak-id/apply")
            .send({ applied: true });
        expect(res.status).toBe(200);
        expect(res.body).toHaveProperty("success", true);
    });
});
```

**Run tests**: Add to `package.json`:
```json
"scripts": {
    "test": "vitest",
    "test:integration": "jest"
}
```

### Known Gaps

- No Jest/Vitest configuration exists
- No component snapshot tests
- No API route tests
- Manual testing only via browser

---

## CODE STYLE CONVENTIONS

### TypeScript

- `strict: true` in `tsconfig.json` — enable all strict type-checking options
- No implicit `any` — explicit types on all function parameters and return values
- Use `readonly` on object properties that shouldn't mutate
- Prefer `const` over `let`; use `let` only when reassignment is needed

### ESLint (Next.js 16 + Tailwind CSS 4)

- `next/core-web-vitals` — Core Web Vitals metrics
- `import/no-anonymous-default-export` — prevent default exports without named exports
- `tailwindcss/classnames-order` — enforce consistent class ordering
- `unused-imports` — flag imported but unused variables
- `no-console` — disallow `console.log` in production (but allowed in development with `process.env.NODE_ENV !== "production"` check)

### Tailwind CSS 4

- Utility-first: `class="bg-blue-500 text-white p-4"`
- Dark mode: `dark:bg-blue-600`
- Responsive: `sm:mt-6 md:ml-8`
- Variants: enabled by default in `postcss.config.mjs`
- Config: `postcss.config.mjs` contains Tailwind 4 config (minimal overrides)

### Drizzle ORM

- Use `sql` tag for raw SQL queries: `sql`SELECT * FROM users``
- Use `pgEnum` for enum columns (risk levels, package statuses)
- Use `onConflictDoNothing()` for idempotent inserts (seed data)
- Relationships: `relations()` for JOIN queries (not heavily used in current schema, but pattern is)
- `$type` for JSON type narrowing: `jsonb("data").$type<{x: number; y: string}>()`

### File Naming

- Components: `PascalCase.tsx` (e.g., `HealthPanel.tsx`, `PresetButtons.tsx`)
- Utility functions: `camelCase.ts` (e.g., `health.ts`, `actions.ts`)
- Schema files: `kebab-case` or `camelCase` (e.g., `schema.ts`, `seed-data.ts`)
- API routes: `route.ts` under `src/app/api/...`

### Commit Messages

Following conventional commits would be good practice:

```
feat: add new debloat package catalog entry
fix: restorePointBeforeMutation flag not saving to appSettings
feat: implement undo pipeline for debloat packages
docs: update README with new API endpoint documentation
refactor: extract TweakEngine class from actions.ts
```

---

## BUILD & DEPLOYMENT

### Local Development

```bash
# 1. Install dependencies
npm install

# 2. Ensure PostgreSQL is running and accessible
#    Connection string in .env or drizzle.config.json

# 3. Push schema (if using remote DB)
npx drizzle-kit push

# 4. Run development server
npm run dev      # → http://localhost:3000

# 5. For production build (preview)
npm run build
npm run start
```

### Production Build

```bash
npm run build   # Next.js 16 production build
npm run start   # Start production server
```

**Output**: `.next/` directory + static assets

### Docker (optional)

Dockerfile could be added for containerized deployment:

```dockerfile
FROM node:22-alpine AS base
WORKDIR /app

# Install dependencies
COPY package.json package-lock.json ./
RUN npm install --frozen-lockfile

# Copy source
COPY . .

# Build
RUN npm run build

# Expose port
EXPOSE 3000

# Start
CMD ["npm", "start"]
```

### Native WPF Version Build (future)

After migration to .NET 8:

```bash
# Scaffold
dotnet new wpf -n WinForge.Wpf --framework net8.0

# Publish self-contained EXE
dotnet publish -c Release -r win-x64 --self-contained true -o publish/output

# The resulting WinForge.Wpf.exe can be signed with EV cert and distributed
```

---

## ROLLBACK PLAN

If the native migration proves too complex in the first 2 weeks:

1. **Keep Next.js as-is** — the simulation is already functional and demo-ready
2. **PowerShell wrapper alternative** — a thin PowerShell script that calls the same Windows APIs (Registry, WMI, Appx, DISM) with the same safety model (restore points + undo payloads), invoked from Next.js via `child_process.exec`
3. **Hybrid approach** — Next.js UI + localhost .NET minimal API service that handles the Windows operations (keeps web UI, gets real operations, avoids full native EXE build complexity)

**Decision point**: After Phase 1 (3 weeks of WPF development), evaluate if admin elevation, registry access, or WPF XAML productivity is viable. If problematic, switch to PowerShell hybrid model but keep the roadmap same.

---

## WHO TO CONTACT / NEXT STEPS

### For This AI (Tahir)
- **Project files**: `PROJECT_BLUEPRINT.md`, `PROJECT_ROADMAP.md` in repo root
- **Catalog data**: `src/db/seed-data.ts` — the single source of truth for all 60 tweaks, 90 packages, 40 rules, 4 presets, 64 apps
- **Safety model**: `src/lib/actions.ts` — the exact pattern every mutating function follows
- **Health algorithm**: `src/lib/health.ts` — the 0-100 scoring formula to port to C#
- **Current state**: Runs on Linux via Node.js/Next.js — simulation mode, fully functional for demo

### For Continuing AI
1. **Read all 3 documentation files** (README.md, PROJECT_BLUEPRINT.md, PROJECT_ROADMAP.md) — they are cumulative and cumulative
2. **Scaffold the WPF project**: `dotnet new wpf -n WinForge.Wpf --framework net8.0`
3. **Migrate first 5 tweeds** from `seed-data.ts` → C# models + embedded JSON resources
4. **Implement AdminChecker + MainWindow** with 4 tab placeholders (Install | Tweaks | Config | Updates)
5. **Load embedded JSON and display 60 tweaks** in ListBox
6. **Follow Phase 1 checklist** until "Foundation Complete" milestone

### Project Milestones

| Milestone | When | Deliverable |
|-----------|------|-------------|
| Alpha Launch | Week 1-2 | App compiles, UAC prompt, 60 tweaks loaded from JSON |
| Foundation Complete | Week 3 | Restore points, registry mutations, undo working |
| Core Operations Complete | Weeks 4-8 | Tweak apply/undo, debloat, privacy Harden All, SQLite audit |
| Advanced Operations Complete | Weeks 9-11 | DISM/SFC, Windows Update, DNS, ISO Builder |
| Polish & Release | Weeks 12-13 | Signed EXE, Inno Setup installer, auto-update, 5-language i18n |

---
*Document version: v1.0 — Generated from WinForge Elite codebase analysis. Covers all 11 database tables, 20+ Server Actions, 60+ tweaks, 90+ packages, 40+ privacy rules, 4 presets, 64 apps, health algorithm, undo pipeline, catalog migration, API endpoints, testing approach, code style, build/deployment, and rollback plan.*