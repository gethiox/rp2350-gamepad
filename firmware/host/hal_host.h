// SPDX-License-Identifier: MIT
//
// hal_host.h - controls for the host (PC) implementation of the HAL, used
// by the unit tests and the simulator.

#ifndef GP_HAL_HOST_H
#define GP_HAL_HOST_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include "gp_protocol.h"

typedef struct {
	// platform
	bool rp2350b; // 48 GPIOs / 8 ADC channels instead of 30 / 4

	// time: when manual_time is set, hal_time_us() returns `time_us`
	bool	 manual_time;
	uint32_t time_us;

	// inputs (index = GPIO for buttons, ADC channel for axes)
	bool	 pin_low[64];
	uint16_t adc[8];

	// USB
	bool mounted;
	bool suspended;
	bool hid_ready;

	// outputs
	uint8_t	 led_rgb[GP_MAX_LEDS][3]; // all chains, in LED numbering order
	uint8_t	 led_pin[GP_MAX_LED_PINS];
	uint8_t	 led_count[GP_MAX_LED_PINS];
	uint8_t	 led_first[GP_MAX_LED_PINS];
	uint32_t led_frames; // hal_led_write calls
	uint8_t	 last_report[64];
	uint16_t last_report_len;
	uint8_t	 last_report_id;
	uint32_t reports_sent;

	// CDC output buffer (device -> host)
	uint8_t cdc_out[1 << 16];
	size_t	cdc_out_len;
	size_t	cdc_space; // simulated free space in the USB TX FIFO

	// storage
	uint8_t flash[2][4096];
	int		fail_write_after; // >= 0: next write is cut off after this many bytes (power loss)
	bool	erase_fails;
	int		writes;

	// system
	bool reboot_requested;
	void (*on_reboot)(void); // optional (simulator)
	void (*on_flash_change)(void);
	void (*on_sim_input)(const gp_sim_input_t* in);
} hal_host_t;

extern hal_host_t hal_host;

// Resets everything except flash contents.
void hal_host_reset(void);
// Resets flash to the erased state (0xFF).
void hal_host_erase_flash(void);

#endif
