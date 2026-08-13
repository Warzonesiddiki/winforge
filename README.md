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
- **COM-heavy features are phased** — WMI restore points, Appx `PackageManager`,
  Task Scheduler, DISM, and Windows Update COM interop are large, hand-rolled
  P/Invoke efforts. They are stubbed now (they return a clear
  `not implemented` error) and land in later phases.

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
  registry/                stdlib-only advapi32 registry client (windows + stub)
  service/                 Service Control Manager: start type, start/stop
  platform/                elevation check + OS identity (build-tagged)
  tweak/                   orchestrator: apply, dry-run, undo, health score
  audit/                   append-only JSONL operation log
  engine/                  concrete Executor (registry+service+guards)
  appmanager/              winget.exe wrapper (streamed progress)
  httpapi/                 dashboard server + JSON API + async install jobs
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

### Operation types

`registry_set_dword`, `registry_set_string`, `registry_delete`,
`service_start_mode`, `service_start`, `service_stop`, `task_disable`,
`task_enable`, `task_delete`, `appx_remove`, `command`.

---

## Roadmap (later phases)

- [ ] WMI restore points (`SystemRestore` via COM)
- [ ] Appx removal (`PackageManager` WinRT interop)
- [ ] Task Scheduler enable/disable/remove (COM)
- [ ] Windows Update search/install (COM `Microsoft.Update.Session`)
- [ ] DNS per-adapter configuration (WMI/IP Helper)
- [ ] Windows features via `dism.exe` (native, no PowerShell)
- [ ] One-click fixes (reset Windows Update, repair image, network reset)
- [ ] ISO builder (MicroWin-style)
- [ ] Plugin system (`%LOCALAPPDATA%\WinForge\plugins\`)
- [ ] Scheduled maintenance task
- [ ] Smart bloatware detection + recommendations

## Security note

WinForge modifies system state. It should be run **as Administrator** (the
dashboard shows elevation status), and its dashboard must remain bound to
localhost. Always review tweaks before applying.
