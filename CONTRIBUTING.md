# Contributing to WinForge

Thanks for your interest in improving WinForge. This document explains how to
set up a development environment, what the verification bar is, and how to
propose changes safely.

## Project layout

| Path | What it is |
|------|------------|
| `cmd/winforge/` | Go engine entrypoint |
| `internal/` | Go engine source (20 packages, stdlib-only) |
| `config/*.json` | Embedded catalogs (240 tweaks, 102 debloat, 83 apps, 40 privacy) |
| `web/` | Zero-dependency dashboard bundled into the .exe |
| `src/` | Next.js web control center (catalog of record for the web UI) |
| `tools/` | Python catalog tooling (parity, converter, locale extractor) |
| `examples/plugins/` | Example Lua and WASM plugin packs |
| `docs/` | Architecture decision records and design notes |
| `WinForge.Elite/` | Archived WPF/.NET scaffold (reference only — do not build new features here) |

The Go engine is the product. The Next.js app is the rich UI and the source
of the web-side catalog. A Python converter (`tools/web_catalog_to_engine.py`)
merges web tweaks into `config/tweaks.json`; `tools/catalog_parity.py` verifies
the two stay consistent.

## Development setup

### Go engine

The engine is stdlib-only (zero third-party modules). Go 1.22+ is required.

```bash
go build ./cmd/...
go test ./cmd/... ./internal/... .
go vet ./cmd/... ./internal/... .
```

> The package list is scoped to `./cmd/... ./internal/... .` deliberately:
> `./...` would recurse into `node_modules/`, where some npm dependencies ship
> `.go` files that are not part of this module.

On Debian-based sandboxes without a system Go, the repo documents a
from-source bootstrap in `docs/GO_TOOLCHAIN_BOOTSTRAP.md`.

### Next.js web app

```bash
npm install
cp .env.example .env   # then set DATABASE_URL
npm run dev
npm run typecheck
npm run lint
npm run build          # must pass with DATABASE_URL unset (lazy DB pool)
```

### Python tooling

```bash
python3 tools/catalog_parity.py            # exits 0 when web/engine catalogs agree
python3 tools/web_catalog_to_engine.py     # dry-run; --apply writes config/tweaks.json
python3 tools/extract_locales.py           # verifies all 5 locales have 56 keys
```

## Verification bar

Run the full battery before opening a PR:

```bash
make verify
```

This runs (mirroring CI):

1. `gofmt` on `cmd/`, `internal/`, `embed.go`
2. `go vet` (linux)
3. `go test` (all packages)
4. `go test -race` (all packages)
5. `go vet` + cross-compile for `GOOS=windows` (PE32+ header check)
6. Catalog parity + converter idempotence + locale sync + ISS/Autounattend dry-run
7. `npm run typecheck` + `npm run lint` + `npm run build`
8. JSON/JS syntax + `git diff --check`

All checks must pass. PRs that skip the battery will be asked to run it.

## Adding a new tweak

1. Add the entry to `src/db/seed-data.ts` (the web catalog of record) with:
   - A stable `id` (kebab-case, prefixed by category where sensible)
   - `operations[]` and `undoOperations[]` — **every** reversible tweak must
     have an explicit revert
   - A `risk` level (`low` / `medium` / `high` / `expert`)
   - Verified sources (learn.microsoft.com policy docs, AtlasOS playbooks)
2. Run `python3 tools/web_catalog_to_engine.py --apply` to merge it into
   `config/tweaks.json`.
3. Run `python3 tools/catalog_parity.py` — it must exit 0.
4. Add/adjust tests if the tweak introduces a new operation shape.
5. Run `make verify`.

### Security boundary for commands

The elevated executor only runs an allowlist of system executables
(`dism`, `w32tm`, `lodctr`, `winmgmt`, `rundll32`, `wevtutil`, `fsutil`,
`setx`, `bcdedit`, `netsh`). **PowerShell, `powercfg`, and `wmic` are
deliberately refused.** If a tweak needs one of those, implement the operation
natively in the engine (as was done for hibernation, processor state, and
NetBIOS) rather than weakening the allowlist.

## Writing plugins

Lua and WASM plugins can only propose a closed set of operations:
registry dword/string/qword sets, registry value deletes, and service
start-mode changes. Commands, Appx removal, tasks, power, NetBIOS, and
recursive key deletes are **forbidden** to plugins. See
`examples/plugins/` for working packs and `docs/LUA_PLUGIN_PLAN.md` /
`docs/WASM_PLUGIN_SANDBOX.md` for the design.

## Commit guidelines

- Keep commits focused; one logical change per commit.
- Write commit messages in the imperative mood ("Add NetBIOS native op").
- Do not commit binaries, `node_modules/`, `.next/`, or `*.tsbuildinfo`.
- Do not modify `.github/workflows/*` directly without maintainer approval
  (the CI token requires the `workflows` permission — see `docs/BLOCKED_ITEMS.md`).
- Update `AGENTS.md` when reality on the ground changes (it is the
  successor-agent memory file).

## Reporting bugs

Open an issue with:

- The `winforge version` output
- Windows version (10 22H2 / 11 24H2 / etc.)
- Whether WinForge was elevated
- The exact command or UI action and the observed error
- Relevant lines from `%LOCALAPPDATA%\WinForge\logs\operations-YYYY-MM-DD.jsonl`

## Licensing

By contributing, you agree your contributions are licensed under the MIT
License (see [LICENSE](LICENSE)).
