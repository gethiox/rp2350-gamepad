#!/bin/sh
# Installs the udev rule so the configurator can talk to the device without
# root rights. Run once: sh install-udev-rule.sh   (asks for your password)
set -e
DIR="$(cd "$(dirname "$0")" && pwd)"
sudo install -m 0644 "$DIR/99-rp2350-gamepad.rules" /etc/udev/rules.d/99-rp2350-gamepad.rules
sudo udevadm control --reload-rules
sudo udevadm trigger
echo "Done. Unplug the gamepad and plug it in again."
