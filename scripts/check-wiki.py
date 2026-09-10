#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""Check wiki language coverage and literal technical content without network access."""

import argparse
import hashlib
from collections import Counter
from pathlib import Path
import re
import sys

LANGUAGES = ("fa", "ru", "zh", "ar", "tr", "ur")
BASE = "https://github.com/Iman/caspian/wiki/"


def literals(text):
    fences = re.findall(r"^```[^\n]*\n.*?^```[^\n]*$", text, re.M | re.S)
    body = re.sub(r"^```[^\n]*\n.*?^```[^\n]*$", "", text, flags=re.M | re.S)
    return Counter(fences), Counter(re.findall(r"`[^`\n]+`", body))


def check(root):
    errors = []
    sources = sorted(p for p in root.glob("*.md") if p.stem.split(".")[-1] not in LANGUAGES)
    for source in sources:
        english = source.read_text(encoding="utf-8")
        expected = literals(english)
        digest = hashlib.sha256(english.encode("utf-8")).hexdigest()
        def tables(text):
            body = re.sub(r"^```[^\n]*\n.*?^```[^\n]*$", "", text, flags=re.M | re.S)
            return [line.count("|") for line in body.splitlines() if line.startswith("|")]
        def external_links(text):
            return {url for url in re.findall(r"\]\((https?://[^)]+)\)", text) if not url.startswith(BASE)}
        for language in LANGUAGES:
            target = root / f"{source.stem}.{language}.md"
            if not target.exists():
                errors.append(f"{target.name}: missing translation")
                continue
            text = target.read_text(encoding="utf-8")
            if f"<!-- English-source-sha256: {digest} -->" not in text:
                errors.append(f"{target.name}: translation needs review against the current English source")
            if literals(text) != expected:
                errors.append(f"{target.name}: code blocks or inline code differ from English")
            if re.findall(r"^#{1,6} ", text, re.M) != re.findall(r"^#{1,6} ", english, re.M):
                errors.append(f"{target.name}: heading structure differs from English")
            if re.search(r"ZXQ[LP]\d|\ufffd", text):
                errors.append(f"{target.name}: unresolved translation marker or encoding error")
            if tables(text) != tables(english):
                errors.append(f"{target.name}: table rows or columns differ from English")
            missing = external_links(english) - external_links(text)
            if missing:
                errors.append(f"{target.name}: missing external source links: {sorted(missing)}")
            for lang in ("", *LANGUAGES):
                suffix = f".{lang}" if lang else ""
                if f"{BASE}{source.stem}{suffix})" not in text:
                    errors.append(f"{target.name}: missing {lang or 'English'} language link")
        for page in [source, *(root / f"{source.stem}.{lang}.md" for lang in LANGUAGES)]:
            if not page.exists():
                continue
            for name in re.findall(re.escape(BASE) + r"([A-Za-z_.-]+)", page.read_text(encoding="utf-8")):
                if not (root / (name + ".md")).exists():
                    errors.append(f"{page.name}: wiki link has no page: {name}")
    return sources, errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", nargs="?", type=Path, default=Path(__file__).resolve().parents[1] / "docs/wiki")
    args = parser.parse_args()
    sources, errors = check(args.directory)
    for error in errors:
        print(error)
    if errors:
        print(f"FAIL: {len(errors)} wiki parity errors")
        return 1
    print(f"PASS: {len(sources)} topics in seven languages; code, headings, navigation and wiki targets match")
    return 0


if __name__ == "__main__":
    sys.exit(main())
