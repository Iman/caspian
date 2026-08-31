#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# run-all.sh runs both BDD suites and reports one verdict.
#
# This is the one command in the README. It installs what is missing, runs the
# HTTP suite and then the browser suite, and prints a summary that says how many
# scenarios each one executed.
#
# It reports the SCENARIO COUNTS and not only the exit codes, because a suite
# that matched nothing exits 0 and proves nothing. That is the same guard
# scripts/gate.sh applies to `go test` after a run that reported no packages,
# and the same false green that `cucumber-js --tags @known-defect` produced here
# on 2026-08-30 by ANDing with the default profile's tag filter.
#
# It does NOT run the scenario tagged known-defect, which fails on purpose. See
# bdd/README.md. Run that one with:
#
#     cd bdd/web && npx cucumber-js --profile defect
#
# READ THIS BEFORE YOU PIPE THIS SCRIPT INTO ANYTHING.
#
#     bash bdd/run-all.sh | tail -40    # WRONG: reports tail's status
#
# A shell pipeline exits with the status of its LAST command. Do this instead:
#
#     bash bdd/run-all.sh > bdd.log 2>&1; echo "exit: $?"

set -o errexit
set -o nounset
set -o pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
root=$(dirname "$script_dir")
cd "$root"

# No colour, anywhere, ever.
export FORCE_COLOR=0

fail=0
summary=""

run_suite() {
    local suite=$1
    local dir="$root/bdd/$suite"
    local log
    log=$(mktemp)

    printf '\n=== %s\n' "$suite"

    if [ ! -d "$dir/node_modules" ]; then
        echo "installing dependencies for $suite (first run only)"
        (cd "$dir" && npm install)
    fi

    local status=0
    (cd "$dir" && npx cucumber-js) > "$log" 2>&1 || status=$?
    cat "$log"

    local ran
    ran=$(sed -n 's/^\([0-9][0-9]*\) scenario.*/\1/p' "$log" | head -n 1)
    ran=${ran:-0}

    if [ "$ran" -eq 0 ]; then
        summary="${summary}$(printf '%-4s %-10s %s' "$suite" "0 scenarios" "NOTHING RAN")
"
        fail=1
    elif [ "$status" -ne 0 ]; then
        summary="${summary}$(printf '%-4s %-10s %s' "$suite" "$ran scenarios" "FAILED")
"
        fail=1
    else
        summary="${summary}$(printf '%-4s %-10s %s' "$suite" "$ran scenarios" "passed")
"
    fi

    rm -f "$log"
}

run_suite api
run_suite web

printf '\nSUMMARY\n'
printf '%s' "$summary"

printf '\n'
if [ "$fail" -ne 0 ]; then
    echo "bdd: FAILED"
    exit 1
fi
echo "bdd: passed"
