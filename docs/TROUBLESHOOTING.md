# Troubleshooting

## SmartScreen: "Windows protected your PC"

WinForge is not code-signed (EV certificates are cost-prohibitive for a free
open-source project). On first run, Windows SmartScreen may block the app:

1. Click **More info**
2. Click **Run anyway**

Verify the download against the SHA-256 checksum published with each GitHub
Release.

## "Access denied" when applying HKLM tweaks

Tweaks under `HKEY_LOCAL_MACHINE` require administrator privileges:

1. Right-click `winforge.exe`
2. Select **Run as administrator**
3. The dashboard pill in the lower-left will show **Administrator** when
   elevated.

The engine deliberately refuses to cross the privilege boundary — a standard-
user process will not read user-writable config when elevated, and vice versa.

## "lua54.dll not found"

The Lua plugin tier needs the Lua runtime next to `winforge.exe`:

1. Download `lua54.dll` from the latest GitHub Release (it is built by
   `native/build-lua.sh`), **or**
2. Build it yourself: `./native/build-lua.sh` (requires Zig ≥ 0.14)
3. Place `lua54.dll` in the same directory as `winforge.exe`

Without it, Lua packs are skipped with a log message; the rest of the engine
works normally.

## "WASM runtime unavailable"

This is expected on current releases. The WASM plugin tier's platform-
independent validation is implemented and tested, but the production Windows
C binding (wasmtime) is deferred until a Windows-native contributor can
verify it. Lua is the shipped scripting tier. See
[ADR-003-wasm-lua-only.md](ADR-003-wasm-lua-only.md).

## "Cannot create restore point"

The engine calls System Restore before every mutation. If it fails:

- Ensure **System Protection** is enabled for the C: drive
  (System Properties → System Protection → Configure)
- Ensure the Volume Shadow Copy service is running
- Check that there is free disk space
- Set `WINFORGE_NO_RESTORE_POINT=1` to suppress automatic restore points
  (not recommended — you lose the safety net)

## Dashboard won't load at 127.0.0.1:8696

- Check that `winforge.exe` is still running (it runs in the foreground)
- Verify no other process is bound to port 8696 (`netstat -ano | findstr 8696`)
- Try a different port: `winforge.exe serve --port 9000`
- Check your firewall allows loopback (most do by default)
- The server binds only to loopback (`127.0.0.1`); it is not reachable
  from the network by design.

## "Session rejected" / 401 error in browser

The dashboard uses a per-session token that rotates on restart. If the
engine was restarted while the page was open, refresh the page to fetch a
new token. If the problem persists, clear your browser's cache for
`127.0.0.1`.

## 429 "Too Many Requests"

The HTTP API rate-limits mutating requests to 5/second (burst 15) per
remote address as defense-in-depth. Wait one second and retry. If you are
scripting the API, add a small delay between calls.

## Build fails: "DATABASE_URL is required" (web app)

The Next.js control center needs PostgreSQL for runtime data. For a
production build **without** a database (e.g. CI):

```bash
DATABASE_URL= npm run build
```

The DB pool is lazily constructed, so the build succeeds without a live
database. For development, copy `.env.example` to `.env` and set
`DATABASE_URL`.

## `go test` fails under `node_modules/`

Always scope Go commands to the source directories:

```bash
go test ./cmd/... ./internal/... .
```

Using `./...` walks into `node_modules/`, where some npm dependencies (e.g.
`flatted`) ship `.go` files that are not part of this module.

## Getting more help

- Search [existing issues](https://github.com/Warzonesiddiki/winforge/issues)
- Open a bug report using the issue template (include `winforge.exe version`,
  Windows version, elevation status, and relevant lines from
  `%LOCALAPPDATA%\WinForge\logs\operations-YYYY-MM-DD.jsonl`)
- Report security issues privately — see [SECURITY.md](../SECURITY.md)
