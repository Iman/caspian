#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# regression.sh is the behaviour-regression layer: every golden test in the
# repository, plus the leak scan over every committed fixture.
#
# It is a sibling of scripts/gate.sh, not a replacement. gate.sh calls it. It is
# separate because it answers a different question and the two must not be
# confused:
#
#     gate.sh        does the code still do what its tests say it should
#     regression.sh  does the code still emit what it emitted yesterday
#
# The second question has no opinion about correctness. A golden pins a known
# defect exactly as willingly as it pins correct behaviour, and two of them here
# do; see internal/panel/testdata/PROVENANCE.md. A green run of this script means
# NOTHING CHANGED. It does not mean anything is right.
#
# Output carries no ANSI escape codes, no colour, no emoji and no em dashes.
#
# READ THIS BEFORE YOU PIPE THIS SCRIPT INTO ANYTHING.
#
#     bash scripts/regression.sh | tail -40      # WRONG: reports tail's status
#     bash scripts/regression.sh | tee reg.log   # WRONG: reports tee's status
#
# The full reasoning, and the measured incident behind it, is in the header of
# scripts/gate.sh. Do this instead:
#
#     bash scripts/regression.sh > reg.log 2>&1; echo "exit: $?"
#
# NO RACE DETECTOR HERE, ON PURPOSE. gate.sh runs the whole suite with -race,
# including every test this script runs. Running them again under -race would
# roughly double the gate for no new information: measured 2026-08-30, the race
# detector takes internal/xcfg from about 15 seconds to 327. What this script
# adds is the assertion that the golden tests EXECUTED, which -race does not
# affect.

set -o errexit
set -o nounset
set -o pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
root=$script_dir
while [ "$root" != "/" ] && [ ! -f "$root/go.mod" ]; do
    root=$(dirname "$root")
done
if [ ! -f "$root/go.mod" ]; then
    echo "regression: could not find go.mod above $script_dir" >&2
    exit 2
fi
cd "$root"

fail=0
step() { printf '\n=== %s\n' "$1"; }
problem() { printf 'FAIL %s\n' "$1"; fail=1; }

out=$(mktemp)
trap 'rm -f "$out" "$out.2"' EXIT

# --- the golden tests -------------------------------------------------------
#
# -count=1 defeats the result cache. Without it a second run prints the first
# run's PASS lines and exits 0 having executed nothing. Same family as the
# pipeline trap above: a status that describes something other than the work you
# cared about.
#
# -v so that the PASS lines can be COUNTED. An exit code of 0 from
# "go test -run Golden" is also what you get when the pattern matches no test at
# all, which is a green run that pinned nothing. The count below is the evidence
# that something executed; the exit code is not.

step "golden tests (-run Golden, -count=1)"
status=0
go test ./... -count=1 -run 'Golden' -v > "$out" 2>&1 || status=$?
grep -E '^(=== RUN|--- (PASS|FAIL|SKIP)|ok |FAIL|    )' "$out" | grep -vE '^=== RUN' || true

if [ "$status" -ne 0 ]; then
    problem "golden tests"
fi

ran=$(grep -c -E '^ *--- PASS: ' "$out" || true)
failed=$(grep -c -E '^ *--- FAIL: ' "$out" || true)
printf '\n%s golden test(s) passed, %s failed\n' "$ran" "$failed"

# The floor is a ratchet in the same sense as the coverage floors in gate.sh: it
# is what the layer MEASURED after the work that introduced it, not a target.
# Raise it in the same change that adds goldens.
#
# Measured 2026-08-30 on this machine: 99 passing golden tests and subtests
# across internal/netcfg, internal/hotspot, internal/xcfg and internal/panel.
# The floor is set well below that so an ordinary addition or removal does not
# trip it, and far enough above zero that a pattern matching nothing does.
min_golden=50
if [ "$ran" -lt "$min_golden" ]; then
    printf 'only %s golden test(s) executed, and the floor is %s.\n' "$ran" "$min_golden"
    printf 'A "go test -run Golden" that matches nothing exits 0 and pins nothing.\n'
    printf 'Either the tests were renamed out of the pattern, or the layer was deleted.\n'
    problem "the golden layer executed almost nothing"
fi

# --- every golden test is reachable by the update script --------------------
#
# A golden test in a package scripts/golden-update.sh does not name is a golden
# nobody can regenerate, which is the failure this whole layer exists to avoid.

step "every package with golden tests is covered by scripts/golden-update.sh"
covered=$(grep -oE '^internal/[a-z]+ Golden' scripts/golden-update.sh | awk '{print $1}' | sort -u)
having=$(grep -rlE '^func TestGolden' --include='*_test.go' internal cmd test 2>/dev/null |
    xargs -n1 dirname | sort -u | sed 's|^\./||')
missing=""
for pkg in $having; do
    case "$pkg" in
        test/goldenscan | test/smoke) continue ;;  # guards, not golden owners
    esac
    if ! echo "$covered" | grep -qx "$pkg"; then
        missing="$missing $pkg"
    fi
done
if [ -n "$missing" ]; then
    echo "these packages declare a TestGolden and are not named in scripts/golden-update.sh:"
    for m in $missing; do echo "  $m"; done
    echo "A golden nobody can regenerate is a golden that will be deleted the first time"
    echo "somebody cannot work out how to update it."
    problem "golden-update.sh does not cover every golden package"
else
    echo "ok"
fi

# --- the leak scan ----------------------------------------------------------
#
# Separate from the golden run above because its verdict is different in kind.
# A golden failure says the output moved. This says a credential is in a
# committed file, which is permanent the moment it is pushed.

step "leak scan over every committed fixture"
scan_status=0
go test ./test/goldenscan -count=1 -v > "$out.2" 2>&1 || scan_status=$?
grep -E '^ *(--- (PASS|FAIL)|scan_test\.go:.*(OPEN FINDING|scan complete))' "$out.2" || true
# The open findings, printed in full every run so they cannot go quiet.
if grep -q 'OPEN FINDING' "$out.2"; then
    printf '\nOPEN FINDINGS, reported and not failing this scan:\n'
    sed -n '/OPEN FINDING/,/^ *scan_test.go:[0-9]*: [^ ]/p' "$out.2" | sed 's/^ */  /' | head -40
fi
if [ "$scan_status" -ne 0 ]; then
    problem "leak scan"
fi
scan_ran=$(grep -c -E '^--- PASS: ' "$out.2" || true)
if [ "$scan_ran" -eq 0 ]; then
    echo "the leak scan reported no passing tests, so nothing was scanned"
    problem "leak scan executed nothing"
fi

# --- the smoke registry -----------------------------------------------------

step "smoke subset registry"
if go test ./test/smoke -count=1 2>&1; then
    echo "ok"
else
    problem "smoke registry"
fi

# --- verdict ----------------------------------------------------------------

printf '\n'
if [ "$fail" -ne 0 ]; then
    echo "regression: FAILED"
    echo
    echo "If the output of this product changed on purpose:"
    echo "    bash scripts/golden-update.sh"
    echo "then READ THE DIFF and commit it."
    exit 1
fi
echo "regression: passed"
echo "This means NOTHING CHANGED. It does not mean anything is correct."
