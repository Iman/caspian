# shellcheck shell=bash
#
# Caspian-BYOC hardware harness: the phone.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Everything that talks to the handset over adb. Every command in this file was
# run against the attached handset on 2026-08-30 and the comment above it says
# what came back. Where a behaviour was NOT observed the comment says so in
# those words, so that a reader can tell a measurement from an expectation.
#
# The device the facts came from. THE SERIAL, THE MODEL AND THE PRODUCT CODE
# BELOW ARE SUBSTITUTES, replaced on 2026-08-31: a serial identifies one
# physical phone and its retail purchase, and the model narrows it to one
# product. Everything else in this block is the real reading, and nothing the
# harness DOES depends on any of it: the serial is passed in at runtime and the
# model is never parsed. The table of substitutes is in
# internal/netcfg/testdata/PROVENANCE.md.
#   ro.product.model      SM-X000F   (a mid-range Android 13 handset)
#   ro.build.version.release 13      (SDK 33)
#   ro.product.cpu.abi    arm64-v8a
#   Chrome                148.0.7778.178, package com.android.chrome
#   adb                   1.0.41, platform-tools 37.0.0
#
# A different handset, or a Chrome upgrade, invalidates the specific
# resource-ids below but not the shape of the approach. ph_preflight is what
# catches that: it fails loudly rather than reading an empty page as a result.

PH_CHROME_PKG='com.android.chrome'
PH_CDP_PORT="${CASPIAN_HW_CDP_PORT:-19222}"
PH_CDP_UP=0

# ---------------------------------------------------------------------------
# Identity and transport.
# ---------------------------------------------------------------------------

# ph_serial resolves the serial to drive. One device attached means no argument
# is needed; more than one means the run refuses rather than picking.
ph_serial() {
  local want="${CASPIAN_HW_SERIAL:-}" list n
  list="$(adb devices | awk 'NR>1 && $2=="device" {print $1}')"
  if [ -n "$want" ]; then
    printf '%s\n' "$list" | grep -qx "$want" \
      || hw_die "serial '$want' is not attached and in state 'device'" \
                "run: adb devices -l"
    printf '%s\n' "$want"
    return 0
  fi
  n="$(printf '%s\n' "$list" | grep -c '[^[:space:]]' || true)"
  [ "$n" = "1" ] \
    || hw_die "expected exactly one attached device, found $n" \
              "run: adb devices -l, then set CASPIAN_HW_SERIAL=<serial>"
  printf '%s\n' "$list"
}

# ph_assert_usb refuses a device reached over adb-over-wifi.
#
# This matters more than it looks. The phone is about to be moved onto the Pi's
# hotspot and then, in the fail-closed step, onto a network that reaches
# nothing. An adb transport that runs over that same wifi disappears exactly
# when the harness needs it, and the run reads as a device failure rather than
# as the result it is.
#
# Measured 2026-08-30: adb devices -l printed
#   PHONE-SERIAL  device usb:1-1 product:x000nnxx model:SM_X000F
# so the usb: token is present for a USB transport. Serial, product, model and
# the usb bus path are substitutes; see the header. What the line is evidence
# for is the FIELD, ' usb:', which is what the grep below looks for.
ph_assert_usb() {
  local serial="$1"
  adb devices -l | grep -F "$serial" | grep -q ' usb:' \
    || hw_die "device $serial is not on a USB transport" \
              "unplug adb-over-wifi (adb usb) and attach the phone by cable. The fail-closed step kills the phone's network, and an adb transport over that network dies with it."
}

ph_sh() {
  local serial="$1"; shift
  adb -s "$serial" shell "$@"
}

# ph_pull_text copies a device file to stdout without CRLF mangling.
# adb shell runs a pty and turns LF into CRLF; adb exec-out does not.
ph_pull_text() {
  local serial="$1" path="$2"
  adb -s "$serial" exec-out cat "$path" 2>/dev/null
}

# ---------------------------------------------------------------------------
# State guard.
#
# The whole point. A capture is only a result if the phone was on the same
# network, with the same default route, at the start and at the end. If it was
# not, the reading is VOID and must be retaken; it is NOT a leak and it is NOT
# a pass. ph_state prints one line; the caller takes it before and after and
# compares.
#
# Measured 2026-08-30 on the attached handset:
#   cmd wifi status              -> 'Wifi is connected to "HomeNet"'
#   cmd connectivity airplane-mode -> 'disabled'
#   dumpsys connectivity         -> 'Active default network: 524'
# All three from the shell uid, no root.
# ---------------------------------------------------------------------------
ph_state() {
  local serial="$1" ssid air net
  ssid="$(ph_sh "$serial" 'cmd wifi status' 2>/dev/null | tr -d '\r' \
          | sed -n 's/^Wifi is connected to "\(.*\)"$/\1/p' | head -1)"
  [ -n "$ssid" ] || ssid='(none)'
  air="$(ph_sh "$serial" 'cmd connectivity airplane-mode' 2>/dev/null | tr -d '\r' | head -1)"
  net="$(ph_sh "$serial" 'dumpsys connectivity 2>/dev/null | grep -m1 "Active default network"' 2>/dev/null \
         | tr -d '\r' | awk '{print $NF}')"
  [ -n "$net" ] || net='(none)'
  printf 'ssid=%s airplane=%s defaultnet=%s\n' "$ssid" "$air" "$net"
}

ph_ssid() {
  ph_state "$1" | sed -n 's/^ssid=\([^ ]*\).*/\1/p'
}

# ph_assert_ssid fails PRECONDITION when the phone is not on the network the
# step is about. It asserts and never acts: ph_join does the acting, and the
# two are kept apart so that a step which only needs to know where the phone is
# cannot move it.
#
# Measured 2026-08-30, from the shell uid with no root: `cmd wifi -h` offers
# set-wifi-enabled, list-networks, forget-network and add-suggestion and has no
# connect-network; asking for it anyway answers
#   SecurityException: Uid 2000 does not have access to connect-network
# and `svc wifi` only enables and disables the radio. An earlier version of this
# comment concluded from that "the join is a human step, every time", which was
# wrong: no API accepts a join from the shell user, and the Settings interface
# is still driveable. See ph_join.
ph_assert_ssid() {
  local serial="$1" want="$2" got
  got="$(ph_ssid "$serial")"
  [ "$got" = "$want" ] \
    || hw_die "the phone is on wifi '$got', not '$want'" \
              "join it with ph_join, or by hand in Settings. No wifi-connect API is open to the shell user on this handset: cmd wifi refuses connect-network with a SecurityException and svc wifi only enables or disables the radio."
}

# ---------------------------------------------------------------------------
# Joining a network without root
# ---------------------------------------------------------------------------
#
# Every programmatic path is closed to the shell user, so this drives the
# Settings interface the way a person does: open the WiFi screen, find the name
# in the dumped view hierarchy, tap it, type the passphrase, press the button.
# Measured on this handset 2026-08-30: `uiautomator dump` returns the hierarchy
# with a bounds attribute on every node, the network names appear as exact node
# text, and `input tap` on the centre of those bounds opens the join dialog.
#
# The passphrase comes from the environment and from nowhere else. It is not in
# local/box.env, which holds the facts the harness cannot detect and is read on
# every run: a WPA2 passphrase written into a file that a harness sources by
# default is a credential at rest for the life of the machine, and this one is
# needed for a few seconds. Export HOTSPOT_PASSPHRASE for the run, or join by
# hand and let ph_assert_ssid check the result.
#
# It returns 2, not 1, when there is no passphrase to work with, so a caller can
# tell "I was not asked to join" from "I tried to join and could not".

# _ph_ui_centre prints "x y" for the first node whose text is exactly $2.
_ph_ui_centre() {
  local serial="$1" want="$2"
  ph_sh "$serial" 'uiautomator dump /sdcard/caspian-ui.xml' >/dev/null 2>&1
  ph_pull_text "$serial" /sdcard/caspian-ui.xml 2>/dev/null | python3 -c '
import sys, re
want = sys.argv[1]
xml = sys.stdin.read()
for m in re.finditer(r"text=\"([^\"]*)\"[^>]*?bounds=\"\[(\d+),(\d+)\]\[(\d+),(\d+)\]\"", xml):
    if m.group(1) == want:
        x1, y1, x2, y2 = (int(g) for g in m.groups()[1:])
        print((x1 + x2) // 2, (y1 + y2) // 2)
        break
' "$want"
}

_ph_ui_tap() {
  local serial="$1" want="$2" coords
  coords="$(_ph_ui_centre "$serial" "$want")"
  [ -n "$coords" ] || return 1
  # shellcheck disable=SC2086
  ph_sh "$serial" "input tap $coords" >/dev/null 2>&1
}

ph_join() {
  local serial="$1" ssid="$2" pass="${HOTSPOT_PASSPHRASE:-}" i

  if [ "$(ph_ssid "$serial")" = "$ssid" ]; then
    hw_info "phone is already on '$ssid'"
    return 0
  fi
  if [ -z "$pass" ]; then
    return 2
  fi

  hw_step "joining the phone to '$ssid' through the Settings interface"
  ph_sh "$serial" 'am start -a android.settings.WIFI_SETTINGS' >/dev/null 2>&1
  sleep 3

  # The name may take a scan or two to appear on screen.
  for i in 1 2 3 4 5 6; do
    [ -n "$(_ph_ui_centre "$serial" "$ssid")" ] && break
    [ "$i" = 6 ] && {
      hw_warn "'$ssid' did not appear on the phone's WiFi screen within 24 seconds"
      return 1
    }
    sleep 4
  done

  _ph_ui_tap "$serial" "$ssid" || { hw_warn "could not tap '$ssid'"; return 1; }
  sleep 2

  # A remembered network joins on the tap alone and shows no passphrase box.
  if [ "$(ph_ssid "$serial")" = "$ssid" ]; then
    hw_info "joined '$ssid' (remembered)"
    return 0
  fi

  ph_sh "$serial" "input text '$pass'" >/dev/null 2>&1
  sleep 1
  for i in Connect Join OK; do
    _ph_ui_tap "$serial" "$i" && break
  done

  for i in 1 2 3 4 5 6 7 8; do
    sleep 3
    if [ "$(ph_ssid "$serial")" = "$ssid" ]; then
      hw_info "joined '$ssid'"
      return 0
    fi
  done
  hw_warn "the phone is still on '$(ph_ssid "$serial")' after 24 seconds"
  return 1
}

# ---------------------------------------------------------------------------
# Cellular. The false-leak hazard.
#
# When the hotspot stops reaching the internet, which is precisely what the
# fail-closed step arranges, Android marks that network unvalidated and moves
# the default route to mobile data. Traffic then flows, the harness sees it
# flow, and reports a LEAK that is nothing of the kind: the packets never went
# near the Pi.
#
# This handset has a SIM (getprop gsm.sim.state -> LOADED,ABSENT, so slot 1 is
# populated) and mobile data was on (settings get global mobile_data -> 1), so
# the hazard is real here and not theoretical.
#
# Measured 2026-08-30, from the shell uid with no root, and restored afterwards:
#   cmd connectivity airplane-mode enable   -> airplane-mode reads 'enabled',
#                                              'Wifi is disabled'
#   cmd wifi set-wifi-enabled enabled       -> 'Wifi is connected to "HomeNet"'
#                                              with airplane mode still on
#   cmd connectivity airplane-mode disable  -> restored
# ---------------------------------------------------------------------------
ph_wifi_only_enter() {
  local serial="$1"
  hw_info "putting the phone into wifi-only (airplane mode on, radio back on)"
  hw_run adb -s "$serial" shell 'cmd connectivity airplane-mode enable' >/dev/null
  hw_is_dry || sleep 4
  hw_run adb -s "$serial" shell 'cmd wifi set-wifi-enabled enabled' >/dev/null
  hw_is_dry || sleep 8
}

ph_wifi_only_leave() {
  local serial="$1"
  hw_info "restoring the phone (airplane mode off, radio on)"
  hw_run adb -s "$serial" shell 'cmd connectivity airplane-mode disable' >/dev/null
  hw_is_dry || sleep 3
  hw_run adb -s "$serial" shell 'cmd wifi set-wifi-enabled enabled' >/dev/null
  hw_is_dry || sleep 6
}

# ph_assert_no_cellular is the read-only alternative for anyone unwilling to
# touch the phone's radios. It does not force anything; it refuses to grade a
# fail-closed result while a cellular fallback exists.
ph_assert_no_cellular() {
  local serial="$1" air
  air="$(ph_sh "$serial" 'cmd connectivity airplane-mode' 2>/dev/null | tr -d '\r' | head -1)"
  [ "$air" = "enabled" ] \
    || hw_die "airplane mode is '$air', so mobile data can carry traffic the hotspot refused" \
              "run the step with --wifi-only, or turn mobile data off by hand. Without this a fail-closed result cannot be told apart from the phone quietly using LTE."
}

# ---------------------------------------------------------------------------
# Chrome. The source that matters, because it is what a user runs, and because
# it exercises DNS, TLS and the whole path rather than a socket.
#
# Three readbacks, in descending order of trust. ph_chrome_text uses the first
# that works and says which one it used.
#
#   1. CDP Runtime.evaluate over the devtools socket.
#      Measured 2026-08-30: `cat /proc/net/unix | grep chrome_devtools` showed
#      NOTHING before Chrome was launched and @chrome_devtools_remote after it,
#      so the socket is created on demand. adb forward to it then answered
#      /json/version with Chrome/148.0.7778.178, and Runtime.evaluate on
#      document.body.innerText returned the page's text.
#      This is the readback that survives a modal dialog covering the page.
#
#   2. uiautomator dump.
#      Measured the same day, and the measurement is the reason step 1 exists:
#      the FIRST dump after launching Chrome contained only a promo dialog
#      ("Chrome notifications make things easier", resource-id
#      com.android.chrome:id/modal_dialog_view) and NEITHER the page body nor
#      the page title. After tapping its negative_button, the next dump held
#      the body text, the title and the URL-bar text. A harness that dumped
#      once and read no IP would have called that UNPROVEN when the page was
#      fine, and a harness that grepped for an old IP would have read the wrong
#      answer entirely.
#
#   3. Screenshot. Not machine-read. Kept so a human can settle an argument.
#      Measured: adb exec-out screencap -p returned 1549897 bytes on this
#      handset, so the screencap-returns-zero-bytes trap seen on emulators
#      elsewhere does not apply here.
#
# What none of them can see: whether Chrome resolved the name over its own
# DNS-over-HTTPS ("Secure DNS") rather than through the Pi's resolver. That is
# invisible to every readback here and to the uplink capture, and it is the
# stated limit of the DNS check. See docs/HARDWARE-TEST.md.
# ---------------------------------------------------------------------------

ph_cdp_up() {
  local serial="$1"
  hw_is_dry && { PH_CDP_UP=1; return 0; }
  adb -s "$serial" forward "tcp:$PH_CDP_PORT" localabstract:chrome_devtools_remote >/dev/null 2>&1 \
    && PH_CDP_UP=1
  return 0
}

ph_cdp_down() {
  local serial="$1"
  [ "$PH_CDP_UP" = "1" ] || return 0
  hw_is_dry || adb -s "$serial" forward --remove "tcp:$PH_CDP_PORT" >/dev/null 2>&1
  PH_CDP_UP=0
}

# ph_chrome_open drives Chrome to a URL by intent.
#
# Measured 2026-08-30: `am start -a android.intent.action.VIEW -d <url>
# -p com.android.chrome` started ChromeTabbedActivity and the page was fetched
# (the local server logged the request).
#
# The URL always carries a unique query parameter. That is the cache defeat, and
# it is the provider-independent one: it needs no header, no devtools call and
# no agreement from the echo service. Chrome's own tab cannot be created
# remotely to get a clean one instead: measured, PUT /json/new returned
# "Could not create new page" on this Chrome.
ph_chrome_open() {
  local serial="$1" url="$2"
  hw_run adb -s "$serial" shell \
    "am start -a android.intent.action.VIEW -d '$url' -p $PH_CHROME_PKG" >/dev/null
}

# ph_chrome_dismiss_modal clears a Chrome modal if one is covering the page.
#
# Returns 0 whether or not it found one. Fails the run only when a modal is
# present and has no button this function knows how to press, because that is
# the case where every later reading would be of the dialog.
ph_chrome_dismiss_modal() {
  local serial="$1" dumpfile="$2" xml bounds cx cy
  hw_is_dry && { printf 'DRY  would dump the UI and dismiss any Chrome modal\n' >&2; return 0; }

  adb -s "$serial" shell 'uiautomator dump /sdcard/caspian_hw_dump.xml' >/dev/null 2>&1
  xml="$(ph_pull_text "$serial" /sdcard/caspian_hw_dump.xml)"
  adb -s "$serial" shell 'rm -f /sdcard/caspian_hw_dump.xml' >/dev/null 2>&1
  [ -n "$dumpfile" ] && printf '%s\n' "$xml" | hw_write "$dumpfile"

  printf '%s' "$xml" | grep -q 'modal_dialog_view' || return 0
  hw_info "a Chrome modal dialog is covering the page; dismissing it"

  bounds="$(printf '%s' "$xml" \
    | tr '<' '\n' \
    | grep 'negative_button' \
    | sed -n 's/.*bounds="\[\([0-9]*\),\([0-9]*\)\]\[\([0-9]*\),\([0-9]*\)\]".*/\1 \2 \3 \4/p' \
    | head -1)"
  if [ -z "$bounds" ]; then
    hw_die "a Chrome modal is on screen and has no negative_button to press" \
      "unlock the phone, dismiss the Chrome dialog by hand, then re-run. Every readback taken now would be of the dialog and not of the page."
  fi
  # shellcheck disable=SC2086
  set -- $bounds
  cx=$(( ($1 + $3) / 2 ))
  cy=$(( ($2 + $4) / 2 ))
  adb -s "$serial" shell "input tap $cx $cy" >/dev/null 2>&1
  sleep 2
  return 0
}

# ph_chrome_text prints the visible text of the Chrome tab whose URL contains
# the given fragment, and prints the method it used on stderr.
ph_chrome_text() {
  local serial="$1" frag="$2" out here
  here="${HW_LIB_DIR:?}"

  if hw_is_dry; then
    printf 'DRY  would read Chrome page text for a tab matching %s\n' "$frag" >&2
    printf '%s\n' "$HW_DRY_SENTINEL"
    return 0
  fi

  if [ "$PH_CDP_UP" = "1" ] && command -v python3 >/dev/null 2>&1; then
    out="$(python3 "$here/../bin/cdp-eval.py" "http://127.0.0.1:$PH_CDP_PORT" "$frag" body 2>/dev/null)"
    if [ -n "$out" ]; then
      printf 'source: chrome via CDP Runtime.evaluate\n' >&2
      printf '%s\n' "$out"
      return 0
    fi
    printf 'note: CDP readback returned nothing, falling back to uiautomator\n' >&2
  fi

  adb -s "$serial" shell 'uiautomator dump /sdcard/caspian_hw_read.xml' >/dev/null 2>&1
  out="$(ph_pull_text "$serial" /sdcard/caspian_hw_read.xml \
         | tr '<' '\n' | sed -n 's/.*[^-]text="\([^"]*\)".*/\1/p')"
  adb -s "$serial" shell 'rm -f /sdcard/caspian_hw_read.xml' >/dev/null 2>&1
  printf 'source: chrome via uiautomator dump\n' >&2
  printf '%s\n' "$out"
}

# ph_chrome_url prints the URL the tab actually settled on. A captive portal or
# a redirect shows up here and nowhere else, and it is the difference between
# "the echo service answered" and "something else answered".
ph_chrome_url() {
  local serial="$1" frag="$2" here
  here="${HW_LIB_DIR:?}"
  hw_is_dry && { printf '%s\n' "$HW_DRY_SENTINEL"; return 0; }
  [ "$PH_CDP_UP" = "1" ] || return 0
  command -v python3 >/dev/null 2>&1 || return 0
  python3 "$here/../bin/cdp-eval.py" "http://127.0.0.1:$PH_CDP_PORT" "$frag" url 2>/dev/null
}

ph_screenshot() {
  local serial="$1" file="$2"
  hw_is_dry && { printf 'DRY  would save a screenshot to %s\n' "$file" >&2; return 0; }
  mkdir -p "$(dirname "$file")"
  adb -s "$serial" exec-out screencap -p > "$file" 2>/dev/null || true
  [ -s "$file" ] || hw_warn "screencap produced an empty file at $file"
}

# ---------------------------------------------------------------------------
# The second source: an HTTP client on the phone that is not Chrome.
#
# Its job is narrow. It guards against reading a cached or stale page, and
# against a readback bug in the Chrome path, by fetching the answer again over
# the same wifi with a different program.
#
# Measured 2026-08-30 on this handset: there is NO curl and NO wget.
# `which curl wget` found neither, and toybox 0.8.6-android lists neither. What
# it does list, and what does work, is nc:
#
#   adb shell '(printf "GET / HTTP/1.0\r\n\r\n"; sleep 3) | toybox nc HOST 80'
#
# returned the full HTTP response. The obvious form, a plain pipe with no
# sleep, returned NOTHING and still exited 0: nc sees EOF on stdin and leaves
# before the response arrives. The -q flag, which the usage text says quits N
# seconds after stdin EOF, did not fix it either. So the sleep is not
# superstition, it is the measured difference between an answer and a silent
# empty string that a careless harness would grade as UNPROVEN.
#
# What this source cannot do: TLS. toybox nc speaks TCP and nothing else, so
# the second source is plain HTTP on port 80. A transparent proxy in the path
# could rewrite it, which is exactly why it is the SECOND source and not the
# first. The first source is Chrome over HTTPS.
# ---------------------------------------------------------------------------
# ph_http_get <serial> <connect-addr> <path> [port] [host-header]
#
# The connect address and the Host header are separate arguments because the
# echo endpoint needs them to differ. Source B dials the pinned literal
# 34.117.59.81 and must still send "Host: ipinfo.io": measured on the handset
# on 2026-08-30, the identical request WITHOUT that header answers
# "HTTP/1.0 404 Not Found / fault filter abort". It is a name-based virtual
# host, and pinning the address is only half of reaching it.
#
# The host header defaults to the connect address, which is what the panel
# check wants.
ph_http_get() {
  local serial="$1" addr="$2" path="$3" port="${4:-80}" hdr="${5:-}"
  [ -n "$hdr" ] || hdr="$addr"
  if hw_is_dry; then
    printf 'DRY  would fetch http://%s:%s%s from the phone with toybox nc (Host: %s)\n' \
      "$addr" "$port" "$path" "$hdr" >&2
    printf '%s\n' "$HW_DRY_SENTINEL"
    return 0
  fi
  adb -s "$serial" shell \
    "(printf 'GET $path HTTP/1.0\r\nHost: $hdr\r\nUser-Agent: caspian-hw\r\nConnection: close\r\n\r\n'; sleep 6) | toybox nc -w 8 $addr $port" \
    2>/dev/null | tr -d '\r'
}

# ph_have_second_source reports whether the non-Chrome source can run at all.
ph_have_second_source() {
  local serial="$1"
  hw_is_dry && return 0
  ph_sh "$serial" 'toybox nc --help >/dev/null 2>&1 && echo yes' 2>/dev/null | tr -d '\r' | grep -q yes
}

# ---------------------------------------------------------------------------
# Preflight. Everything that must be true before a measurement means anything.
# ---------------------------------------------------------------------------
ph_preflight() {
  local serial="$1" pkg

  hw_step "phone preflight"
  hw_info "serial: $serial"
  ph_assert_usb "$serial"
  hw_info "transport: USB, confirmed"

  hw_is_dry && { hw_info "dry run: skipping on-device checks"; return 0; }

  pkg="$(ph_sh "$serial" "pm list packages $PH_CHROME_PKG" 2>/dev/null | tr -d '\r')"
  [ -n "$pkg" ] || hw_die "Chrome ($PH_CHROME_PKG) is not installed" \
                          "install Chrome. It is the source that matters; a different browser is a different measurement."

  ph_sh "$serial" 'dumpsys power | grep -q "mWakefulness=Awake" && echo awake' 2>/dev/null \
    | tr -d '\r' | grep -q awake \
    || hw_warn "the phone screen is asleep. uiautomator dump and Chrome rendering both need it awake and unlocked."

  if ph_have_second_source "$serial"; then
    hw_info "second source: toybox nc, present"
  else
    hw_warn "second source: toybox nc is NOT available on this device."
    hw_warn "every result will be SINGLE-SOURCE, which this project does not accept as a pass."
  fi

  hw_info "state: $(ph_state "$serial")"
}
