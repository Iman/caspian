#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/service-action.sh"
test_loaded=1
alive=1
polls=0
boots=0
boot_attempts=0
fail_stop=0
launchctl() {
  case "$1" in
    print) [[ "$test_loaded" == 1 ]] || return 1; echo 'pid = 12345' ;;
    bootout) [[ "$fail_stop" == 0 ]] || return 1; test_loaded=0 ;;
    bootstrap)
      [[ "$alive" == 0 ]] || { echo 'new job raced old process' >&2; return 1; }
      boot_attempts=$((boot_attempts+1))
      # launchd can briefly reject a bootstrap after unloading a job.
      [[ "$boot_attempts" -gt 1 ]] || return 1
      boots=$((boots+1)); test_loaded=1 ;;
    enable|kickstart) return 0 ;;
    *) return 1 ;;
  esac
}
kill() { [[ "$alive" == 1 ]]; }
sleep() { polls=$((polls+1)); alive=0; }
stop_job test
[[ "$test_loaded" == 0 && "$polls" == 1 ]] || exit 1
start_job test /test.plist
[[ "$test_loaded" == 1 && "$boots" == 1 && "$boot_attempts" == 2 ]] || exit 1
fail_stop=1
if stop_job test; then echo 'ignored failed stop' >&2; exit 1; fi
[[ "$test_loaded" == 1 && "$boots" == 1 ]] || exit 1
test_loaded=0
stop_job test
printf 'macOS service lifecycle tests passed\n'
