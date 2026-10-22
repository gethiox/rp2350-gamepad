#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
#
# Builds and packages the configurator for one platform into dist/.
#
#   scripts/package-configurator.sh linux amd64 v1.2.3
#   scripts/package-configurator.sh windows amd64 v1.2.3   (run on Windows, bash)
#   scripts/package-configurator.sh macos universal v1.2.3 (run on macOS)
set -euo pipefail

OS="$1"
ARCH="$2"
VERSION="${3:-dev}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"
NAME="rp2350-gamepad-configurator"
PKG="$NAME-$VERSION-$OS-$ARCH"
LDFLAGS="-s -w -X github.com/gethiox/rp2350-gamepad/configurator/internal/app.Version=$VERSION"
mkdir -p "$DIST"
cd "$ROOT/configurator"
export CGO_ENABLED=1

build() { # out [extra go flags...]
	local out="$1"
	shift
	go build -trimpath -ldflags "$LDFLAGS $*" -o "$out" ./cmd/$NAME
}

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

case "$OS" in
linux)
	mkdir -p "$STAGE/$PKG"
	build "$STAGE/$PKG/$NAME"
	cp "$ROOT/packaging/linux/99-rp2350-gamepad.rules" "$ROOT/packaging/linux/install-udev-rule.sh" "$STAGE/$PKG/"
	cp "$ROOT/LICENSE" "$ROOT/THIRD_PARTY_NOTICES.md" "$STAGE/$PKG/"
	cp "$ROOT/docs/linux.md" "$STAGE/$PKG/README-linux.md"
	tar -C "$STAGE" --owner=0 --group=0 -czf "$DIST/$PKG.tar.gz" "$PKG"
	;;
windows)
	mkdir -p "$STAGE/$PKG"
	build "$STAGE/$PKG/$NAME.exe" "-H=windowsgui"
	cp "$ROOT/LICENSE" "$STAGE/$PKG/LICENSE.txt"
	cp "$ROOT/THIRD_PARTY_NOTICES.md" "$STAGE/$PKG/THIRD_PARTY_NOTICES.md"
	(cd "$STAGE" && 7z a -tzip -bd "$DIST/$PKG.zip" "$PKG" >/dev/null)
	;;
macos)
	APP="$STAGE/rp2350-gamepad configurator.app"
	mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
	(export GOARCH=arm64 CC="clang -arch arm64" && build "$STAGE/$NAME-arm64")
	(export GOARCH=amd64 CC="clang -arch x86_64" CGO_CFLAGS="-arch x86_64" CGO_LDFLAGS="-arch x86_64" && build "$STAGE/$NAME-amd64")
	lipo -create -output "$APP/Contents/MacOS/$NAME" "$STAGE/$NAME-arm64" "$STAGE/$NAME-amd64"
	sed "s/@VERSION@/${VERSION#v}/g" "$ROOT/packaging/macos/Info.plist" >"$APP/Contents/Info.plist"
	cp "$ROOT/LICENSE" "$ROOT/THIRD_PARTY_NOTICES.md" "$APP/Contents/Resources/"
	codesign --force --deep --sign - "$APP" # ad-hoc signature (not notarised)
	(cd "$STAGE" && ditto -c -k --keepParent "rp2350-gamepad configurator.app" "$DIST/$PKG.zip")
	;;
*)
	echo "unknown OS $OS" >&2
	exit 1
	;;
esac
echo "packaged $(ls "$DIST" | grep "$PKG")"
