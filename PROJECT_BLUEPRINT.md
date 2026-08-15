# WinForge Elite — Project Blueprint
## From Web Simulation to Native Windows Utility

### Vision
Replicate Chris Titus Tech's Windows Utility as a native WPF/.NET desktop application that directly manipulates Windows (registry, services, appx, DISM, etc.) — currently the Next.js app **simulates** these operations against PostgreSQL; the goal is a **real** Windows executable (`irm domain.com/win | iex` or signed EXE).

### Core Philosophy
- **Safety first**: Every mutation creates a system restore point before changes
- **Full undo**: JSON-encoded undo payloads for complete reversibility
- **Risk classification**: Low/Medium/High/Expert with UI badges and warnings
- **Protected resources**: Critical system components locked from modification
- **Audit trail**: Every operation logged with timestamp, change details, undo data

---

## ARCHITECTURE

### Current (Next.js Simulation)
```
┌─────────────────────────────────────────────────────┐
│  Next.js 16 (App Router)                           │
│  - Frontend: React components, Tailwind CSS 4      │
│  - Backend: Server Actions + API routes            │
│  - Database: PostgreSQL + Drizzle ORM              │
│  - State: Fully simulated via DB + in-memory algo  │
└─────────────────────────────────────────────────────┘
```

### Target (Native Windows Utility)
```
┌─────────────────────────────────────────────────────┐
│  WPF / WinForms .NET 8+ Executable                 │
│  - Single self-contained EXE (~20MB)               │
│  - Requires Admin (UAC prompt on first run)        │
│  - Direct Windows API calls (no simulation)        │
│  - Embedded catalog JSON (from current seed-data)  │
└─────────────────────────────────────────────────────┘
```

### Recommended Hybrid (Transition Phase)
```
┌─────────────────────────────────────────────────────┐
│  WPF Front (.NET 8)                                  │
│  - XAML UI: 4 tabs (Install | Tweaks | Config |    │
│    Updates)                                          │
│  - Communicates via localhost HTTPS to:            │
│  ┌─────────────────────────────────────────────┐   │
│  │  .NET Worker Service (localhost)             │   │
│  │  - RegistryService: HKLM/HKCU read/write     │   │
│  │  - ServiceManager: ServiceController + sc.exe│   │
│  │  - DismService: DismApi.dll P/Invoke         │   │
│  │  - WindowsUpdate: IUpdateSession COM        │   │
│  └─────────────────────────────────────────────┘   │
│  - Or direct P/Invoke if self-contained EXE       │   │
└─────────────────────────────────────────────────────┘
```

### Data Flow (New)
1. **Catalog** → Embedded JSON resources (migrated from `src/db/seed-data.ts`)
2. **Operation** → Direct Windows API (RegistrySetValue, ServiceController, DismRestoreHealth, etc.)
3. **Snapshot** → Registry key export + service state + appx status (before mutation)
4. **Undo** → Restore point + apply inverse operations from snapshot
5. **Audit** → Event log + SQLite audit DB (local `%APPDATA%\WinForge\audit.db`)

---

## TECHNOLOGY STACK

| Layer | Recommendation | Why |
|-------|---------------|-----|
| **UI Framework** | **WPF (.NET 8)** | Native Windows, XAML, hardware acceleration, MVVM pattern, matches video demo |
| **Runtime** | **.NET 8** (LTS) | Cross-platform if needed, but target is Windows only |
| **Package Format** | **Self-contained EXE** or **SMS (MSIX)** | No .NET runtime requirement for user, auto-updates |
| **Registry Access** | `Microsoft.Win32.Registry` + `RegistryKey` | Full HKLM/HKCU access, value enumeration |
| **WMI/Service** | `System.Management` (`ManagementObjectSearcher`) OR `ServiceController` + `sc.exe` | Query/change service state, scheduled tasks |
| **Appx Package** | `System.AppX.PackageManager` NuGet OR PowerShell SDK (`Get-AppxPackage`) | Remove/Appx packages, query installed |
| **DISM Operations** | `Microsoft.Dism` NuGet package | DismOpenSession, DismGetImageInfo, DismRestoreHealth, DismMountImage |
| **ISO Building** | Windows ADK (`Oscdimg.exe` + `DismApi`) | Mount WIM → modify → unmount → generate ISO |
| **Windows Update** | COM `IUpdateSession` / `IUpdateSearcher` / `IUpdateCollator` | Search/download/install updates, hide/unhide |
| **Restore Points** | `Microsoft.VisualBasic.ApplicationServices` + WMI `CreateRestorePoint` | SRSetRestorePoint API, sequence numbering |
| **Undo/Payload** | `System.Text.Json` + custom `UndoPayload` model | JSON serialization of before/after state |
| **Installer** | **Inno Setup** or **WiX** for first-run EXE registration | Creates Start Menu folder, auto-elevate, uninstall |
| **Localization** | **5 languages** (en-US, es-ES, fr-FR, de-DE, zh-CN) via XAML `Language` binding and resource files `.resx` | README specifies: Language selector in Settings; matching Chris Titus Tech utility i18n; .NET 8 built-in satellite assembly support |

---

## NON-FUNCTIONAL REQUIREMENTS

| Requirement | Target | Measurement |
|-------------|--------|-------------|
| **Startup time** | < 3 seconds from double-click to main window | Stopwatch on fresh launch |
| **Memory footprint** | < 150 MB RAM at idle | Profiling via dotnet-counters or Visual Studio Diagnostic Tools |
| **UI responsiveness** | < 100ms input → visual feedback | UI thread profiling; no blocking API calls on main thread |
| **Error handling** | Graceful degradation — UI disables non-critical features on API failure | Try/catch around every Windows API; show error banner, not crash |
| **Code quality** | 0 errors, < 5 warnings in `dotnet build` | `dotnet build --nologo` exit code 0 |
| **Test coverage** | ≥ 80% on core services (TweakEngine, RegistryService, RestorePointService) | `dotnet test` with coverlet; exclude UI XAML unit tests (too brittle) |
| **Binary size** | < 25 MB self-contained EXE | `dotnet publish -c Release -r win-x64 --self-contained true` |
| **SmartScreen/Windows Defender** | 0 false-positive blocks on signed EXE | Test on 3 fresh Windows 11 VMs with default security settings |
| **Restore point reliability** | ≥ 95% success rate on `SRSetRestorePoint` | Log success/failure count over 100+ test runs |
| **Undo payload size** | < 64 KB per operation | JSON serialization limit; audit DB column limit |

---

## DATA MIGRATION PLAN

### Current Catalog Data (Next.js)
- `src/db/seed-data.ts` → 60+ tweaks, 90+ debloat packages, 40+ privacy rules
- Each entry has: `id`, `name`, `description`, `category`, `risk`, `defaultEnabled`, `tags`, `operations[]`, `undoOperations[]`, `warningMessage`, `breaksFeatures[]`

### Migration Steps

**Step 1: Extract & Normalize**
```csharp
// C# model equivalent to TweakSeed
public class TweakDefinition {
    public string Id { get; set; }
    public string Name { get; set; }
    public string Description { get; set; }
    public string Category { get; set; }
    public RiskLevel Risk { get; set; }  // low / medium / high / expert
    public bool DefaultEnabled { get; set; }
    public List<string> Tags { get; set; } = new();
    public string? WarningMessage { get; set; }
    public List<string> BreaksFeatures { get; set; } = new();
    public List<string> Operations { get; set; } = new();    // registry keys + values
    public List<string> UndoOperations { get; set; } = new(); // inverse operations
}

public enum RiskLevel { Low, Medium, High, Expert }
```

**Step 2: Embed as Resources**
- Add `tweaks.json`, `debloat.json`, `privacy.json` to WPF project `Resources` folder
- Build Action = `Embedded Resource`
- Load at runtime: `Assembly.GetExecutingAssembly().GetManifestResourceStream(...)`

**Step 3: Seed on First Run**
```csharp
if (!File.Exists(localStatePath "catalog_seeded.txt")) {
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

**Step 4: Preserve User State**
- On migration: `applied` toggles, package statuses, enabled privacy rules → preserved in local SQLite or registry `HKLM\SOFTWARE\WinForge\State`
- Never overwrite user-applied state during catalog updates

---

## ROADMAP — 4 Phases

### Phase 1 — Foundation (3 weeks)
**Goal**: Admin elevation + Registry service + Restore points + Basic UI shell

| Milestone | Deliverable |
|-----------|-------------|
| 1.1 | WPF `MainWindow.xaml` with 4 tab framework (Install | Tweaks | Config | Updates) |
| 1.2 | `AdminChecker.cs` — auto-restart as admin if UAC not elevated |
| 1.3 | `RegistryService.cs` — read/write HKLM/HKCU, value enumeration |
| 1.4 | `RestorePointService.cs` — `SRSetRestorePoint` WMI call, sequence tracking |
| 1.5 | Embedded catalog JSON (tweaks/debloat/privacy from seed-data) |
| 1.6 | First-run wizard: "Create initial restore point, load catalog" |

**Success Criteria**: App launches as admin, loads 60+ tweaks from embedded JSON, creates restore point, can apply a tweak (writes to registry key), undo works via restore point.

---

### Phase 2 — Core Operations (4 weeks)
**Goal**: Tweaks engine + Debloat + Privacy rules + Full undo

| Milestone | Deliverable |
|-----------|-------------|
| 2.1 | `TweakEngine.cs` — apply/undo pipeline: snapshot → execute → audit → log |
| 2.2 | Registry operations executor: `RegSetValueEx`, `RegDeleteValue` with error handling |
| 2.3 | Debloat engine: `Get-AppxPackage` / `Remove-AppxPackage -AllUsers` via PowerShell SDK |
| 2.4 | Privacy rules: registry policy toggles under `HKLM\SOFTWARE\Policies\Microsoft\Windows\` |
| 2.5 | "Breaks features" warnings UI: show `breaksFeatures` list when tweak has high risk |
| 2.6 | Per-operation undo: each tweak operation recorded with inverse payload in SQLite audit DB |

**Success Criteria**: User can select tweaks → click "Apply" → restore point created → registry values changed → "Undo" button restores previous values → "Breaks features" warning displayed if applicable.

---

### Phase 3 — Advanced Operations (3 weeks)
**Goal**: Repair (DISM/SFC) + Windows Updates + DNS + ISO builder

| Milestone | Deliverable |
|-----------|-------------|
| 3.1 | `DismService.cs` — `Microsoft.Dism` NuGet: ScanHealth, RestoreHealth via DismApi.dll |
| 3.2 | `SfcService.cs` — execute `sfc /scannow` via `Process.Start`, capture output |
| 3.3 | `WindowsUpdateService.cs` — COM `IUpdateSession`: search, download, install, hide/unhide updates |
| 3.4 | `DnsService.cs` — `Set-DnsClientServerAddress` PowerShell or `netsh interface ip set dns` |
| 3.5 | `WindowsFeatureService.cs` — `Enable-WindowsOptionalFeature` / `Disable-WindowsOptionalFeature` |
| 3.6 | `IsoBuilderService.cs` — Windows ADK: Mount WIM → modify Appx packages → unmount → `Oscdimg` → SHA-256 |
| 3.7 | Network stack reset: `netsh int ip reset` + `netsh winsock reset` |

**Success Criteria**: Full system check runs SFC+DISM, updates can be searched/installed/hidden, DNS changes take effect, ISO can be built with bloatware removal and privacy tweak injection.

---

### Phase 4 — Polish & Release (2 weeks)
**Goal**: Signing, installer, auto-update, documentation

| Milestone | Deliverable |
|-----------|-------------|
| 4.1 | **Code signing**: Purchase EV certificate, sign EXE → "Verified Publisher" in UAC |
| 4.2 | **Installer**: Inno Setup or WiX — creates `C:\Program Files\WinForge\`, Start Menu, auto-elevate, uninstall |
| 4.3 | **Auto-update**: SmoothUpdate or custom HTTPS endpoint with SHA-256 verification |
| 4.4 | **Setup script** (`irm domain.com/win | iex`) that downloads latest EXE or instems via Scoop/Choco |
| 4.5 | **End-user documentation**: CHM help file, "What each tweak does" accordion, restore point guide |
| 4.6 | **Safety disclaimer**: Full-screen warning on expert/high-risk tweaks, "I understand this modifies Windows" checkbox |
| 4.7 | **Beta release**: Publish to GitHub Releases, collect feedback from 20-50 power users |

**Success Criteria**: Signed EXE runs without SmartScreen blocks, installer installs cleanly, auto-update works, user guides complete.

---

## MIGRATION FROM CURRENT CODEBASE

### What to Reuse (from Next.js app)
1. **Catalog data**: 60+ tweaks, 90+ debloat packages, 40+ privacy rules — exact same definitions
2. **Risk levels & categories**: `low`/`medium`/`high`/`expert` + tag system
3. **Health score algorithm**: `src/lib/health.ts` → port logic to C# `WinForge.Health.cs`
4. **Preset system**: 4 presets (Standard/Gaming/Privacy/Work) → XAML data binding
5. **Audit/undo structure**: `operationHistory` table design → adapt to SQLite + JSON payloads
6. **Quick wins generation**: Same scoring formula (50 baseline + bonuses/penalties)

### What to Rebuild (WPF native)
1. **UI**: Next.js/React → WPF/XAML + MVVM (styles, colors, animations differ)
2. **Data layer**: PostgreSQL → **SQLite** (embedded, no server needed) OR direct file JSON
3. **Simulation → Real**: DB state transitions → direct Windows API calls
4. **Deployment**: Next.js server → self-contained EXE + optional localhost service
5. **State persistence**: `localStorage` → `%APPDATA%\WinForge\` or SQLite

### Data Conversion Script (One-Time)
```typescript
// Run once: convert seed-data.ts → Resources/tweaks.json
// Then: msbuild /t:GenerateEmbeddedResources WinForge.Wpf.csproj
```

---

## IMMEDIATE NEXT STEPS

1. **Scaffold WPF project**: `dotnet new wpf -n WinForge.Wpf`
2. **Add .NET 8 SDK** reference
3. **Migrate first 5 tweaks** from seed-data to C# models + embedded JSON
4. **Implement AdminChecker + MainWindow** with 4 tab placeholders
5. **Create Phase 1 milestone** checklist and start coding
6. **Set up localization project structure** — add 5 `.resx` resource files (en-US, es-ES, fr-FR, de-DE, zh-CN) in `WinForge.Wpf/Resources/`; bind XAML `Language="{x:Static properties:Resources.AppLanguage}"`; use `CultureInfo` in `AdminChecker.cs` on first run

### Folder Structure (Proposed)
```
WinForge.Wpf/
├── WinForge.Wpf.csproj
├── App.xaml / App.xaml.cs
├── MainWindow.xaml + .cs
├── Resources/
│   ├── tweaks.json (embedded from seed-data)
│   ├── debloat.json
│   └── privacy.json
├── Models/
│   ├── TweakDefinition.cs
│   ├── DebloatPackage.cs
│   ├── PrivacyRule.cs
│   └── RiskLevel.cs
├── Services/
│   ├── AdminChecker.cs
│   ├── RegistryService.cs
│   ├── RestorePointService.cs
│   ├── TweakEngine.cs
│   ├── DebloatEngine.cs
│   ├── PrivacyEngine.cs
│   ├── DismService.cs
│   ├── WindowsUpdateService.cs
│   └── IsoBuilderService.cs
├── ViewModels/
│   ├── MainViewModel.cs
│   ├── TweaksViewModel.cs
│   ├── InstallViewModel.cs
│   ├── ConfigViewModel.cs
│   └── UpdatesViewModel.cs
├── Views/ (XAML pages for each tab)
├── Helpers/
│   └── JsonSnapshot.cs
└── Properties/
    └── Settings.settings
```

---
*Blueprint v1.0 — Generated from current WinForge Next.js codebase analysis. Target: native Windows WPF utility matching Chris Titus Tech Windows Utility spec.*