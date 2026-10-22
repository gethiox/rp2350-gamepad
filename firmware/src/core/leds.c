// SPDX-License-Identifier: MIT
#include "leds.h"

#include <string.h>

uint32_t gp_led_frame_us(bool dither, unsigned longest_chain) {
	uint32_t base  = dither ? GP_LED_FRAME_US_DITHER : GP_LED_FRAME_US_PLAIN;
	uint32_t chain = longest_chain * GP_LED_US_PER_LED + GP_LED_LATCH_US;
	return chain > base ? chain : base;
}

void gp_led_dither_next(uint8_t* counter) { *counter = (uint8_t)(*counter + 53u); }

uint8_t gp_led_quantize(uint16_t v16, uint8_t counter, bool dither) {
	uint8_t out = (uint8_t)(v16 >> 8);
	if (dither && out < 255 && (v16 & 0xFFu) > counter)
		out++;
	return out;
}

static void add(gp_rgbf_t* a, const gp_color_t* c, float k) {
	a->r += (float)c->r * (1.0f / 255.0f) * k;
	a->g += (float)c->g * (1.0f / 255.0f) * k;
	a->b += (float)c->b * (1.0f / 255.0f) * k;
}

static inline bool has(const uint32_t m[2], int l) { return (m[l / 32] >> (l % 32)) & 1u; }

static float clamp01(float v) { return v < 0.0f ? 0.0f : (v > 1.0f ? 1.0f : v); }

// Lights the n LEDs idx[0..n-1] (in order, all on one LED pin) for one axis.
static void effect(const gp_axis_cfg_t* a, const gp_axis_led_t* in, const uint8_t* idx, int n, gp_rgbf_t* acc) {
	if (n == 0)
		return;
	switch (n == 1 ? GP_LED_FX_GLOW : a->led_effect) { // a lone LED is an indicator: it glows
	case GP_LED_FX_BAR: {
		// The LEDs are segments [j, j+1] of a line 0..n. The bar covers
		// [0, pos*n] (linear) or [n/2, n/2 + pos*n/2] (centred); each LED
		// shows how much of its segment is covered.
		float lo, hi;
		if (in->centred) {
			float c = n * 0.5f, e = c + in->pos * c;
			lo = e < c ? e : c;
			hi = e < c ? c : e;
		} else {
			lo = 0.0f;
			hi = clamp01(in->pos) * (float)n;
		}
		for (int j = 0; j < n; j++) {
			float s = lo > (float)j ? lo : (float)j;
			float e = hi < (float)(j + 1) ? hi : (float)(j + 1);
			if (e > s)
				add(&acc[idx[j]], &a->color, e - s);
		}
		break;
	}
	case GP_LED_FX_DOT: {
		float p = in->centred ? (in->pos + 1.0f) * 0.5f : in->pos;
		p		= clamp01(p) * (float)(n - 1);
		int	  j = (int)p;
		float f = p - (float)j;
		if (j >= n - 1) {
			j = n - 1;
			f = 0.0f;
		}
		add(&acc[idx[j]], &a->color, 1.0f - f);
		if (f > 0.0f)
			add(&acc[idx[j + 1]], &a->color, f);
		break;
	}
	default: { // glow
		float m = in->mag;
		if (!(m > 0.0f))
			break;
		float k = m * m; // perceptually smoother ramp than linear
		for (int j = 0; j < n; j++)
			add(&acc[idx[j]], &a->color, k);
		break;
	}
	}
}

void gp_led_compose_inputs(const gp_config_t* look, int num_axes, int num_buttons, int num_leds, const uint8_t* led_pin,
						   const gp_axis_led_t* axes, const uint32_t buttons[2], gp_rgbf_t* acc) {
	uint8_t idx[GP_MAX_LEDS];
	if (num_leds > GP_MAX_LEDS)
		num_leds = GP_MAX_LEDS;
	for (int i = 0; i < num_axes; i++) {
		// one run per LED pin: a chain shows the effect on its own
		int n = 0;
		for (int l = 0; l <= num_leds; l++) {
			if (n > 0 && (l == num_leds || led_pin[l] != led_pin[idx[0]])) {
				effect(&look->axes[i], &axes[i], idx, n, acc);
				n = 0;
			}
			if (l < num_leds && has(look->axes[i].led_mask, l))
				idx[n++] = (uint8_t)l;
		}
	}
	for (int i = 0; i < num_buttons; i++) {
		if (!((buttons[i / 32] >> (i % 32)) & 1u))
			continue;
		for (int l = 0; l < num_leds; l++)
			if (has(look->buttons[i].led_mask, l))
				add(&acc[l], &look->buttons[i].color, 1.0f);
	}
}

float gp_led_breathe(uint32_t now_us, uint32_t period_us) {
	uint32_t half = period_us / 2;
	uint32_t ph	  = now_us % period_us;
	return ph < half ? (float)ph / (float)half : (float)(period_us - ph) / (float)half;
}
