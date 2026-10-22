// SPDX-License-Identifier: MIT
//
// config.h - defaults, limits and validation of configuration and
// calibration. The firmware is the single source of truth for what is
// valid; the configurator asks the device (VALIDATE_CONFIG) and only
// mirrors the numeric ranges from gp_limits_t to bound its widgets.

#ifndef GP_CONFIG_H
#define GP_CONFIG_H

#include <stdbool.h>

#include "gp_protocol.h"

#define GP_DZ_SUM_MAX		900	 // centre + outer deadzone, permille
#define GP_CAL_MIN_SPAN		512	 // ADC counts an axis must travel to be (re)calibrated
#define GP_CAL_IMPORT_SPAN	256	 // minimum span accepted when importing calibration
#define GP_RADIAL_MIN		50	 // smallest valid radial entry
#define GP_RADIAL_MAX		5800 // > sqrt(2) * 4095
#define GP_RESET_HOLD_GAP	2000 // reset hold must exceed calibration hold by this

void gp_config_defaults(gp_config_t* cfg);
void gp_calibration_defaults(gp_calibration_t* cal);
void gp_limits_build(gp_limits_t* lim); // schema + platform + ranges

// Validation. Returns the number of errors written to `out` (0 = valid).
// `out` may be NULL when only the verdict is needed.
int gp_config_validate(const gp_config_t* cfg, const gp_limits_t* lim, gp_error_list_t* out);
int gp_calibration_validate(const gp_calibration_t* cal, const gp_limits_t* lim, gp_error_list_t* out);

// Helpers shared with the rest of the core.
static inline bool gp_mask_has(const uint32_t mask[2], unsigned bit) {
	return bit < 64 && (mask[bit / 32] >> (bit % 32)) & 1u;
}
bool gp_usage_allowed(uint8_t usage);
unsigned gp_led_total(const gp_config_t* cfg); // LEDs on all enabled pins (valid configs only)

#endif
