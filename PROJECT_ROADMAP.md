# WinForge Elite — Project Roadmap
## Phase-Gated Migration from Next.js Simulation → Native WPF Utility

### Roadmap Overview
| Phase | Title | Duration | Primary Goal | Key Deliverable |
|-------|-------|----------|-------------|-----------------|
| 1 | Foundation | 3 weeks | Admin elevation + Registry + Restore points | `WinForge.exe` launches as admin, loads catalog, creates RP |
| 2 | Core Operations | 4 weeks | Tweaks engine + Debloat + Privacy + Undo | Apply/undo any tweak, restore point safety net |
| 2.5 | Software Installer | 2 weeks | 60+ apps + winget batch install + category filtering | Installed detection, progress tracking, status badges |
| 3 | Advanced Operations | 3 weeks | DISM/SFC + Windows Update + DNS + ISO builder | Full system check, update management, ISO creation |
| 4 | Polish & Release | 2 weeks | Code signing + Installer + Auto-update + Docs | Signed EXE, Inno Setup installer, auto-update flow |
| *5* | *Localization & Polish* | *1 week (parallel with Phase 4)* | *5-language support + final UI polish* | *XAML Language binding, resource files, language selector in Settings* |

--- (Spoiler: Phase 5 runs in parallel with Phase 4 weeks 3-4. All 5 languages: en-US, es-ES, fr-FR, de-DE, zh-CN must be functionally complete before Beta release.)

---

## PHASE 1 — Foundation (3 Weeks)

### Week 1: Project Scaffold + Admin + Basic UI
| Day | Deliverable |
|-----|-------------|
| 1-2 | `dotnet new wpf -n WinForge.Wpf` → .NET 8 project |
| 3-4 | Add `Microsoft.Win32.Registry` usage, basic XAML with 4 TabControl tabs |
| 5 | `AdminChecker.cs`: Check `WindowsIdentity.IsAdmin` → if false, `RestartAsAdministrator()` via `process.StartInfo.Verb = "runas"` |
| 6-7 | Embed first catalog JSON (tweaks) from `seed-data.ts` → `Resources/tweaks.json` |

**Milestone 1 "Alpha Launch"**: 
- [ ] App compiles and runs
- [ ] UAC prompt appears on first run (or auto-restart)
- [ ] 60 tweaks loaded from embedded JSON, displayed in ListBox
- [ ] "Apply" button exists but does nothing yet (placeholder)

---

### Week 2: Registry Service + Restore Points
| Day | Deliverable |
|-----|-------------|
| 1-2 | `RegistryService.cs`: `KeyExists()`, `GetValue()`, `SetValue()`, `DeleteValue()` |
| 3-4 | WMI Restore Point: `ManagementObjectSearcher("SELECT * FROM Win32_SystemRestore")` → `CreateRestorePoint(description, 12, 100)` (reason codes: 12=manual, 100=other) |
| 5 | `SnapshotRegistryKeys(string keyPath)` → export to JSON file in `%APPDATA%\WinForge\Snapshots\` |
| 6-7 | Basic Tweak Apply pipeline: `Snapshot → SetRegistryValues → LogAudit → Revalidate` |

**Milestone 2 "Registry + RP"**:
- [ ] Tweak "Disable Telemetry" (`tel-disable-telemetry`) applies `HKLM\SOFTWARE\Policies\Microsoft\Windows\DataCollection:AllowTelemetry=0`
- [ ] Restore point created BEFORE mutation
- [ ] "Undo" button restores `AllowTelemetry=1`
- [ ] Undo failure gracefully handled (event log + message box)

---

### Week 3: First-Run Wizard + Safety Guards
| Day | Deliverable |
|-----|-------------|
| 1-2 | First-run flow: "Create initial restore point from current state" + "Load catalog" + "Explain safety model" |
| 3-4 | Protected resources list: certain tweak IDs marked `protected: true` → UI shows greyed-out with lock icon + "Cannot be modified" tooltip |
| 5-7 | Warning system: if `breaksFeatures` has items, show `MessageBox.Show($"This may break: {breaksFeatures.Join(", ")}", "Warning", MessageBoxButton.OK, MessageBoxImage.Warning)` before apply |

**Milestone 3 "Foundation Complete"**:
- [ ] App launches as admin (or requests elevation)
- [ ] Restore point created before any mutation
- [ ] Registry values change correctly
- [ ] Undo restores previous state
- [ ] Protected tweaks disabled in UI
- [ ] Warnings shown for risky tweaks

**Phase 1 Exit Criteria**:
✅ All above working  
✅ No crashes on tweak apply/undo  
✅ Catalog JSON embedded and loading correctly  
✅ Admin elevation working via `runas` verb

---

## PHASE 2 — Core Operations (4 Weeks)

### Week 1: Tweak Apply/Undo Pipeline (Full Version)
| Day | Deliverable |
|-----|-------------|
| 1-2 | `TweakEngine.Apply(TweakDef tweak)` method: |
|   |   a. `CreateRestorePoint($"Before: {tweak.Name}")` |
|   |   b. `Snapshot.CurrentState()` → JSON file + SQLite audit |
|   |   c. Execute `tweak.Operations` (registry keys + values) |
|   |   d. `LogAudit(new { Kind="tweak", Id=tweak.Id, Field="applied", OldValue=prev, NewValue=true })` |
|   |   e. `db.Tweaks.SetApplied(tweak.Id, true)` |
| 3-4 | `TweakEngine.Undo(string operationId)`: |
|   |   a. Read undo payload from SQLite (last audit row for this tweak) |
|   |   b. Execute inverse operations (`tweak.UndoOperations`) |
|   |   c. `db.Tweaks.SetApplied(tweak.Id, false)` |
|   |   d. Log `Undo` operation with `canUndo=false` |
| 5-7 | Per-tweak UI: "Applied" badge (green), "Undo" button visible only when `applied=true` |

**Milestone 4 "Tweak Pipeline"**:
- [ ] Apply button enables/disables correctly based on `tweaks.applied` state
- [ ] Undo button restores exactly the previous registry values
- [ ] Audit log entry created for every apply/undo
- [ ] Snapshot file written to disk (for debugging/manual recovery)

---

### Week 2: Debloat Engine (Appx Packages)
| Day | Deliverable |
|-----|-------------|
| 1-2 | `DebloatEngine.RemovePackage(string packageName)`: |
|   |   a. Check `package.Category == "Protected"` → if yes, show lock icon + "Cannot remove" |
|   |   b. `Get-AppxPackage -Name "PackageName"` → if found, `Remove-AppxPackage -Package "PackageName" -AllUsers` |
|   |   c. If not found → "already removed or not installed" |
|   |   d. Update `debloatPackages.status = "removed"` in local SQLite |
|   |   e. Log audit: `kind=debloat, id=packageName, field=status, oldValue=installed, newValue=removed` |
| 3-4 | `DebulkEngine.InstallPackage(string packageName)`: reverse operation (winget install or Appx add-back) |
| 5-7 | Bulk operations: `BulkRemovePackages(string[] names)` → iterate + call `RemovePackage` + create single restore point |

**Milestone 5 "Debloat"**:
- [ ] Remove Microsoft.BingNews package → Appx tile gone from Start menu
- [ ] "Protected" packages (Microsoft.Store, etc.) show lock + cannot remove
- [ ] Bulk remove 5 packages → single restore point + all undone together
- [ ] Status badges: "installed" (green), "removed" (grey), "protected" (grey with lock)

---

### Week 2.5: Software Installer (60+ Apps)
| Day | Deliverable |
|-----|-------------|
| 1-2 | `AppInstallEngine.Install(id)`: lookup winget manifest → `winget install --id "PackageID"` → capture progress output → update `applications.installed = true` in SQLite + log audit |
| 3-4 | `AppInstallEngine.BatchInstall(ids)`: iterate selected apps → show progress bar in UI → report successes/failures → log each as separate audit entry |
| 5-7 | Installed detection: `applications.installed` check against SQLite + system `winget list` → badge "installed" / "not installed" per app; Category filtering (Browsers, Dev Tools, Media, Utilities, Comms, Security, Gaming) |

**Milestone 5.5 "Software Installer"**:
- [ ] Install Google Chrome via winget → appears in Installed list with version badge
- [ ] Batch install 5 apps → progress bar updates per-app → final "X/Y installed" message
- [ ] Category filtering works (toggle Browsers → show/hide apps)
- [ ] Uninstall flow: `winget uninstall --id "PackageID"` → flip `installed` badge

---

### Week 3: Privacy Rules + "Harden All"
| Day | Deliverable |
|-----|-------------|
| 1-2 | `PrivacyRule` model: `Id`, `Name`, `Category`, `Risk`, `DefaultEnabled`, `Enabled` (current state) |
| 2-4 | `PrivacyEngine.HardenAll()`: iterate all `privacyRules` where `Enabled=false` → set `Enabled=true` + create restore point + log audit per rule |
| 5-7 | Individual rule toggle: click → flip `Enabled` state + snapshot + audit |
| 6-7 | Privacy score calculation (port from `src/lib/health.ts`): `enabledCount / totalCount * 100` → display gauge |

**Milestone 6 "Privacy"**:
- [ ] "Harden All" button → all 40+ rules enabled → privacy score 100
- [ ] Individual toggles work (enable/disable)
- [ ] Score updates live when rules change
- [ ] "May affect" warnings for high-risk rules (e.g., `priv-recall` risk=high)

---

### Week 4: Full Undo + Audit DB
| Day | Deliverable |
|-----|-------------|
| 1-2 | SQLite schema: `OperationHistory(Id, Timestamp, Kind, Target, PreviousValue, NewValue, Risk, CanUndo, Undone, UndoData JSON)` |
| 3-4 | "Undo All Today" button: select all `canUndo=1 ∧ undone=0 ∧ timestamp≥today` → loop `undoOperation(id)` |
| 5-7 | Export audit → CSV: `OperationHistory.ExportCsv(path)` for user records |
| 6-7 | Restore point chain view: "View all restore points" → list with descriptions + "Restore to this point" |

**Milestone 7 "Undo/Audit"**:
- [ ] Every apply/undo creates SQLite row with full undo payload
- [ ] "Undo All Today" works (undoes all reversible ops from current day)
- [ ] CSV export includes: timestamp, operation type, target, risk, undo data preview
- [ ] Restore point history shows chronological list with descriptions

**Phase 2 Exit Criteria**:
✅ Tweak apply/undo works for all 60+ tweaks  
✅ Debloat remove/reinstall works for 90+ packages  
✅ Privacy Harden All + individual toggles work  
✅ SQLite audit DB has complete history  
✅ "Undo All Today" functions correctly  
✅ Protected packages correctly blocked  
✅ Warnings shown for breaksFeatures  
✅ Software Installer: batch install + category filtering + installed detection badges  
✅ Per-app undo payload logged to audit DB

---

## PHASE 3 — Advanced Operations (3 Weeks)

### Week 1: Repair (DISM + SFC)
| Day | Deliverable |
|-----|-------------|
| 1-2 | `DismService.ScanHealth()`: `Microsoft.Dism.DismApi.ScanHealth(image)` → return error codes |
| 3-4 | `DismService.RestoreHealth()`: `Microsoft.Dism.DismApi.RestoreHealth(image)` → "The restore operation completed successfully" |
| 5-7 | `SfcService.RunSfc()`: `Process.Start("sfc", "/scannow")` → redirect stdout/stderr to UI richtextbox → "Windows Resource Protection did not find any integrity violations" |
| 6-7 | Log both operations to audit DB with `risk: "low"/"medium"`, `canUndo: false` (system scans are one-way) |

**Milestone 8 "Repair"**:
- [ ] DISM Scan Health → shows "No corruption detected" or lists errors
- [ ] DISM Restore Health → "Successfully repaired" or "Failed - source not available"
- [ ] SFC /scannow → captures output in UI, doesn't block app
- [ ] Both logged as irreversible operations (canUndo=false)

---

### Week 2: Windows Updates + DNS
| Day | Deliverable |
|-----|-------------|
| 1-2 | `WindowsUpdateService.SearchUpdates()`: COM `IUpdateSession -> CreateSearcher -> Search("IsInstalled=0")` → return list of `{Id, Title, KB, Severity, Installed}` |
| 3-4 | `WindowsUpdateService.Install(updateIds)`: download + install selected updates → progress bar + log |
| 5-6 | `WindowsUpdateService.Hide(unableIds)`: COM `IUpdateCollator -> Hide updates` |
| 6-7 | `DnsService.SetPreset(string presetId)`: from `src/db/dns-presets.ts` → `Set-DnsClientServerAddress -Source "DHCP"` or static IP config |

**Milestone 9 "Updates + DNS"**:
- [ ] Update search returns list with KB numbers, severity badges
- [ ] Install selected updates → progress bar, success/failure message
- [ ] Hide/unhide updates → KB marked hidden in UI
- [ ] DNS preset changes take effect immediately (or require restart)

---

### Week 3: ISO Builder (MicroWin-style)
| Day | Deliverable |
|-----|-------------|
| 1-2 | Windows ADK check: `if (!Directory.Exists("C:\Program Files\Windows Kits\10")) → show "Install Windows ADK for ISO building"` |
| 3-4 | `IsoBuilderService.MountWim(string isoPath, string mountDir)`: `Dismount-Image -ImagePath ... -MountDir ...` |
| 5-6 | Remove bloatware packages from mounted WIM: `Remove-AppxPackage -Package "PackageName" -Online -WhenUsed` (via DismApi) |
| 7 | Inject privacy tweaks into mounted WIM registry hive: `Reg Load HKLM\COMPUTER "C:\path\SOFTWARE"` → write keys → `Reg Unload` |
| 7 | Unmount + generate ISO: `Dismount-Image` → `Oscdimg -beta -o -udf -xor -iso -ch -n "WinForge-Win11-Pro.iso" "mountDir\"` |
| 7 | Calculate SHA-256 of output ISO → display to user |

**Milestone 10 "ISO Builder"**:
- [ ] User selects Windows 11 Pro ISO → app validates Windows ADK presence
- [ ] "Remove bloatware" option → removes 70+ Appx packages from offline image
- [ ] "Apply privacy tweaks" → injects registry hive modifications
- [ ] "Remove Edge/OneDrive/Recall" options → checkboxes
- [ ] "TPM/Secure Boot bypass" → patches appraiserres.dll (simulated or actual depending on legality)
- [ ] Output ISO path + SHA-256 checksum displayed
- [ ] Log window shows each step: "Mounting ISO...", "Removing packages...", "Injecting tweaks...", "Generating ISO..."

**Phase 3 Exit Criteria**:
✅ DISM SFC operations work and are logged  
✅ Windows Update search/install/hide works  
✅ DNS preset applies network config  
✅ ISO builder: mount → modify → unmount → generate ISO + SHA-256  
✅ User can select bloatware removal + privacy tweak injection options  
✅ All advanced ops logged to audit DB

---

## PHASE 4 — Polish & Release (2 Weeks)

### Week 1: Code Signing + Installer
| Day | Deliverable |
|-----|-------------|
| 1-2 | Purchase **EV code signing certificate** (from DigiCert/GlobalSign) → `signtool sign /f cert.pfx /p password WinForge.Wpf.exe` |
| 3-4 | Inno Setup script: |
|   |   - App name: "WinForge Elite" |
|   |   - Publisher: "WinForge Software" |
|   |   - Start Menu folder |
|   |   - Desktop shortcut creation option |
|   |   - "Run as administrator" checkbox |
|   |   - Uninstall support |
|   |   - Readme with safety disclaimer |
| 5-7 | Test signed EXE: SmartScreen should show "Verified Publisher" |

**Milestone 11 "Signing + Installer"**:
- [ ] EXE signed with EV cert → no SmartScreen warning
- [ ] Inno Setup installer creates proper install directory
- [ ] Start Menu shortcuts work (Launch, Undo History, Documentation)
- [ ] Uninstall removes all files + SQLite DB + snapshots
- [ ] Auto-update check mechanism (basic: compare local version vs GitHub releases API)

---

### Week 2: Auto-Update + Documentation + Release
| Day | Deliverable |
|-----|-------------|
| 1-2 | Auto-update stub: `WinForge.Wpf.exe --check-update` → HTTPS API `https://api.winforge.app/version` → compare `currentVersion` vs `latestVersion` → if newer, download `.exe` from GitHub Releases → `Process.Start("WinForge.Update.exe")` |
| 3-4 | CHM help file or embedded "?" button → opens default browser to `https://winforge.app/docs` |
| 5-6 | Safety disclaimer screen on first run: "WinForge modifies Windows system settings. Creates restore points before every change. Expert-level tweaks may affect system stability. By clicking Continue, you acknowledge ..." |
| 7 | Beta release strategy: Publish to GitHub Releases → invite 20-50 power users → collect feedback form → prioritize Phase 1 fixes |

**Milestone 12 "Release"**:
- [ ] Auto-update checks and downloads newer versions
- [ ] Help/documentation accessible
- [ ] Full safety disclaimer on first run
- [ ] Beta testers invited, feedback form live
- [ ] GitHub Release v1.0.0 with "Foundation Complete" features

**Phase 4 Exit Criteria**:
✅ Signed EXE runs without SmartScreen blocks  
✅ Inno Setup installer installs + uninstalls cleanly  
✅ Auto-update finds and installs point releases  
[ ] Help/docs accessible  
[ ] First-run disclaimer present  
[ ] Beta testers on board, feedback collected  
[ ] GitHub Release published with clear roadmap

---

## SUCCESS METRICS (Target Product)

| Metric | Target | How Measured |
|--------|--------|--------------|
| **Launch time** | < 3 seconds | Stopwatch on first run |
| **Tweak apply latency** | < 500ms per tweak | Time from "Apply" button to UI update |
| **Restore point creation** | < 2 seconds | WMI `CreateRestorePoint` timing |
| **Undo accuracy** | 100% — restores exact previous state | Compare registry values before/after undo |
| **Catalog completeness** | 100% — all 60 tweaks, 90 packages, 40 rules | Parse seed-data.ts → verify embedded JSON has all entries |
| **Protected resources** | 0 accidental modifications | UI shows lock, prevents API call |
| **Code signing** | EV cert, SmartScreen "Verified Publisher" | `signtool verify` + UI test |
| **Installer reliability** | 0 failures on clean Windows 11 install | Inno Setup test on 3 fresh VMs |
| **Auto-update success** | ≥ 95% | Beta tester reports over 4 weeks |

---

## ROLLBACK PLAN IF NEEDED

If the native app proves too complex in first 2 weeks:

1. **Keep Next.js as-is** (simulation mode) — it's already functional
2. **Build a thin PowerShell wrapper** instead of C# WPF:
   ```powershell
   # Windows-admin script that does the real ops
   # Called from Next.js via `child_process.exec` with quoted params
   # Still has safety (RP + undo) but uses PS native cmdlets
   ```
3. **Hybrid approach**: Next.js UI + localhost .NET service for the Windows ops (keeps web UI, gets real operations)

**Decision point**: After Phase 1 (Week 3), evaluate if WPF path is viable. If admin elevation or registry access proves problematic, switch to PowerShell hybrid model but keep the roadmap same.

---
*Roadmap v1.0 — Based on WinForge Next.js codebase + Chris Titus Tech Windows Utility specifications. All timelines assume 1 senior .NET developer + 1 part-time catalog migration specialist.*