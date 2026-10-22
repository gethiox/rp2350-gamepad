// SPDX-License-Identifier: MIT
//
// input.h - ADC smoothing and button debouncing.

#ifndef GP_INPUT_H
#define GP_INPUT_H

#include <stdbool.h>
#include <stdint.h>

#include "gp_protocol.h"

#define GP_FILTER_MAX 128

typedef struct {
	uint16_t hist[GP_MAX_AXES][GP_FILTER_MAX];
	uint32_t sum[GP_MAX_AXES];
	uint8_t	 n;	   // window length
	uint8_t	 mode; // GP_SMOOTH_*
	uint8_t	 idx;  // slot that receives the next sample
	bool	 seeded;
} gp_filter_t;

void gp_filter_init(gp_filter_t* f, uint8_t samples, uint8_t mode);
void gp_filter_reseed(gp_filter_t* f); // next sample fills the whole window
// Pushes one sample per channel and writes the filtered values.
void gp_filter_step(gp_filter_t* f, const uint16_t* in, uint16_t* out, int count);

// Debounce: a change is accepted immediately, then further changes are
// ignored for `debounce_us` (no added latency on the first edge).
typedef struct {
	bool	 state;
	uint32_t last_change;
} gp_debounce_t;

void gp_debounce_init(gp_debounce_t* d, uint32_t now, uint32_t debounce_us);
bool gp_debounce_update(gp_debounce_t* d, bool raw, uint32_t now, uint32_t debounce_us);

#endif
