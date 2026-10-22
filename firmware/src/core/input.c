// SPDX-License-Identifier: MIT
#include "input.h"

#include <string.h>

void gp_filter_init(gp_filter_t* f, uint8_t samples, uint8_t mode) {
	memset(f, 0, sizeof(*f));
	if (samples < 1)
		samples = 1;
	if (samples > GP_FILTER_MAX)
		samples = GP_FILTER_MAX;
	f->n	= samples;
	f->mode = mode <= GP_SMOOTH_TRIANGLE ? mode : GP_SMOOTH_SIMPLE;
}

void gp_filter_reseed(gp_filter_t* f) { f->seeded = false; }

void gp_filter_step(gp_filter_t* f, const uint16_t* in, uint16_t* out, int count) {
	const uint8_t n = f->n;
	if (count > GP_MAX_AXES)
		count = GP_MAX_AXES;

	if (!f->seeded) {
		for (int c = 0; c < count; c++) {
			for (int j = 0; j < n; j++)
				f->hist[c][j] = in[c];
			f->sum[c] = (uint32_t)in[c] * n;
		}
		f->idx	  = 0;
		f->seeded = true;
	}

	for (int c = 0; c < count; c++) {
		uint16_t* h = f->hist[c];
		f->sum[c]	= f->sum[c] - h[f->idx] + in[c];
		h[f->idx]	= in[c];

		if (f->mode == GP_SMOOTH_SIMPLE || n == 1) {
			out[c] = (uint16_t)(f->sum[c] / n);
			continue;
		}
		// Oldest sample sits at idx+1, newest at idx.
		uint32_t wsum = 0, wtotal = 0;
		int		 half = n / 2;
		int		 k	  = f->idx + 1;
		for (int j = 0; j < n; j++, k++) {
			if (k >= n)
				k = 0;
			uint32_t w;
			if (f->mode == GP_SMOOTH_LINEAR)
				w = (uint32_t)(j + 1); // newest weighs most
			else
				w = (uint32_t)(j <= half ? j + 1 : n - j); // triangular window
			wsum += (uint32_t)h[k] * w;
			wtotal += w;
		}
		out[c] = (uint16_t)((wsum + wtotal / 2) / wtotal);
	}
	f->idx = (uint8_t)(f->idx + 1 >= n ? 0 : f->idx + 1);
}

void gp_debounce_init(gp_debounce_t* d, uint32_t now, uint32_t debounce_us) {
	d->state	   = false;
	d->last_change = now - debounce_us; // first edge is accepted immediately
}

bool gp_debounce_update(gp_debounce_t* d, bool raw, uint32_t now, uint32_t debounce_us) {
	if (raw != d->state && (uint32_t)(now - d->last_change) >= debounce_us) {
		d->state	   = raw;
		d->last_change = now;
	}
	return d->state;
}
