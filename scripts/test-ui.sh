#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
export NO_COLOR=1
ROOT="$(cd -- "$(dirname -- "$0")/.." && pwd)"
OUT="$ROOT/ui/build/unit-tests"
mkdir -p "$OUT"
STARTED="$(python3 -c 'import time; print(time.time())')"
printf '%s\n' "$STARTED" > "$OUT/started.txt"
cd "$ROOT/ui"
flutter --suppress-analytics pub get
flutter --suppress-analytics analyze --no-pub
flutter --suppress-analytics test --no-pub --coverage \
  --coverage-path="$OUT/flutter.lcov" --reporter json > "$OUT/flutter-tests.json"
cd "$ROOT"
go test -count=1 -coverprofile="$OUT/go.cover" -json ./... > "$OUT/go-tests.json"
python3 scripts/check-unit-coverage.py \
  --lcov "$OUT/flutter.lcov" --flutter-results "$OUT/flutter-tests.json" \
  --go-profile "$OUT/go.cover" --go-results "$OUT/go-tests.json" --since "$STARTED"
