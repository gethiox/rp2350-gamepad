// SPDX-License-Identifier: MIT
//
// hal.h - the small hardware interface the portable core relies on.
// Implemented by src/rp2350/hal_rp2350.c on the device and by
// host/hal_host.c for the simulator and unit tests.

#ifndef GP_HAL_H
#define GP_HAL_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include "gp_protocol.h"

// --- platform description ---
// Fills the platform part of the limits (pin masks, counts).
void hal_describe_platform(gp_limits_t* limits);
const char* hal_board_name(void);
void hal_chip_id(uint8_t out[8]);

// --- time ---
uint32_t hal_time_us(void);

// --- GPIO / ADC ---
void	 hal_button_init(uint8_t pin); // input with pull-up
bool	 hal_button_pressed(uint8_t pin); // true when pulled low
void	 hal_adc_init(uint8_t pin);
uint16_t hal_adc_read(uint8_t pin); // 0..4095
int		 hal_adc_channel(uint8_t pin); // ADC channel index or -1

// --- LEDs ---
// LED pin `index` drives a WS2812 chain of `count` LEDs on GPIO `pin`.
void hal_led_init(uint8_t index, uint8_t pin, uint8_t count);
// Starts sending one frame (rgb[0] = first LED of the chain). Does not
// block; the caller keeps frames GP_LED_LATCH_US apart (gp_led_frame_us).
void hal_led_write(uint8_t index, const uint8_t (*rgb)[3], uint8_t count);

// --- persistent storage (two sectors) ---
#define HAL_STORAGE_SECTOR_SIZE 4096u
const uint8_t* hal_storage_read(uint8_t slot); // memory-mapped view of a slot
bool		   hal_storage_write(uint8_t slot, const uint8_t* data, size_t len); // erase + program
bool		   hal_storage_erase(uint8_t slot);

// --- USB ---
bool   hal_usb_mounted(void);
bool   hal_usb_suspended(void);
bool   hal_hid_ready(void);
bool   hal_hid_send(uint8_t report_id, const uint8_t* data, uint16_t len);
size_t hal_cdc_write_available(void);
size_t hal_cdc_write(const uint8_t* data, size_t len);
void   hal_cdc_flush(void);

// --- system ---
void hal_watchdog_feed(void);
// Reboot now (the app delays this itself so responses can be flushed).
void hal_reboot(void);

#ifdef GP_HOST_BUILD
// Simulator only: override inputs (GP_CMD_SIM_INPUT).
void hal_sim_input(const gp_sim_input_t* in);
#endif

#endif
