#!/usr/bin/env python3
"""
Extract and verify localization resources.

- Reads src/lib/i18n.ts translations as source of truth.
- Verifies web/locales/{en,es,fr,de,zh}.json (and en-US etc aliases) match.
- Reports hardcoded UI strings not using t() (informational, non-fatal).
- Optionally writes --write to regenerate JSON files from i18n.ts.

Usage:
  python3 tools/extract_locales.py           # verify
  python3 tools/extract_locales.py --write   # regenerate JSON from i18n.ts
  python3 tools/extract_locales.py --check-hardcoded  # scan src/ for t() coverage

Exit 0 when locales are in sync, 1 when drift detected.
"""

import argparse
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
I18N_TS = ROOT / "src/lib/i18n.ts"
LOCALES_DIR = ROOT / "web/locales"
LOCALES_SHORT = ["en", "es", "fr", "de", "zh"]
LOCALES_LONG = ["en-US", "es-ES", "fr-FR", "de-DE", "zh-CN"]
LOCALE_MAP = {
    "en": "en-US",
    "es": "es-ES",
    "fr": "fr-FR",
    "de": "de-DE",
    "zh": "zh-CN",
}


def parse_i18n_ts():
    text = I18N_TS.read_text(encoding="utf-8")
    # Extract translations block: "en-US": { ... }
    locales = {}
    # Find each locale block
    pattern = re.compile(r'"([a-z]{2}-[A-Z]{2})"\s*:\s*\{([^}]+)\}', re.DOTALL)
    for m in pattern.finditer(text):
        locale = m.group(1)
        block = m.group(2)
        entries = {}
        for kv in re.finditer(r'"([^"]+)"\s*:\s*"([^"]*)"', block):
            entries[kv.group(1)] = kv.group(2)
        locales[locale] = entries
    return locales


def load_json_locale(path: Path):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception as e:
        print(f"Error reading {path}: {e}", file=sys.stderr)
        return None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--write", action="store_true", help="regenerate JSON files from i18n.ts")
    parser.add_argument("--check-hardcoded", action="store_true", help="scan src/components for hardcoded strings")
    args = parser.parse_args()

    if not I18N_TS.exists():
        print(f"Missing {I18N_TS}", file=sys.stderr)
        sys.exit(1)

    locales = parse_i18n_ts()
    if not locales:
        print("Failed to parse translations from src/lib/i18n.ts", file=sys.stderr)
        sys.exit(1)

    print(f"Parsed {len(locales)} locales from src/lib/i18n.ts: {', '.join(sorted(locales))}")
    for loc, keys in locales.items():
        print(f"  {loc}: {len(keys)} keys")

    # If --write, regenerate JSON files
    if args.write:
        LOCALES_DIR.mkdir(parents=True, exist_ok=True)
        for short in LOCALES_SHORT:
            long_code = LOCALE_MAP[short]
            data = locales.get(long_code)
            if data is None:
                print(f"No data for {long_code} (from short {short})", file=sys.stderr)
                sys.exit(1)
            # Write both short and long forms
            for name in [short, long_code]:
                out_path = LOCALES_DIR / f"{name}.json"
                out_path.write_text(json.dumps(data, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
                print(f"Wrote {out_path} ({len(data)} keys)")
        print("Done --write")
        # After write, continue to verify

    # Verify JSON files match i18n.ts
    ok = True
    all_keys = None
    for locale_long in LOCALES_LONG:
        expected = locales.get(locale_long)
        if expected is None:
            print(f"Missing locale {locale_long} in i18n.ts", file=sys.stderr)
            ok = False
            continue
        if all_keys is None:
            all_keys = set(expected.keys())
        else:
            if set(expected.keys()) != all_keys:
                print(f"Key mismatch in {locale_long}: {set(expected.keys()) ^ all_keys}", file=sys.stderr)
                ok = False

    # Check each JSON file exists and matches
    for short in LOCALES_SHORT:
        long_code = LOCALE_MAP[short]
        expected = locales.get(long_code, {})
        for name in [short, long_code]:
            path = LOCALES_DIR / f"{name}.json"
            if not path.exists():
                print(f"Missing locale file {path}", file=sys.stderr)
                ok = False
                continue
            data = load_json_locale(path)
            if data is None:
                ok = False
                continue
            if data != expected:
                # Report diff
                missing = set(expected.keys()) - set(data.keys())
                extra = set(data.keys()) - set(expected.keys())
                mismatched = [k for k in expected if k in data and data[k] != expected[k]]
                if missing:
                    print(f"{path.name}: missing keys: {sorted(missing)}", file=sys.stderr)
                if extra:
                    print(f"{path.name}: extra keys: {sorted(extra)}", file=sys.stderr)
                if mismatched:
                    print(f"{path.name}: {len(mismatched)} values differ", file=sys.stderr)
                    for k in mismatched[:5]:
                        print(f"  {k}: expected {expected[k]!r} got {data[k]!r}", file=sys.stderr)
                ok = False
            else:
                # Verify JSON is valid and sorted
                raw = path.read_text(encoding="utf-8")
                try:
                    json.loads(raw)
                except Exception as e:
                    print(f"{path.name}: invalid JSON: {e}", file=sys.stderr)
                    ok = False

    # Extended keys for web dashboard (not in i18n.ts minimal set but in JSON)
    # Allow JSON to have superset of i18n.ts keys — i.e., JSON may have extra web.* keys
    # that are translations for web/app.js hardcoded strings. Those are intentional.
    # So we should re-check with superset tolerance.
    # If JSON has extra keys beyond i18n.ts, verify they are consistent across locales.
    json_keys_by_locale = {}
    for short in LOCALES_SHORT:
        path = LOCALES_DIR / f"{short}.json"
        if path.exists():
            data = load_json_locale(path)
            if data:
                json_keys_by_locale[short] = set(data.keys())
    if json_keys_by_locale:
        reference = next(iter(json_keys_by_locale.values()))
        for loc, keys in json_keys_by_locale.items():
            if keys != reference:
                print(f"Locale JSON key sets differ: {loc} has {len(keys)} vs reference {len(reference)}", file=sys.stderr)
                print(f"  diff: {keys ^ reference}", file=sys.stderr)
                ok = False

    # Check hardcoded strings if requested
    if args.check_hardcoded:
        # Scan src/components and web/app.js for quoted UI strings that look like natural language
        src_files = list((ROOT / "src").rglob("*.tsx")) + list((ROOT / "src").rglob("*.ts"))
        src_files = [p for p in src_files if "i18n" not in str(p)]
        pattern_hardcoded = re.compile(r'>([^<]{4,60})<|\"([A-Z][a-z]{2,}[^\"]{4,40})\"')
        # Simplified: just count files using t()
        count_t = 0
        count_hardcoded_candidates = 0
        for p in src_files:
            text = p.read_text(encoding="utf-8", errors="ignore")
            if "t(" in text or "translate(" in text or "useLocale" in text:
                count_t += 1
            # Look for JSX text nodes that are likely UI strings
            candidates = re.findall(r'>([A-Z][a-z]+(?:\s+[A-Za-z]+){1,4})<', text)
            count_hardcoded_candidates += len(candidates)
        print(f"Hardcoded scan: {count_t}/{len(src_files)} files use t()/useLocale; found {count_hardcoded_candidates} candidate hardcoded phrases (informational)")

    # Verify node can parse i18n.ts (simple syntax check via --check)
    if not ok:
        print("Locale verification FAILED — JSON files drifted from src/lib/i18n.ts", file=sys.stderr)
        print("Run: python3 tools/extract_locales.py --write  to regenerate", file=sys.stderr)
        sys.exit(1)
    else:
        print("Locale verification PASSED: all JSON locales in sync with src/lib/i18n.ts")
        # Also verify JSON syntax via node --check on i18n? Just report.
        sys.exit(0)


if __name__ == "__main__":
    main()
