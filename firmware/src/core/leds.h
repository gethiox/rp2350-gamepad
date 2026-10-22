// SPDX-License-Identifier: MIT
//
// leds.h - LED colour composition and output quantisation.
//
// Each input adds its colour to the LEDs in its mask (additive mixing), the
// sum is clamped, scaled by the global brightness and converted to the
// WS2812's 8-bit levels.
//
// LEDs are numbered across all LED pins: pin 1's chain first, then pin 2's
// chain, and so on. An axis's Bar and Dot effects run along the selected
// LEDs of each pin separately; where only one LED of a pin is selected, that
// LED glows instead (a classic single-LED indicator).
//
// Quantisation:
//  * Dithering ON: ordered temporal dithering. The 8 fractional bits of the
//    16-bit value are compared with a frame counter that advances by 53
//    each frame, so a level between two 8-bit steps is shown as the right
//    mix of both.
//  * Dithering OFF: the upper 8 bits.

#ifndef GP_LEDS_H
#define GP_LEDS_H

#include <stdbool.h>
#include <stdint.h>

#include "gp_protocol.h"

#define GP_LED_FRAME_US_DITHER 400
#define GP_LED_FRAME_US_PLAIN  2000
#define GP_LED_US_PER_LED	   30  // 24 bits at 800 kHz
#define GP_LED_LATCH_US		   300 // low time that latches a WS2812 chain (newer parts need >280 us)

typedef struct {
	float r, g, b;
} gp_rgbf_t;

// What an axis contributes to its LEDs.
typedef struct {
	float mag;	   // 0..1 travel from rest (Glow)
	float pos;	   // centred axes: -1..1, linear axes: 0..1 (Bar, Dot)
	bool  centred; // pos is -1..1 around a centre
} gp_axis_led_t;

// Frame period for the given longest chain: never so short that a frame
// would start before the previous one has been sent and latched.
uint32_t gp_led_frame_us(bool dither, unsigned longest_chain);

// Advances the dither pattern; call once per LED frame.
void gp_led_dither_next(uint8_t* counter);

// Converts a 16-bit linear intensity to an 8-bit LED level.
uint8_t gp_led_quantize(uint16_t v16, uint8_t counter, bool dither);

// Adds input colours to `acc` (num_leds entries). led_pin[l] is the LED pin
// that LED l belongs to.
void gp_led_compose_inputs(const gp_config_t* look, int num_axes, int num_buttons, int num_leds, const uint8_t* led_pin,
						   const gp_axis_led_t* axes, const uint32_t buttons[2], gp_rgbf_t* acc);

// Triangle wave 0..1..0 with the given period.
float gp_led_breathe(uint32_t now_us, uint32_t period_us);

#endif
