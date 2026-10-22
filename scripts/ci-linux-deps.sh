#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Installs what raylib (X11 + Wayland) and the host build need on Ubuntu.
set -euo pipefail
sudo apt-get update
sudo apt-get install -y --no-install-recommends \
	build-essential cmake ninja-build pkg-config \
	libgl1-mesa-dev libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev \
	libxkbcommon-dev libwayland-dev
