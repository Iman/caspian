#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "$0")/../.." && pwd)"
VERSION="${1:-dev}"
case "$VERSION" in *[!a-zA-Z0-9._-]*) echo 'Invalid version' >&2; exit 2 ;; esac
test "$(uname -s)" = Linux || { echo 'Build the Linux package on Linux.' >&2; exit 2; }
case "$(uname -m)" in x86_64) ARCH=amd64; FLUTTER_ARCH=x64 ;; aarch64) ARCH=arm64; FLUTTER_ARCH=arm64 ;; *) echo 'Linux desktop requires x64 or arm64.' >&2; exit 2 ;; esac
bash "$ROOT/scripts/build-ui.sh" web "$VERSION"
bash "$ROOT/scripts/build-ui.sh" linux "$VERSION"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
STAGE="$WORK/Caspian"
mkdir -p "$STAGE/backend" "$ROOT/dist"
cp -R "$ROOT/ui/build/linux/$FLUTTER_ARCH/release/bundle/." "$STAGE/"
test -x "$STAGE/caspian_ui"
cd "$ROOT"
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -tags flutterui -trimpath -buildvcs=false \
  -ldflags "-s -w -X main.version=$VERSION -X caspianbyoc.org/caspian/internal/panel.Version=$VERSION" \
  -o "$STAGE/backend/caspian" ./cmd/caspian
cp install.sh uninstall.sh "$STAGE/"
cp packaging/linux/install-desktop.sh packaging/linux/uninstall-desktop.sh packaging/linux/service-action.sh packaging/linux/caspian.desktop packaging/linux/README.md "$STAGE/"
cp LICENSE NOTICE "$STAGE/"
cp third_party/libxray-share/LICENSE "$STAGE/libxray-share-LICENSE.txt"
chmod 0755 "$STAGE/install-desktop.sh"
(cd "$STAGE/backend" && sha256sum caspian | sed "s/  caspian$/  caspian-linux-$ARCH/" > SHA256SUMS)
tar -czf "$ROOT/dist/Caspian-$VERSION-linux-$ARCH.tar.gz" -C "$WORK" Caspian
printf 'created dist/Caspian-%s-linux-%s.tar.gz\n' "$VERSION" "$ARCH"
