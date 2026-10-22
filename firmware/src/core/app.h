// SPDX-License-Identifier: MIT
//
// app.h - the portable firmware application (everything except USB
// descriptors and hardware drivers). Driven by the platform main loop.

#ifndef GP_APP_H
#define GP_APP_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#include "gp_protocol.h"

typedef struct {
	uint8_t reset_reason; // GP_RESET_*
	uint8_t crash_count;  // consecutive watchdog resets
	bool	safe_mode;	  // ignore stored config
	bool	simulator;
} gp_boot_info_t;

#define GP_SAFE_MODE_CRASHES 3 // consecutive watchdog resets that trigger safe mode
#define GP_SLEEP_UNMOUNTED_MS 5000

// Loads storage, validates, initialises inputs and LEDs.
void gp_app_init(const gp_boot_info_t* boot);
// One iteration of the main loop.
void gp_app_task(void);
// Bytes received on the configuration serial port.
void gp_app_cdc_rx(const uint8_t* data, size_t len);

// For the USB layer (valid after gp_app_init).
const gp_config_t* gp_app_config(void);
// True while the device considers the host asleep/off (LEDs dark, inputs paused).
bool gp_app_sleeping(void);
// True while a calibration session runs.
bool gp_app_calibrating(void);

#endif
