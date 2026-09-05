#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Build a drag-and-run macOS distribution image. The image contains a native
# double-click installer app, the Go binary and the launchd plists. The app
# uses macOS's own administrator-authorization dialog and does not download a
# closed-source runtime.
set -euo pipefail

VERSION="${1:-dev}"
ARCH="${2:-$(uname -m)}"
if [ "$ARCH" = x86_64 ]; then ARCH=amd64; fi
OUT="${3:-dist/Caspian-${VERSION}-macos-${ARCH}.dmg}"

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
NUMERIC_VERSION="${VERSION#v}"
if [[ "$NUMERIC_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then BUNDLE_VERSION="$NUMERIC_VERSION"; fi
WORK="$(mktemp -d "${TMPDIR:-/tmp}/caspian-dmg.XXXXXX")"
STAGE="$WORK/Caspian"
APP="$STAGE/Caspian.app"
CONTENTS="$APP/Contents"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$CONTENTS/MacOS" "$CONTENTS/Resources" "$(dirname -- "$OUT")"
CLANG_MODULE_CACHE_PATH="$WORK/clang-cache" swift "$ROOT/packaging/darwin/make-icon.swift" "$CONTENTS/Resources/Caspian.icns"
CGO_ENABLED=0 GOOS=darwin GOARCH="$ARCH" \
	go build -trimpath -buildvcs=false \
		-ldflags "-s -w -X main.version=$VERSION -X caspianbyoc.org/caspian/internal/panel.Version=$VERSION" \
		-o "$CONTENTS/Resources/caspian" "$ROOT/cmd/caspian"

cp "$ROOT/packaging/darwin/install-darwin.sh" "$CONTENTS/Resources/"
cp "$ROOT/packaging/darwin/reset-password.sh" "$CONTENTS/Resources/"
cp "$ROOT/packaging/darwin/service-action.sh" "$CONTENTS/Resources/"
cp "$ROOT/LICENSE" "$CONTENTS/Resources/LICENSE.txt"
cp "$ROOT/NOTICE" "$CONTENTS/Resources/NOTICE.txt"
cp "$ROOT/third_party/libxray-share/LICENSE" "$CONTENTS/Resources/libxray-share-LICENSE.txt"
cp "$ROOT/packaging/darwin/"org.caspianbyoc.caspian*.plist "$CONTENTS/Resources/"
cat > "$CONTENTS/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleDisplayName</key><string>Caspian</string>
  <key>CFBundleExecutable</key><string>Caspian</string>
  <key>CFBundleIdentifier</key><string>org.caspianbyoc.caspian</string>
  <key>CFBundleName</key><string>Caspian</string>
  <key>CFBundleIconFile</key><string>Caspian.icns</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$BUNDLE_VERSION</string>
  <key>CFBundleVersion</key><string>$BUNDLE_VERSION</string>
  <key>CaspianVersion</key><string>$VERSION</string>
  <key>LSMinimumSystemVersion</key><string>13.0</string>
  <key>NSAppTransportSecurity</key><dict><key>NSAllowsLocalNetworking</key><true/></dict>
</dict></plist>
EOF
SWIFT_ARCH="$ARCH"
if [ "$ARCH" = amd64 ]; then SWIFT_ARCH=x86_64; fi
CLANG_MODULE_CACHE_PATH="$WORK/clang-cache" swiftc -O -target "$SWIFT_ARCH-apple-macos13.0" \
	"$ROOT/packaging/darwin/CaspianControl.swift" -o "$CONTENTS/MacOS/Caspian"
chmod 0755 "$CONTENTS/MacOS/Caspian" "$CONTENTS/Resources/caspian" "$CONTENTS/Resources/install-darwin.sh"
codesign --force --sign - "$CONTENTS/Resources/caspian"
codesign --force --sign - "$APP"
codesign --verify --deep --strict "$APP"

cat > "$STAGE/README.txt" <<EOF
Caspian $VERSION for macOS ($ARCH)

Double-click “Caspian.app”. macOS will ask for your administrator
password when you choose “Install / Update”. Save the panel password shown
after installation, then choose “Open Panel”. These are different passwords.
Use the panel's password section to change it, or “Reset Password” in this app
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
