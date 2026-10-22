# Firmware architecture

```
src/core/          portable C, compiled for the RP2350 and for the PC
  app.c            main loop step, command handling, LED composition
  config.c         defaults, limits, validation (with readable messages)
  storage.c        power-loss safe persistence (two flash slots)
  calib.c          calibration sessions and the stick gesture recogniser
  axis.c           calibrated ADC -> -1..1 (centred / linear / stick)
  input.c          ADC smoothing, button debouncing
  leds.c           colour mixing, axis LED effects, ordered dithering
  hid.c            HID report descriptor and report packing
  frame.c, crc32.c framing of the serial protocol
  hal.h            the hardware interface the core uses
src/rp2350/        hal_rp2350.c, usb_descriptors.c, main.c (watchdog), ws2812.pio
host/              hal_host.c, unit tests, simulator
```

Everything in `src/core` runs unchanged on the PC. That is how the
calibration, storage and protocol logic is tested without hardware, and
how the configurator's integration tests talk to the real firmware code.

## Main loop

`main.c` runs:
- `tud_task()`, which services USB;
- feeding received serial bytes to the protocol parser;
- `gp_app_task()`;
- feeding the watchdog.

`gp_app_task()`:
- samples inputs at the poll rate (1 kHz by default) and runs smoothing,
  debouncing and axis processing;
- sends a HID report at the report rate;
- renders LED frames: every 400 µs with dithering, every 2 ms without,
  and never faster than the longest chain can be sent and latched
  (30 µs per LED + 300 µs).

## Settings storage

Two 4 KiB flash sectors ("slots") sit at the end of the first 2 MiB of
flash. A save works like this:
1. It writes a complete record into the slot that does **not** hold the
   newest valid record. A record is a header with a sequence number, the
   format versions and CRCs, followed by the configuration and the
   calibration.
2. It reads the record back to verify it.

At boot the newest slot whose CRCs are valid wins. If power is lost during
a save, the previous record is still intact. The unit tests cut the write
at every 8-byte position to prove this.

What happens to stored data at boot:
- **Firmware updates** keep both slots, because a UF2 only writes the
  firmware area.
- **Format check:** records are only used if their format version matches
  this firmware. A future version adds a migration in `gp_storage_load`.
- **Validation:** the loaded configuration is validated against this
  firmware's rules. If it fails, defaults are used and the configurator is
  told (`GP_HELLO_CONFIG_DEFAULTED`).
- **Build check:** the post-link step fails the build if the firmware
  image would reach the storage area.

## Validation

`gp_config_validate()` checks everything the firmware relies on:
- counts against both the schema capacity and the chip;
- every pin's role (axes need ADC pins) and that pins are unique across
  axes, buttons, LED pins and the calibration button;
- LED chains: 1..64 LEDs per pin, 64 in total, and a single LED on the
  board's built-in LED pin;
- numeric ranges, and that the deadzone sum stays ≤ 90 %;
- strings: printable ASCII, NUL terminated, zero padded;
- HID usages: unique axes, at most two Slider/Dial;
- stick groups: two different, enabled axes that belong to no other
  stick;
- unknown flag bits and non-zero reserved bytes.

Every problem gets a field ID, an index and a message, at most 16 per
response. The unit tests fuzz it with 20 000 mutated configurations under
ASan/UBSan.

## Watchdog and safe mode

The watchdog runs with a 1 s timeout. Flash operations feed it before and
after erasing. Watchdog scratch registers 0 and 1 count consecutive
watchdog resets; they survive a watchdog reset but not a power cycle. The
counter is cleared after 10 s of healthy running. After 3 consecutive
watchdog resets the device starts in **safe mode**: defaults are active,
the stored configuration is left untouched, the status LED breathes red,
and the configurator shows a banner. Saving a configuration or a factory
reset ends safe mode.

## Calibration

- **Axes:** an axis is updated only if it was moved through at least 512
  ADC counts (about 12 % of the range). Its centre is the average of the
  rest positions at the start and the end, or the start position if they
  differ by more than about 3 % of the range.
- **Sticks:** the user pushes **up** and then rotates **clockwise**. The
  recogniser works like this:
  1. The furthest point of the initial push, inside a cone anchored to
     the push direction, is "up".
  2. Leaving that cone near the edge starts the rotation. The signed
     angle swept is accumulated.
  3. 350° completes the gesture. The sign of the sweep tells whether the
     wiring is mirrored, so "right" is "up" rotated the way the user
     turned.
  4. "Up" and "right" are snapped to the raw axes. The snap must be within
     35° (it never is for nonsense input). This yields swap, flip-H and
     flip-V.
  5. Returning to the centre starts a new attempt; the last completed
     attempt counts.

  The stick's path is recorded at about 450 points per turn. At *Finish*
  the gesture is evaluated against the final rest position, falling back
  to the start position. It therefore doesn't matter if the stick was
  deflected when calibration started.

  The furthest deflection in each of 32 raw-angle bins becomes the radial
  map. Gaps of up to 3 bins are interpolated; more than that means the
  calibration is incomplete.
- **Results:** a stick that was touched but not completed is stored as
  uncalibrated (neutral defaults). An untouched stick keeps its previous
  calibration. Calibration records are keyed by GPIO pins, not by axis
  slots, so reordering axes in the configuration keeps them.

## Stick processing

The raw offset from the calibrated centre is divided by the interpolated
radial maximum for its angle (in the raw frame). That gives the deflection
0..1, which then goes through the centre/edge deadzones. The orientation
from calibration, plus the optional user adjustments, maps it to
horizontal/up. *Square* output scales the direction so that diagonals
reach the corners.

## LEDs

Each LED pin drives a chain of WS2812 LEDs, one PIO state machine and one
DMA channel per pin. LEDs are numbered across the pins in order, and
every input has a 64-bit LED mask.

Each input adds its colour to the LEDs in its mask:
- **Buttons** light their LEDs fully while pressed.
- **Axes** depend on their effect:
  - *Glow*: every LED in proportion to the square of the deflection.
  - *Bar*: the LEDs are segments of a line; the bar covers the part
    between the rest position and the current one (centred axes start
    in the middle). Each LED shows how much of its segment is covered.
  - *Dot*: a point at the axis position, split between the two nearest
    LEDs so it moves smoothly.

  Bar and Dot are applied to the selected LEDs of each LED pin
  separately. A pin with only one selected LED gets Glow, so a chain and
  a single indicator LED can follow the same axis.

The status LEDs get their pattern on top. The sum is clamped, scaled by
the brightness and converted to 16-bit.

- **Dithering on:** ordered temporal dithering. The low 8 bits are
  compared with a counter that advances by 53 every frame (400 µs), so
  over 256 frames each level between two 8-bit steps is shown in the
  right proportion: 256× finer steps.
- **Dithering off:** the upper 8 bits.

The board's built-in LED pin (`GP_BOARD_BUILTIN_LED_PIN`, GPIO 22 in
`boards/rp2350_generic.h`) is reported to the configurator and is always a
single LED.

## Sleep

The device counts the host as asleep when USB is suspended, or when it has
not been enumerated for 5 s (the PC is off but USB still has power). While
asleep, the LEDs go dark, input processing pauses and the loop idles. The
option *Stay lit while the PC sleeps* keeps everything running. Remote
wake-up is not offered, so the device never wakes the PC.

## USB

- **Device class:** Miscellaneous / IAD (EF/02/01), the correct class for
  a CDC + HID composite device.
- **Configurable identity:** VID, PID, manufacturer, product and serial
  are configurable. An empty serial means the chip's unique ID.
- **HID report:** button bits, then signed 16-bit axes. The report
  descriptor is generated from the configuration.
- **Power and version:** `bcdDevice` carries the firmware's major.minor
  version. The declared current is 100 mA + 60 mA per LED, capped at
  500 mA.
