#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
export NO_COLOR=1
ROOT="$(cd -- "$(dirname -- "$0")/.." && pwd)"
TARGET="${1:-web}"
VERSION="${2:-dev}"
case "$TARGET" in web|macos|linux) ;; *) echo 'Usage: build-ui.sh web|macos|linux [version]' >&2; exit 2 ;; esac
case "$VERSION" in *[!a-zA-Z0-9._-]*) echo 'Invalid version' >&2; exit 2 ;; esac
BUILD_NAME="${VERSION#v}"
BUILD_NAME="${BUILD_NAME%%-*}"
if ! [[ "$BUILD_NAME" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then BUILD_NAME=0.0.0; fi
cd "$ROOT/ui"
flutter --suppress-analytics pub get
if [ "$TARGET" = web ]; then
  # Keep CanvasKit local so recovery also loads without an internet connection.
  flutter --suppress-analytics build web --release --no-web-resources-cdn --pwa-strategy=none --build-name "$BUILD_NAME" --dart-define="CASPIAN_VERSION=$VERSION"
  test -s build/web/index.html
  test -s build/web/main.dart.js
  mkdir -p "$ROOT/internal/panel/flutter"
  # Retain the tracked placeholder, replacing only generated build contents.
  for entry in "$ROOT/internal/panel/flutter/"*; do
    [ -e "$entry" ] || continue
    rm -rf -- "$entry"
  done
  cp -R build/web/. "$ROOT/internal/panel/flutter/"
else
  flutter --suppress-analytics build "$TARGET" --release --build-name "$BUILD_NAME" --dart-define="CASPIAN_VERSION=$VERSION"
fi
