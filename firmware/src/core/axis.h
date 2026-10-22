// SPDX-License-Identifier: MIT
//
// axis.h - turning calibrated ADC values into -1..1 outputs.

#ifndef GP_AXIS_H
#define GP_AXIS_H

#include <stdint.h>

#include "gp_protocol.h"

// Effective calibration of one channel (defaults when uncalibrated or broken).
typedef struct {
	uint16_t min, center, max;
} gp_span_t;

gp_span_t gp_channel_span(const gp_cal_channel_t* ch);

// Rescales a magnitude 0..1 through centre and outer deadzones (fractions).
float gp_deadzone(float m, float center_dz, float outer_dz);

// Centred axis (stick): -1 at min, 0 at centre, +1 at max.
float gp_axis_centered(uint16_t raw, gp_span_t s, float center_dz, float outer_dz);

// Linear axis (throttle/slider): -1 at min .. +1 at max.
// low_dz applies at the min end, high_dz at the max end.
float gp_axis_linear(uint16_t raw, gp_span_t s, float low_dz, float high_dz);

// Interpolated maximum radius for a raw angle (radians, -pi..pi).
float gp_radial_lookup(const uint16_t radial[GP_RADIAL_BINS], float angle);
int	  gp_radial_bin(float angle);

// Stick group. dx/dy are raw offsets from the calibrated centre in the
// calibration's raw frame. Produces horizontal (right = +) and vertical
// (up = +) outputs in -1..1.
typedef struct {
	const uint16_t* radial; // NULL = uncalibrated (uniform radius)
	uint8_t			orient; // GP_ORIENT_* from calibration
	uint8_t			user_flags; // GP_GROUP_* from config
	uint8_t			shape;
	float			center_dz;
	float			outer_dz;
} gp_group_params_t;

void gp_group_compute(float dx, float dy, const gp_group_params_t* p, float* out_h, float* out_v);

// -1..1 -> HID value
int16_t gp_axis_to_hid(float v);

#endif
