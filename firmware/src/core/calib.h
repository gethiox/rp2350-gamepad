// SPDX-License-Identifier: MIT
//
// calib.h - calibration session.
//
// Standalone axes: an axis is recalibrated only if it was moved through at
// least GP_CAL_MIN_SPAN ADC counts during the session; otherwise its
// previous calibration is kept.
//
// Stick groups: push the stick UP, then rotate it CLOCKWISE for at least
// one full turn. The push defines "up"; the rotation direction tells
// whether the wiring is mirrored, which fixes both swap and inversion.
// Letting the stick return to the centre starts a new attempt, so a false
// start can simply be repeated; the last completed gesture is used.
//
// The path of every stick is recorded (a few hundred points per turn). At
// the end the gesture is evaluated against the final rest position (the
// stick has been let go), falling back to the position at the start, so it
// does not matter whether the stick happened to be deflected at one end.
// A group that was never touched keeps its previous calibration; a group
// that was touched but not completed is marked uncalibrated.

#ifndef GP_CALIB_H
#define GP_CALIB_H

#include <stdbool.h>
#include <stdint.h>

#include "gp_protocol.h"

#define GP_CAL_TOUCH_R	  300.0f // deflection (ADC counts) that counts as "touched"
#define GP_CAL_UP_CONE	  25.0f	 // degrees: leaving this cone around "up" starts rotation
#define GP_CAL_SWEEP_DONE 350.0f // degrees of rotation required
#define GP_CAL_MAX_GAP	  3		 // missing radial bins in a row that can be interpolated
#define GP_CAL_SNAP_COS	  0.819f // cos(35 deg): "up" must be within 35 deg of a raw axis
#define GP_CAL_TRACE	  2048	 // recorded points per stick (ring buffer)
#define GP_CAL_TRACE_STEP 24	 // minimum distance (ADC counts) between recorded points

// The push-up-then-rotate gesture recogniser.
typedef struct {
	uint8_t state; // GP_CALGRP_*
	bool	touched;
	float	ux, uy, ur; // captured "up" vector (furthest point of the push)
	float	anchor;		// direction of the push; "up" may only be refined within a cone around it
	float	prev_angle;
	bool	prev_valid;
	float	sweep; // radians, signed
	bool	have_done;
	float	done_ux, done_uy, done_sweep;
} gp_gesture_t;

void gp_gesture_step(gp_gesture_t* g, float dx, float dy);

typedef struct {
	uint16_t start, seen_min, seen_max;
} gp_cal_ch_session_t;

typedef struct {
	bool		 active; // group enabled and valid when the session started
	uint8_t		 ax_h, ax_v; // axis slots: raw a from ax_h, raw b from ax_v
	gp_gesture_t live;		 // relative to the start position (progress display)
	uint16_t	 trace_a[GP_CAL_TRACE];
	uint16_t	 trace_b[GP_CAL_TRACE];
	uint16_t	 trace_n;	 // valid points (<= GP_CAL_TRACE)
	uint16_t	 trace_head; // next write position
	uint16_t	 bins_live[GP_RADIAL_BINS]; // coverage display
} gp_cal_group_session_t;

typedef struct {
	bool				   active;
	gp_cal_ch_session_t	   ch[GP_MAX_AXES]; // per axis slot
	gp_cal_group_session_t grp[GP_MAX_GROUPS];
} gp_calib_t;

void gp_calib_start(gp_calib_t* s, const gp_config_t* cfg, const uint16_t* raw);
void gp_calib_update(gp_calib_t* s, const gp_config_t* cfg, const uint16_t* raw);
// Applies the session to `cal` (in place). `adc_channel_of(pin)` maps pins
// to calibration channel indices.
void gp_calib_finish(gp_calib_t* s, const gp_config_t* cfg, const uint16_t* raw, gp_calibration_t* cal,
					 gp_cal_result_t* res, int (*adc_channel_of)(uint8_t pin));
void gp_calib_cancel(gp_calib_t* s);
void gp_calib_telemetry(const gp_calib_t* s, const gp_config_t* cfg, gp_telemetry_t* t);

// Exposed for tests: derive orientation flags from the captured up vector
// and rotation direction. Returns false if "up" is too far from a raw axis.
bool gp_calib_orientation(float ux, float uy, float sweep, uint8_t* orient);
// Fills small gaps in a radial map. Returns false if coverage is insufficient.
bool gp_calib_fill_radial(const uint16_t* bins, uint16_t* out);
// Evaluates a recorded path against a rest position.
bool gp_calib_analyze(const gp_cal_group_session_t* gs, float ca, float cb, uint8_t* orient, uint16_t* radial,
					  bool* touched);

#endif
