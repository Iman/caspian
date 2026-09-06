#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
[[ $(uname -s) == Darwin && $(id -u) == 0 ]] || { echo 'Run on the Mac with administrator privileges.' >&2; exit 1; }
exec /usr/local/bin/caspian reset-password
