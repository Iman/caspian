#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# fetch-public-lists.sh downloads public share-link lists for the opt-in survey
# in internal/link (TestPublicListsParseAndBuild). The survey measures what this
# repository's parser and the vendored engine accept from the wild, and why
# they refuse the rest.
#
# The files land under local/, which is gitignored and skipped by the privacy
# scan, because a public list is a list of strangers' servers and nothing in
# this repository may carry a server address. Nothing here connects to any of
# them; the survey parses and builds, and stops there.
#
#     bash scripts/fetch-public-lists.sh
#     CASPIAN_PUBLIC_LISTS_DIR=local/public-lists go test ./internal/link/ -run TestPublicLists -v -count=1
#
# Pass a directory to use a different destination. Output carries no ANSI
# escape codes, no colour, no emoji and no em dashes.

set -o errexit
set -o nounset
set -o pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
root=$(dirname "$script_dir")
dest=${1:-"$root/local/public-lists"}
mkdir -p "$dest"

# owner/repo  branch  path
lists='
freefq/free master v2
cbusifabcap/daily_free_vpn main Z.txt
hello-world-1989/cn-news main end-gfw-together
hello-world-1989/cn-news main end-gfw-together-ss
hello-world-1989/cn-news main clash.yaml
ebrasha/free-v2ray-public-list main V2Ray-Config-By-EbraSha.txt
ebrasha/free-v2ray-public-list main separated-protocols/hysteria2_configs.txt
ebrasha/free-v2ray-public-list main separated-protocols/trojan_configs.txt
'

count=0
while read -r repo branch path; do
  [ -z "$repo" ] && continue
  out="$dest/$(echo "$repo" | tr '/' '_')__$(basename "$path")"
  if curl -fsSL "https://raw.githubusercontent.com/$repo/$branch/$path" -o "$out"; then
    printf '%-64s %9s bytes\n' "$(basename "$out")" "$(wc -c < "$out" | tr -d ' ')"
    count=$((count + 1))
  else
    printf '%-64s FAILED\n' "$repo/$path"
  fi
done <<< "$lists"

echo "fetched $count lists into $dest"
echo "run: CASPIAN_PUBLIC_LISTS_DIR=$dest go test ./internal/link/ -run TestPublicLists -v -count=1"
