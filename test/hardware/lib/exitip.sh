# shellcheck shell=bash
#
# Caspian-BYOC hardware harness: capturing an exit IP, twice, from two sources.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# A connect is not a result. A green switch is not a result. A tun device being
# up is not a result. The only thing this file will call a result is an address
# that a remote server reported back after real traffic reached it, captured
# twice by two programs that share no cache.
#
#   Source A  Chrome on the phone, over HTTPS. The one that matters, because it
#             is what a user runs and because it drags the whole path behind it:
#             DNS, TLS, HTTP, and Chrome's own opinions about all three.
#   Source B  toybox nc on the phone, over plain HTTP on port 80. Not Chrome, so
#             it shares no cache, no connection pool and no code with A. Its
#             limit is TLS: it has none. Measured 2026-08-30, this handset has
#             no curl and no wget, so nc is not a preference, it is the whole of
#             what is available.
#
# Different providers on purpose. One provider answering both would let a single
# stale CDN edge produce two agreeing wrong answers.
#
# ---------------------------------------------------------------------------
# BOTH ENDPOINTS ARE PINNED TO IP ADDRESSES. DO NOT "FIX" THEM BACK TO NAMES.
# ---------------------------------------------------------------------------
#
# The first draft of this file used icanhazip.com and ifconfig.me. Measured on
# 2026-08-30, from the Pi and then confirmed from the phone, the resolver this
# LAN hands out SINKHOLES them. `ping` on the handset for
# icanhazip.com, ifconfig.me, api.ipify.org and checkip.amazonaws.com all
# answered 127.0.0.1; the same names answered :: from the Pi. Only ipinfo.io
# survived, at 34.117.59.81, from both vantages.
#
# Two dead endpoints would merely be annoying. This is worse, and it is the
# reason the addresses are pinned rather than the names simply swapped:
#
#   UNTUNNELLED, the phone uses that sinkholing resolver and every echo lookup
#   fails. TUNNELLED, DNS goes through the appliance's own resolver chain
#   inside the tunnel and the same names resolve. So a box that changed nothing
#   but the DNS server, and tunnelled no traffic whatsoever, would show exactly
#   the signature this harness looks for: nothing before, an answer after. It
#   would have been graded a pass.
#
# Pinning the addresses removes DNS from the exit-IP question completely.
# Neither source resolves anything, so the exit IP answers only "where did the
# packets go", and the separate port-53 capture in steps/dnsleak.sh answers
# only "where did the lookups go". Neither can mask the other any more.
#
# The addresses are anycast and may move. That is handled by validating the
# response SHAPE at runtime rather than by trusting the pin: ei_read_a and
# ei_read_b return "shape" for anything they do not recognise, and a shape
# failure is UNPROVEN, never a guess and never a pass.
#
#   Source A  https://1.1.1.1/cdn-cgi/trace
#             MEASURED through Chrome on the handset: returned ip=<addr>,
#             loc=GB, warp=off, tls=TLSv1.3, over http/2. TLS to the bare IP
#             works, so the certificate carries the address in a SAN. The
#             cache-busting query string is accepted and the endpoint still
#             answers.
#             NOT usable for source B: plain port 80 on 1.1.1.1 answers 301.
#
#   Source B  http://34.117.59.81/ip  with  Host: ipinfo.io
#             MEASURED from the handset with toybox nc: 200, body
#             "203.0.113.20", agreeing with source A and with the value the
#             coordinator measured from the Pi. THAT ADDRESS IS A SUBSTITUTE:
#             the reading was the test household's own public address, which
#             names an ISP account and a billing address, so it was replaced on
#             2026-08-31 with an RFC 5737 documentation address. What the
#             measurement establishes is that the two sources AGREED, and that
#             survives the substitution. The table of substitutes is in
#             internal/netcfg/testdata/PROVENANCE.md.
#             THE HOST HEADER IS REQUIRED:
#             measured without it, the same request answers
#             "HTTP/1.0 404 Not Found / fault filter abort". It is a name-based
#             virtual host and the pinned address alone is not enough.
#
# All three readings of the same address on 2026-08-30 agreed: source A through
# Chrome, source B through nc, and the coordinator's independent probe from the
# Pi.

EI_ECHO_A_URL="${CASPIAN_HW_ECHO_A:-https://1.1.1.1/cdn-cgi/trace}"
EI_ECHO_B_ADDR="${CASPIAN_HW_ECHO_B_ADDR:-34.117.59.81}"
EI_ECHO_B_HOST="${CASPIAN_HW_ECHO_B_HOST:-ipinfo.io}"
EI_ECHO_B_PATH="${CASPIAN_HW_ECHO_B_PATH:-/ip}"

# ei_nonce is the cache defeat. A unique query string is the only cache defeat
# that needs no cooperation from the provider, no header the browser might
# ignore, and no devtools call. Chrome cannot be asked for a fresh tab:
# measured 2026-08-30, PUT /json/new answered "Could not create new page".
ei_nonce() { printf 'c%s%s' "$(date -u '+%H%M%S')" "$$"; }

# ---------------------------------------------------------------------------
# The readers. Each prints "<kind>\t<value>".
#
#   ip      value is the exit address
#   blocked value is empty. The navigation failed and said so
#   shape   value is a status code or empty. Something answered and it was not
#           what this endpoint answers with. UNPROVEN, never a guess
#   empty   value is empty. Nothing came back at all
# ---------------------------------------------------------------------------

# ei_read_a parses a Cloudflare cdn-cgi/trace body.
#
# It reads the "ip=" FIELD. It does not read the first IP literal in the body,
# and the difference is not cosmetic. The measured response is:
#
#     fl=985f9
#     h=1.1.1.1          <- first literal in the body
#     ip=203.0.113.20    <- the exit address, substituted as above
#
# "h" is the endpoint's own address. It is 1.1.1.1 no matter what the tunnel is
# doing, so a first-literal read returns a CONSTANT: every config would report
# the same exit address, and the switch test would announce "the exit IP did not
# change" about a box that had switched correctly. selftest/run.sh pins this.
ei_read_a() {
  local body="$1" ip
  if [ -z "$body" ]; then printf 'empty\t\n'; return 0; fi
  if printf '%s' "$body" | grep -qE 'ERR_[A-Z_]+|DNS_PROBE_[A-Z_]+'; then
    printf 'blocked\t\n'; return 0
  fi
  ip="$(printf '%s\n' "$body" | sed -n 's/^ip=\([0-9a-fA-F.:]\{3,\}\)[[:space:]]*$/\1/p' | head -1)"
  if [ -n "$ip" ]; then printf 'ip\t%s\n' "$ip"; return 0; fi
  printf 'shape\t\n'
}

# ei_read_b parses a raw HTTP/1.0 response from the phone's nc.
#
# The headers are split off before anything is read out of the body. The
# measured response carries "via: 1.1 google", and other deployments add
# x-forwarded-for; parsing the whole response would pick an address out of a
# header sooner or later.
ei_read_b() {
  local raw="$1" status body ip
  if [ -z "$raw" ]; then printf 'empty\t\n'; return 0; fi
  if printf '%s' "$raw" | grep -qE 'ERR_[A-Z_]+|DNS_PROBE_[A-Z_]+'; then
    printf 'blocked\t\n'; return 0
  fi
  status="$(printf '%s\n' "$raw" | sed -n 's|^HTTP/1\.[01] \([0-9][0-9][0-9]\).*|\1|p' | head -1)"
  if [ -z "$status" ]; then printf 'shape\t\n'; return 0; fi
  if [ "$status" != "200" ]; then printf 'shape\t%s\n' "$status"; return 0; fi
  body="$(printf '%s\n' "$raw" | awk 'f { print } /^[[:space:]]*$/ { f = 1 }')"
  ip="$(printf '%s' "$body" | hw_first_ip)"
  if [ -n "$ip" ]; then printf 'ip\t%s\n' "$ip"; return 0; fi
  printf 'shape\t\n'
}

ei_kind()  { printf '%s' "$1" | cut -f1; }
ei_value() { printf '%s' "$1" | cut -f2; }

# ei_extract reads one IP out of a response body, or prints nothing.
#
# Nothing is the honest answer far more often than it looks. Chrome renders an
# error page for a failed navigation, and that page is text like any other:
# measured on this handset, a navigation to a name that cannot resolve produced
# a body reading "This site can't be reached ... DNS_PROBE_FINISHED_NXDOMAIN".
# A harness that grepped that for digits would find none and call it UNPROVEN,
# which is right by luck. ei_classify_body says WHY instead.
ei_extract() {
  hw_first_ip
}

# ei_classify_body prints one word: ip, blocked, or empty.
ei_classify_body() {
  local body="$1"
  if [ -z "$body" ]; then printf 'empty\n'; return 0; fi
  if printf '%s' "$body" | grep -qE 'ERR_[A-Z_]+|DNS_PROBE_[A-Z_]+|ERR_INTERNET_DISCONNECTED'; then
    printf 'blocked\n'; return 0
  fi
  if [ -n "$(printf '%s' "$body" | hw_first_ip)" ]; then printf 'ip\n'; return 0; fi
  printf 'empty\n'
}

# ---------------------------------------------------------------------------
# ei_capture <serial> <evidence-dir>
#
# Writes evidence into the directory and prints one line:
#
#   a=<ip|-> b=<ip|-> a_kind=<ip|blocked|empty> b_kind=<...> void=<0|1>
#
# void=1 means the phone changed network state between the first sample and the
# last. That reading is VOID: it must be retaken, and it is NOT a leak. Reading
# it as a leak is the specific mistake this field exists to prevent.
# ---------------------------------------------------------------------------
ei_capture() {
  local serial="$1" dir="$2"
  local nonce url before after body_a body_b ip_a ip_b kind_a kind_b void final_url read_a read_b

  mkdir -p "$dir"
  nonce="$(ei_nonce)"
  before="$(ph_state "$serial")"
  printf 'before: %s\n' "$before" | hw_write "$dir/state.txt"

  # ---- Source A: Chrome ----
  case "$EI_ECHO_A_URL" in
    *\?*) url="${EI_ECHO_A_URL}&caspian=${nonce}" ;;
    *)    url="${EI_ECHO_A_URL}?caspian=${nonce}" ;;
  esac
  hw_info "source A: driving Chrome to the echo service (cache-busting nonce $nonce)"
  ph_chrome_open "$serial" "$url"
  hw_is_dry || sleep 7
  ph_chrome_dismiss_modal "$serial" "$dir/ui-dump.xml"
  hw_is_dry || sleep 2

  body_a="$(ph_chrome_text "$serial" "$nonce" 2>"$dir/source-a-method.txt")"
  printf '%s\n' "$body_a" | hw_write "$dir/source-a-body.txt"
  hw_guard "$dir/source-a-method.txt"

  final_url="$(ph_chrome_url "$serial" "$nonce")"
  if [ -n "$final_url" ]; then
    printf '%s\n' "$final_url" | hw_write "$dir/source-a-final-url.txt"
    case "$final_url" in
      *"$nonce"*) : ;;
      "$HW_DRY_SENTINEL") : ;;
      *) hw_warn "Chrome settled on a URL that does not carry the nonce. A redirect or a captive portal answered, not the echo service." ;;
    esac
  fi

  ph_screenshot "$serial" "$dir/source-a-screen.png"

  # ---- Source B: not Chrome ----
  hw_info "source B: fetching from the phone with toybox nc (plain HTTP, no TLS, pinned address)"
  body_b="$(ph_http_get "$serial" "$EI_ECHO_B_ADDR" "${EI_ECHO_B_PATH}?caspian=${nonce}" 80 "$EI_ECHO_B_HOST")"
  printf '%s\n' "$body_b" | hw_write "$dir/source-b-body.txt"

  after="$(ph_state "$serial")"
  printf 'after:  %s\n' "$after" | hw_redact >> "$dir/state.txt"
  hw_guard "$dir/state.txt"

  void=0
  if [ "$before" != "$after" ]; then
    void=1
    hw_warn "the phone's network state changed during the capture:"
    hw_warn "  before: $before"
    hw_warn "  after:  $after"
  fi

  # Under --dry-run nothing was fetched, so nothing is graded. Emitting "-"
  # here would look identical to a real capture that produced no IP, and the
  # step would report UNPROVEN about a measurement that never happened.
  if hw_is_dry; then
    printf 'a=dry b=dry a_kind=dry b_kind=dry void=0\n'
    return 0
  fi

  # Each source is read by the parser for THAT endpoint. There is no generic
  # "find an IP somewhere in the text" path any more: source A's body contains
  # the endpoint's own constant address before the exit address, and source B's
  # headers contain addresses too.
  read_a="$(ei_read_a "$body_a")"
  read_b="$(ei_read_b "$body_b")"
  kind_a="$(ei_kind "$read_a")"; ip_a="$(ei_value "$read_a")"
  kind_b="$(ei_kind "$read_b")"; ip_b="$(ei_value "$read_b")"
  [ "$kind_a" = "ip" ] || ip_a='-'
  [ "$kind_b" = "ip" ] || ip_b='-'
  [ -n "$ip_a" ] || ip_a='-'
  [ -n "$ip_b" ] || ip_b='-'
  case "$kind_a" in shape) hw_warn "source A answered in a shape this harness does not recognise. The pinned address may have moved. UNPROVEN, not a guess." ;; esac
  case "$kind_b" in shape) hw_warn "source B answered in a shape this harness does not recognise${ip_b:+ (HTTP $ip_b)}. Check the Host header and the pinned address. UNPROVEN, not a guess." ;; esac

  printf 'a=%s b=%s a_kind=%s b_kind=%s void=%s\n' "$ip_a" "$ip_b" "$kind_a" "$kind_b" "$void"
}

# ei_field pulls one field out of an ei_capture line.
ei_field() {
  printf '%s\n' "$2" | tr ' ' '\n' | sed -n "s/^$1=//p" | head -1
}

# ---------------------------------------------------------------------------
# ei_agree decides what the two sources together are worth.
#
# Both agreeing is the only full result. One source alone is reported as
# SINGLE-SOURCE and is explicitly not a pass: the second source exists to catch
# a cached or stale page, and without it that check did not happen. Two sources
# disagreeing is its own finding and is never averaged away.
# ---------------------------------------------------------------------------
ei_agree() {
  local a="$1" b="$2"
  if [ "$a" != '-' ] && [ "$b" != '-' ]; then
    if [ "$a" = "$b" ]; then printf 'agree %s\n' "$a"
    else printf 'disagree %s\n' "$a"; fi
    return 0
  fi
  if [ "$a" != '-' ]; then printf 'single-a %s\n' "$a"; return 0; fi
  if [ "$b" != '-' ]; then printf 'single-b %s\n' "$b"; return 0; fi
  printf 'none -\n'
}
