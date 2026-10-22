#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
#
# Reproducible firmware build (Linux x86-64).
#
# Pins the Pico SDK and the Arm toolchain (verified by SHA-256), derives
# SOURCE_DATE_EPOCH from the last commit and strips build paths, so the same
# commit always produces byte-identical .uf2 files - on CI and on your PC.
#
#   scripts/build-firmware.sh            # -> dist/rp2350-gamepad-<version>.uf2
#   GP_VERSION=v1.2.3 scripts/build-firmware.sh
#
# Downloads are cached in .cache/ (git-ignored).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CACHE="${GP_CACHE_DIR:-$ROOT/.cache}"
DIST="$ROOT/dist"
BUILD="$ROOT/build/firmware"

SDK_TAG="2.2.0"
SDK_COMMIT="a1438dff1d38bd9c65dbd693f0e5db4b9ae91779"
TINYUSB_COMMIT="86ad6e56c1700e85f1c5678607a762cfe3aa2f47"
TC_VERSION="14.2.1-1.1"
TC_NAME="xpack-arm-none-eabi-gcc-${TC_VERSION}-linux-x64.tar.gz"
TC_URL="https://github.com/xpack-dev-tools/arm-none-eabi-gcc-xpack/releases/download/v${TC_VERSION}/${TC_NAME}"
TC_SHA256="ed8c7d207a85d00da22b90cf80ab3b0b2c7600509afadf6b7149644e9d4790a6"

if [[ "$(uname -s)-$(uname -m)" != "Linux-x86_64" ]]; then
	echo "This script pins a Linux x86-64 toolchain. On other systems follow docs/building.md." >&2
	exit 1
fi
missing=()
for tool in git cmake ninja python3 curl sha256sum tar; do
	command -v "$tool" >/dev/null || missing+=("$tool")
done
if ((${#missing[@]})); then
	echo "Missing tools: ${missing[*]}" >&2
	if command -v apt-get >/dev/null; then
		pkgs=()
		for t in "${missing[@]}"; do
			case "$t" in
			ninja) pkgs+=(ninja-build) ;;
			sha256sum) pkgs+=(coreutils) ;;
			*) pkgs+=("$t") ;;
			esac
		done
		echo "On Debian/Ubuntu: sudo apt-get install ${pkgs[*]}" >&2
	fi
	exit 1
fi
mkdir -p "$CACHE" "$DIST"

# --- toolchain -----------------------------------------------------------------
TC_DIR="$CACHE/xpack-arm-none-eabi-gcc-${TC_VERSION}"
if [[ ! -x "$TC_DIR/bin/arm-none-eabi-gcc" ]]; then
	echo ">> downloading $TC_NAME"
	curl -fsSL -o "$CACHE/$TC_NAME" "$TC_URL"
	echo "$TC_SHA256  $CACHE/$TC_NAME" | sha256sum -c -
	tar -xzf "$CACHE/$TC_NAME" -C "$CACHE"
	rm -f "$CACHE/$TC_NAME"
fi
export PATH="$TC_DIR/bin:$PATH"

# --- Pico SDK ------------------------------------------------------------------
SDK_DIR="$CACHE/pico-sdk-$SDK_TAG"
if [[ ! -d "$SDK_DIR/.git" ]]; then
	echo ">> cloning pico-sdk $SDK_TAG"
	git clone -q --depth 1 --branch "$SDK_TAG" https://github.com/raspberrypi/pico-sdk "$SDK_DIR"
	git -C "$SDK_DIR" submodule update -q --init --depth 1 lib/tinyusb
fi
[[ "$(git -C "$SDK_DIR" rev-parse HEAD)" == "$SDK_COMMIT" ]] || { echo "unexpected pico-sdk commit" >&2; exit 1; }
[[ "$(git -C "$SDK_DIR/lib/tinyusb" rev-parse HEAD)" == "$TINYUSB_COMMIT" ]] || { echo "unexpected tinyusb commit" >&2; exit 1; }
export PICO_SDK_PATH="$SDK_DIR"

# --- build -----------------------------------------------------------------------
VERSION="${GP_VERSION:-$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)}"
export SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git -C "$ROOT" log -1 --format=%ct 2>/dev/null || echo 0)}"
export TZ=UTC LC_ALL=C

echo ">> building firmware $VERSION (SOURCE_DATE_EPOCH=$SOURCE_DATE_EPOCH)"
rm -rf "$BUILD"
cmake -S "$ROOT/firmware" -B "$BUILD" -G Ninja -DCMAKE_BUILD_TYPE=Release -DGP_VERSION="$VERSION" >/dev/null
ninja -C "$BUILD"

cp "$BUILD/rp2350-gamepad.uf2" "$DIST/rp2350-gamepad-$VERSION.uf2"
cp "$BUILD/rp2350-gamepad.elf" "$DIST/rp2350-gamepad-$VERSION.elf"
cp "$BUILD/reset-settings.uf2" "$DIST/rp2350-gamepad-reset-settings.uf2"
echo ">> results in dist/:"
(cd "$DIST" && sha256sum rp2350-gamepad-"$VERSION".uf2 rp2350-gamepad-"$VERSION".elf rp2350-gamepad-reset-settings.uf2)
