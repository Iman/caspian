#!/usr/bin/env bash
#
# Offline checks of the harness logic. No phone, no Pi, no network.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# These are the parts of the harness that CAN be tested without hardware: the
# scheme gate, the host extractor, the redaction filter, the verdict ordering
# and the partial-run detector. They exist because the rest of the harness
# cannot be tested until the session happens, and the parts that decide what
# gets printed and what gets redacted are exactly the parts where a silent bug
# would be worst.
#
# Every fixture address here is from RFC 5737 documentation space
# (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24). Nothing real is in this file.

set -u

HW_HARNESS_DIR="$(cd -- "$(dirname -- "$0")/.." && pwd)"
HW_LIB_DIR="$HW_HARNESS_DIR/lib"
export HW_HARNESS_DIR HW_LIB_DIR

# shellcheck source=lib/common.sh
. "$HW_LIB_DIR/common.sh"
# shellcheck source=lib/config.sh
. "$HW_LIB_DIR/config.sh"
# shellcheck source=lib/exitip.sh
. "$HW_LIB_DIR/exitip.sh"
# shellcheck source=steps/baseline.sh
. "$HW_HARNESS_DIR/steps/baseline.sh"

PASS=0
FAIL=0

ok()   { PASS=$((PASS+1)); printf 'ok   %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf 'FAIL %s\n' "$1"; printf '       want: %s\n       got:  %s\n' "$2" "$3"; }
is()   { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1" "$2" "$3"; fi; }
has()  { case "$3" in *"$2"*) ok "$1" ;; *) bad "$1" "contains '$2'" "$3" ;; esac; }
hasnt(){ case "$3" in *"$2"*) bad "$1" "does NOT contain '$2'" "$3" ;; *) ok "$1" ;; esac; }

TMP="$(mktemp -d -t caspian-hw-selftest)"
trap 'rm -rf "$TMP"' EXIT
export CASPIAN_HW_LOCAL="$TMP/local"
mkdir -p "$CASPIAN_HW_LOCAL/configs"

# ---------------------------------------------------------------------------
printf '\n== scheme gate ==\n'
# ---------------------------------------------------------------------------
# The one check that can rot: the harness list and the Go list must agree.
if cfg_selftest_schemes; then PASS=$((PASS+1)); else FAIL=$((FAIL+1)); fi

mk() { printf '%s\n' "$2" > "$CASPIAN_HW_LOCAL/configs/$1.conf"; }

mk reality-a 'vless://11111111-2222-3333-4444-555555555555@192.0.2.10:443?security=reality&sni=www.example.com&fp=chrome&pbk=AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHHIIIIJJJJKKK&sid=0123abcd&type=tcp&flow=xtls-rprx-vision#box-a'
mk trojan-b  'trojan://s3cret-password-here@198.51.100.20:443?security=tls&sni=b.example.net#box-b'
mk hy2-c     'hysteria2://auth-string-9x@203.0.113.30:8443/?sni=c.example.org#box-c'
mk ss-d      'ss://YWVzLTI1Ni1nY206c29tZXBhc3N3b3Jk@192.0.2.40:8388#box-d'
mk socks-e   'socks://dXNlcjpwYXNz@198.51.100.50:1080#box-e'
# vmess:// base64 of {"v":"2","ps":"box-f","add":"203.0.113.60","port":"443","id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","net":"ws"}
mk vmess-f   "vmess://$(printf '{"v":"2","ps":"box-f","add":"203.0.113.60","port":"443","id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","net":"ws"}' | base64 | tr -d '\n')"
mk tuic-out  'tuic://uuid:pass@192.0.2.99:443#nope'

is 'scheme of a vless link'      'vless'     "$(cfg_scheme reality-a)"
is 'scheme of a hysteria2 link'  'hysteria2' "$(cfg_scheme hy2-c)"
is 'scheme of a vmess link'      'vmess'     "$(cfg_scheme vmess-f)"

out="$(cfg_assert_supported reality-a 2>&1)"; is 'vless is in scope' 'vless' "$out"
out="$(cfg_assert_supported tuic-out 2>&1 || true)"
has 'tuic is refused by name'                    'tuic' "$out"
has 'tuic refusal says xray-core does not carry it' 'does not carry' "$out"
hasnt 'tuic refusal does not quote the link'     '192.0.2.99' "$out"

# ---------------------------------------------------------------------------
printf '\n== host extraction ==\n'
# ---------------------------------------------------------------------------
is 'vless host'      '192.0.2.10'   "$(cfg_host reality-a)"
is 'trojan host'     '198.51.100.20' "$(cfg_host trojan-b)"
is 'hysteria2 host'  '203.0.113.30' "$(cfg_host hy2-c)"
is 'ss host (b64 userinfo form)' '192.0.2.40' "$(cfg_host ss-d)"
is 'socks host'      '198.51.100.50' "$(cfg_host socks-e)"
is 'vmess host (base64 json "add")' '203.0.113.60' "$(cfg_host vmess-f)"

mk ss-legacy "ss://$(printf 'aes-256-gcm:pw@203.0.113.70:8388' | base64 | tr -d '\n')#box-g"
is 'ss host (fully base64 legacy form)' '203.0.113.70' "$(cfg_host ss-legacy)"

mk v6-h 'vless://11111111-2222-3333-4444-555555555555@[2001:db8::1]:443?security=reality#box-h'
is 'bracketed IPv6 host is unwrapped' '2001:db8::1' "$(cfg_host v6-h)"

# ---------------------------------------------------------------------------
printf '\n== redaction ==\n'
# ---------------------------------------------------------------------------
hw_secrets_init "$TMP/secrets.tsv"
cfg_register_all_secrets

leaky="$TMP/leaky.txt"
{
  printf 'the exit was 192.0.2.10 today\n'
  printf 'raw: vless://11111111-2222-3333-4444-555555555555@192.0.2.10:443?security=reality&sni=www.example.com&fp=chrome&pbk=AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHHIIIIJJJJKKK&sid=0123abcd&type=tcp&flow=xtls-rprx-vision#box-a\n'
  printf 'password was s3cret-password-here\n'
  printf 'and the key AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHHIIIIJJJJKKK\n'
  printf 'harmless line with 8.8.4.4 in it\n'
} | hw_write "$leaky"

# This assertion comes FIRST and the rest depend on it.
#
# Without it, every "hasnt" below passes trivially when hw_guard has deleted the
# file, because an empty string contains nothing. That is exactly what happened
# on the first run of this selftest on 2026-08-30: two real defects were hidden
# behind four green lines. A test that cannot fail is not a test.
if [ -s "$leaky" ]; then ok 'hw_write left a readable file behind'; else bad 'hw_write left a readable file behind' 'a non-empty file' 'missing or empty (hw_guard deleted it: a secret survived redaction)'; fi

body="$(cat "$leaky" 2>/dev/null)"
hasnt 'the server address is gone'      '192.0.2.10'            "$body"
hasnt 'the whole share link is gone'    'vless://'              "$body"
hasnt 'the trojan password is gone'     's3cret-password-here'  "$body"
hasnt 'the reality public key is gone'  'AAAABBBBCCCCDDDDEEEE'  "$body"
has   'the box is named instead'        '<box:reality-a>'       "$body"
has   'unrelated text survives'         '8.8.4.4'               "$body"

# Defect found 2026-08-30: "security=reality" registered the word "reality" as a
# secret, and the label "reality-a" then came out of the redactor as
# "<param:reality-a>-a". The report is required to print the box NAME, so a
# redactor that eats the name has destroyed the result.
labels="$TMP/labels.txt"
printf 'the box was reality-a and the other was trojan-b\n' | hw_write "$labels"
lbody="$(cat "$labels" 2>/dev/null)"
is 'a label survives redaction untouched' 'the box was reality-a and the other was trojan-b' "$lbody"

# Defect found the same day: printf '%s' left no trailing newline, so the LAST
# query parameter of every link was dropped by `while read` and never
# registered. The last parameter of reality-a is flow=xtls-rprx-vision.
last="$TMP/last.txt"
printf 'trailing param xtls-rprx-vision here\n' | hw_write "$last"
hasnt 'the LAST query parameter of a link is registered too' 'xtls-rprx-vision' "$(cat "$last" 2>/dev/null)"

# Placeholder freezing: once a stretch is replaced, no later secret may match
# inside it. sni=www.example.com is a registered secret; if it were allowed to
# match inside a placeholder that already replaced the whole link, the output
# would be nested nonsense.
frozen="$TMP/frozen.txt"
printf 'vless://11111111-2222-3333-4444-555555555555@192.0.2.10:443?security=reality&sni=www.example.com&fp=chrome&pbk=AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHHIIIIJJJJKKK&sid=0123abcd&type=tcp&flow=xtls-rprx-vision#box-a\n' | hw_write "$frozen"
is 'a whole link becomes exactly one placeholder' '<config:reality-a>' "$(cat "$frozen" 2>/dev/null)"

# hw_guard must delete a file that still holds a secret. Written past hw_write
# on purpose, which is the mistake it exists to catch.
sneaky="$TMP/sneaky.txt"
printf 'oops 192.0.2.10\n' > "$sneaky"
( hw_guard "$sneaky" ) >/dev/null 2>&1
if [ -f "$sneaky" ]; then bad 'hw_guard deletes a file holding a secret' 'deleted' 'still there'; else ok 'hw_guard deletes a file holding a secret'; fi

# ---------------------------------------------------------------------------
printf '\n== labels never carry an address ==\n'
# ---------------------------------------------------------------------------
out="$( ( hw_guard_name 'vless-192.0.2.10' ) 2>&1 || true )"
has 'an address-shaped label is refused' 'shaped like an IPv4 address' "$out"
out="$( ( hw_guard_name 'box a' ) 2>&1 || true )"
has 'a label with a space is refused' 'outside A-Za-z0-9._-' "$out"
if ( hw_guard_name 'reality-a' ) >/dev/null 2>&1; then ok 'a plain label is accepted'; else bad 'a plain label is accepted' 'accepted' 'refused'; fi

# ---------------------------------------------------------------------------
printf '\n== verdict ordering ==\n'
# ---------------------------------------------------------------------------
printf 'reality-a\t192.0.2.10\ntrojan-b\t198.51.100.20\n' > "$CASPIAN_HW_LOCAL/boxes.tsv"

is 'no exit IP is UNPROVEN' \
   "$HW_UNPROVEN no exit IP was captured" \
   "$(cfg_classify reality-a '' '203.0.113.200')"

is 'a matching exit IP is a PASS named by the box' \
   "$HW_PASS reality-a" \
   "$(cfg_classify reality-a '192.0.2.10' '203.0.113.200')"

c="$(cfg_classify reality-a '198.51.100.20' '203.0.113.200')"
is 'an exit IP owned by another config FAILS' "$HW_FAIL" "${c%% *}"
has 'and names that other box, never its address' "trojan-b" "${c#* }"
hasnt 'and does NOT print the other box address' '198.51.100.20' "${c#* }"

c="$(cfg_classify reality-a '203.0.113.250' '203.0.113.200')"
is 'an unknown exit IP FAILS' "$HW_FAIL" "${c%% *}"
has 'and the unknown address IS printed, since it is no config secret' '203.0.113.250' "${c#* }"

# The ordering that matters most: the baseline check runs BEFORE the match, so
# a box whose server address somehow equals the baseline reports LEAK.
is 'the baseline check outranks a matching address' \
   "$HW_LEAK the exit IP equals the untunnelled baseline" \
   "$(cfg_classify reality-a '192.0.2.10' '192.0.2.10')"

# ---------------------------------------------------------------------------
printf '\n== reading a response body ==\n'
# ---------------------------------------------------------------------------
is 'a bare IP body is an ip'      'ip'      "$(ei_classify_body '203.0.113.7')"
is 'an empty body is empty'       'empty'   "$(ei_classify_body '')"
is 'a body with no IP is empty'   'empty'   "$(ei_classify_body 'Welcome to nginx')"
# The exact string measured from Chrome on the attached handset on 2026-08-30,
# after navigating to a name under .invalid.
is 'a Chrome DNS error page is blocked, not empty' 'blocked' \
   "$(ei_classify_body 'This site can not be reached

Check if there is a typo in caspian-probe.invalid.

DNS_PROBE_FINISHED_NXDOMAIN
Reload')"
is 'a Chrome offline page is blocked' 'blocked' \
   "$(ei_classify_body 'No internet ERR_INTERNET_DISCONNECTED')"

is 'first IP from a noisy body' '198.51.100.9' \
   "$(printf 'Your IP is 198.51.100.9 (via 203.0.113.1)\n' | hw_first_ip)"

# ---------------------------------------------------------------------------
printf '\n== source A: cdn-cgi/trace ==\n'
# ---------------------------------------------------------------------------
# The body below is the VERBATIM response measured through Chrome on the
# attached handset on 2026-08-30, with only the exit address swapped for a
# documentation-space one.
#
# Read the second line. "h=1.1.1.1" comes BEFORE "ip=", so the FIRST IP literal
# in this body is the echo service's own address and not the exit address at
# all. Worse, it is a CONSTANT: it is 1.1.1.1 whatever the tunnel is doing. A
# harness that took the first literal would report the same value for every
# config and the switch test would say "the exit IP did not change" about a box
# that was working perfectly.
TRACE_BODY='fl=985f9
h=1.1.1.1
ip=203.0.113.45
ts=1788051919.000
visit_scheme=https
uag=Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Mobile Safari/537.36
colo=LHR
sliver=050-tier1
http=http/2
loc=GB
tls=TLSv1.3
sni=off
warp=off
gateway=off
rbi=off
kex=X25519MLKEM768'

is 'the naive first-literal read returns the WRONG address' '1.1.1.1' \
   "$(printf '%s\n' "$TRACE_BODY" | hw_first_ip)"
is 'source A reads the ip= field, not the first literal' 'ip	203.0.113.45' \
   "$(ei_read_a "$TRACE_BODY")"
is 'source A on an empty body' 'empty	' "$(ei_read_a '')"
is 'source A on a Chrome error page' 'blocked	' \
   "$(ei_read_a 'This site can not be reached

ERR_CONNECTION_TIMED_OUT')"
# An anycast address can move and put something else on the other end. A body
# that is not the shape we know is UNPROVEN, never a guess.
is 'source A on an unrecognised shape is a shape failure' 'shape	' \
   "$(ei_read_a '<html><body>Welcome to nginx 203.0.113.99</body></html>')"
is 'source A refuses a trace body with no ip= field' 'shape	' \
   "$(ei_read_a 'fl=985f9
h=1.1.1.1
loc=GB')"

# ---------------------------------------------------------------------------
printf '\n== source B: ipinfo over plain HTTP ==\n'
# ---------------------------------------------------------------------------
# Verbatim from the handset on 2026-08-30, address swapped. Note "via: 1.1
# google" in the headers: header text is why the body must be split off before
# anything is parsed out of it.
B_OK='HTTP/1.0 200 OK
Content-Length: 13
access-control-allow-origin: *
content-type: text/plain; charset=utf-8
date: Sun, 30 Aug 2026 01:04:52 GMT
via: 1.1 google

203.0.113.45'

# Measured: this is what comes back when the Host header is omitted. ipinfo is
# a name-based virtual host and the pinned address alone is not enough.
B_NOHOST='HTTP/1.0 404 Not Found
Content-Length: 18
content-type: text/plain
via: 1.1 google
date: Sun, 30 Aug 2026 01:04:56 GMT

fault filter abort'

is 'source B reads the body of a 200'      'ip	203.0.113.45' "$(ei_read_b "$B_OK")"
is 'source B reports a non-200 as a shape failure' 'shape	404' "$(ei_read_b "$B_NOHOST")"
is 'source B on an empty response'         'empty	'       "$(ei_read_b '')"
is 'source B ignores addresses in headers' 'ip	203.0.113.45' \
   "$(ei_read_b 'HTTP/1.0 200 OK
x-forwarded-for: 198.51.100.77
via: 1.1 google

203.0.113.45')"
is 'no IP means no output' '' "$(printf 'nothing here\n' | hw_first_ip)"

# ---------------------------------------------------------------------------
printf '\n== two sources ==\n'
# ---------------------------------------------------------------------------
is 'both agreeing'        'agree 203.0.113.5'    "$(ei_agree '203.0.113.5' '203.0.113.5')"
is 'both disagreeing'     'disagree 203.0.113.5' "$(ei_agree '203.0.113.5' '203.0.113.6')"
is 'only Chrome answered' 'single-a 203.0.113.5' "$(ei_agree '203.0.113.5' '-')"
is 'only nc answered'     'single-b 203.0.113.6' "$(ei_agree '-' '203.0.113.6')"
is 'neither answered'     'none -'               "$(ei_agree '-' '-')"

# ---------------------------------------------------------------------------
printf '\n== a partial run looks partial ==\n'
# ---------------------------------------------------------------------------
RUN="$TMP/run"; mkdir -p "$RUN"
hw_ledger_init "$RUN" 3
hw_ledger_start "$RUN" "one"; hw_ledger_end "$RUN" "one" "OK"
hw_ledger_start "$RUN" "two"
out="$(hw_ledger_report "$RUN" 2>&1 || true)"
has 'a run killed mid-step says PARTIAL' 'PARTIAL' "$out"
has 'and says how many finished'         '1 of 3'  "$out"
hw_ledger_end "$RUN" "two" "OK"
hw_ledger_start "$RUN" "three"; hw_ledger_end "$RUN" "three" "OK"
out="$(hw_ledger_report "$RUN" 2>&1 || true)"
hasnt 'a complete run does not say PARTIAL' 'PARTIAL' "$out"

# ---------------------------------------------------------------------------
printf '\n== fingerprints compare without disclosing ==\n'
# ---------------------------------------------------------------------------
FP="$TMP/fp"; mkdir -p "$FP"
f1="$(hw_fp '192.0.2.10' "$FP")"
f2="$(hw_fp '192.0.2.10' "$FP")"
f3="$(hw_fp '198.51.100.20' "$FP")"
is  'the same address fingerprints the same' "$f1" "$f2"
if [ "$f1" = "$f3" ]; then bad 'different addresses differ' 'different' 'same'; else ok 'different addresses fingerprint differently'; fi
hasnt 'the fingerprint does not contain the address' '192.0.2' "$f1"
FP2="$TMP/fp2"; mkdir -p "$FP2"
if [ "$(hw_fp '192.0.2.10' "$FP2")" = "$f1" ]; then
  bad 'a different run salts differently' 'different' 'same'
else
  ok 'a different run salts differently, so fingerprints are not a lookup table'
fi

# ---------------------------------------------------------------------------
printf '\n== dry run refuses to produce a verdict ==\n'
# ---------------------------------------------------------------------------
HW_DRY=1
out="$( ( hw_require_measured "$HW_DRY_SENTINEL" ) 2>&1 || true )"
has 'a sentinel value cannot reach a verdict' 'nothing was measured' "$out"
HW_DRY=0
is 'and the switch goes back off afterwards' '0' "$HW_DRY"

printf '\n----------------------------------------\n'
printf '%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
