# Third-party software

rp2350-gamepad is MIT licensed (see `LICENSE`). The release binaries also
contain the following third-party components; their licences require that
these notices are distributed with the binaries.

## Firmware (`.uf2` / `.elf`)

| Component | Licence | Source |
|---|---|---|
| Raspberry Pi Pico SDK 2.2.0 | BSD-3-Clause, © Raspberry Pi (Trading) Ltd. | https://github.com/raspberrypi/pico-sdk |
| TinyUSB (bundled with the SDK) | MIT, © Ha Thach (tinyusb.org) | https://github.com/hathach/tinyusb |
| WS2812 PIO program (`firmware/src/rp2350/ws2812.pio`) | BSD-3-Clause, © Raspberry Pi (Trading) Ltd. | https://github.com/raspberrypi/pico-examples |

## Configurator

| Component | Licence | Source |
|---|---|---|
| raylib (incl. GLFW, stb, miniaudio, …) | zlib, © Ramon Santamaria and contributors | https://github.com/raysan5/raylib |
| raylib-go | zlib, © Milan Nikolic | https://github.com/gen2brain/raylib-go |
| go.bug.st/serial | BSD-3-Clause, © Cristian Maglie | https://github.com/bugst/go-serial |
| golang.org/x/sys, Go standard library | BSD-3-Clause, © The Go Authors | https://go.dev |
| Dosis font | SIL Open Font License 1.1, © The Dosis Project Authors | `configurator/assets/fonts/OFL-Dosis.txt` |
| Space Mono font | SIL Open Font License 1.1, © The Space Mono Project Authors | `configurator/assets/fonts/OFL-SpaceMono.txt` |

The full licence texts are available at the linked sources. The Dosis
weights shipped here are static instances generated from the upstream
variable font (`Dosis[wght].ttf`, google/fonts) with fontTools; as modified
OFL fonts they remain under the OFL.
