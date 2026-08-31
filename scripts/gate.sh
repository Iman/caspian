#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# gate.sh is the gate. It runs formatting, vet, the whole test suite with the
# race detector, and a per-package coverage floor.
#
# Named gate.sh, not check.sh, because "caspian check" is a user-facing
# subcommand that reports what the box looks like. This is the developer-facing
# build gate. Two different things; keep the two words apart.
#
# It is runnable from anywhere in the repository: it finds the module root by
# walking up from its own location to the directory holding go.mod, and every
# path below is relative to that.
#
# Output carries no ANSI escape codes, no colour, no emoji and no em dashes, so
# that it is readable in a log file, in a CI web view and through a pipe.
#
# READ THIS BEFORE YOU PIPE THIS SCRIPT INTO ANYTHING.
#
#     bash scripts/gate.sh | tail -40      # WRONG: reports tail's status, not the gate's
#     bash scripts/gate.sh | tee gate.log  # WRONG: reports tee's status
#
# A shell pipeline exits with the status of its LAST command, so piping this
# script into tail, tee, head or grep throws away the exit code that says
# whether the gate passed, and hands the caller a 0 that means "tail worked".
# Every use of that shape is a false green.
#
# Measured on 2026-08-30: this exact trap hid this gate's own first failure. It
# printed its FAILED verdict and exited 1, and the harness that ran it through
# "| tail -40" reported success, so the gate went unobserved-failing for three
# runs, including the run that first proved it worked. It was the THIRD instance of the same shape in this project on one day:
# a clean build reported while "| head" hid the compiler error, and a mutation
# reported as caught while it had silently failed to apply.
#
# Do this instead:
#
#     bash scripts/gate.sh > gate.log 2>&1; echo "exit: $?"
#     set -o pipefail                      # if you must pipe, at minimum this
#
# and in CI, run it unpiped and let the runner read the exit code.
#
# THE FLOORS ARE A RATCHET, NOT AN ASPIRATION.
#
# Every number in the table below is what the package MEASURED after the work
# that introduced it, not a target somebody hoped for. That is the whole point:
# a floor set above what the package achieves fails the build on day one and
# gets deleted; a floor set below what it achieves lets coverage fall silently
# back to it. Raise a floor in the same change that raises the coverage.
#
# A package with no row here is NOT gated on coverage. It still has to compile,
# pass vet and pass its tests. Read the absence of a row as "no floor agreed
# yet", never as "this package is covered".

set -o errexit
set -o nounset
set -o pipefail

# --- locate the module root -------------------------------------------------

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
root=$script_dir
while [ "$root" != "/" ] && [ ! -f "$root/go.mod" ]; do
    root=$(dirname "$root")
done
if [ ! -f "$root/go.mod" ]; then
    echo "check: could not find go.mod above $script_dir" >&2
    exit 2
fi
cd "$root"

# --- the coverage floors ----------------------------------------------------
#
# One row per gated package:
#
#     <import path suffix> <floor as a percentage> [GOOS the floor applies to]
#
# Measured on 2026-08-30 with: go test -count=1 -race -cover ./...
#
# THE THIRD FIELD, AND WHY IT IS NOT A WAY OF LOWERING A FLOOR.
#
# internal/engine gained code that only exists on Linux: the release of the TUN
# device the engine creates, which needs netlink and a real /dev/net/tun. Its
# tests carry a linux build tag and skip without root, so on a developer Mac
# that code is not compiled in and not run, and the package reads 89.3 per cent
# against a floor of 98 that it met the day the floor was written.
#
# There are three ways to answer that and two of them are wrong. Lowering the
# floor to 89.3 tells every future reader that this package is allowed to be
# 89.3 per cent covered, which is false everywhere it actually ships. Deleting
# the row loses the ratchet entirely. So the floor stays at the number the
# package achieves where its code runs, and it is enforced there.
#
# A row with no third field is enforced everywhere, which is every other row.
# A row naming a GOOS is enforced on that GOOS and reported as NOT HERE
# elsewhere, loudly, so a green run on a Mac cannot be mistaken for a package
# having been measured.

floors=$(
    /bin/cat <<'EOF'
internal/engine 98.0 linux
internal/link 98.6
internal/hotspot 99.1
internal/state 96.9
internal/xcfg 97.0
EOF
)

goos=$(go env GOOS)

module=$(awk '/^module /{print $2; exit}' go.mod)

fail=0
step() { printf '\n=== %s\n' "$1"; }
problem() { printf 'FAIL %s\n' "$1"; fail=1; }

# --- gofmt ------------------------------------------------------------------

step "gofmt"
# gofmt -l lists files whose formatting differs. Anything listed is a failure.
# -s is not used: it rewrites correct code into shorter code and that is a
# review opinion, not a formatting fact.
unformatted=$(gofmt -l . 2>/dev/null || true)
if [ -n "$unformatted" ]; then
    echo "these files are not gofmt clean:"
    echo "$unformatted" | sed 's/^/  /'
    echo "fix with: gofmt -w <file>"
    problem "gofmt"
else
    echo "ok"
fi

# --- go vet -----------------------------------------------------------------

step "go vet"
if go vet ./... 2>&1; then
    echo "ok"
else
    problem "go vet"
fi

# --- tests, with the race detector ------------------------------------------
#
# -count=1 defeats the test result cache. Without it a second run prints the
# first run's PASS lines and exits 0 having executed nothing, which is a green
# gate that tested no code. Same family as the pipeline trap in the header: a
# status that describes something other than the work you cared about.
#
# The output is kept so the coverage floors below are computed from THIS run
# rather than from a stored number.

step "go test -count=1 -race -cover ./..."
test_output=$(mktemp)
trap 'rm -f "$test_output"' EXIT

test_status=0
go test -count=1 -race -cover ./... 2>&1 | tee "$test_output" || test_status=$?
if [ "$test_status" -ne 0 ]; then
    problem "go test"
fi

# Assert the run actually executed packages. A suite that matched nothing exits
# 0 and proves nothing, which is the failure mode this line exists to catch.
executed=$(grep -c -E '^(ok|FAIL|---)' "$test_output" || true)
if [ "$executed" -eq 0 ]; then
    echo "the test run reported no packages at all; nothing was executed"
    problem "go test executed nothing"
fi

# --- the coverage floors ----------------------------------------------------

step "coverage floors"
printf '%-28s %8s %8s  %s\n' "PACKAGE" "FLOOR" "ACTUAL" "RESULT"

while read -r pkg floor only_on; do
    [ -z "$pkg" ] && continue

    if [ -n "$only_on" ] && [ "$only_on" != "$goos" ]; then
        printf '%-28s %8s %8s  %s\n' "$pkg" "$floor" "-" "NOT HERE (needs $only_on)"
        printf '     this package has code that only exists on %s, so the figure this run\n' "$only_on"
        printf '     would produce says nothing about it. Run the gate on %s to enforce it.\n' "$only_on"
        continue
    fi

    # Lines look like:
    #   ok  <module>/internal/link  0.9s  coverage: 98.6% of statements
    line=$(grep -E "[[:space:]]${module}/${pkg}[[:space:]]" "$test_output" | head -n 1 || true)
    if [ -z "$line" ]; then
        printf '%-28s %8s %8s  %s\n' "$pkg" "$floor" "-" "NO RESULT"
        problem "coverage: $pkg produced no test result"
        continue
    fi

    actual=$(echo "$line" | sed -n 's/.*coverage: \([0-9.]*\)% of statements.*/\1/p')
    if [ -z "$actual" ]; then
        printf '%-28s %8s %8s  %s\n' "$pkg" "$floor" "-" "NO COVERAGE"
        problem "coverage: $pkg reported no coverage figure"
        continue
    fi

    # awk rather than bash, which has no floating point comparison.
    if awk -v a="$actual" -v f="$floor" 'BEGIN { exit (a + 0 < f + 0) ? 0 : 1 }'; then
        printf '%-28s %8s %8s  %s\n' "$pkg" "$floor" "$actual" "BELOW FLOOR"
        problem "coverage: $pkg is at ${actual}%, below its floor of ${floor}%"
    else
        printf '%-28s %8s %8s  %s\n' "$pkg" "$floor" "$actual" "ok"
    fi
done <<EOF
$floors
EOF

# --- the behaviour-regression layer -----------------------------------------
#
# Added 2026-08-30. Everything above answers "does the code still do what its
# tests say it should". This answers a different question: "does the code still
# EMIT what it emitted yesterday". A property test is blind to everything it does
# not name, so a page can lose a section, a status document can lose a field, and
# a message key can disappear from one language with every test above green.
#
# It is a separate script because its verdict means something different, and
# because it has to be runnable on its own while somebody is iterating on a
# golden. It is called from here rather than left optional because a regression
# layer nobody runs is a regression layer that does not exist.
#
# The golden tests themselves have ALREADY RUN above, inside "go test ./...", so
# a failure there fails this gate whether or not this step runs. What this step
# adds is what the plain run cannot say: that the golden tests EXECUTED at all
# (a -run pattern that matches nothing exits 0), that every golden package is
# reachable by scripts/golden-update.sh, and the leak scan's open findings
# printed in full.

step "behaviour regression (scripts/regression.sh)"
if [ ! -f "$root/scripts/regression.sh" ]; then
    # Refuse to pass on a missing gate rather than skipping it, the same way
    # release-all.sh refuses to run without its release gate file. A check that
    # silently disappears is worse than one that fails.
    echo "scripts/regression.sh is missing. The behaviour-regression layer cannot be run,"
    echo "so nothing checked whether this product still emits what it emitted yesterday."
    problem "regression.sh is missing"
elif bash "$root/scripts/regression.sh"; then
    :
else
    problem "behaviour regression"
fi

# --- the smoke subset -------------------------------------------------------
#
# Run LAST and not first, deliberately. Smoke is a strict subset of what has
# already run above, so it can never find something the full run did not. What
# it is here for is to prove the subset itself still works: that every test
# named in test/smoke/smoke.list still exists, that each package in it still
# executes something, and that the whole thing is still fast enough that anybody
# will run it. A smoke subset that has quietly rotted is discovered on the day
# somebody needed it, which is the worst day to discover it.

step "smoke subset (scripts/smoke.sh)"
if [ ! -f "$root/scripts/smoke.sh" ]; then
    echo "scripts/smoke.sh is missing, so the fast subset is unrunnable"
    problem "smoke.sh is missing"
elif bash "$root/scripts/smoke.sh"; then
    :
else
    problem "smoke subset"
fi

# --- verdict ----------------------------------------------------------------

printf '\n'
if [ "$fail" -ne 0 ]; then
    echo "gate: FAILED"
    exit 1
fi
echo "gate: passed"
