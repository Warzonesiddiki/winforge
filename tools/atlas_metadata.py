#!/usr/bin/env python3
"""Backfill atlas-* tweak metadata from the AtlasOS playbook repository.

Each engine tweak id ``atlas-<slug>`` maps 1:1 to a file
``src/playbook/Configuration/tweaks/**/<slug>.yml`` in the AtlasOS repo
(https://github.com/Atlas-OS/Atlas). This tool copies that file's ``title:``
and ``description:`` scalars into the ``name``/``description`` fields of
``config/tweaks.json``.

Zero-fabrication guard: before applying, the tool cross-checks operation
signatures between the YAML actions (registry values/keys, services,
scheduled tasks, run commands) and the engine ops of the tweak. A tweak
whose ops do not overlap with the matched YAML is reported as SUSPECT and
skipped (no metadata applied) so a wrong title is never attached.

Usage:
    python3 tools/atlas_metadata.py --atlas-src /path/to/Atlas [--apply]

Default mode is a dry-run report; ``--apply`` writes config/tweaks.json.
Exit code 0 when every metadata-less atlas-* tweak is resolved, 1 otherwise
(CI-friendly).
"""
import argparse
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
TWEAKS_JSON = ROOT / "config" / "tweaks.json"

# ------------------------------------------------------------ Atlas YAML scan

SCALAR = re.compile(r"^(title|description):\s*(.*)$")


def unquote_yaml_scalar(raw: str) -> str:
    """Decode the scalar forms Atlas uses: plain, 'single', or \"double\"."""
    s = raw.strip()
    if len(s) >= 2 and s[0] == "'" and s[-1] == "'":
        return s[1:-1].replace("''", "'")
    if len(s) >= 2 and s[0] == '"' and s[-1] == '"':
        # YAML double-quoted: only unescape the common sequences Atlas uses.
        out, i = [], 0
        escapes = {"n": "\n", "t": "\t", '"': '"', "\\": "\\", "0": "\0"}
        while i < len(s):
            c = s[i]
            if c == "\\" and i + 1 < len(s) and s[i + 1] in escapes:
                out.append(escapes[s[i + 1]])
                i += 2
            else:
                out.append(c)
                i += 1
        return "".join(out)
    return s


def kv(inline: str, key: str):
    """Extract key: 'value' / key: value from an inline YAML map string."""
    m = re.search(
        r"(?:^|[{,]\s*)" + re.escape(key) + r"\s*:\s*(?:'((?:[^']|'')*)'|\"([^\"]*)\"|([^,}]+))",
        inline,
    )
    if not m:
        return None
    a, b, c = m.groups()
    return (a.replace("''", "'") if a is not None else b if b is not None else c).strip()


HIVE_RE = re.compile(r"^(HKLM|HKCU|HKCR|HKU|HKCC)\\+(.+)$", re.IGNORECASE)


def collapse_bs(s: str) -> str:
    """Collapse repeated backslashes without re.sub (trailing-\\ replacement bug)."""
    while "\\\\" in s:
        s = s.replace("\\\\", "\\")
    return s


def split_hive(full_path: str):
    """'HKCU\\Software\\X' -> ('HKCU', 'Software\\X')."""
    m = HIVE_RE.match(full_path.strip())
    if not m:
        return "", collapse_bs(full_path).lower()
    return m.group(1).upper(), collapse_bs(m.group(2)).lower()


TAG_RE = re.compile(r"!(\w+):[ \t]*")  # never span newlines: block maps follow ':'


def scan_braced_map(text: str, start: int) -> str:
    """Return text[start:] up to the closing '}' of a braced inline map,
    respecting single-quoted sections (which may themselves contain braces,
    e.g. registry paths with CLSID '{F5FB2C77-...}') and '' escapes."""
    depth, i, in_q = 0, start, False
    while i < len(text):
        c = text[i]
        if in_q:
            if c == "'":
                if i + 1 < len(text) and text[i + 1] == "'":
                    i += 2
                    continue
                in_q = False
        elif c == "'":
            in_q = True
        elif c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return text[start : i + 1]
        i += 1
    return text[start:]


def iter_tag_maps(text: str):
    """Yield (tag, map_text) for every ``!tag: {inline}`` and every
    ``!tag:`` followed by an indented block map (folded into inline form)."""
    for m in TAG_RE.finditer(text):
        tag, start = m.group(1), m.end()
        if start < len(text) and text[start] == "{":
            yield tag, scan_braced_map(text, start)
        else:
            lines = []
            for line in text[start:].splitlines():
                s = line.strip()
                if not s:
                    continue
                if line[0] in " \t" and ":" in s and not s.startswith(("-", "!")):
                    lines.append(s)
                else:
                    break
            if lines:
                yield tag, ", ".join(lines)


def atlas_op_sigs(text: str):
    """Operation signatures from raw Atlas playbook YAML text.

    Returns a set of tuples so engine ops can be matched against them.
    """
    sigs = set()
    for tag, inline in iter_tag_maps(text):
        if tag == "registryValue":
            full = kv(inline, "path")
            if not full:
                continue
            hive, path = split_hive(full)
            value = kv(inline, "value") or ""
            op = (kv(inline, "operation") or "").lower()
            if op == "delete":
                sigs.add(("regdel", hive, path, value.lower()))
            sigs.add(("regval", hive, path, value.lower()))
        elif tag == "registryKey":
            full = kv(inline, "path")
            if not full:
                continue
            hive, path = split_hive(full)
            sigs.add(("regkey", hive, path, ""))
        elif tag == "service":
            name = kv(inline, "name")
            if name:
                sigs.add(("svc", name.lower()))
        elif tag == "scheduledTask":
            path = kv(inline, "path")
            op = (kv(inline, "operation") or "").lower()
            if path:
                sigs.add(("task", collapse_bs(path).lower()))
                if op:
                    sigs.add(("taskop", collapse_bs(path).lower(), op))
        elif tag == "run":
            exe = kv(inline, "exe")
            if exe:
                sigs.add(("run", exe.lower().replace('"', "")))
        elif tag == "cmd":
            sigs.add(("shell",))
            cmd = kv(inline, "command")
            if cmd:
                sigs.add(("run", cmd.lower().split()[0].replace('"', "")))
        elif tag == "powerShell":
            sigs.add(("shell",))
    return sigs


def scan_atlas(atlas_src: Path):
    root = atlas_src / "src" / "playbook" / "Configuration" / "tweaks"
    if not root.is_dir():
        sys.exit(f"error: {root} not found — clone https://github.com/Atlas-OS/Atlas "
                 f"and pass --atlas-src <path>")
    meta = {}
    for yml in sorted(root.rglob("*.yml")):
        text = yml.read_text(encoding="utf-8")
        title = desc = None
        for line in text.splitlines():
            m = SCALAR.match(line.strip()) if not line.startswith((" ", "-")) else None
            if not m:
                m = SCALAR.match(line)
            if m:
                if m.group(1) == "title" and title is None:
                    title = unquote_yaml_scalar(m.group(2))
                elif m.group(1) == "description" and desc is None:
                    desc = unquote_yaml_scalar(m.group(2))
            if title is not None and desc is not None:
                break
        if title is None or desc is None:
            continue  # no top-level metadata; not a tweak definition file
        meta[yml.stem] = {
            "title": title,
            "description": desc,
            "file": str(yml.relative_to(atlas_src)),
            "sigs": atlas_op_sigs(text),
        }
    return meta


# ------------------------------------------------------------- engine sigs

def engine_op_sigs(tweak, apply_only=True):
    """Signatures of the tweak's apply-side ops (Atlas `actions:` are apply-side;
    revert ops are ignored for overlap scoring)."""
    sigs = []
    cols = ("operations",) if apply_only else ("operations", "revert")
    for coll in cols:
        for op in tweak.get(coll) or []:
            t = op.get("type", "")
            hive = (op.get("hive") or "").upper()
            path = collapse_bs(op.get("path") or "").lower()
            name = (op.get("name") or "").lower()
            if t.startswith("registry_set_"):
                sigs.append(("regval", hive, path, name))
            elif t == "registry_delete":
                sigs.append(("regdel", hive, path, name))
            elif t.startswith("service_"):
                sigs.append(("svc", name.lower()))
            elif t.startswith("task_"):
                sigs.append(("task", path))
            elif t == "command":
                exe = (op.get("value") or op.get("program") or op.get("command") or "")
                sigs.append(("run", exe.lower().replace('"', "")))
                sigs.append(("shell",))
            else:
                sigs.append((t,))
    return sigs


def overlap(engine_sigs, atlas_sigs):
    applyable = [s for s in set(engine_sigs) if s[0] not in ("shell",)]
    if not applyable:
        return 1.0, 0, 0
    hits = sum(1 for s in applyable if s in atlas_sigs)
    return hits / len(applyable), hits, len(applyable)


# ---------------------------------------------------------------------- main

def needs_metadata(tw):
    return not tw.get("name") or tw.get("name") == tw.get("id")


def main():
    ap = argparse.ArgumentParser(description="Backfill atlas-* metadata from the AtlasOS repo")
    ap.add_argument("--atlas-src", default="/tmp/atlas-src",
                    help="path to a clone of https://github.com/Atlas-OS/Atlas")
    ap.add_argument("--apply", action="store_true", help="write config/tweaks.json")
    ap.add_argument("--min-overlap", type=float, default=0.5,
                    help="minimum op-signature overlap to trust a match (default 0.5)")
    ap.add_argument("--force-suspect", action="store_true",
                    help="apply metadata even when op overlap is below threshold (prints WARNING)")
    args = ap.parse_args()

    meta = scan_atlas(Path(args.atlas_src))
    data = json.loads(TWEAKS_JSON.read_text(encoding="utf-8"))

    applied = skipped_suspect = not_found = already_named = 0
    suspects, missing = [], []
    for tw in data["tweaks"]:
        tid = tw["id"]
        if not tid.startswith("atlas-"):
            continue
        if not needs_metadata(tw):
            already_named += 1
            continue
        slug = tid[len("atlas-"):]
        entry = meta.get(slug)
        if entry is None:
            not_found += 1
            missing.append(f"{tid}  (no {slug}.yml in AtlasOS playbook)")
            continue
        ratio, hits, total = overlap(engine_op_sigs(tw), entry["sigs"])
        if ratio < args.min_overlap and not args.force_suspect:
            skipped_suspect += 1
            suspects.append(
                f"{tid}  <- {entry['file']}  overlap {hits}/{total} "
                f"({ratio:.0%}) < {args.min_overlap:.0%}"
            )
            continue
        flag = "  [forced]" if ratio < args.min_overlap else ""
        print(f"OK  {tid:48s} overlap {hits}/{total:2d} ({ratio:5.0%})  <- {entry['file']}{flag}")
        tw["name"] = entry["title"].strip()
        tw["description"] = entry["description"].strip()
        applied += 1

    print()
    print(f"Atlas files with metadata: {len(meta)}")
    print(f"applied {applied} · suspect-skipped {skipped_suspect} · not-found {not_found} · already-named {already_named}")
    if suspects:
        print("\nSUSPECT (skipped, verify manually):")
        for s in suspects:
            print("  " + s)
    if missing:
        print("\nNOT FOUND in AtlasOS repo:")
        for s in missing:
            print("  " + s)

    if args.apply and applied:
        out = json.dumps(data, indent=2, ensure_ascii=True) + "\n"
        TWEAKS_JSON.write_text(out, encoding="utf-8")
        print(f"\nwrote {TWEAKS_JSON}")

    return 0 if (skipped_suspect == 0 and not_found == 0) else 1


if __name__ == "__main__":
    sys.exit(main())
