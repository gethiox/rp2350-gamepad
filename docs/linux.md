# Linux: one-time setup

The gamepad itself needs nothing on Linux. It is a standard USB HID
device and works in games right away.

The **configurator** talks to the device through a small serial port
(`/dev/ttyACM0` or similar). Most distributions only let members of a
special group (`dialout` or `uucp`) open serial ports. ModemManager, which
is installed on many desktops, also likes to "test" every new serial
device for a modem. That delays the configurator by a few seconds.

The release includes a udev rule that fixes both.

## Install the rule (recommended)

In the unpacked configurator folder, run:

```sh
sh install-udev-rule.sh
```

It asks for your password, copies `99-rp2350-gamepad.rules` to
`/etc/udev/rules.d/` and reloads udev. **Unplug the gamepad and plug it in
again**; then start the configurator.

To do the same by hand:

```sh
sudo cp 99-rp2350-gamepad.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
sudo udevadm trigger
```

The rule gives the user sitting at the computer access to the device
(`TAG+="uaccess"`). It does not make the port world-writable.

## If you changed the USB vendor/product ID

The rule matches the default ID `ACDC:2137`. If you gave your device a
different ID in the configurator, edit the rule and add a copy of both
lines with your values, written in lower case:

```
SUBSYSTEM=="tty", ATTRS{idVendor}=="1209", ATTRS{idProduct}=="0001", MODE="0660", TAG+="uaccess", ENV{ID_MM_DEVICE_IGNORE}="1"
SUBSYSTEM=="usb", ATTRS{idVendor}=="1209", ATTRS{idProduct}=="0001", ENV{ID_MM_DEVICE_IGNORE}="1"
```

## Alternative: group membership

Instead of the rule, you can add yourself to the serial group and log out
and in again:

```sh
sudo usermod -aG dialout "$USER"    # Debian, Ubuntu, Fedora
sudo usermod -aG uucp "$USER"       # Arch, openSUSE
```

## Checking

- `ls -l /dev/ttyACM*` shows the port while the gamepad is connected.
- The configurator's log (click the bottom bar) says *No permission to
  open …* when access is still missing.
- `journalctl -f` while plugging in shows whether ModemManager still
  probes the device.

## Wayland and X11

The configurator supports both. If it does not start under Wayland, try
`GDK_BACKEND=x11` or start it from an X11 session, and report the log
output in an issue.
