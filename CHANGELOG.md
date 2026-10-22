# Changelog

## Unreleased - first public release

The project was renamed from *stick-extension-v2* to **rp2350-gamepad**.
Settings stored by earlier development builds are not carried over: after
flashing, the device starts with defaults and needs to be calibrated once.

### Firmware

#### Added

- **Settings checked on the device.** Every configuration is validated on
  the device with readable messages, both before saving and at boot.
  Invalid stored data falls back to defaults instead of crashing.
- **Settings storage.**
  - Power-loss safe: two flash slots, sequence numbers, CRCs and
    read-back verification.
  - Kept across firmware updates, with a format version so future
    releases can migrate settings.
- **Reliability.**
  - Watchdog with crash counting and a safe mode.
  - Sleep: the LEDs go dark and processing pauses when the computer
    sleeps or is off, with an option to stay lit.
- **Calibration.**
  - Only the axes you move are updated.
  - Sticks are calibrated by pushing up and rotating clockwise;
    orientation and mirroring are detected automatically.
  - Repeated or false-started gestures are handled.
- **Axes and sticks.**
  - Per-axis invert, linear mode (throttles) and optional per-axis
    deadzones.
  - Per-stick deadzones and orientation adjustments.
- **Configurable USB identity.** Manufacturer, product and serial number
  (the chip ID by default), report rate and report ID, and gamepad or
  joystick device type.
- **Live LED preview** of unsaved colour and brightness changes, without
  touching gamepad behaviour.
- **LED chains:** each LED pin drives a chain of WS2812 LEDs (64 LEDs in
  total). Chains are sent by DMA, so long chains never stall the input
  loop.
- **Axis LED effects:** Glow, Bar (level meter) and Dot (moving point),
  applied per LED chain; a single selected LED glows as an indicator.
- **Recovery image** `reset-settings.uf2`, which erases the settings
  without reflashing.
- **Reproducible builds**, with versioning from git tags.

#### Fixed (compared with stick-extension-v2)

- **Config writes were accepted unchecked.** Out-of-range counts could
  overflow arrays and crash the device.
- **Every rebuild of the firmware reset the configuration.** It was keyed
  to the build timestamp.
- **Interrupted saves lost data.** Configuration and calibration shared
  one sector that was erased in place, so an interrupted save lost both.
- **Calibration could produce broken axes.**
  - An axis that was not moved got min > max.
  - An incomplete stick gesture made the stick saturate at about 5 %
    deflection.
- **USB descriptor problems.**
  - The device class was wrong for an IAD composite device.
  - Remote wakeup was advertised but not implemented.
  - The serial number was hard-coded.
- **UART pins were claimed.** The board-support init claimed the UART on
  GPIO 0/1 and the LED on GPIO 25.

### Configurator

The configurator was rewritten, with its own UI toolkit replacing raygui.

- **One window.** All settings on one scrolling page, live input in every
  card, and a resizable window with an adjustable interface size.
- **Connecting to other firmware:** a device that answers in another
  protocol (for example older firmware) is reported once as incompatible
  instead of being retried in a loop.
- **Log:** monospace, scrollable, with mouse selection and copy. The
  status bar sits on top of the log and moves with it.
- **Frame rate setting** for high refresh rate displays.
- **Clear edit state.**
  - Unsaved changes are marked per field and can be undone individually.
  - Device-reported errors are shown on the affected fields.
  - Edits survive reconnects and calibration.
- **Always in sync with the device.**
  - The configuration is re-read after every save, and the save is
    verified after the reboot.
  - Calibration is re-read when a session ends, even when it is finished
    with the button on the device.
- **Live views.** A stick visualiser with trail and roundness metrics,
  and live calibration progress.
- **Backups, confirmations and settings.** Backup export/import (also by
  drag and drop), confirmations for destructive actions, and settings
  (scale, USB IDs, live preview, backup folder).
- **Status bar with a log drawer**, plus a log file for troubleshooting.
