# shellcheck shell=bash
#
# Step 3: the config switch.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Two configs in sequence. Each must pass on its own, and the exit IP must
# CHANGE between them.
#
# This is the test that catches the failure the other tests cannot see: a box
# that reports connected, that produces a real exit IP, that matches nothing
# obviously wrong, and that is still on the previous server because applying the
# second config did nothing. Prove-one-config passes that box twice. Only
# comparing the two readings catches it.
#
# The comparison is done on salted fingerprints, not on addresses. A matching
# exit IP is a server address, and writing "203.0.113.10 became 198.51.100.7"
# into a summary would put two of them in a file. hw_fp gives a value that can
# be compared for equality and cannot be read backwards.

step_switch() {
  local serial="$1" run="$2" first="$3" second="$4"
  local rc1 rc2 ip1 ip2 fp1 fp2

  hw_ledger_start "$run" "switch:$first->$second"

  hw_step "config switch: '$first' then '$second'"
  hw_info "each must pass on its own AND the exit IP must change between them"

  cfg_require "$first"; cfg_assert_supported "$first" >/dev/null
  cfg_require "$second"; cfg_assert_supported "$second" >/dev/null
  if [ "$first" = "$second" ]; then
    hw_ledger_end "$run" "switch:$first->$second" "PRECONDITION"
    hw_die "both configs are '$first'" "give two different labels. Switching a config for itself proves nothing."
  fi

  step_prove "$serial" "$run" "$first"; rc1=$?
  step_prove "$serial" "$run" "$second"; rc2=$?

  hw_step "comparing the two exit addresses"

  if hw_is_dry; then
    printf 'DRY  would compare the two captured exit addresses by salted fingerprint\n'
    hw_ledger_end "$run" "switch:$first->$second" "DRY"
    return "$HW_PRECONDITION"
  fi

  if [ "$rc1" != "$HW_PASS" ] || [ "$rc2" != "$HW_PASS" ]; then
    hw_say ""
    hw_say "RESULT the switch test cannot be graded."
    hw_say "  '$first' was $(hw_verdict_name "$rc1"), '$second' was $(hw_verdict_name "$rc2")."
    hw_say "  a switch is only meaningful between two readings that were each proven."
    hw_ledger_end "$run" "switch:$first->$second" "UNPROVEN"
    [ "$rc1" = "$HW_LEAK" ] && return "$HW_LEAK"
    [ "$rc2" = "$HW_LEAK" ] && return "$HW_LEAK"
    return "$HW_UNPROVEN"
  fi

  ip1="$(cat "${HW_TMPDIR:?}/exit-$first.ip" 2>/dev/null)"
  ip2="$(cat "${HW_TMPDIR:?}/exit-$second.ip" 2>/dev/null)"
  if [ -z "$ip1" ] || [ -z "$ip2" ]; then
    hw_ledger_end "$run" "switch:$first->$second" "UNPROVEN"
    hw_say ""
    hw_say "RESULT UNPROVEN: one of the two steps left no captured address to compare."
    return "$HW_UNPROVEN"
  fi

  fp1="$(hw_fp "$ip1" "$run")"
  fp2="$(hw_fp "$ip2" "$run")"
  {
    printf 'first\t%s\t%s\n' "$first" "$fp1"
    printf 'second\t%s\t%s\n' "$second" "$fp2"
    printf 'changed\t%s\n' "$( [ "$fp1" = "$fp2" ] && printf 'no' || printf 'yes' )"
  } | hw_write "$run/03-switch/result.tsv"

  hw_say ""
  if [ "$fp1" = "$fp2" ]; then
    hw_say "RESULT FAIL: the exit IP did NOT change when the config changed."
    hw_say "  '$first' and '$second' both egressed from the same address (fingerprint $fp1)."
    hw_say "  both matched their own config, so both boxes share that address, or the second"
    hw_say "  config was never applied and the box is still on the first server."
    hw_say "  check: does local/boxes.tsv give the two labels an address in common?"
    hw_ledger_end "$run" "switch:$first->$second" "FAIL"
    return "$HW_FAIL"
  fi

  hw_say "RESULT PASS: the exit IP changed with the config."
  hw_say "  '$first'  -> fingerprint $fp1, matched its own box"
  hw_say "  '$second' -> fingerprint $fp2, matched its own box"
  hw_say "  the two differ, so the box really did move to the second server."
  hw_ledger_end "$run" "switch:$first->$second" "OK"
  return "$HW_PASS"
}
