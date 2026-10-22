// SPDX-License-Identifier: MIT
#include "calib.h"

#include <math.h>
#include <string.h>

#include "axis.h"
#include "config.h"

#define PI_F  3.14159265358979323846f
#define DEG(x) ((x) * PI_F / 180.0f)

static float wrap_angle(float a) {
	while (a > PI_F)
		a -= 2.0f * PI_F;
	while (a < -PI_F)
		a += 2.0f * PI_F;
	return a;
}

static bool group_usable(const gp_config_t* cfg, int g) {
	const gp_group_cfg_t* gr = &cfg->groups[g];
	return gr->enabled && gr->axis_h < cfg->num_axes && gr->axis_v < cfg->num_axes && gr->axis_h != gr->axis_v;
}

// ---------------------------------------------------------------------------
// Gesture recogniser
// ---------------------------------------------------------------------------

void gp_gesture_step(gp_gesture_t* g, float dx, float dy) {
	float r = sqrtf(dx * dx + dy * dy);
	if (r < GP_CAL_TOUCH_R) {
		if (r < GP_CAL_TOUCH_R * 0.5f && g->state != GP_CALGRP_IDLE) {
			// Back at the centre: the next push starts a new attempt.
			g->state = GP_CALGRP_IDLE;
			g->sweep = 0;
		}
		g->prev_valid = false;
		return;
	}
	float ang = atan2f(dy, dx);
	switch (g->state) {
	case GP_CALGRP_IDLE:
		g->touched = true;
		g->state   = GP_CALGRP_UP;
		g->ux	   = dx;
		g->uy	   = dy;
		g->ur	   = r;
		g->anchor  = ang;
		break;
	case GP_CALGRP_UP: {
		// The cone is anchored to the push direction, not to the current
		// estimate: otherwise noise along the rim would let "up" creep
		// around the circle.
		float diff = wrap_angle(ang - g->anchor);
		if (r > g->ur * 1.15f) {
			// Still clearly pushing outwards (maybe on a curved path).
			g->ux	  = dx;
			g->uy	  = dy;
			g->ur	  = r;
			g->anchor = ang;
		} else if (fabsf(diff) <= DEG(GP_CAL_UP_CONE)) {
			if (r > g->ur) { // furthest point of the push within the cone
				g->ux = dx;
				g->uy = dy;
				g->ur = r;
			}
		} else if (r >= 0.7f * g->ur) {
			// Left the cone near the edge: rotation has started. Count the
			// angle travelled since "up".
			g->state	  = GP_CALGRP_ROTATING;
			g->sweep	  = wrap_angle(ang - atan2f(g->uy, g->ux));
			g->prev_angle = ang;
			g->prev_valid = true;
		}
		break;
	}
	case GP_CALGRP_ROTATING:
	case GP_CALGRP_DONE:
		if (r >= 0.5f * g->ur) {
			if (g->prev_valid)
				g->sweep += wrap_angle(ang - g->prev_angle);
			g->prev_angle = ang;
			g->prev_valid = true;
		} else {
			g->prev_valid = false; // near the centre the angle is meaningless
		}
		g->state = fabsf(g->sweep) >= DEG(GP_CAL_SWEEP_DONE) ? GP_CALGRP_DONE : GP_CALGRP_ROTATING;
		if (g->state == GP_CALGRP_DONE) {
			g->have_done  = true;
			g->done_ux	  = g->ux;
			g->done_uy	  = g->uy;
			g->done_sweep = g->sweep;
		}
		break;
	}
}

// ---------------------------------------------------------------------------
// Session
// ---------------------------------------------------------------------------

void gp_calib_start(gp_calib_t* s, const gp_config_t* cfg, const uint16_t* raw) {
	memset(s, 0, sizeof(*s));
	s->active = true;
	for (int i = 0; i < cfg->num_axes && i < GP_MAX_AXES; i++) {
		s->ch[i].start = s->ch[i].seen_min = s->ch[i].seen_max = raw[i];
	}
	for (int g = 0; g < GP_MAX_GROUPS; g++) {
		gp_cal_group_session_t* gs = &s->grp[g];
		if (!group_usable(cfg, g))
			continue;
		gs->active = true;
		gs->ax_h   = cfg->groups[g].axis_h;
		gs->ax_v   = cfg->groups[g].axis_v;
	}
}

static void record(gp_cal_group_session_t* gs, uint16_t a, uint16_t b) {
	if (gs->trace_n > 0) {
		uint16_t last = (uint16_t)((gs->trace_head + GP_CAL_TRACE - 1) % GP_CAL_TRACE);
		int		 da = (int)a - gs->trace_a[last], db = (int)b - gs->trace_b[last];
		if (da * da + db * db < GP_CAL_TRACE_STEP * GP_CAL_TRACE_STEP)
			return;
	}
	gs->trace_a[gs->trace_head] = a;
	gs->trace_b[gs->trace_head] = b;
	gs->trace_head				= (uint16_t)((gs->trace_head + 1) % GP_CAL_TRACE);
	if (gs->trace_n < GP_CAL_TRACE)
		gs->trace_n++;
}

void gp_calib_update(gp_calib_t* s, const gp_config_t* cfg, const uint16_t* raw) {
	if (!s->active)
		return;
	for (int i = 0; i < cfg->num_axes && i < GP_MAX_AXES; i++) {
		if (raw[i] < s->ch[i].seen_min)
			s->ch[i].seen_min = raw[i];
		if (raw[i] > s->ch[i].seen_max)
			s->ch[i].seen_max = raw[i];
	}
	for (int g = 0; g < GP_MAX_GROUPS; g++) {
		gp_cal_group_session_t* gs = &s->grp[g];
		if (!gs->active)
			continue;
		uint16_t a = raw[gs->ax_h], b = raw[gs->ax_v];
		record(gs, a, b);
		float dx = (float)a - (float)s->ch[gs->ax_h].start;
		float dy = (float)b - (float)s->ch[gs->ax_v].start;
		gp_gesture_step(&gs->live, dx, dy);
		float r = sqrtf(dx * dx + dy * dy);
		if (r >= GP_CAL_TOUCH_R) {
			int bin = gp_radial_bin(atan2f(dy, dx));
			if (r > gs->bins_live[bin])
				gs->bins_live[bin] = (uint16_t)(r > 65535.0f ? 65535.0f : r);
		}
	}
}

bool gp_calib_analyze(const gp_cal_group_session_t* gs, float ca, float cb, uint8_t* orient, uint16_t* radial,
					  bool* touched) {
	gp_gesture_t g;
	memset(&g, 0, sizeof(g));
	uint16_t bins[GP_RADIAL_BINS];
	memset(bins, 0, sizeof(bins));
	uint16_t first = (uint16_t)((gs->trace_head + GP_CAL_TRACE - gs->trace_n) % GP_CAL_TRACE);
	for (uint16_t k = 0; k < gs->trace_n; k++) {
		uint16_t i	= (uint16_t)((first + k) % GP_CAL_TRACE);
		float	 dx = (float)gs->trace_a[i] - ca;
		float	 dy = (float)gs->trace_b[i] - cb;
		gp_gesture_step(&g, dx, dy);
		float r = sqrtf(dx * dx + dy * dy);
		if (r >= GP_CAL_TOUCH_R) {
			int b = gp_radial_bin(atan2f(dy, dx));
			if (r > bins[b])
				bins[b] = (uint16_t)(r > 65535.0f ? 65535.0f : r);
		}
	}
	*touched = g.touched;
	return g.have_done && gp_calib_orientation(g.done_ux, g.done_uy, g.done_sweep, orient) &&
		   gp_calib_fill_radial(bins, radial);
}

bool gp_calib_orientation(float ux, float uy, float sweep, uint8_t* orient) {
	float ul = sqrtf(ux * ux + uy * uy);
	if (!(ul > 0.0f) || sweep == 0.0f)
		return false;
	// Right = up rotated 90 degrees in the direction the user turned
	// (clockwise from the user's point of view). A negative raw sweep means
	// the raw frame has the same handedness as the user's frame.
	float rx, ry;
	if (sweep < 0) {
		rx = uy;
		ry = -ux;
	} else {
		rx = -uy;
		ry = ux;
	}
	uint8_t o = 0;
	if (fabsf(uy) >= fabsf(ux)) {
		if (fabsf(uy) < GP_CAL_SNAP_COS * ul)
			return false;
		if (uy < 0)
			o |= GP_ORIENT_INVERT_V;
		if (rx < 0)
			o |= GP_ORIENT_INVERT_H;
	} else {
		if (fabsf(ux) < GP_CAL_SNAP_COS * ul)
			return false;
		o |= GP_ORIENT_SWAP;
		if (ux < 0)
			o |= GP_ORIENT_INVERT_V;
		if (ry < 0)
			o |= GP_ORIENT_INVERT_H;
	}
	*orient = o;
	return true;
}

bool gp_calib_fill_radial(const uint16_t* bins, uint16_t* out) {
	uint16_t peak = 0;
	for (int b = 0; b < GP_RADIAL_BINS; b++)
		if (bins[b] > peak)
			peak = bins[b];
	if (peak < GP_CAL_TOUCH_R)
		return false;
	uint16_t thr = (uint16_t)(peak / 2);
	bool	 ok[GP_RADIAL_BINS];
	int		 valid = 0;
	for (int b = 0; b < GP_RADIAL_BINS; b++) {
		ok[b] = bins[b] >= thr;
		valid += ok[b];
	}
	if (valid < GP_RADIAL_BINS - 2 * GP_CAL_MAX_GAP)
		return false;

	for (int b = 0; b < GP_RADIAL_BINS; b++) {
		if (ok[b]) {
			out[b] = bins[b];
			continue;
		}
		// Nearest valid neighbours on both sides (wrapping around the circle).
		int lo = b - 1, hi = b + 1;
		while (!ok[(lo + GP_RADIAL_BINS) % GP_RADIAL_BINS])
			lo--;
		while (!ok[hi % GP_RADIAL_BINS])
			hi++;
		if (hi - lo - 1 > GP_CAL_MAX_GAP)
			return false;
		float t	 = (float)(b - lo) / (float)(hi - lo);
		float v0 = bins[(lo + GP_RADIAL_BINS) % GP_RADIAL_BINS];
		float v1 = bins[hi % GP_RADIAL_BINS];
		out[b]	 = (uint16_t)(v0 + (v1 - v0) * t + 0.5f);
	}
	for (int b = 0; b < GP_RADIAL_BINS; b++) {
		if (out[b] < GP_RADIAL_MIN)
			out[b] = GP_RADIAL_MIN;
		if (out[b] > GP_RADIAL_MAX)
			out[b] = GP_RADIAL_MAX;
	}
	return true;
}

static uint16_t choose_center(uint16_t start, uint16_t end, uint16_t min, uint16_t max) {
	uint16_t span = (uint16_t)(max - min);
	uint16_t tol  = span / 33 > 32 ? span / 33 : 32; // ~3 %
	uint32_t c	  = (uint32_t)(start > end ? start - end : end - start) <= tol ? ((uint32_t)start + end) / 2 : start;
	uint16_t lo	  = (uint16_t)(min + span / 20), hi = (uint16_t)(max - span / 20);
	if (c < lo)
		c = lo;
	if (c > hi)
		c = hi;
	return (uint16_t)c;
}

void gp_calib_finish(gp_calib_t* s, const gp_config_t* cfg, const uint16_t* raw, gp_calibration_t* cal,
					 gp_cal_result_t* res, int (*adc_channel_of)(uint8_t pin)) {
	memset(res, 0, sizeof(*res));
	if (!s->active)
		return;

	for (int i = 0; i < cfg->num_axes && i < GP_MAX_AXES; i++) {
		gp_cal_ch_session_t* cs = &s->ch[i];
		int					 ch = adc_channel_of(cfg->axes[i].pin);
		if (ch < 0 || ch >= GP_MAX_AXES)
			continue;
		if ((uint16_t)(cs->seen_max - cs->seen_min) < GP_CAL_MIN_SPAN) {
			res->channel[i] = GP_CALRES_UNCHANGED;
			continue;
		}
		gp_cal_channel_t* c = &cal->channels[ch];
		c->calibrated		= 1;
		c->reserved			= 0;
		c->min				= cs->seen_min;
		c->max				= cs->seen_max;
		c->center			= choose_center(cs->start, raw[i], c->min, c->max);
		res->channel[i]		= GP_CALRES_UPDATED;
	}

	for (int g = 0; g < GP_MAX_GROUPS; g++) {
		gp_cal_group_session_t* gs = &s->grp[g];
		if (!gs->active || !gs->live.touched) {
			res->group[g] = GP_CALRES_UNCHANGED;
			continue;
		}
		uint8_t pin_a = cfg->axes[gs->ax_h].pin, pin_b = cfg->axes[gs->ax_v].pin;
		// Drop stale records for the same pair stored in other slots.
		for (int k = 0; k < GP_MAX_GROUPS; k++) {
			gp_cal_group_t* o = &cal->groups[k];
			if (k != g && o->calibrated &&
				((o->pin_a == pin_a && o->pin_b == pin_b) || (o->pin_a == pin_b && o->pin_b == pin_a)))
				o->calibrated = 0;
		}
		// Rest position: where the stick is now (it has been let go), or
		// where it was when the session started.
		float centers[2][2] = { { raw[gs->ax_h], raw[gs->ax_v] },
								{ s->ch[gs->ax_h].start, s->ch[gs->ax_v].start } };
		uint16_t radial[GP_RADIAL_BINS];
		uint8_t	 orient = 0;
		int		 used	= -1;
		for (int k = 0; k < 2 && used < 0; k++) {
			bool touched;
			if (gp_calib_analyze(gs, centers[k][0], centers[k][1], &orient, radial, &touched))
				used = k;
		}
		gp_cal_group_t* out = &cal->groups[g];
		memset(out, 0, sizeof(*out));
		out->pin_a = pin_a;
		out->pin_b = pin_b;
		if (used < 0) {
			// Touched but incomplete: treat as uncalibrated.
			out->calibrated = 0;
			out->center_a	= 2048;
			out->center_b	= 2048;
			for (int b = 0; b < GP_RADIAL_BINS; b++)
				out->radial[b] = 2047;
			res->group[g] = GP_CALRES_FAILED;
			continue;
		}
		out->calibrated = 1;
		out->orient		= orient;
		out->center_a	= (uint16_t)centers[used][0];
		out->center_b	= (uint16_t)centers[used][1];
		memcpy(out->radial, radial, sizeof(radial));
		res->group[g] = GP_CALRES_UPDATED;
	}
	s->active = false;
}

void gp_calib_cancel(gp_calib_t* s) { s->active = false; }

void gp_calib_telemetry(const gp_calib_t* s, const gp_config_t* cfg, gp_telemetry_t* t) {
	for (int i = 0; i < cfg->num_axes && i < GP_MAX_AXES; i++) {
		const gp_cal_ch_session_t* cs	= &s->ch[i];
		uint16_t				   span = (uint16_t)(cs->seen_max - cs->seen_min);
		t->cal_min[i]					= cs->seen_min;
		t->cal_max[i]					= cs->seen_max;
		t->cal_center[i]				= cs->start;
		t->cal_channel_state[i]			= span >= GP_CAL_MIN_SPAN ? GP_CALCH_OK : (span >= 64 ? GP_CALCH_MOVING : GP_CALCH_IDLE);
	}
	for (int g = 0; g < GP_MAX_GROUPS; g++) {
		const gp_cal_group_session_t* gs = &s->grp[g];
		if (!gs->active)
			continue;
		const gp_gesture_t* ge = &gs->live;
		t->cal_group_state[g]  = ge->have_done ? GP_CALGRP_DONE : ge->state;
		float deg			   = fabsf(ge->have_done ? ge->done_sweep : ge->sweep) * 180.0f / PI_F;
		t->cal_group_sweep[g]  = (uint16_t)(deg > 65535.0f ? 65535.0f : deg);
		uint16_t peak		   = 0;
		for (int b = 0; b < GP_RADIAL_BINS; b++)
			if (gs->bins_live[b] > peak)
				peak = gs->bins_live[b];
		uint8_t n = 0;
		for (int b = 0; b < GP_RADIAL_BINS; b++)
			if (peak && gs->bins_live[b] >= peak / 2)
				n++;
		t->cal_group_coverage[g] = n;
	}
}
