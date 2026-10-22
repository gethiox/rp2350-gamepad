### Downloads

| File | What |
|---|---|
| `rp2350-gamepad-<version>.uf2` | Firmware: copy it onto the `RP2350` drive (hold BOOT while plugging in). |
| `rp2350-gamepad-reset-settings.uf2` | Optional: erases stored settings (configuration + calibration) without touching the firmware. |
| `rp2350-gamepad-<version>.elf` | Firmware with debug symbols, for developers. |
| `rp2350-gamepad-configurator-<version>-windows-amd64.zip` | Configurator for Windows 10/11. |
| `rp2350-gamepad-configurator-<version>-macos-universal.zip` | Configurator for macOS 11+ (Apple Silicon and Intel). The first time, right-click → Open (not notarised). |
| `rp2350-gamepad-configurator-<version>-linux-amd64.tar.gz` / `-linux-arm64` | Configurator for Linux. Run `install-udev-rule.sh` once. |
| `99-rp2350-gamepad.rules` | The Linux udev rule on its own. |
| `SHA256SUMS` | Checksums of all files. |

Use the configurator and firmware from the same release.

### Verify your download

```sh
sha256sum --ignore-missing -c SHA256SUMS
gh attestation verify <file> --repo gethiox/rp2350-gamepad
```

The firmware is reproducible: `scripts/build-firmware.sh` at this tag
produces identical files. See
[docs/verifying-releases.md](https://github.com/gethiox/rp2350-gamepad/blob/main/docs/verifying-releases.md).

See [CHANGELOG.md](https://github.com/gethiox/rp2350-gamepad/blob/main/CHANGELOG.md) for changes.
