// SPDX-License-Identifier: MIT
#include "axis.h"

#include <math.h>

#define PI_F 3.14159265358979323846f

static float clampf(float v, float lo, float hi) { return v < lo ? lo : (v > hi ? hi : v); }

gp_span_t gp_channel_span(const gp_cal_channel_t* ch) {
	gp_span_t s = { 0, 2048, 4095 };
	if (ch && ch->calibrated && ch->min < ch->center && ch->center < ch->max && ch->max <= GP_ADC_MAX) {
		s.min	 = ch->min;
		s.center = ch->center;
		s.max	 = ch->max;
	}
	return s;
}

float gp_deadzone(float m, float center_dz, float outer_dz) {
	if (!(m > center_dz)) // also catches NaN
		return 0.0f;
	float span = 1.0f - center_dz - outer_dz;
	if (span < 0.05f)
		span = 0.05f; // validation keeps the sum <= 90 %, this is a last line of defence
	return clampf((m - center_dz) / span, 0.0f, 1.0f);
}

float gp_axis_centered(uint16_t raw, gp_span_t s, float center_dz, float outer_dz) {
	float t;
	if (raw >= s.center)
		t = (float)(raw - s.center) / (float)(s.max - s.center);
	else
		t = -(float)(s.center - raw) / (float)(s.center - s.min);
	t		= clampf(t, -1.0f, 1.0f);
	float m = gp_deadzone(fabsf(t), center_dz, outer_dz);
	return t < 0 ? -m : m;
}

float gp_axis_linear(uint16_t raw, gp_span_t s, float low_dz, float high_dz) {
	float t = clampf((float)((int)raw - (int)s.min) / (float)(s.max - s.min), 0.0f, 1.0f);
	return gp_deadzone(t, low_dz, high_dz) * 2.0f - 1.0f;
}

int gp_radial_bin(float angle) {
	int b = (int)floorf((angle + PI_F) / (2.0f * PI_F) * (float)GP_RADIAL_BINS);
	if (b < 0)
		b = 0;
	if (b >= GP_RADIAL_BINS)
		b = GP_RADIAL_BINS - 1;
	return b;
}

float gp_radial_lookup(const uint16_t radial[GP_RADIAL_BINS], float angle) {
	// Bin b is centred at b + 0.5; interpolate between neighbouring centres.
	float f	 = (angle + PI_F) / (2.0f * PI_F) * (float)GP_RADIAL_BINS - 0.5f;
	float fl = floorf(f);
	int	  b1 = (int)fl % GP_RADIAL_BINS;
	if (b1 < 0)
		b1 += GP_RADIAL_BINS;
	int	  b2 = (b1 + 1) % GP_RADIAL_BINS;
	float t	 = f - fl;
	return (float)radial[b1] * (1.0f - t) + (float)radial[b2] * t;
}

void gp_group_compute(float dx, float dy, const gp_group_params_t* p, float* out_h, float* out_v) {
	*out_h = 0.0f;
	*out_v = 0.0f;
	float r = sqrtf(dx * dx + dy * dy);
	if (!(r > 0.0f))
		return;

	float max_r = 2047.0f;
	if (p->radial) {
		max_r = gp_radial_lookup(p->radial, atan2f(dy, dx));
		if (max_r < 50.0f)
			max_r = 2047.0f;
	}
	float m = gp_deadzone(r / max_r, p->center_dz, p->outer_dz);
	if (m <= 0.0f)
		return;

	// Map the raw frame to (horizontal, up) using the calibrated orientation.
	float h = dx, v = dy;
	if (p->orient & GP_ORIENT_SWAP) {
		h = dy;
		v = dx;
	}
	if (p->orient & GP_ORIENT_INVERT_H)
		h = -h;
	if (p->orient & GP_ORIENT_INVERT_V)
		v = -v;
	// User adjustments on top.
	if (p->user_flags & GP_GROUP_SWAP) {
		float t = h;
		h		= v;
		v		= t;
	}
	if (p->user_flags & GP_GROUP_INVERT_H)
		h = -h;
	if (p->user_flags & GP_GROUP_INVERT_V)
		v = -v;

	float uh = h / r, uv = v / r;
	if (p->shape == GP_SHAPE_SQUARE) {
		float big = fmaxf(fabsf(uh), fabsf(uv));
		uh /= big;
		uv /= big;
	}
	*out_h = clampf(uh * m, -1.0f, 1.0f);
	*out_v = clampf(uv * m, -1.0f, 1.0f);
}

int16_t gp_axis_to_hid(float v) {
	v = clampf(v, -1.0f, 1.0f);
	if (v != v)
		v = 0.0f;
	return (int16_t)lrintf(v * 32767.0f);
}
