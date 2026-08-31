#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# run.sh runs the browser suite.
#
# It is a convenience and not the only way in. The suite starts its own
# appliance and its own browser from features/support/hooks.js, so a bare
# `npx cucumber-js` in this directory works exactly as well, which is what makes
# it possible to run one feature file or one scenario by line number. The README
# lists all four invocations.
#
# There is no Selenium server to start and no display to arrange. The reference
# project this suite is modelled on needed both, in 2016; selenium-webdriver
# talks to chromedriver directly, chromedriver is fetched and cached by Selenium
# Manager on the first run, and Chrome runs headless.
#
# READ THIS BEFORE YOU PIPE THIS SCRIPT INTO ANYTHING.
#
#     bash run.sh | tail -40      # WRONG: reports tail's status, not the suite's
#     bash run.sh | tee run.log   # WRONG: reports tee's status
#
# A shell pipeline exits with the status of its LAST command. scripts/gate.sh
# records what that trap cost this project. Do this instead:
#
#     bash bdd/web/run.sh > web.log 2>&1; echo "exit: $?"

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
