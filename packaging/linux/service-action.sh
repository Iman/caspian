#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail

case "${1:-}" in start|stop|restart) ;; *) exit 2 ;; esac
[[ $(id -u) == 0 ]] || exit 1
stop_services() {
  systemctl disable --now caspian-panel.service
  systemctl disable --now caspian.service
}
start_services() {
  systemctl enable --now caspian.service
  systemctl enable --now caspian-panel.service
}
case "$1" in
  start) start_services ;;
  stop) stop_services ;;
  restart) stop_services; start_services ;;
esac
