#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# smoke.sh runs the smallest set of tests that would catch a broken build.
#
#     bash scripts/smoke.sh
#
# The membership list is test/smoke/smoke.list, which explains what is in it and
# why. This script is the runner.
#
# WHAT A GREEN SMOKE RUN MEANS, AND WHAT IT DOES NOT.
#
# It means the tree compiles and the load-bearing paths still work. It does NOT
# mean the gate would pass. Smoke carries no race detector, no coverage floors,
# and one or two tests per package rather than the package. Smoke green and gate
# red is an ordinary outcome. Only scripts/gate.sh gates anything.
#
# THE EXIT CODE IS NOT THE EVIDENCE, AND THAT IS THE WHOLE DESIGN OF THIS FILE.
#
#     go test ./internal/panel -run '^TestThatWasRenamed$'
#     ok      caspianbyoc.org/caspian/internal/panel   0.2s
#     exit 0
#
# Nothing executed. So this script runs every package with -v and COUNTS the
# PASS lines, refuses a package that produced none, and refuses a run whose
# total is below a floor. That is the artefact, not the status line. The same
# reasoning, and the three measured false greens behind it, are in the header of
# scripts/gate.sh.
#
# Output carries no ANSI escape codes, no colour, no emoji and no em dashes.
#
# Do not pipe this into tail, tee, head or grep: a pipeline exits with the
# status of its last command and throws away the one you wanted. Redirect
# instead:
#
#     bash scripts/smoke.sh > smoke.log 2>&1; echo "exit: $?"

set -o errexit
set -o nounset
set -o pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
root=$script_dir
while [ "$root" != "/" ] && [ ! -f "$root/go.mod" ]; do
    root=$(dirname "$root")
done
if [ ! -f "$root/go.mod" ]; then
    echo "smoke: could not find go.mod above $script_dir" >&2
    exit 2
fi
cd "$root"

list="test/smoke/smoke.list"
if [ ! -f "$list" ]; then
    echo "smoke: $list is missing, so there is no subset to run" >&2
    exit 2
fi

fail=0
total=0
out=$(mktemp)
trap 'rm -f "$out"' EXIT

start=$(date +%s)

echo "smoke subset, from $list"
echo "no race detector, no coverage floors: this is a broken-build check, not the gate"
echo

# --- it has to build first --------------------------------------------------
#
# A package whose test binary does not compile is reported by go test as a
# failure, but only for that package. This builds everything, including the code
# with no tests, because "the tree compiles" is the first thing a smoke run is
# for.

printf '=== go build ./...\n'
if go build ./... 2>&1; then
    echo "ok"
else
    printf 'FAIL go build\n'
    fail=1
fi

# --- the subset -------------------------------------------------------------

printf '\n%-24s %8s  %s\n' "PACKAGE" "PASSED" "RESULT"

while read -r pkg pattern; do
    case "$pkg" in
        '' | \#*) continue ;;
    esac
    [ -z "${pattern:-}" ] && continue

    status=0
    go test "./$pkg" -count=1 -run "$pattern" -v > "$out" 2>&1 || status=$?
    passed=$(grep -c -E '^ *--- PASS: ' "$out" || true)
    total=$((total + passed))

    if [ "$status" -ne 0 ]; then
        printf '%-24s %8s  %s\n' "$pkg" "$passed" "FAILED"
        grep -E '^ *(--- FAIL|\s+[a-z_]+\.go:[0-9]+:)' "$out" | head -20 | sed 's/^/     /'
        fail=1
        continue
    fi
    if [ "$passed" -eq 0 ]; then
        printf '%-24s %8s  %s\n' "$pkg" "$passed" "EXECUTED NOTHING"
        printf '     "%s" matched no test in %s. go test exited 0 having run nothing,\n' "$pattern" "$pkg"
        printf '     which is a green result that tested no code. Fix the pattern in %s.\n' "$list"
        fail=1
        continue
    fi
    printf '%-24s %8s  %s\n' "$pkg" "$passed" "ok"
done < "$list"

# --- the floor --------------------------------------------------------------
#
# Measured 2026-08-30 on this machine: 50 passing tests and subtests across 10
# packages, in 15 seconds. The floor is set well below that so adding or removing
# an entry does not trip it, and far enough above zero that a list that has
# quietly emptied does.
min_total=25
printf '\n%s test(s) executed in total\n' "$total"
if [ "$total" -lt "$min_total" ]; then
    printf 'the floor is %s. A smoke subset this small is not a smoke subset.\n' "$min_total"
    fail=1
fi

elapsed=$(( $(date +%s) - start ))
printf '%s seconds\n' "$elapsed"
# Not a failure: a slow machine is not a broken build. It is said out loud
# because a smoke subset that stops being fast stops being run.
if [ "$elapsed" -gt 30 ]; then
    printf 'NOTE: the smoke subset took longer than its 30 second budget.\n'
    printf '      A smoke check nobody waits for is a smoke check nobody runs.\n'
    printf '      Move the slowest entry out of %s.\n' "$list"
fi

printf '\n'
if [ "$fail" -ne 0 ]; then
    echo "smoke: FAILED"
    exit 1
fi
echo "smoke: passed"
echo "This is not the gate. Run: bash scripts/gate.sh"
