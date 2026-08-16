#!/usr/bin/env python3
"""Convert the web app's canonical tweak catalog into the Go engine's op format.

Reads src/db/seed-data.ts (64 tweaks with human-readable operation strings) and
merges them into config/tweaks.json as native engine tweaks. Every mapping is
mechanical and documented below; simulation-only artifacts (Appx reverts,
abbreviated paths without a known expansion, "(Default)" value names, WMI ops
other than SetTcpipNetbios) are skipped and reported — never fabricated.

Mapping rules:
  HKCU\\<path>: Name = 0x…/n   → registry_set_dword
  HKCU\\<path>: Name = "text"  → registry_set_string
  HKCU\\<path>: (Default) = …  → registry op with empty name (the Windows
                                 default value; RegSetValueExW documents that
                                 a NULL/empty value name addresses it)
  undo "= default"             → registry_delete (restores the Windows default)
  undo "HK..\\<key> -> removed" → registry_delete_key (recursive; reverts a
                                 key the apply side created)
  "HKCU\\...\\X"               → expanded via the ABBREV map below (else skipped)
  bcdedit/fsutil …             → command (exe + tokenized args)
  powercfg /hibernate on|off   → power_hibernate true|false (native
                                 CallNtPowerInformation SystemReserveHiberFile)
  powercfg -setacvalueindex SCHEME_CURRENT SUB_PROCESSOR PROCTHROTTLEMIN n
                               → power_processor_state {"min": n} (native
                                 PowerWriteACValueIndex)
  WMI …SetTcpipNetbios(n)      → netbios n (native registry writes to
                                 NetBT\\Parameters\\Interfaces NetbiosOptions —
                                 no WMI/PowerShell)
  Disable/Enable-MMAgent …     → SKIPPED (PowerShell-only; security boundary)
  Service A, B -> Mode         → one service_start_mode op per service
  Scheduled Task: path -> X    → task_disable / task_enable
  risk "expert"                → "high" (engine has no expert tier)
  "Automatic (Delayed)"        → "auto" (engine's closest supported mode)

Usage: python3 tools/web_catalog_to_engine.py [--apply] [--report-only]
"""
import argparse
import json
import re
import shlex
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from catalog_parity import parse_web_catalog  # noqa: E402

ROOT = Path(__file__).resolve().parent.parent

# Documented expansions for abbreviated registry paths in the web catalog.
ABBREV = {
    "HKCU\\...\\Explorer\\Advanced": "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Advanced",
    "HKCU\\...\\Advanced": "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Advanced",
    "HKCU\\...\\SearchSettings": "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\SearchSettings",
    "HKLM\\...\\PrefetchParameters": "SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Memory Management\\PrefetchParameters",
}

SERVICE_MODE = {
    "disabled": "disabled",
    "disable": "disabled",
    "manual": "manual",
    "demand": "manual",
    "automatic": "auto",
    "auto": "auto",
    "automatic (delayed)": "auto",
}

SERVICE_OP = re.compile(r"^Service\s+(?P<names>[A-Za-z0-9_. -]+(?:\s*,\s*[A-Za-z0-9_. -]+)*)\s*->\s*(?P<mode>.+)$")
TASK_OP = re.compile(r"^Scheduled Task:\s*(?P<path>.+?)\s*->\s*(?P<action>.+)$")
REG_OP = re.compile(r"^(?P<hive>HKLM|HKCU|HKCR)\\(?P<path>[^:]+):\s*(?P<name>[^=]+?)\s*=\s*(?P<value>.+)$")
WMI_NETBIOS = re.compile(r"^WMI Win32_NetworkAdapterConfiguration\.SetTcpipNetbios\((\d+)\)$")
KEY_REMOVED = re.compile(r"^(?P<hive>HKLM|HKCU|HKCR)\\(?P<path>[^:]+?)\s*->\s*removed$")
HIBERNATE = re.compile(r"^powercfg\s+/hibernate\s+(?P<state>on|off)$", re.IGNORECASE)
PROC_THROTTLE = re.compile(
    r"^powercfg\s+-setacvalueindex\s+SCHEME_CURRENT\s+SUB_PROCESSOR\s+"
    r"(?P<setting>PROCTHROTTLEMIN|PROCTHROTTLEMAX)\s+(?P<pct>\d+)$",
    re.IGNORECASE,
)

report = []


def skip(reason, tweak_id, raw):
    report.append(f"    - SKIP [{tweak_id}] {raw[:70]}… ({reason})")


def expand_path(hive, path):
    if "..." not in path:
        return path
    for abbrev, full in ABBREV.items():
        if abbrev.lower().startswith(hive.lower() + "\\") and path.startswith(abbrev.split("\\", 1)[1]):
            rest = path[len(abbrev.split("\\", 1)[1]):]
            return full + rest
    return None


def parse_int(value):
    try:
        v = int(value, 0)
        if 0 <= v <= 0xFFFFFFFF:
            return v
    except ValueError:
        pass
    return None


def convert_op(raw, tweak_id, is_undo=False):
    """Convert one web operation string to a list of engine op dicts."""
    s = raw.strip()
    ops = []
    if (m := KEY_REMOVED.match(s)):
        # "<hive>\<key> -> removed" only appears as the undo of a tweak whose
        # apply side creates that key, so a recursive key delete restores the
        # pre-apply state (registry_delete_key refuses shallow paths).
        if is_undo:
            ops.append({"type": "registry_delete_key", "hive": m["hive"], "path": m["path"].strip()})
        else:
            skip("'-> removed' is an undo narrative, not an apply op", tweak_id, raw)
        return ops
    if s.startswith(("HKCU\\", "HKLM\\", "HKCR\\")):
        for hive, path, name, value in re.findall(
            r"(HKLM|HKCU|HKCR)\\([^:]+):\s*([^=,]+?)\s*=\s*([^,]+)", s
        ):
            full = expand_path(hive, path)
            if full is None:
                skip("abbreviated path with no documented expansion", tweak_id, raw)
                continue
            name = name.strip()
            if name == "(Default)":
                # RegSetValueExW: a NULL/empty value name addresses the key's
                # unnamed (default) value; the engine passes the empty name
                # through, so the default value is a plain registry op.
                name = ""
            if name == "PagingFiles":
                skip("PagingFiles is REG_MULTI_SZ — a string/dword write would corrupt pagefile config", tweak_id, raw)
                continue
            if value.strip().lower() == "default":
                if is_undo:
                    ops.append({"type": "registry_delete", "hive": hive, "path": full, "name": name})
                else:
                    skip("'default' as an apply value is not a mutation", tweak_id, raw)
                continue
            if (n := parse_int(value)) is not None:
                ops.append({"type": "registry_set_dword", "hive": hive, "path": full, "name": name, "value": n})
            else:
                # The seed keeps TS escape sequences (\") in the captured op
                # string; unescape them before trimming the surrounding quotes
                # so `(Default) = \"\"` yields the empty string.
                text = value.strip().replace('\\"', '"').strip('"')
                ops.append({"type": "registry_set_string", "hive": hive, "path": full, "name": name, "value": text})
        return ops
    if (m := SERVICE_OP.match(s)):
        mode = SERVICE_MODE.get(m["mode"].strip().lower())
        if mode is None:
            skip(f"unknown service mode {m['mode']!r}", tweak_id, raw)
            return ops
        for name in m["names"].split(","):
            n = name.strip()
            if n:
                ops.append({"type": "service_start_mode", "name": n, "value": mode})
        return ops
    if (m := TASK_OP.match(s)):
        path = m["path"].strip()
        if not path.startswith("\\"):
            path = "\\" + path
        action = m["action"].strip().lower()
        if action == "disabled":
            ops.append({"type": "task_disable", "path": path})
        elif action == "enabled":
            ops.append({"type": "task_enable", "path": path})
        else:
            skip(f"unknown task action {m['action']!r}", tweak_id, raw)
        return ops
    if (m := WMI_NETBIOS.match(s)):
        # Native equivalent: NetbiosOptions under
        # HKLM\SYSTEM\CurrentControlSet\Services\NetBT\Parameters\Interfaces\Tcpip_*
        # (0 = DHCP setting, 1 = enable, 2 = disable) — the documented registry
        # backing of Win32_NetworkAdapterConfiguration.SetTcpipNetbios.
        ops.append({"type": "netbios", "value": int(m[1])})
        return ops
    if s.startswith(("Disable-MMAgent", "Enable-MMAgent")):
        skip("PowerShell-only MMAgent cmdlet — engine never invokes PowerShell from its elevated executor (security boundary)", tweak_id, raw)
        return ops
    if s.startswith("Appx:"):
        skip("Appx narrative op (engine handles Appx via the debloat catalog)", tweak_id, raw)
        return ops
    if (m := HIBERNATE.match(s)):
        # Native equivalent of `powercfg /hibernate on|off`:
        # CallNtPowerInformation(SystemReserveHiberFile, &BOOLEAN) commits or
        # decommits the hibernation file.
        ops.append({"type": "power_hibernate", "value": m["state"].lower() == "on"})
        return ops
    if (m := PROC_THROTTLE.match(s)):
        # Native equivalent of `powercfg -setacvalueindex SCHEME_CURRENT
        # SUB_PROCESSOR PROCTHROTTLEMIN/MAX n`: PowerWriteACValueIndex on the
        # processor subgroup (54533251-82be-4824-96c1-47b60b740d00) followed
        # by PowerSetActiveScheme.
        field = "min" if m["setting"].upper() == "PROCTHROTTLEMIN" else "max"
        ops.append({"type": "power_processor_state", "value": {field: int(m["pct"])}})
        return ops
    if s.lower().startswith("powercfg"):
        skip("powercfg is not an allowlisted elevated command and this form has no native mapping (power_scheme/power_hibernate/power_processor_state cover the supported forms)", tweak_id, raw)
        return ops
    # Plain commands: bcdedit, fsutil, … (elevated-executor allowlist)
    try:
        tokens = shlex.split(s, posix=True)
    except ValueError:
        skip("unparseable command line", tweak_id, raw)
        return ops
    if tokens and tokens[0].lower() in {"bcdedit", "fsutil"}:
        ops.append({"type": "command", "value": tokens[0], "args": tokens[1:]})
    else:
        skip(f"unrecognized or non-allowlisted command {tokens[0] if tokens else '?'!r}", tweak_id, raw)
    return ops


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--apply", action="store_true", help="write merged catalog to config/tweaks.json")
    args = ap.parse_args()

    web_tweaks = parse_web_catalog()
    eng = json.load(open(ROOT / "config/tweaks.json"))["tweaks"]
    converted, excluded, skipped_ops = [], [], 0
    for wt in web_tweaks:
        ops, revert = [], []
        for raw in wt["raw_ops"]:
            ops.extend(convert_op(raw, wt["id"]))
        for raw in wt["raw_undo"]:
            revert.extend(convert_op(raw, wt["id"], is_undo=True))
        if not ops:
            excluded.append(wt["id"])
            skipped_ops += len(wt["raw_ops"])
            continue
        entry = {
            "id": wt["id"],
            "name": wt["name"],
            "category": wt["category"],
            "description": wt["description"],
            "risk": "high" if wt["risk"] == "expert" else wt["risk"],
            "reversible": bool(revert),
            "operations": ops,
        }
        if revert:
            entry["revert"] = revert
        converted.append(entry)

    if args.apply:
        web_ids = {wt["id"] for wt in web_tweaks}
        converted_ids = {e["id"] for e in converted}
        # Drop engine entries the web catalog owns but that are no longer
        # converted (e.g. previously merged tweaks that now hit a security
        # exclusion such as the PowerShell/powercfg command boundary).
        stale = [t for t in eng if t["id"] in web_ids and t["id"] not in converted_ids]
        for t in stale:
            report.append(f"    - REMOVED stale merge `{t['id']}` (no longer convertible)")
        # Rebuild: all non-web entries + the freshly converted web entries.
        merged = [t for t in eng if t["id"] not in web_ids]
        merged.extend(converted)
        with open(ROOT / "config/tweaks.json", "w") as f:
            json.dump({"tweaks": merged}, f, indent=2)
            f.write("\n")
        print(f"\nconfig/tweaks.json: WRITTEN — engine tweaks now: {len(merged)}")

    print(f"web tweaks: {len(web_tweaks)} · converted: {len(converted)} · excluded (no real ops): {len(excluded)}")
    if excluded:
        print("excluded:", ", ".join(excluded))
    print("\n".join(report) if report else "(no skipped ops)")
    if not args.apply:
        print(f"\nconfig/tweaks.json: dry-run — engine tweaks would be: "
              f"{len([t for t in eng if t['id'] not in {e['id'] for e in converted}]) + len(converted)}")


if __name__ == "__main__":
    main()
