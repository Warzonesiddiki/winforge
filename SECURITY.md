# Security Policy

## Reporting a vulnerability

WinForge is a system-modification tool, so security issues are taken
seriously. **Please do not open public GitHub issues for security
vulnerabilities.** Instead, email the maintainers with:

- A description of the issue and its impact
- Steps to reproduce (proof-of-concept if possible)
- The WinForge version and Windows version affected
- Whether the issue is in the Go engine, the web dashboard, or the Next.js app

We aim to acknowledge within 72 hours and provide a fix timeline within
14 days.

## Threat model

WinForge runs locally and modifies Windows system state. The security model
defends against:

- **Cross-origin / DNS-rebinding attacks** against the loopback dashboard
  (`127.0.0.1:8696`). The server enforces a loopback `Host` header,
  same-origin `Origin`/`Sec-Fetch-Site` checks, and a per-instance
  `X-WinForge-Token` session token on every mutating request (32 random
  bytes, `crypto/rand`, constant-time compare).
- **Privilege confusion** between elevated and standard-user processes. An
  elevated WinForge deliberately ignores user-writable config override
  directories, plugins, and the user-profile audit log, so a planted file
  cannot turn into an administrator mutation.
- **Command injection** through catalog or plugin operations. The elevated
  executor only runs an allowlist of Windows system binaries by absolute
  path; PowerShell, `powercfg`, and `wmic` are refused.
- **Registry destruction**. `registry_delete_key` requires a path at least
  two components deep; protected services cannot be started/stopped/disabled;
  every mutation best-effort-creates a system restore point (throttled to
  one per hour).
- **Audit log tampering**. The append-only JSONL log rejects symlinks and
  non-regular files at open time, verifies the file did not change between
  `Lstat` and `open`, and bounds all reads.
- **Runaway plugins**. Lua scripts are bounded by a 10M-instruction hook;
  WASM modules have a 10M fuel budget, 4 MiB size cap, and only whitelisted
  host imports.

### Out of scope

- Physical access to the machine
- A malicious administrator (the user running elevated WinForge)
- Malware already running with the same or higher privilege
- The unsigned-release SmartScreen warning (see README — EV signing is a
  documented non-goal for the free open-source distribution)

## Security-relevant configuration

| Setting | Default | Purpose |
|---------|---------|---------|
| `WINFORGE_NO_RESTORE_POINT` | unset | Set to `1` to disable automatic restore points (not recommended) |
| `WINFORGE_DATA_DIR` | `%LOCALAPPDATA%\WinForge` | Override the data/log/plugin directory |
| Dashboard bind host | `127.0.0.1` | Cannot be changed to a non-loopback address |

## Disclosure timeline

1. Report received → acknowledge within 72 hours
2. Triage → confirm and assign severity within 7 days
3. Fix prepared on a private branch
4. Patch released + advisory published (GitHub Security Advisory)
5. Public disclosure after users have had reasonable time to update
