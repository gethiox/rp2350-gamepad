# Building from source

```
firmware/
  src/core/      portable firmware logic (no hardware access)
  src/rp2350/    RP2350 drivers, USB descriptors, main loop
  boards/        generic RP2350A board definition
  host/          PC build of the core: unit tests, simulator, layout dump
configurator/    Go + raylib desktop app
protocol/        layout.json - the binary contract, generated from C
scripts/         reproducible firmware build, packaging, CI helpers
packaging/       udev rule, macOS bundle info, release notes
```

## Firmware

### Reproducible build (Linux x86-64)

```sh
scripts/build-firmware.sh          # -> dist/rp2350-gamepad-<version>.uf2
```

This downloads the pinned Pico SDK and Arm toolchain into `.cache/` the
first time. It needs `git`, `cmake`, `ninja`, `python3`, `curl` and `tar`;
if any are missing it lists all of them at once.

### With your own toolchain (any OS)

You need the [Pico SDK](https://github.com/raspberrypi/pico-sdk) 2.2.0
(with the `lib/tinyusb` submodule) and an `arm-none-eabi` GCC. The
Raspberry Pi VS Code extension installs both.

```sh
export PICO_SDK_PATH=/path/to/pico-sdk
cmake -S firmware -B build/firmware -G Ninja -DCMAKE_BUILD_TYPE=Release
ninja -C build/firmware
# build/firmware/rp2350-gamepad.uf2, reset-settings.uf2
```

The build fails if the firmware image would grow into the settings
storage (the last 8 KiB of the first 2 MiB of flash).

### Other boards

`boards/rp2350_generic.h` works for RP2350A boards with at least 2 MB of
flash. It deliberately defines no default UART/LED pins, so every GPIO is
free. `GP_BOARD_BUILTIN_LED_PIN` names the board's own WS2812 (GPIO 22);
the configurator labels it "built-in" and keeps it at one LED. Set it to
`0xFF` for boards without one. If your board has GPIOs wired to on-board parts, list them in
`GP_BOARD_RESERVED_PINS` (a 64-bit mask, see `hal_rp2350.c`) and the
configurator will not offer them.

## Firmware tests and simulator (any PC, no hardware)

The portable core is also compiled for the PC:

```sh
cmake -S firmware/host -B build/host -G Ninja
cmake --build build/host
ctest --test-dir build/host --output-on-failure
```

- `gp_tests` covers validation, flash storage (including simulated power
  loss at every byte), calibration in all 8 wiring orientations with
  noise, framing, fuzzed commands and LED dithering. It runs under
  AddressSanitizer and UndefinedBehaviorSanitizer.
- `rp2350-gamepad-sim` runs the complete firmware logic on your PC and
  speaks the device protocol on stdin/stdout. It generates stick, axis and
  button motion, including the calibration gesture.
- `layout_dump` prints the binary layout of every protocol structure. The
  `layout_up_to_date` test fails if `protocol/layout.json` doesn't match
  the C headers.

## Configurator

Requirements:
- Go 1.25 or newer, with cgo (a C compiler).
- Linux: the raylib dependencies (`scripts/ci-linux-deps.sh` installs them
  on Ubuntu/Debian: X11, Wayland, OpenGL headers).
- Windows: MinGW-w64 GCC in `PATH`.
- macOS: the Xcode command line tools.

```sh
cd configurator
go build ./cmd/rp2350-gamepad-configurator
go test ./...
```

On Linux you can build an X11-only binary (no Wayland headers needed) with
`go build -tags x11 …`.

### Running against the simulator

```sh
./rp2350-gamepad-configurator -simulator ../build/host/rp2350-gamepad-sim \
    -simulator-args "--flash /tmp/sim-flash.bin"
```

Simulator options: `--rp2350b` emulates the 48-pin chip (8 axes), and
`--orient N` changes how stick 1 is wired (to try out calibration).

### Go ⟷ firmware integration tests

`internal/device` has tests that drive the real firmware core through the
simulator: handshake, validation errors, save → reboot → verify, factory
reset, calibration and LED preview. They run when the simulator path is
given:

```sh
RP2350_GAMEPAD_SIM=$PWD/../build/host/rp2350-gamepad-sim go test ./internal/device/
```

`internal/proto` checks every Go structure and constant against
`protocol/layout.json`. Together these tests guarantee that the
configurator and the firmware from the same commit agree, byte for byte.

## Releases

Push a tag such as `v1.0.0`. `.github/workflows/release.yml` then:
- runs all tests;
- builds the firmware reproducibly;
- builds the configurator for Linux (amd64, arm64), Windows (amd64) and
  macOS (universal);
- writes `SHA256SUMS`;
- creates signed build provenance for every file;
- publishes the GitHub release.

Tags containing `-` (for example `v1.1.0-rc1`) are marked as
pre-releases.
