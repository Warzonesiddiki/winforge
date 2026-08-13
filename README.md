# WinForge

A **self-contained Windows tuning & maintenance toolkit**, shipped as a single
static binary with a built-in web dashboard and a CLI. Zero third-party module
dependencies, zero runtime requirements.

```
winforge.exe            # double-click → opens the dashboard in your browser
winforge serve          # run the dashboard without opening a browser
winforge list           # list tweaks and their applied state
winforge scan           # show the health score
winforge apply  --id <id> [--dry-run]
winforge undo   --id <id>
winforge history
winforge install --id <winget-id>
winforge search  <query>
winforge restore-point [--description "…"]
winforge restore-points    # list existing restore points (WMI)
winforge plugins           # list installed plugins
winforge run-maintenance   # verify tweak states + upgrade apps
winforge schedule          # register the weekly maintenance task
winforge unschedule        # remove the weekly maintenance task
  winforge build-iso --source <dir> --output <iso> [--label <label>] [--edition <name>]...
  winforge build-iso --source <dir> --list-editions
  winforge updates [--installed]       # search Windows Update
  winforge install-updates             # download + install available updates
  winforge reset-windows-update | repair-image | flush-dns | network-reset
winforge set-dns --primary <ip> [--secondary <ip>] [--adapter <name>]
winforge enable-feature  --name <feature>
winforge disable-feature --name <feature>
winforge version
```

---

## Why Go, single binary, embedded web UI

WinForge manipulates Windows internals (registry, services, WMI, Appx, DISM,
Windows Update). It is not a CRUD app — its value is Windows-native glue. This
design optimizes for the stated constraints:

| Constraint | How it's met |
|------------|--------------|
| **Self-contained** | One static `.exe`; config + dashboard UI embedded via `go:embed`. |
| **Zero dependencies** | Standard library only. Registry/Services via raw `advapi32` P/Invoke (`syscall`). No `go get`, no `vendor/`. |
| **Cross-compilable** | `GOOS=windows GOARCH=amd64 go build` from Linux/macOS/Windows. |
| **Hybrid engine + UI** | Native Go engine and the web dashboard live in *one* binary (no FFI, no second toolchain). |

### Trade-offs (accepted deliberately)

- **No native WPF/Mica chrome** — the dashboard is served on `127.0.0.1` and
  opened in the browser. This is also a *security* choice: a system-control
  surface must not be network-reachable.
- **COM-heavy features are phased** — restore-point *listing* is implemented via
  raw WMI COM interop (the `SystemRestore` class), while Appx `PackageManager`,
  Task Scheduler, and Windows Update COM interop remain large, hand-rolled
  P/Invoke efforts that land in later phases.

---

## Build

Requires **Go 1.22+**. From any OS:

```bash
# Local / Windows build
go build -o winforge.exe ./cmd/winforge

# Cross-compile the Windows exe from Linux/macOS
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o winforge.exe ./cmd/winforge

# Stamp a version
go build -ldflags "-X winforge/internal/app.Version=1.0.0" -o winforge.exe ./cmd/winforge
```

> The binary is static and does **not** require `CGO_ENABLED=1`. No runtime is
> needed on the target machine.

Run tests (portable logic — orchestrator, config, audit, health):

```bash
go test ./...
```

---

## Architecture

```
cmd/winforge/              entrypoint (thin)
embed.go                   embeds web/ and config/ into the binary
web/                       dashboard (zero-dependency HTML/CSS/JS)
config/                    default tweaks, applications, DNS, protected services
internal/
  app/                     composition root (wires everything, shared by CLI+HTTP)
  config/                  config models + loader (embedded → user override)
  plugin/                  plugin discovery + merge (manifest.json + tweaks.json)
  registry/                stdlib-only advapi32 registry client (windows + stub)
  service/                 Service Control Manager: start type, start/stop
  platform/                elevation check + OS identity (build-tagged)
  tweak/                   orchestrator: apply, dry-run, undo, health score
  audit/                   append-only JSONL operation log
  engine/                  concrete Executor (registry+service+scheduler+appx)
  appmanager/              winget.exe wrapper (streamed progress)
  restorepoint/            System Restore points via SRSetRestorePointW P/Invoke
  scheduler/               Task Scheduler control via schtasks.exe
  bloatware/               bloatware detection (registry uninstall keys + rules)
  isobuilder/              ISO builder (dism edition export + oscdimg)
  updater/                 Windows Update search/install (COM Microsoft.Update.Session)
  maintenance/             one-click fixes + DNS + Windows features via DISM/netsh
  httpapi/                 dashboard server + JSON API + async jobs
  cli/                     command-line interface
```

### Platform partitioning

Windows-specific code lives in `*_windows.go` files; non-Windows equivalents in
`*_other.go`. Everything above that seam (config, orchestration, audit, health,
HTTP API, CLI) is portable and unit-tested, so the logic is verifiable on any
OS while the mutation layer targets Windows.

### Safety model

- Every mutation is **logged** (JSONL under `%LOCALAPPDATA%\WinForge\logs\`).
- Registry operations record their **previous value**, enabling **per-row undo**
  from the History view.
- Reversible tweaks carry an explicit **revert list**.
- A **system restore point** is created (best-effort, throttled to once/hour)
  before the first mutation — the safety-first policy. Disable with the
  `WINFORGE_NO_RESTORE_POINT` env var.
- A **protected-services** list blocks start-mode changes to critical services
  (e.g. `WinDefend`).
- **Dry-run** (`--dry-run`, or the API's `dryRun` flag) reports what *would*
  change without mutating anything.
- The dashboard binds to `127.0.0.1` only.

### Health score

```
100 - (unapplied_low × 2) - (unapplied_medium × 5) - (unapplied_high × 10) - (bloatware × 3)
```

clamped to `[0, 100]`.

### Scheduled maintenance

`winforge schedule` registers a weekly Task Scheduler task that runs
`winforge run-maintenance`. The maintenance pass re-applies any tweak that is
not in its target state and upgrades outdated winget apps (`winget upgrade
--all`), streaming progress to the dashboard. Each pass is recorded in the
audit log, and a restore point is taken (throttled) before any mutation.

### Bloatware detection

The dashboard scans the registry uninstall keys (HKLM 32/64-bit and HKCU) and
matches display names against a curated bloatware list (exact names plus
family signatures). When more than 5 bloatware apps are found, the dashboard
shows a recommendation banner and the count is folded into the health score.

### ISO builder

`winforge build-iso` builds a bootable Windows ISO from an extracted (or
mounted) installation source. It optionally slims the image by exporting only
the chosen editions (`dism /Export-Image`) and rebuilds a BIOS/UEFI-bootable
ISO with `oscdimg.exe` from the Windows ADK Deployment Tools. The user's source
directory is never modified — edition slimming happens in a scratch copy.

### Windows Update

`winforge updates` searches for available (or installed) updates and
`winforge install-updates` downloads and installs them. Both use the Windows
Update Agent COM API (`Microsoft.Update.Session`) via raw ole32/oleaut32
P/Invoke — no PowerShell, no `wuauclt` shell-outs.

---

## Configuration

Defaults are embedded; drop overrides into `%LOCALAPPDATA%\WinForge\config\`
(or set `WINFORGE_DATA_DIR`):

| File | Purpose |
|------|---------|
| `tweaks.json` | Declarative tweaks: ordered operations + optional revert list. |
| `applications.json` | winget app catalog (50 apps across categories). |
| `dns.json` | DNS presets (Cloudflare, Google, Quad9, OpenDNS). |
| `protectedServices.json` | Services that must not be modified. |

### Plugins

Drop a plugin into `%LOCALAPPDATA%\WinForge\plugins\<name>\` (or
`<dataDir>\plugins\<name>\`) to extend WinForge without recompiling. A plugin
directory contains:

| File | Purpose |
|------|---------|
| `manifest.json` | `{"name","version","description","author"}` metadata. |
| `tweaks.json` | Extra tweaks, same schema as the built-in `tweaks.json`. |

Plugins are scanned at startup and their tweaks are merged into the
configuration. On id collisions, built-in (and user-override) tweaks win.
Malformed plugins are skipped (best-effort).

### Operation types

`registry_set_dword`, `registry_set_string`, `registry_delete`,
`service_start_mode`, `service_start`, `service_stop`, `task_disable`,
`task_enable`, `task_delete`, `appx_remove`, `command`.

---

## Roadmap (later phases)

- [x] System Restore points (`SRSetRestorePointW` P/Invoke — no WMI/COM)
- [x] Task Scheduler enable/disable/delete (`schtasks.exe`)
- [x] Windows features via `dism.exe` (native, no PowerShell)
- [x] One-click fixes (reset Windows Update, repair image, network reset, flush DNS)
- [x] DNS per-adapter configuration (`netsh`, `net.Interfaces` discovery)
- [x] Provisioned Appx removal via `dism.exe`
- [x] List existing restore points (WMI `SystemRestore` class)
- [ ] Per-user Appx removal (`PackageManager` WinRT interop)
- [x] ISO builder (MicroWin-style: edition slim via dism + bootable ISO via oscdimg)
- [x] Plugin system (`%LOCALAPPDATA%\WinForge\plugins\`)
- [x] Scheduled maintenance task registration + `run-maintenance`
- [x] Smart bloatware detection + recommendations
- [x] Windows Update search/install (COM `Microsoft.Update.Session`)

## Security note

WinForge modifies system state. It should be run **as Administrator** (the
dashboard shows elevation status), and its dashboard must remain bound to
localhost. Always review tweaks before applying.
