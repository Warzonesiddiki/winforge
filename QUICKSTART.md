# WinForge Quick Start

## For users

WinForge ships as a single self-contained `winforge.exe` (~6.5 MB, no runtime
required). Download the latest release from the
[Releases page](https://github.com/Warzonesiddiki/winforge/releases).

### Run the dashboard

Double-click `winforge.exe`, or from PowerShell:

```powershell
.\winforge.exe
```

This starts the dashboard at **http://127.0.0.1:8696** and opens it in your
default browser. The server binds only to loopback and requires a
per-session token for every change — see [SECURITY.md](SECURITY.md).

### Apply a tweak from the CLI

```powershell
# See what would change without modifying anything
.\winforge.exe apply --id tel-disable-telemetry --dry-run

# Apply it for real (creates a restore point first)
.\winforge.exe apply --id tel-disable-telemetry

# Undo it
.\winforge.exe undo --id tel-disable-telemetry
```

### Other common commands

```powershell
.\winforge.exe scan                          # health score
.\winforge.exe list                          # all tweaks + applied state
.\winforge.exe history                       # operation audit log
.\winforge.exe plugins                       # list installed plugin packs
.\winforge.exe install --id Mozilla.Firefox  # winget package
.\winforge.exe restore-point                 # manually create a restore point
.\winforge.exe run-maintenance               # re-verify + upgrade apps
.\winforge.exe help                          # full command list
```

### SmartScreen warning

WinForge is not code-signed (EV certificates are cost-prohibitive for a free
open-source project). On first run, Windows SmartScreen may show
"Windows protected your PC." Click **More info → Run anyway**. You can verify
the download against the SHA-256 checksum published with each release.

## For contributors

### Prerequisites

- Go 1.22+ (stdlib-only; no module download needed)
- Node.js 20+ and npm (for the Next.js control center)
- Python 3.10+ (for catalog tooling)

### Build the engine

```bash
# Linux/macOS (cross-compile for Windows)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
  -o winforge.exe ./cmd/winforge
```

### Run the web app

```bash
npm install
cp .env.example .env       # set DATABASE_URL
npm run dev                # http://localhost:3000
```

### Verify everything

```bash
make verify
```

This runs gofmt, vet, tests (with race detector), Windows cross-compile,
catalog parity, TypeScript typecheck, ESLint, production build, and syntax
checks. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.
