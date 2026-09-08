#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Build a drag-and-run macOS distribution image. The image contains a native
# Flutter application, the Go binary and the launchd plists.
set -euo pipefail

VERSION="${1:-dev}"
ARCH="${2:-$(uname -m)}"
if [ "$ARCH" = x86_64 ]; then ARCH=amd64; fi
OUT="${3:-dist/Caspian-${VERSION}-macos-${ARCH}.dmg}"

# A release job must never publish an app whose visible version differs from
# the tag GitHub Actions is releasing. Local preview builds may keep a suffix
# (for example v0.2.4-ux.1) without impersonating that published artefact.
if [ "${GITHUB_REF_TYPE:-}" = tag ] && [ -n "${GITHUB_REF_NAME:-}" ] && [ "$VERSION" != "$GITHUB_REF_NAME" ]; then
	printf 'caspian: build version %s does not match CI release tag %s\n' "$VERSION" "$GITHUB_REF_NAME" >&2
	exit 2
fi

case "$ARCH" in
	arm64|amd64) ;;
	*) printf 'caspian: unsupported macOS architecture %s\n' "$ARCH" >&2; exit 2 ;;
esac

case "$(uname -s)" in
	Darwin) ;;
	*) printf 'caspian: DMG packaging must run on macOS (hdiutil is required)\n' >&2; exit 2 ;;
esac

ROOT="$(cd -- "$(dirname -- "$0")/../.." && pwd)"
case "$VERSION" in
  *[!a-zA-Z0-9._-]*) printf 'caspian: invalid version\n' >&2; exit 2 ;;
esac
BUNDLE_VERSION=0.0.0
# Go 1.26 supports macOS 12; Go 1.27 raised its runtime minimum to 13.
GO_VERSION="$(go env GOVERSION)"
case "$GO_VERSION" in
  go1.26*) MACOS_MINIMUM=12.0 ;;
  go1.27*) MACOS_MINIMUM=13.0 ;;
  *) printf 'caspian: unverified macOS minimum for Go %s\n' "$GO_VERSION" >&2; exit 2 ;;
esac
NUMERIC_VERSION="${VERSION#v}"
NUMERIC_VERSION="${NUMERIC_VERSION%%-*}"
if [[ "$NUMERIC_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then BUNDLE_VERSION="$NUMERIC_VERSION"; fi
WORK="$(mktemp -d "${TMPDIR:-/tmp}/caspian-dmg.XXXXXX")"
STAGE="$WORK/Caspian"
APP="$STAGE/Caspian.app"
CONTENTS="$APP/Contents"
trap 'rm -rf "$WORK"' EXIT

bash "$ROOT/scripts/build-ui.sh" web "$VERSION"
bash "$ROOT/scripts/build-ui.sh" macos "$VERSION"
mkdir -p "$STAGE" "$(dirname -- "$OUT")"
cp -R "$ROOT/ui/build/macos/Build/Products/Release/Caspian.app" "$APP"
test -d "$CONTENTS/Frameworks"
# The Flutter build is universal, while each release bundles one Go backend.
# Ship matching native executables so the other architecture cannot launch a
# GUI whose bundled service cannot run on that machine.
UI_ARCH="$ARCH"
if [ "$UI_ARCH" = amd64 ]; then UI_ARCH=x86_64; fi
UI_BINARY="$CONTENTS/MacOS/Caspian"
if [ "$(lipo -archs "$UI_BINARY")" != "$UI_ARCH" ]; then
  lipo "$UI_BINARY" -thin "$UI_ARCH" -output "$WORK/caspian-ui"
  chmod 0755 "$WORK/caspian-ui"
  mv "$WORK/caspian-ui" "$UI_BINARY"
fi
test "$(lipo -archs "$UI_BINARY")" = "$UI_ARCH"
# The familiar Applications alias gives somebody who has never installed a
# DMG an obvious permanent home for Caspian. Password recovery must remain
# available after the image is ejected.
ln -s /Applications "$STAGE/Applications"
CLANG_MODULE_CACHE_PATH="$WORK/clang-cache" swift "$ROOT/packaging/darwin/make-icon.swift" "$CONTENTS/Resources/Caspian.icns"
CGO_ENABLED=0 GOOS=darwin GOARCH="$ARCH" \
	go build -tags flutterui -trimpath -buildvcs=false \
		-ldflags "-s -w -X main.version=$VERSION -X caspianbyoc.org/caspian/internal/panel.Version=$VERSION" \
		-o "$CONTENTS/Resources/caspian" "$ROOT/cmd/caspian"
test "$(lipo -archs "$CONTENTS/Resources/caspian")" = "$UI_ARCH"

cp "$ROOT/packaging/darwin/install-darwin.sh" "$CONTENTS/Resources/"
cp "$ROOT/packaging/darwin/reset-password.sh" "$CONTENTS/Resources/"
cp "$ROOT/packaging/darwin/service-action.sh" "$CONTENTS/Resources/"
cp "$ROOT/LICENSE" "$CONTENTS/Resources/LICENSE.txt"
cp "$ROOT/NOTICE" "$CONTENTS/Resources/NOTICE.txt"
cp "$ROOT/third_party/libxray-share/LICENSE" "$CONTENTS/Resources/libxray-share-LICENSE.txt"
cp "$ROOT/packaging/darwin/"org.caspianbyoc.caspian*.plist "$CONTENTS/Resources/"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $BUNDLE_VERSION" "$CONTENTS/Info.plist"
/usr/libexec/PlistBuddy -c "Add :CaspianVersion string $VERSION" "$CONTENTS/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleIconFile Caspian.icns" "$CONTENTS/Info.plist"
/usr/libexec/PlistBuddy -c "Set :LSMinimumSystemVersion $MACOS_MINIMUM" "$CONTENTS/Info.plist"
chmod 0755 "$CONTENTS/Resources/caspian" "$CONTENTS/Resources/install-darwin.sh"
# Flutter includes signed frameworks; preserve their signatures and sign the
# changed outer application only after adding the backend and setup resources.
codesign --force --sign - "$CONTENTS/Resources/caspian"
codesign --force --sign - "$APP"
codesign --verify --deep --strict "$APP"

cat > "$STAGE/README.txt" <<EOF
Caspian $VERSION for macOS ($ARCH)
Requires macOS $MACOS_MINIMUM or later.

Drag "Caspian.app" onto "Applications", then open the copy in Applications.
Follow the setup guide inside the app. If the service is unavailable, choose
"Install and continue" and approve the macOS administrator prompt.
If services are already installed but stopped, choose "Start existing services".
The guide waits for the service to respond. Save the Caspian password shown,
confirm that you saved it, and continue to sign in inside the application.
The Caspian password differs from your administrator password.
After signing in, follow the connection checklist to add a configuration,
set up the hotspot, switch Caspian on, and connect another device.
After replacing the app with an update, open "Service recovery" and choose
"Install services" to
update its background services. Your saved configuration and password remain.
Use the password section to change it, or "Reset password" in this app
if you have forgotten it. Keep a copy of Caspian.app outside the disk image.

Terminal fallback:
  sudo CASPIAN_LOCAL_BINARY="\$PWD/Caspian.app/Contents/Resources/caspian" \\
    bash "\$PWD/Caspian.app/Contents/Resources/install-darwin.sh"

The installer creates the two launchd services and a least-privilege _caspian
account. A wired uplink is required: macOS cannot use the same built-in Wi-Fi
radio as both the internet connection and the hotspot, and USB Wi-Fi dongles
are not an AP backend supported by macOS.
EOF

hdiutil create -quiet -volname "Caspian $VERSION" -srcfolder "$STAGE" -format UDZO "$WORK/Caspian.dmg"
hdiutil verify -quiet "$WORK/Caspian.dmg"
mv -f "$WORK/Caspian.dmg" "$OUT"
printf 'created %s\n' "$OUT"
