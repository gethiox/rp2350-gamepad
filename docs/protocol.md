# Configuration protocol

The device exposes two USB functions: a HID gamepad (what games see) and a
CDC serial port that the configurator uses. All structures are defined in
[`firmware/src/core/gp_protocol.h`](../firmware/src/core/gp_protocol.h).
That header is the single source of truth.

## Framing

```
0xAA 0x55 | cmd (1) | length (2, LE) | payload (length) | CRC-32 (4, LE)
```

- The CRC-32 is IEEE 802.3 (as in zlib or Go's `crc32.ChecksumIEEE`),
  computed over `cmd`, both length bytes and the payload.
- The maximum payload is 2048 bytes.
- Both sides resynchronise byte by byte. Garbage, such as ModemManager
  probes, or a corrupted frame never hides a valid frame that follows it.
- A response uses the request's command code. Its first payload byte is a
  status: `0` OK, `1` invalid (an error list follows), `2` wrong length,
  `3` not possible right now, `4` flash write failed, `5` unknown
  command.
- A response with a non-OK status may be just that one status byte.
- The host sends one request at a time and repeats it after a timeout.
  Only HELLO, READ and TELEMETRY are retried automatically.

## Commands

| Code | Command | Request | Response |
|---|---|---|---|
| 0x01 | HELLO | – | `gp_hello_t`: protocol and format versions, sizes, flags, firmware version, board, chip ID |
| 0x02 | GET_LIMITS | – | `gp_limits_t`: capacities, valid pins per role, numeric ranges per field |
| 0x03 | READ_CONFIG | – | `gp_config_t` currently in use |
| 0x04 | VALIDATE_CONFIG | `gp_config_t` | error list (nothing is changed) |
| 0x05 | WRITE_CONFIG | `gp_config_t` | error list; on OK it is stored and the device restarts after about 150 ms |
| 0x06 | READ_CALIBRATION | – | `gp_calibration_t` |
| 0x07 | WRITE_CALIBRATION | `gp_calibration_t` | error list; applied immediately and stored |
| 0x08 | TELEMETRY | – | `gp_telemetry_t`: inputs, outputs, LEDs, calibration progress, diagnostics |
| 0x09 | CAL_START | – | status |
| 0x0A | CAL_FINISH | – | status + `gp_cal_result_t` (stored) |
| 0x0B | CAL_CANCEL | – | status |
| 0x0C | LED_PREVIEW | `gp_config_t` | status; only the LED look is used, and it expires after 1.5 s unless repeated |
| 0x0D | LED_PREVIEW_STOP | – | status |
| 0x0E | FACTORY_RESET | – | status, then the settings are erased and the device restarts |
| 0x0F | REBOOT | – | status, then the device restarts |
| 0x10 | RESET_CALIBRATION | – | status (calibration defaults, stored) |
| 0x70 | SIM_INPUT | `gp_sim_input_t` | simulator builds only |

An error list is `status, count, truncated, reserved` followed by `count`
× `gp_error_t`. Each entry has a field ID (`GP_F_*`), an item index and a
readable message. The configurator highlights the field and shows the
firmware's message unchanged.

## Validation lives in the firmware

The configurator never decides what is valid.
- It reads `gp_limits_t` to bound its controls (ranges, which pins can do
  what).
- It sends every draft to VALIDATE_CONFIG while you edit.
- WRITE_CONFIG runs the same checks.
- The firmware also validates whatever it loads from flash at boot, and
  falls back to defaults rather than running with a bad configuration.

## Changing the protocol

1. Edit `gp_protocol.h`.
   - Keep every structure free of implicit padding. The static asserts
     and `-Wpadded` help.
   - Bump `GP_CONFIG_VERSION` / `GP_CAL_VERSION` if a stored structure
     changes, and `GP_PROTO_VERSION` if commands or payloads change.
2. If a stored structure changed, add a migration from the previous
   version in `storage.c` (`gp_storage_load`). Otherwise users lose
   their settings when they update, and get defaults instead.
3. Regenerate the contract and update the Go mirror:
   ```sh
   cmake --build build/host --target layout_dump
   build/host/layout_dump > protocol/layout.json
   # then edit configurator/internal/proto/types.go / consts.go
   go test ./configurator/internal/proto/   # must pass
   ```
4. CI fails if `layout.json` is stale or if the Go structures or constants
   differ from it.

A configurator refuses, with a clear message, to talk to firmware with a
different protocol or data format version. Use matching releases.
