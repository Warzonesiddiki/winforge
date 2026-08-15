#!/usr/bin/env python3
"""WinForge catalog parity checker.

Compares the web app's canonical catalog (src/db/seed-data.ts) against the
native Go engine's embedded configs (config/*.json) and reports:

  1. engine tweaks whose operations match a web tweak but lack metadata
     (name/description backfill candidates);
  2. web tweaks with no engine equivalent (engine catalog gaps);
  3. debloat package gaps (web families missing from config/debloat.json);
  4. application gaps (web winget ids missing from config/applications.json).

Usage: python3 tools/catalog_parity.py [--fix] [--write-report docs/CATALOG_PARITY.md]
Exit code 0 when no gaps, 1 when gaps exist (CI-friendly).
"""
import argparse
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

# ---------------------------------------------------------------- web catalog

SERVICE_OP = re.compile(r"^Service\s+(?P<name>[A-Za-z0-9_. -]+)\s*->\s*(?P<mode>.+)$")
TASK_OP = re.compile(r"^Scheduled Task:\s*(?P<path>.+?)\s*->\s*(?P<action>.+)$")

# Power-scheme aliases used by the engine catalog (mirrors internal/power/power.go).
POWER_ALIASES = {
    "balanced": "381b4222-f694-41f0-9685-ff5bb260df2e",
    "high-performance": "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c",
    "power-saver": "a1841308-3541-4fab-bc81-f71556f20b4a",
    "ultimate": "e9a42b02-d5df-448d-aa00-03f14749eb61",
}


def norm(x):
    return re.sub(r"\s+", "", x).strip('"').lower()


def parse_web_tweak_ops(op_strings):
    """Parse the web app's human-readable operation strings into signatures."""
    regs, commands, services, tasks, power_guids = [], [], [], [], []
    for raw in op_strings:
        s = raw.strip()
        if s.startswith(("HKCU\\", "HKLM\\", "HKCR\\")):
            if "..." in s.split(":", 1)[0]:
                continue  # abbreviated path, unmatchable
            for hive, path, name, value in re.findall(
                r"(HKLM|HKCU|HKCR)\\([^:]+):\s*([^=,]+?)\s*=\s*([^,]+)", s
            ):
                regs.append((hive, norm(path), norm(name), norm(value)))
        elif (m := SERVICE_OP.match(s)):
            services.append((norm(m["name"]), m["mode"].strip().lower()))
        elif (m := TASK_OP.match(s)):
            tasks.append((norm(m["path"]), m["action"].strip().lower()))
        else:
            commands.append(s.lower().split(" (native api equivalent)")[0].strip())
            for guid in re.findall(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", s.lower()):
                power_guids.append(norm(guid))
    return regs, commands, services, tasks, power_guids


def parse_web_catalog():
    src = (ROOT / "src/db/seed-data.ts").read_text()
    block = src[src.index("export const tweaksSeed") : src.index("const debloatRaw")]
    tweaks = []
    for m in re.finditer(
        r't\("(?P<id>[a-z0-9-]+)",\s*"(?P<name>[^"]+)",\s*"(?P<desc>[^"]+)",\s*'
        r'"(?P<cat>[^"]+)",\s*"(?P<risk>[a-z]+)",\s*(?P<def>true|false),\s*'
        r'\[(?P<tags>[^\]]*)\],\s*\[(?P<ops>[^\]]*)\],\s*\[(?P<undo>[^\]]*)\]',
        block,
    ):
        ops = [o.replace("\\\\", "\\") for o in re.findall(r'"((?:[^"\\]|\\.)*)"', m["ops"])]
        undo = [o.replace("\\\\", "\\") for o in re.findall(r'"((?:[^"\\]|\\.)*)"', m["undo"])]
        tweaks.append(
            {
                "id": m["id"],
                "name": m["name"],
                "description": m["desc"],
                "category": m["cat"],
                "risk": m["risk"],
                "ops": parse_web_tweak_ops(ops),
                "undo": parse_web_tweak_ops(undo),
                "raw_ops": ops,
                "raw_undo": undo,
            }
        )
    return tweaks


# ---------------------------------------------------------------- engine configs

def engine_tweak_sigs(t):
    regs, commands, services, tasks, power_guids = [], [], [], [], []
    for op in t["operations"]:
        tpe = op.get("type", "")
        if tpe.startswith("registry_"):
            value = op.get("value", "")
            if isinstance(value, str):
                value = value.replace('"', "")
            regs.append(
                (op.get("hive", ""), norm(op.get("path", "")), norm(op.get("name", "")), norm(str(value)))
            )
        elif tpe == "command":
            cmd = (op.get("value", "") + " " + " ".join(op.get("args", []))).strip().lower()
            # Normalize powershell wrappers so they compare against the raw web command.
            cmd = cmd.removeprefix("powershell -noprofile -command ")
            commands.append(cmd)
        elif tpe.startswith("service_"):
            services.append((norm(op.get("name", "")), op.get("mode", op.get("value", "")).lower()))
        elif tpe.startswith("task_"):
            tasks.append((norm(op.get("path", "")), tpe.removeprefix("task_").lower()))
        elif tpe == "power_scheme":
            raw = norm(str(op.get("value", "")))
            power_guids.append(raw)
            power_guids.append(POWER_ALIASES.get(raw, raw))
    return regs, commands, services, tasks, power_guids


def engine_has_op_match(web_ops, eng_ops):
    """True when at least one web operation signature is present in the engine tweak."""
    wr, wc, ws, wt, wp = web_ops
    er, ec, es, et, ep = eng_ops
    for sig in wr:
        if sig in er:
            return True
    for cmd in wc:
        if any(cmd in e or e in cmd for e in ec):
            return True
    for s in ws:
        if s in es:
            return True
    for t in wt:
        if t in et:
            return True
    for g in wp:
        if g in ep:
            return True
    return False


# ---------------------------------------------------------------- main

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--fix", action="store_true", help="backfill engine metadata from web matches")
    ap.add_argument("--write-report", metavar="PATH")
    args = ap.parse_args()

    web_tweaks = parse_web_catalog()
    eng = json.load(open(ROOT / "config/tweaks.json"))["tweaks"]
    eng_ids = {t["id"] for t in eng}
    eng_sigs = [(t, engine_tweak_sigs(t)) for t in eng]

    report = []
    fixed = 0

    for wt in web_tweaks:
        if wt["id"] in eng_ids:
            continue  # merged directly from the web catalog (tools/web_catalog_to_engine.py)
        matches = [t for t, sigs in eng_sigs if engine_has_op_match(wt["ops"], sigs)]
        if not matches:
            report.append(f"- ❌ GAP — web tweak `{wt['id']}` ({wt['name']}) has no engine equivalent")
            continue
        backfilled = False
        for t in matches:
            if not t.get("name") or not t.get("description"):
                if args.fix:
                    t["name"], t["description"] = wt["name"], wt["description"]
                    fixed += 1
                    backfilled = True
                    report.append(
                        f"- ✅ BACKFILLED `{t['id']}` ← {wt['id']} ({wt['name']}, {wt['category']})"
                    )
                else:
                    report.append(
                        f"- ⚠️ METADATA-GAP `{t['id']}` matches web {wt['id']} ({wt['name']}) but has empty name/description"
                    )
        if matches and not backfilled:
            report.append(
                f"- ✅ EQUIVALENT — web tweak `{wt['id']}` ({wt['name']}) is covered by engine tweak(s) "
                + ", ".join(f"`{t['id']}`" for t in matches)
            )

    if args.fix and fixed:
        with open(ROOT / "config/tweaks.json", "w") as f:
            json.dump({"tweaks": eng}, f, indent=2)
            f.write("\n")

    # debloat (scoped to the debloatRaw block so tags arrays can't false-positive)
    seed_src = (ROOT / "src/db/seed-data.ts").read_text()
    debloat_block = seed_src[seed_src.index("const debloatRaw") : seed_src.index("export const debloatSeed")]
    web_debloat = re.findall(
        r'\["(?P<fam>[^"]+)",\s*"(?P<name>[^"]+)",\s*"(?P<cat>[^"]+)",\s*"(?P<risk>[a-z]+)"\]',
        debloat_block,
    )
    debloat_path = ROOT / "config/debloat.json"
    debloat_cfg = json.load(open(debloat_path))
    eng_debloat = {d["family"] for d in debloat_cfg["entries"]}
    missing_debloat = [(f, n, c) for f, n, c, _ in web_debloat if f not in eng_debloat]
    debloat_descriptions = {
        "Microsoft Bloat": "Inbox Microsoft application.",
        "OEM Apps": "Pre-installed OEM application.",
        "Advertising": "Promotional or advertising application.",
        "Gaming": "Xbox gaming component.",
        "Social": "Promoted social application.",
        "Widgets": "Windows Widgets experience.",
        "AI/Copilot": "Windows AI component.",
        "Protected": "Critical system component — protected from removal.",
    }
    if args.fix and missing_debloat:
        for fam, name, cat in missing_debloat:
            debloat_cfg["entries"].append(
                {
                    "family": fam,
                    "name": name,
                    "category": cat,
                    "description": debloat_descriptions.get(cat, f"{cat} application."),
                }
            )
        with open(debloat_path, "w") as f:
            json.dump(debloat_cfg, f, indent=2)
            f.write("\n")
        report.append(f"- ✅ DEBLOAT SYNC — added {len(missing_debloat)} package(s) to config/debloat.json")
    else:
        for fam, name, cat in missing_debloat:
            report.append(f"- ❌ DEBLOAT GAP — {fam} ({name}, {cat}) not in config/debloat.json")

    # apps (web category names are kept verbatim; the engine UI derives tabs dynamically)
    web_apps = re.findall(
        r'\{ id: "(?P<id>[^"]+)", name: "(?P<name>[^"]+)", publisher: "(?P<pub>[^"]+)", '
        r'category: "(?P<cat>[^"]+)"',
        seed_src,
    )
    apps_path = ROOT / "config/applications.json"
    apps_cfg = json.load(open(apps_path))
    eng_apps = {a["id"] for a in apps_cfg["applications"]}
    missing_apps = [(i, n, p, c) for i, n, p, c in web_apps if i not in eng_apps]
    if args.fix and missing_apps:
        for app_id, name, pub, cat in missing_apps:
            apps_cfg["applications"].append(
                {
                    "id": app_id,
                    "name": name,
                    "category": cat,
                    "description": f"{name} — {pub}.",
                    "tags": [cat.lower().replace(" ", "-")],
                }
            )
        with open(apps_path, "w") as f:
            json.dump(apps_cfg, f, indent=2)
            f.write("\n")
        report.append(f"- ✅ APP SYNC — added {len(missing_apps)} application(s) to config/applications.json")
    else:
        for app_id, name, cat in missing_apps:
            report.append(f"- ❌ APP GAP — {app_id} ({name}, {cat}) not in config/applications.json")

    counts = (
        "\n## Summary\n\n"
        f"- Web tweaks: {len(web_tweaks)} · Engine tweaks: {len(eng)}\n"
        f"- Engine tweaks needing metadata: {sum(1 for t in eng if not t.get('name'))}\n"
        f"- Backfilled this run: {fixed}\n"
        f"- Debloat {('added' if args.fix else 'gaps')}: {len(missing_debloat)} · "
        f"App {('added' if args.fix else 'gaps')}: {len(missing_apps)}\n"
    )
    report.insert(0, counts)

    out = "\n".join(report)
    if args.write_report:
        path = ROOT / args.write_report
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(
            "# WinForge Catalog Parity Report\n\nGenerated by tools/catalog_parity.py\n\n" + out + "\n"
        )
    print(out)
    sys.exit(1 if (not args.fix and (missing_debloat or missing_apps)) else 0)


if __name__ == "__main__":
    main()
