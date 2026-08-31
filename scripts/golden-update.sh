#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# golden-update.sh rewrites every golden file in the repository.
#
# THIS IS THE ONE COMMAND. A golden file nobody can regenerate rots: the first
# time somebody cannot work out how to update it they comment the test out, and
# the file stops recording anything while still looking as though it does. So
# there is one command, it is named in the failure message of every golden test,
# and it is named in every PROVENANCE.md.
#
#     bash scripts/golden-update.sh
#
# THEN READ THE DIFF. That is not a pleasantry. The entire value of this layer is
# that a change to what the product emits arrives as a diff a person approves.
# Running this and committing without reading converts the layer into a slower
# way of having no layer at all.
#
# Output carries no ANSI escape codes, no colour, no emoji and no em dashes.
#
# BEFORE YOU PIPE THIS INTO ANYTHING, read the header of scripts/gate.sh. The
# same trap applies: a shell pipeline exits with the status of its LAST command,
# so "bash scripts/golden-update.sh | tee log" reports whether tee worked.
#
#     bash scripts/golden-update.sh > update.log 2>&1; echo "exit: $?"

set -o errexit
set -o nounset
set -o pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
root=$script_dir
while [ "$root" != "/" ] && [ ! -f "$root/go.mod" ]; do
    root=$(dirname "$root")
done
if [ ! -f "$root/go.mod" ]; then
    echo "golden-update: could not find go.mod above $script_dir" >&2
    exit 2
fi
cd "$root"

# Every package that owns golden files, and the -run pattern that regenerates
# them. A package whose golden tests are not named here does not get updated by
# this script, and scripts/regression.sh fails when a golden test exists in a
# package this list does not cover.
packages="
internal/netcfg Golden
internal/hotspot Golden
internal/xcfg Golden
internal/panel Golden
"

echo "rewriting every golden file in the repository"
echo

fail=0
while read -r pkg pattern; do
    [ -z "$pkg" ] && continue
    printf '=== %s (-run %s -update)\n' "$pkg" "$pattern"
    if go test "./$pkg" -count=1 -run "$pattern" -update; then
        :
    else
        echo "FAILED to update $pkg"
        fail=1
    fi
done <<EOF
$packages
EOF

echo
if [ "$fail" -ne 0 ]; then
    echo "golden-update: FAILED"
    echo "Some goldens were not rewritten. The tree is now in a mixed state; fix the"
    echo "failure above and run this again before reading any diff."
    exit 1
fi

echo "golden-update: done"
echo
echo "NOW READ THE DIFF:"
echo "    git diff --stat -- '*testdata*'"
echo "    git diff -- '*testdata*'"
echo
echo "Two things to check before you commit it:"
echo "  1. Every change is one you meant to make."
echo "  2. No credential is in it. scripts/regression.sh runs the leak scan;"
echo "     run that too, it is not optional after an update."
