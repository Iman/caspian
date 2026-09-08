#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "$0")/../.." && pwd)"
export FORCE_COLOR=0
export NO_COLOR=1
test -d "$ROOT/bdd/web/node_modules" || npm ci --prefix "$ROOT/bdd/web"
cd "$ROOT/bdd/flutter"
mkdir -p reports
REPORT_DIR="$(mktemp -d "$PWD/reports/run.XXXXXX")"
export CASPIAN_BDD_REPORT_DIR="$REPORT_DIR"
REPORT="$REPORT_DIR/cucumber.json"
STARTED="$(node -p 'Date.now()')"
STATUS=0
node ../web/node_modules/@cucumber/cucumber/bin/cucumber-js --format "json:$REPORT" "$@" || STATUS=$?
node verify-report.js "$REPORT" "$STARTED" || STATUS=1
printf 'Flutter BDD report: %s\n' "$REPORT"
exit "$STATUS"
