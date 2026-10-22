// SPDX-License-Identifier: MIT
//
// Generic RP2350A board with at least 2 MB of QSPI flash.
//
// Unlike the Pico 2 board file this defines no default UART, LED, I2C or
// SPI pins, so the SDK never touches a GPIO on its own: every pin is free
// for buttons, axes and LEDs. GPIO 22 is marked as the board's built-in
// WS2812 (see GP_BOARD_BUILTIN_LED_PIN). Works on Pico 2 style boards and on the many
// third-party "RP2350 USB" / "RP2350 Zero" boards.

// pico_cmake_set PICO_PLATFORM=rp2350

#ifndef _BOARDS_RP2350_GENERIC_H
#define _BOARDS_RP2350_GENERIC_H

pico_board_cmake_set(PICO_PLATFORM, rp2350)

#define PICO_RP2350A 1

// Works with W25Q-series and compatible flash chips (common on RP2350 boards).
#define PICO_BOOT_STAGE2_CHOOSE_W25Q080 1
#ifndef PICO_FLASH_SPI_CLKDIV
#define PICO_FLASH_SPI_CLKDIV 2
#endif

// Only the first 2 MB are assumed; the settings storage lives inside them,
// so the firmware works on any flash size >= 2 MB.
pico_board_cmake_set_default(PICO_FLASH_SIZE_BYTES, (2 * 1024 * 1024))
#ifndef PICO_FLASH_SIZE_BYTES
#define PICO_FLASH_SIZE_BYTES (2 * 1024 * 1024)
#endif

// The board's own WS2812. It is always a single LED; the configurator labels
// it "built-in". Set to 0xFF (GP_PIN_NONE) for boards without one.
#ifndef GP_BOARD_BUILTIN_LED_PIN
#define GP_BOARD_BUILTIN_LED_PIN 22
#endif

pico_board_cmake_set_default(PICO_RP2350_A2_SUPPORTED, 1)
#ifndef PICO_RP2350_A2_SUPPORTED
#define PICO_RP2350_A2_SUPPORTED 1
#endif

#endif
