#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
export NO_COLOR=1
ROOT="$(cd -- "$(dirname -- "$0")/.." && pwd)"
TARGET="${1:-macos}"
case "$TARGET" in macos|linux|windows) ;; *) echo 'Choose macos, linux or windows.' >&2; exit 2 ;; esac
if [[ $# -gt 0 ]]; then shift; fi
WORK="$(mktemp -d)"
FIXTURE="$WORK/fixture"
if [[ "$TARGET" = windows ]]; then FIXTURE="$FIXTURE.exe"; fi
EVIDENCE="$ROOT/ui/build/native-tests/$TARGET"
mkdir -p "$EVIDENCE"
python3 -c 'import time; print(time.time())' > "$EVIDENCE/started.txt"
FIXTURE_PID=''
cleanup() {
  if [[ -n "$FIXTURE_PID" ]]; then kill "$FIXTURE_PID" 2>/dev/null || true; wait "$FIXTURE_PID" 2>/dev/null || true; fi
  rm -rf "$WORK"
}
trap cleanup EXIT
cd "$ROOT"
go build -tags flutterui -o "$FIXTURE" ./bdd/harness
"$FIXTURE" > "$WORK/fixture.json" 2> "$WORK/fixture.log" &
FIXTURE_PID=$!
for attempt in {1..100}; do
  [[ -s "$WORK/fixture.json" ]] && break
  kill -0 "$FIXTURE_PID" || { cat "$WORK/fixture.log" >&2; exit 1; }
  sleep 0.1
done
ORIGIN="$(python3 -c 'import json,sys; print(json.loads(open(sys.argv[1]).readline())["url"])' "$WORK/fixture.json")"
cd "$ROOT/ui"
flutter --suppress-analytics test integration_test/application_test.dart -d "$TARGET" \
  --dart-define="CASPIAN_TEST_ORIGIN=$ORIGIN" "$@" --reporter json > "$EVIDENCE/results.jsonl" 2> "$EVIDENCE/build.log" || {
    cat "$EVIDENCE/build.log" "$EVIDENCE/results.jsonl" >&2
    exit 1
  }
python3 - "$ROOT" "$EVIDENCE" <<'PY'
import importlib.util
import json
from pathlib import Path
import sys

root, evidence = map(Path, sys.argv[1:])
spec = importlib.util.spec_from_file_location('coverage_gate', root / 'scripts/check-unit-coverage.py')
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)
results = evidence / 'results.jsonl'
gate.fresh(results, float((evidence / 'started.txt').read_text()))
# Flutter's native builder may include progress lines before JSON test events.
rows = [json.loads(line) for line in results.read_text().splitlines() if line.startswith('{')]
passed, skipped = gate.flutter_counts(rows)
if skipped:
    raise SystemExit('Native integration must not skip tests')
print(f'Native integration: {passed} passed, {skipped} skipped')
PY
