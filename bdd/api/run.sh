#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# run.sh runs the HTTP suite.
#
# It is a convenience and not the only way in. The suite starts its own
# appliance from features/support/hooks.js, so a bare `npx cucumber-js` in this
# directory works exactly as well, which is what makes it possible to run one
# feature file or one scenario by line number. The README lists all four
# invocations.
#
# There is no browser here at all. This suite talks to the panel's endpoints
# over HTTP, keeps a session cookie, and scrapes the per-session form token out
# of rendered HTML the way any client would have to.
#
# READ THIS BEFORE YOU PIPE THIS SCRIPT INTO ANYTHING.
#
#     bash run.sh | tail -40      # WRONG: reports tail's status, not the suite's
#     bash run.sh | tee run.log   # WRONG: reports tee's status
#
# A shell pipeline exits with the status of its LAST command. scripts/gate.sh
# records what that trap cost this project. Do this instead:
#
#     bash bdd/api/run.sh > api.log 2>&1; echo "exit: $?"

set -o errexit
set -o nounset
set -o pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
cd "$script_dir"

if [ ! -d node_modules ]; then
    echo "installing dependencies (first run only)"
    npm install
fi

# No colour, anywhere, ever. cucumber.js sets this too, for the case where
# somebody runs cucumber-js directly; setting it here as well costs nothing and
# means neither path can regress on its own.
export FORCE_COLOR=0

exec npx cucumber-js "$@"
