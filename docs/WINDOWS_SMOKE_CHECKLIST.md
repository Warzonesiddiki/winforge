# WinForge — Windows Runtime Smoke Checklist (BLK-6)

> **Why this file exists:** the development sandbox is Debian Linux with no
> Windows runtime (BLK-6 in [BLOCKED_ITEMS.md](./BLOCKED_ITEMS.md)). Windows
> behavior is verified by cross-compilation, `GOOS=windows go vet`/`go test -c`,
> and code reading — but the items below can only be proven on a real Windows
> machine. Execute this checklist on Windows 10 22H2 or Windows 11 with the
> current `winforge.exe` (built by CI or via
> `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o winforge.exe ./cmd/winforge`).
>
> **Recording protocol:** for every item note PASS / FAIL / N/A, the exact
> command, and capture console output (append `> logs\NN.txt 2>&1` or copy the
> terminal). File issues for failures; do not silently patch.

## Preconditions

- [ ] Test machine is disposable or snapshotted (VM checkpoint / full backup).
- [ ] `winforge.exe` SHA-256 recorded, built from a known commit.
- [ ] Both an elevated (Administrator) and a non-elevated terminal available.

## 1. Basics (non-elevated)

- [ ] 1.1 `winforge.exe version` prints the version banner.
- [ ] 1.2 `winforge.exe list` renders **240 tweaks**, every row has a
      human-readable name (no blank names — the atlas metadata backfill and the
      2026-08-16 additions must all render).
- [ ] 1.3 `winforge.exe scan` prints a health score plus applied/unapplied
      counts. NOTE: the count excludes one-way ops by design
      (`UnverifiableTweaks`) — do not report the mismatch vs. 240 as a bug.
- [ ] 1.4 `winforge.exe history` on a fresh data dir prints an empty history,
      not an error.

## 2. Elevation behavior

- [ ] 2.1 Applying an HKLM tweak from a NON-elevated shell fails with a clear
      access-denied style message (no partial writes recorded as success).
- [ ] 2.2 The same command from an elevated shell succeeds.
- [ ] 2.3 Elevated processes ignore user-writable config overrides and plugin
      directories (verify: drop a plugin dir as a standard user, run elevated
      `list`, plugin tweaks must NOT appear — UAC boundary).

## 3. Restore points

- [ ] 3.1 `winforge.exe restore-point --description "winforge smoke"` (elevated)
      succeeds.
- [ ] 3.2 The point is visible via `Get-ComputerRestorePoint` (PowerShell, run
      manually — WinForge itself never invokes PowerShell) or `rstrui.exe`.

## 4. Tweak apply / verify / undo roundtrip

- [ ] 4.1 Elevated: apply `tel-disable-telemetry`. Verify with `reg query
      "HKLM\SOFTWARE\Policies\Microsoft\Windows\DataCollection" /v AllowTelemetry`
      → `0x0`.
- [ ] 4.2 `winforge.exe scan` now counts it applied; `history` shows the entry
      with a captured previous value.
- [ ] 4.3 Undo via history restores the prior state (value or absence);
      `history` shows the undo entry; a second undo of the same entry is
      refused ("already been undone").

## 5. NEW native ops (added 2026-08-16 — first Windows validation)

- [ ] 5.1 **power_hibernate**: elevated, apply `pwr-hibernation` (disables
      hibernation via `CallNtPowerInformation(SystemReserveHiberFile)`).
      Verify `powercfg /a` (manual) no longer lists Hibernate and
      `C:\hiberfil.sys` is gone. Undo re-enables it (hiberfil.sys returns).
      Dry-run (`--dry-run`) must report changed=true/false correctly against
      the current state (reads via `IsPwrHibernateAllowed`).
- [ ] 5.2 **power_processor_state**: apply `pwr-processor-mgmt` (min=50).
      Verify with `powercfg /q SCHEME_CURRENT SUB_PROCESSOR PROCTHROTTLEMIN`
      (manual) → AC index 0x32. Undo restores min=5. Confirm the change
      surfaces in Power Options → Processor power management.
- [ ] 5.3 **netbios**: apply `net-disable-netbios`. Verify every
      `HKLM\SYSTEM\CurrentControlSet\Services\NetBT\Parameters\Interfaces\Tcpip_*`
      subkey has `NetbiosOptions = 2` and the adapter WINS tab shows
      "Disable NetBIOS over TCP/IP". Undo sets 0 (DHCP default).
- [ ] 5.4 **registry default value + registry_delete_key**: apply
      `ui-classic-context`, verify
      `HKCU\SOFTWARE\Classes\CLSID\{86ca1aa0-34aa-4e8b-a509-50c905bae2a2}\InprocServer32`
      exists with an empty `(Default)` value and Explorer (after restart)
      shows the classic context menu. Undo removes the whole CLSID key
      (`registry_delete_key`); confirm the key is gone.

## 6. NEW privacy tweaks (added 2026-08-16, spot checks)

- [ ] 6.1 `winforge-deny-camera-access` → Settings > Privacy & security >
      Camera shows access denied;
      `HKLM\...\CapabilityAccessManager\ConsentStore\webcam\Value = Deny`.
      Undo deletes the value and Settings control returns.
- [ ] 6.2 `winforge-disable-recall` (on a Copilot+ machine, else N/A) →
      Recall snapshot saving off.
- [ ] 6.3 `winforge-disable-mdns` → after reboot, `netstat -ano | findstr 5353`
      shows no svchost listener (Chromium browsers may still open their own).

## 7. Guards must refuse

- [ ] 7.1 A `service_stop` against a protected service (e.g. `WinDefend`)
      refuses with a "protected" message BEFORE touching the SCM.
- [ ] 7.2 A malformed service name (`"WinDefend "` with a trailing space, or a
      name with a backslash) is rejected by validation, not by the SCM.
- [ ] 7.3 Elevated `command` op with a non-allowlisted executable (e.g.
      powershell) refuses with "not an allowlisted Windows system executable".

## 8. Dashboard

- [ ] 8.1 `winforge.exe serve` → http://localhost:8696 renders; tweak toggles,
      health gauge, history view work; `GET /api/status` and `/api/health`
      return JSON.
- [ ] 8.2 Applying a tweak from the dashboard writes the same audit entries as
      the CLI.

## 9. Appx / bloatware

- [ ] 9.1 `winforge.exe bloatware` lists detected packages.
- [ ] 9.2 Remove one safe package (e.g. `Microsoft.BingWeather`); it disappears
      from the list and from Start.

## 10. Maintenance & scheduling

- [ ] 10.1 `winforge.exe run-maintenance` completes and prints a summary; the
      audit log records a maintenance entry.
- [ ] 10.2 `schedule` registers the weekly task (visible in Task Scheduler);
      `unschedule` removes it.

## 11. ISO builder (needs real Windows media; else N/A)

- [ ] 11.1 `winforge.exe build-iso --source <mounted-iso-dir> --list-editions`
      lists editions via dism.
- [ ] 11.2 A slim build completes and the output ISO boots in a VM.

## Sign-off

| Field | Value |
|---|---|
| Windows build |  |
| winforge.exe commit / SHA-256 |  |
| Date / operator |  |
| Items passed / failed / N/A |  |
