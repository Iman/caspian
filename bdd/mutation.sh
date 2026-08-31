#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# mutation.sh is the evidence that the two Cucumber suites are worth running.
#
# A scenario nobody has watched fail is not evidence. test/bdd makes that point
# in behaviour_test.go and enforces it with TestEveryScenarioCanFail, which runs
# every scenario a second time with a named fault injected and requires red.
# This is the same job for the suites under bdd/web and bdd/api.
#
# HOW IT WORKS
#
# One cucumber run per suite, with CASPIAN_MUTATION=1. In that mode the Before
# hook looks up the scenario's OWN tag in bdd/defects.json and rebuilds the
# appliance carrying that defect, so every scenario runs against a build with
# its own subject broken and nothing else. bdd/mutation-report.js then reads the
# JSON report and prints one row per scenario.
#
# It was one cucumber process per scenario at first, which meant one Chrome per
# scenario. On this machine that leaked browsers and wedged on the fourteenth
# row, measured 2026-08-30. One process, one browser, and an appliance rebuilt
# between scenarios gives the same isolation without the cost.
#
# TWO GUARDS, BOTH FROM MISTAKES MADE IN THIS PROJECT.
#
# A run that matches no scenarios exits 0 and proves nothing. That happened
# here: `cucumber-js --tags @known-defect` against a profile carrying
# `tags: not @known-defect` ANDed the two, matched nothing, and reported
# success. So both this script and mutation-report.js refuse a run that
# executed nothing.
#
# And this script never pipes a command whose exit code it needs. Piping into
# tail, tee, head or grep reports the LAST command's status, which is a green
# meaning "tail worked". scripts/gate.sh records that trap and the three times
# it bit this project in one day.
#
# Output carries no ANSI escape codes, no colour, no emoji and no em dashes.
#
# Usage:
#
#     bash bdd/mutation.sh            # both suites
#     bash bdd/mutation.sh web        # the browser suite only
#     bash bdd/mutation.sh api        # the HTTP suite only

set -o errexit
set -o nounset
set -o pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
root=$(dirname "$script_dir")
cd "$root"

only=${1:-all}

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

export FORCE_COLOR=0

fail=0

run_suite() {
    local suite=$1
    local dir="$root/bdd/$suite"
    local report="$work/$suite.json"
    local log="$work/$suite.log"

    printf '\n=== mutation run: %s\n' "$suite"

    if [ ! -d "$dir/node_modules" ]; then
        echo "installing dependencies for $suite (first run only)"
        (cd "$dir" && npm install)
    fi

    # The "all" profile, because the default profile excludes the known-defect
    # scenario and a mutation run has to cover every scenario there is.
    local status=0
    (
        cd "$dir"
        CASPIAN_MUTATION=1 npx cucumber-js --profile all --format "json:$report"
    ) > "$log" 2>&1 || status=$?

    # The cucumber exit code is EXPECTED to be non-zero here: nearly every
    # scenario is supposed to fail. It is recorded and not acted on. What
    # decides the verdict is the per-scenario table below.
    printf 'cucumber exit code: %s (non-zero is expected in a mutation run)\n' "$status"

    if [ ! -s "$report" ]; then
        echo "no cucumber report was written, so nothing can be checked"
        sed -n '1,40p' "$log"
        fail=1
        return
    fi

    local report_status=0
    node "$root/bdd/mutation-report.js" "$suite" "$report" || report_status=$?
    if [ "$report_status" -ne 0 ]; then
        fail=1
        printf '\n--- the last 40 lines of the %s run ---\n' "$suite"
        sed -n "$(( $(wc -l < "$log") > 40 ? $(wc -l < "$log") - 40 : 1 )),\$p" "$log"
    fi
}

printf 'MUTATION RUN\n'
printf 'Every scenario runs against an appliance carrying the defect its own tag names.\n'
printf 'The registry is bdd/defects.json; the defects are in bdd/harness/main.go.\n'

if [ "$only" = "all" ] || [ "$only" = "api" ]; then
    run_suite api
fi
if [ "$only" = "all" ] || [ "$only" = "web" ]; then
    run_suite web
fi

printf '\n'
if [ "$fail" -ne 0 ]; then
    echo "mutation: FAILED"
    exit 1
fi
echo "mutation: passed"
