// SPDX-License-Identifier: MIT
//
// Host unit tests for the firmware core. Built with AddressSanitizer and
// UndefinedBehaviorSanitizer (see host/CMakeLists.txt), so memory errors
// and undefined behaviour fail the run as well.

#include <math.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "app.h"
#include "axis.h"
#include "calib.h"
#include "config.h"
#include "crc32.h"
#include "frame.h"
#include "hal.h"
#include "hal_host.h"
#include "hid.h"
#include "leds.h"
#include "storage.h"

// ---------------------------------------------------------------------------
// Minimal test framework
// ---------------------------------------------------------------------------

static int failures, checks;
static const char* current;

#define CHECK(cond)                                                                    \
	do {                                                                               \
		checks++;                                                                      \
		if (!(cond)) {                                                                 \
			failures++;                                                                \
			fprintf(stderr, "  FAIL %s:%d [%s]: %s\n", __FILE__, __LINE__, current, #cond); \
		}                                                                              \
	} while (0)

#define CHECK_NEAR(a, b, eps) CHECK(fabs((double)(a) - (double)(b)) <= (eps))

#define RUN(fn)                     \
	do {                            \
		current = #fn;              \
		int before = failures;      \
		fn();                       \
		printf("%s %s\n", failures == before ? "ok  " : "FAIL", #fn); \
	} while (0)

static uint32_t rng = 12345;
static uint32_t rnd(void) {
	rng ^= rng << 13;
	rng ^= rng >> 17;
	rng ^= rng << 5;
	return rng;
}

static gp_limits_t limits_a(void) {
	hal_host.rp2350b = false;
	gp_limits_t l;
	gp_limits_build(&l);
	return l;
}

// ---------------------------------------------------------------------------
// CRC / framing
// ---------------------------------------------------------------------------

static void test_crc32(void) {
	CHECK(gp_crc32("123456789", 9) == 0xCBF43926u);
	uint32_t c = gp_crc32_update(gp_crc32("1234", 4), "56789", 5);
	CHECK(c == 0xCBF43926u);
}

typedef struct {
	int		 count;
	uint8_t	 cmd[64];
	uint16_t len[64];
	uint8_t	 first[64];
} frames_t;

static void collect(void* ctx, uint8_t cmd, const uint8_t* p, uint16_t len) {
	frames_t* f = ctx;
	if (f->count < 64) {
		f->cmd[f->count]   = cmd;
		f->len[f->count]   = len;
		f->first[f->count] = len ? p[0] : 0;
	}
	f->count++;
}

static void test_frames(void) {
	static gp_frame_rx_t rx;
	uint8_t				 buf[GP_PROTO_MAX_PAYLOAD + GP_FRAME_OVERHEAD];
	uint8_t				 payload[300];
	for (int i = 0; i < 300; i++)
		payload[i] = (uint8_t)i;

	// round trip, fed in one piece and byte by byte
	frames_t f = { 0 };
	gp_frame_rx_init(&rx);
	size_t n = gp_frame_encode(buf, 0x42, payload, 10, payload + 10, 290);
	gp_frame_rx_feed(&rx, buf, n, collect, &f);
	CHECK(f.count == 1 && f.cmd[0] == 0x42 && f.len[0] == 300 && f.first[0] == 0);
	for (size_t i = 0; i < n; i++)
		gp_frame_rx_feed(&rx, buf + i, 1, collect, &f);
	CHECK(f.count == 2);

	// junk (including fake sync bytes) before a valid frame
	uint8_t junk[] = { 'A', 'T', '\r', 0xAA, 0x00, 0xAA, 0x55, 0x01, 0xFF, 0xFF, 0x13 };
	gp_frame_rx_feed(&rx, junk, sizeof(junk), collect, &f);
	gp_frame_rx_feed(&rx, buf, n, collect, &f);
	CHECK(f.count == 3);

	// corrupted frame followed by a valid one
	uint8_t bad[sizeof(buf)];
	memcpy(bad, buf, n);
	bad[20] ^= 1;
	gp_frame_rx_feed(&rx, bad, n, collect, &f);
	gp_frame_rx_feed(&rx, buf, n, collect, &f);
	CHECK(f.count == 4);
	CHECK(rx.crc_errors >= 1);

	// a fake header announcing a big frame must not swallow a real frame
	uint8_t fake[] = { 0xAA, 0x55, 0x05, 0x00, 0x04 }; // len 1024
	gp_frame_rx_feed(&rx, fake, sizeof(fake), collect, &f);
	size_t small = gp_frame_encode(buf, 0x08, NULL, 0, NULL, 0);
	for (int k = 0; k < 120; k++) // 1080 bytes of valid frames
		gp_frame_rx_feed(&rx, buf, small, collect, &f);
	CHECK(f.count >= 4 + 100); // at most the frames inside the fake length can be lost

	// fuzz: random junk interleaved with frames; every frame not overlapped
	// by junk containing a sync is still received and nothing crashes
	gp_frame_rx_init(&rx);
	frames_t g = { 0 };
	int		 sent = 0;
	for (int k = 0; k < 2000; k++) {
		uint8_t r[64];
		int		len = (int)(rnd() % 64);
		for (int i = 0; i < len; i++)
			r[i] = (uint8_t)rnd();
		gp_frame_rx_feed(&rx, r, (size_t)len, collect, &g);
		size_t m = gp_frame_encode(buf, (uint8_t)(rnd() & 0x7F), payload, (uint16_t)(rnd() % 200), NULL, 0);
		gp_frame_rx_feed(&rx, buf, m, collect, &g);
		sent++;
	}
	CHECK(g.count >= sent * 9 / 10);
}

// ---------------------------------------------------------------------------
// Config validation
// ---------------------------------------------------------------------------

static int has_error(const gp_error_list_t* e, uint8_t field) {
	for (int i = 0; i < e->count; i++)
		if (e->errors[i].field == field)
			return 1;
	return 0;
}

static void test_config_defaults_valid(void) {
	gp_config_t		c;
	gp_error_list_t e;
	gp_limits_t		l = limits_a();
	gp_config_defaults(&c);
	CHECK(gp_config_validate(&c, &l, &e) == 0);
	for (int i = 0; i < e.count; i++)
		fprintf(stderr, "    %s\n", e.errors[i].message);

	hal_host.rp2350b = true;
	gp_limits_build(&l);
	CHECK(l.platform_axes == 8 && l.gpio_count == 48);
	// defaults use pins 26..29 for axes; on RP2350B those are not ADC pins
	CHECK(gp_config_validate(&c, &l, &e) > 0);
	CHECK(has_error(&e, GP_F_AXIS_PIN));
	hal_host.rp2350b = false;

	gp_calibration_t cal;
	gp_calibration_defaults(&cal);
	l = limits_a();
	CHECK(gp_calibration_validate(&cal, &l, NULL) == 0);
}

typedef void (*mutator_t)(gp_config_t*);
static void m_pin_conflict(gp_config_t* c) { c->buttons[1].pin = c->buttons[0].pin; }
static void m_axis_not_adc(gp_config_t* c) { c->axes[0].pin = 3; }
static void m_led_conflict(gp_config_t* c) { c->led_pins[0] = c->axes[1].pin; }
static void m_too_many_leds(gp_config_t* c) { c->num_led_pins = 200; }
static void m_builtin_chain(gp_config_t* c) { c->led_counts[0] = 4; } // GPIO 22 is the built-in LED
static void m_chain_zero(gp_config_t* c) { c->led_counts[1] = 0; }
static void m_chain_total(gp_config_t* c) { c->led_counts[1] = 40; c->led_counts[2] = 30; }
static void m_btn_mask_high(gp_config_t* c) { c->buttons[1].led_mask[1] = 0x1; }
static void m_led_effect(gp_config_t* c) { c->axes[0].led_effect = 9; }
static void m_too_many_axes(gp_config_t* c) { c->num_axes = 5; }
static void m_too_many_buttons(gp_config_t* c) { c->num_buttons = 65; }
static void m_dz_sum(gp_config_t* c) { c->center_dz = 500; c->outer_dz = 450; }
static void m_axis_dz(gp_config_t* c) { c->axes[0].flags |= GP_AXIS_CUSTOM_DZ; c->axes[0].outer_dz = 600; }
static void m_bad_string(gp_config_t* c) { c->usb_product[0] = '\x01'; }
static void m_empty_product(gp_config_t* c) { memset(c->usb_product, 0, GP_STR_LEN); }
static void m_unterminated(gp_config_t* c) { memset(c->usb_serial, 'A', GP_STR_LEN); }
static void m_trailing(gp_config_t* c) { c->usb_manufacturer[30] = 'x'; }
static void m_vid0(gp_config_t* c) { c->usb_vid = 0; }
static void m_rate(gp_config_t* c) { c->hid_rate_hz = 0; }
static void m_poll(gp_config_t* c) { c->poll_rate_hz = 0; }
static void m_samples(gp_config_t* c) { c->smoothing_samples = 0; }
static void m_mode(gp_config_t* c) { c->smoothing_mode = 9; }
static void m_reset_hold(gp_config_t* c) { c->reset_hold_ms = c->cal_hold_ms + 100; }
static void m_status_mask(gp_config_t* c) { c->status_led_mask[0] = 0x80; }
static void m_axis_led_mask(gp_config_t* c) { c->axes[2].led_mask[0] = 0x40; }
static void m_usage_dup(gp_config_t* c) { c->axes[1].hid_usage = c->axes[0].hid_usage; }
static void m_usage_bad(gp_config_t* c) { c->axes[1].hid_usage = 0x01; }
static void m_sliders(gp_config_t* c) {
	c->axes[0].hid_usage = GP_USAGE_SLIDER;
	c->axes[1].hid_usage = GP_USAGE_SLIDER;
	c->axes[2].hid_usage = GP_USAGE_DIAL;
}
static void m_group_same(gp_config_t* c) { c->groups[0].enabled = 1; c->groups[0].axis_v = c->groups[0].axis_h; }
static void m_group_overlap(gp_config_t* c) {
	c->groups[0].enabled = 1;
	c->groups[1].enabled = 1;
	c->groups[1].axis_h	 = 1;
}
static void m_group_disabled_axis(gp_config_t* c) { c->groups[1].enabled = 1; c->num_axes = 3; }
static void m_reserved(gp_config_t* c) { c->reserved1 = 1; }
static void m_color_reserved(gp_config_t* c) { c->buttons[0].color.reserved = 1; }
static void m_header(gp_config_t* c) { c->version = 99; }
static void m_btn_no_pin(gp_config_t* c) { c->num_buttons = 9; } // button 9 has no pin by default
static void m_axis_flags(gp_config_t* c) { c->axes[0].flags = 0x80; }

static void test_config_invalid(void) {
	struct {
		mutator_t m;
		uint8_t	  field;
	} cases[] = {
		{ m_pin_conflict, GP_F_BUTTON_PIN }, { m_axis_not_adc, GP_F_AXIS_PIN }, { m_led_conflict, GP_F_AXIS_PIN },
		{ m_too_many_leds, GP_F_NUM_LED_PINS }, { m_builtin_chain, GP_F_LED_COUNT }, { m_chain_zero, GP_F_LED_COUNT },
		{ m_chain_total, GP_F_LED_COUNT }, { m_btn_mask_high, GP_F_BUTTON_LED_MASK }, { m_led_effect, GP_F_AXIS_LED_EFFECT }, { m_too_many_axes, GP_F_NUM_AXES }, { m_too_many_buttons, GP_F_NUM_BUTTONS },
		{ m_dz_sum, GP_F_OUTER_DZ }, { m_axis_dz, GP_F_AXIS_OUTER_DZ }, { m_bad_string, GP_F_USB_PRODUCT },
		{ m_empty_product, GP_F_USB_PRODUCT }, { m_unterminated, GP_F_USB_SERIAL }, { m_trailing, GP_F_USB_MANUFACTURER },
		{ m_vid0, GP_F_USB_VID }, { m_rate, GP_F_HID_RATE }, { m_poll, GP_F_POLL_RATE }, { m_samples, GP_F_SMOOTHING_SAMPLES },
		{ m_mode, GP_F_SMOOTHING_MODE }, { m_reset_hold, GP_F_RESET_HOLD }, { m_status_mask, GP_F_STATUS_LED_MASK },
		{ m_axis_led_mask, GP_F_AXIS_LED_MASK }, { m_usage_dup, GP_F_AXIS_USAGE }, { m_usage_bad, GP_F_AXIS_USAGE },
		{ m_sliders, GP_F_AXIS_USAGE }, { m_group_same, GP_F_GROUP_AXES }, { m_group_overlap, GP_F_GROUP_AXES },
		{ m_group_disabled_axis, GP_F_GROUP_AXES }, { m_reserved, GP_F_HEADER }, { m_color_reserved, GP_F_BUTTON_COLOR },
		{ m_header, GP_F_HEADER }, { m_btn_no_pin, GP_F_BUTTON_PIN }, { m_axis_flags, GP_F_AXIS_FLAGS },
	};
	gp_limits_t l = limits_a();
	for (size_t i = 0; i < sizeof(cases) / sizeof(cases[0]); i++) {
		gp_config_t		c;
		gp_error_list_t e;
		gp_config_defaults(&c);
		cases[i].m(&c);
		int n = gp_config_validate(&c, &l, &e);
		CHECK(n > 0);
		CHECK(e.status == GP_STATUS_INVALID);
		if (!has_error(&e, cases[i].field)) {
			fprintf(stderr, "    case %zu: expected field %u, got:", i, cases[i].field);
			for (int k = 0; k < e.count; k++)
				fprintf(stderr, " [%u] %s;", e.errors[k].field, e.errors[k].message);
			fprintf(stderr, "\n");
			CHECK(0);
		}
		for (int k = 0; k < e.count; k++)
			CHECK(memchr(e.errors[k].message, 0, GP_ERROR_MSG_LEN) != NULL);
	}
}

static void test_config_fuzz(void) {
	gp_limits_t l = limits_a();
	for (int k = 0; k < 20000; k++) {
		gp_config_t c;
		gp_config_defaults(&c);
		// flip a few random bytes after the header
		int flips = 1 + (int)(rnd() % 6);
		for (int i = 0; i < flips; i++) {
			size_t off = 8 + rnd() % (sizeof(c) - 8);
			((uint8_t*)&c)[off] = (uint8_t)rnd();
		}
		gp_error_list_t e;
		int				n = gp_config_validate(&c, &l, &e);
		CHECK(n >= 0 && e.count <= GP_MAX_ERRORS);
		CHECK((n == 0) == (e.status == GP_STATUS_OK));
	}
}

// ---------------------------------------------------------------------------
// Storage
// ---------------------------------------------------------------------------

static void make_cfg(gp_config_t* c, uint16_t marker) {
	gp_config_defaults(c);
	c->usb_pid = marker;
}

static void test_storage_basic(void) {
	hal_host_reset();
	hal_host_erase_flash();
	gp_config_t		   c, c2;
	gp_calibration_t   cal, cal2;
	gp_storage_state_t st;
	gp_storage_load(&c, &cal, &st);
	CHECK(st.empty && !st.config_loaded && st.slot == -1);

	make_cfg(&c, 0x1111);
	gp_calibration_defaults(&cal);
	cal.channels[0].calibrated = 1;
	CHECK(gp_storage_save(&c, &cal, &st));
	CHECK(st.slot == 0 && st.seq == 1);
	make_cfg(&c, 0x2222);
	CHECK(gp_storage_save(&c, &cal, &st));
	CHECK(st.slot == 1 && st.seq == 2);

	gp_storage_state_t st2;
	gp_storage_load(&c2, &cal2, &st2);
	CHECK(st2.config_loaded && st2.cal_loaded && !st2.empty && !st2.recovered);
	CHECK(c2.usb_pid == 0x2222 && cal2.channels[0].calibrated == 1);
	CHECK(st2.slot == 1 && st2.seq == 2);

	// damage the newest slot: the older record is used and reported
	hal_host.flash[1][100] ^= 0x5A;
	gp_storage_load(&c2, &cal2, &st2);
	CHECK(st2.config_loaded && c2.usb_pid == 0x1111 && st2.recovered);
	// the next save must go to the damaged slot, never over the good one
	make_cfg(&c, 0x3333);
	CHECK(gp_storage_save(&c, &cal, &st2));
	CHECK(st2.slot == 1);
	gp_storage_load(&c2, &cal2, &st2);
	CHECK(c2.usb_pid == 0x3333 && !st2.recovered);

	CHECK(gp_storage_erase_all(&st2));
	gp_storage_load(&c2, &cal2, &st2);
	CHECK(st2.empty);
}

static void test_storage_power_loss(void) {
	// Interrupt the second save at every possible point: the result must be
	// either the old or the new record, never defaults or garbage.
	for (int cut = 0; cut <= 1280; cut += 8) {
		hal_host_reset();
		hal_host_erase_flash();
		gp_config_t		   c, out;
		gp_calibration_t   cal, cal_out;
		gp_storage_state_t st;
		gp_storage_load(&c, &cal, &st);
		make_cfg(&c, 0xAAAA);
		gp_calibration_defaults(&cal);
		CHECK(gp_storage_save(&c, &cal, &st));
		make_cfg(&c, 0xBBBB);
		hal_host.fail_write_after = cut;
		bool ok					  = gp_storage_save(&c, &cal, &st);
		gp_storage_load(&out, &cal_out, &st);
		CHECK(st.config_loaded);
		CHECK(out.usb_pid == 0xAAAA || out.usb_pid == 0xBBBB);
		if (ok)
			CHECK(out.usb_pid == 0xBBBB);
		else
			CHECK(out.usb_pid == 0xAAAA);
	}
}

static void test_storage_seq_wrap(void) {
	hal_host_reset();
	hal_host_erase_flash();
	gp_config_t		   c;
	gp_calibration_t   cal;
	gp_storage_state_t st;
	gp_storage_load(&c, &cal, &st);
	st.slot = 1;
	st.seq	= 0xFFFFFFFEu;
	make_cfg(&c, 1);
	CHECK(gp_storage_save(&c, &cal, &st)); // seq FFFFFFFF in slot 0
	make_cfg(&c, 2);
	CHECK(gp_storage_save(&c, &cal, &st)); // seq 0 in slot 1
	gp_config_t out;
	gp_storage_load(&out, &cal, &st);
	CHECK(out.usb_pid == 2 && st.seq == 0);
}

// ---------------------------------------------------------------------------
// Axis math
// ---------------------------------------------------------------------------

static void test_axis_math(void) {
	gp_span_t s = { 1000, 2000, 3000 };
	CHECK_NEAR(gp_axis_centered(2000, s, 0, 0), 0, 1e-6);
	CHECK_NEAR(gp_axis_centered(3000, s, 0, 0), 1, 1e-6);
	CHECK_NEAR(gp_axis_centered(1000, s, 0, 0), -1, 1e-6);
	CHECK_NEAR(gp_axis_centered(4095, s, 0, 0), 1, 1e-6);
	CHECK_NEAR(gp_axis_centered(0, s, 0, 0), -1, 1e-6);
	CHECK_NEAR(gp_axis_centered(2040, s, 0.05f, 0), 0, 1e-6); // inside centre deadzone
	CHECK_NEAR(gp_axis_centered(2950, s, 0, 0.05f), 1, 1e-6); // inside outer deadzone
	CHECK_NEAR(gp_axis_centered(2525, s, 0.05f, 0.05f), (0.525 - 0.05) / 0.9, 1e-4);
	CHECK_NEAR(gp_axis_linear(1000, s, 0, 0), -1, 1e-6);
	CHECK_NEAR(gp_axis_linear(3000, s, 0, 0), 1, 1e-6);
	CHECK_NEAR(gp_axis_linear(2000, s, 0, 0), 0, 1e-6);
	CHECK_NEAR(gp_axis_linear(1050, s, 0.05f, 0), -1, 1e-6);
	CHECK(gp_axis_to_hid(1.0f) == 32767 && gp_axis_to_hid(-1.0f) == -32767 && gp_axis_to_hid(0) == 0);
	CHECK(gp_axis_to_hid(NAN) == 0 && gp_axis_to_hid(5.0f) == 32767);

	// broken calibration falls back to defaults instead of dividing by zero
	gp_cal_channel_t bad = { 1, 0, 4095, 2000, 0 };
	gp_span_t		 d	 = gp_channel_span(&bad);
	CHECK(d.min == 0 && d.max == 4095);

	uint16_t radial[GP_RADIAL_BINS];
	for (int b = 0; b < GP_RADIAL_BINS; b++)
		radial[b] = (uint16_t)(1000 + b * 10);
	// bin centre angles return the bin value exactly
	for (int b = 0; b < GP_RADIAL_BINS; b++) {
		float a = -3.14159265f + (b + 0.5f) * 2 * 3.14159265f / GP_RADIAL_BINS;
		CHECK_NEAR(gp_radial_lookup(radial, a), radial[b], 0.5);
	}
}

// ---------------------------------------------------------------------------
// Calibration
// ---------------------------------------------------------------------------

static gp_config_t cal_cfg(void) {
	gp_config_t c;
	gp_config_defaults(&c);
	c.groups[0].enabled = 1; // axes 0 (h) and 1 (v), pins 26/27
	return c;
}

// User frame (x right, y up) -> raw ADC values for a wiring variant.
static void wire(int orient, double ux, double uy, uint16_t* a, uint16_t* b) {
	double x = ux, y = uy;
	if (orient & 1) {
		x = uy;
		y = ux;
	}
	if (orient & 2)
		x = -x;
	if (orient & 4)
		y = -y;
	*a = (uint16_t)lround(2048 + 1600 * x);
	*b = (uint16_t)lround(2048 + 1600 * y);
}

static void run_gesture(gp_calib_t* s, const gp_config_t* c, int orient, double turns, int clockwise) {
	uint16_t raw[GP_MAX_AXES] = { 2048, 2048, 2048, 2048, 2048, 2048, 2048, 2048 };
	gp_calib_start(s, c, raw);
	for (int i = 0; i <= 50; i++) { // push up
		wire(orient, 0, i / 50.0, &raw[0], &raw[1]);
		gp_calib_update(s, c, raw);
	}
	int steps = (int)(400 * turns);
	for (int i = 0; i <= steps; i++) {
		double a = 3.14159265 / 2 + (clockwise ? -1 : 1) * (double)i / 400 * 2 * 3.14159265;
		wire(orient, cos(a), sin(a), &raw[0], &raw[1]);
		gp_calib_update(s, c, raw);
	}
	for (int i = 50; i >= 0; i--) { // back to centre
		wire(orient, 0, 0, &raw[0], &raw[1]);
		gp_calib_update(s, c, raw);
	}
}

static void test_calibration_orientations(void) {
	gp_config_t c = cal_cfg();
	for (int orient = 0; orient < 8; orient++) {
		static gp_calib_t s;
		gp_calibration_t  cal;
		gp_calibration_defaults(&cal);
		run_gesture(&s, &c, orient, 1.25, 1);
		uint16_t		raw[GP_MAX_AXES] = { 2048, 2048, 2048, 2048 };
		gp_cal_result_t res;
		gp_calib_finish(&s, &c, raw, &cal, &res, hal_adc_channel);
		CHECK(res.group[0] == GP_CALRES_UPDATED);
		CHECK(cal.groups[0].calibrated == 1);
		CHECK(cal.groups[0].pin_a == 26 && cal.groups[0].pin_b == 27);
		// Standalone channels were moved enough to be calibrated too.
		CHECK(res.channel[0] == GP_CALRES_UPDATED && res.channel[1] == GP_CALRES_UPDATED);
		CHECK(res.channel[2] == GP_CALRES_UNCHANGED);

		gp_group_params_t p = { cal.groups[0].radial, cal.groups[0].orient, 0, GP_SHAPE_CIRCLE, 0.0f, 0.0f };
		struct {
			double ux, uy, eh, ev;
		} probes[] = { { 1, 0, 1, 0 }, { -1, 0, -1, 0 }, { 0, 1, 0, 1 }, { 0, -1, 0, -1 }, { 0.5, 0.5, 0.5, 0.5 } };
		for (size_t k = 0; k < sizeof(probes) / sizeof(probes[0]); k++) {
			uint16_t a, b;
			wire(orient, probes[k].ux, probes[k].uy, &a, &b);
			float h, v;
			gp_group_compute((float)a - cal.groups[0].center_a, (float)b - cal.groups[0].center_b, &p, &h, &v);
			CHECK_NEAR(h, probes[k].eh, 0.05);
			CHECK_NEAR(v, probes[k].ev, 0.05);
		}
	}
}

static void test_calibration_incomplete(void) {
	gp_config_t		  c = cal_cfg();
	static gp_calib_t s;
	gp_calibration_t  cal;
	gp_calibration_defaults(&cal);
	cal.groups[0].calibrated = 1; // previous calibration exists
	cal.groups[0].pin_a		 = 26;
	cal.groups[0].pin_b		 = 27;
	run_gesture(&s, &c, 0, 0.5, 1); // only half a turn
	uint16_t		raw[GP_MAX_AXES] = { 2048, 2048, 2048, 2048 };
	gp_cal_result_t res;
	gp_calib_finish(&s, &c, raw, &cal, &res, hal_adc_channel);
	CHECK(res.group[0] == GP_CALRES_FAILED);
	CHECK(cal.groups[0].calibrated == 0);

	// untouched: the previous calibration is kept
	gp_calibration_defaults(&cal);
	cal.groups[0].calibrated = 1;
	cal.groups[0].pin_a		 = 26;
	cal.groups[0].pin_b		 = 27;
	cal.groups[0].orient	 = 5;
	gp_calib_start(&s, &c, raw);
	for (int i = 0; i < 100; i++) {
		uint16_t r2[GP_MAX_AXES] = { (uint16_t)(2048 + (i % 3)), 2048, 2048, 2048 };
		gp_calib_update(&s, &c, r2);
	}
	gp_calib_finish(&s, &c, raw, &cal, &res, hal_adc_channel);
	CHECK(res.group[0] == GP_CALRES_UNCHANGED);
	CHECK(cal.groups[0].calibrated == 1 && cal.groups[0].orient == 5);
	CHECK(res.channel[0] == GP_CALRES_UNCHANGED);
}

static void test_calibration_false_start(void) {
	// The user wiggles the stick and rotates a bit the wrong way, lets go,
	// then does the gesture properly: the proper attempt must win.
	gp_config_t		  c = cal_cfg();
	static gp_calib_t s;
	gp_calibration_t  cal;
	gp_calibration_defaults(&cal);
	uint16_t raw[GP_MAX_AXES] = { 2048, 2048, 2048, 2048, 2048, 2048, 2048, 2048 };
	gp_calib_start(&s, &c, raw);
	for (int i = 0; i <= 200; i++) { // starts at the left, turns counter-clockwise half a turn
		double a = 3.14159265 + (double)i / 400 * 2 * 3.14159265;
		wire(3, cos(a), sin(a), &raw[0], &raw[1]);
		gp_calib_update(&s, &c, raw);
	}
	wire(3, 0, 0, &raw[0], &raw[1]);
	gp_calib_update(&s, &c, raw);
	// now the real gesture (continues the same session)
	for (int i = 0; i <= 50; i++) {
		wire(3, 0, i / 50.0, &raw[0], &raw[1]);
		gp_calib_update(&s, &c, raw);
	}
	for (int i = 0; i <= 450; i++) {
		double a = 3.14159265 / 2 - (double)i / 400 * 2 * 3.14159265;
		wire(3, cos(a), sin(a), &raw[0], &raw[1]);
		gp_calib_update(&s, &c, raw);
	}
	wire(3, 0, 0, &raw[0], &raw[1]);
	for (int i = 0; i < 10; i++) // wander a little after finishing
		gp_calib_update(&s, &c, raw);
	gp_cal_result_t res;
	gp_calib_finish(&s, &c, raw, &cal, &res, hal_adc_channel);
	CHECK(res.group[0] == GP_CALRES_UPDATED);
	gp_group_params_t p = { cal.groups[0].radial, cal.groups[0].orient, 0, GP_SHAPE_CIRCLE, 0, 0 };
	uint16_t a, b;
	float	 h, v;
	wire(3, 1, 0, &a, &b);
	gp_group_compute((float)a - cal.groups[0].center_a, (float)b - cal.groups[0].center_b, &p, &h, &v);
	CHECK_NEAR(h, 1, 0.05);
	CHECK_NEAR(v, 0, 0.05);
	wire(3, 0, 1, &a, &b);
	gp_group_compute((float)a - cal.groups[0].center_a, (float)b - cal.groups[0].center_b, &p, &h, &v);
	CHECK_NEAR(h, 0, 0.05);
	CHECK_NEAR(v, 1, 0.05);
}

static void test_calibration_noisy_deflected_start(void) {
	// Session starts while the stick is held at an angle, the signal is
	// noisy and the gate is not perfectly round; the stick is let go before
	// finishing. Every wiring variant must still be detected correctly.
	gp_config_t c = cal_cfg();
	for (int orient = 0; orient < 8; orient++) {
		static gp_calib_t s;
		gp_calibration_t  cal;
		gp_calibration_defaults(&cal);
		uint16_t raw[GP_MAX_AXES] = { 2048, 2048, 2048, 2048, 2048, 2048, 2048, 2048 };
		double	 noise			  = 0;
#define FEED(ux, uy)                                                                   \
	do {                                                                               \
		noise = (double)((int)(rnd() % 41) - 20) / 1600.0;                             \
		wire(orient, (ux) + noise, (uy) - noise, &raw[0], &raw[1]);                     \
		gp_calib_update(&s, &c, raw);                                                  \
	} while (0)
		wire(orient, cos(2.4), sin(2.4), &raw[0], &raw[1]); // deflected at the start
		gp_calib_start(&s, &c, raw);
		for (int i = 0; i < 80; i++) // wobble along the rim, then back to the centre
			FEED(cos(2.4 + i * 0.01), sin(2.4 + i * 0.01));
		for (int i = 40; i >= 0; i--)
			FEED(cos(3.2) * i / 40.0, sin(3.2) * i / 40.0);
		for (int i = 0; i <= 60; i++)
			FEED(0.02 * sin(i), i / 60.0);
		for (int i = 0; i <= 520; i++) {
			double a  = 3.14159265 / 2 - (double)i / 400 * 2 * 3.14159265;
			double rr = 1.0 + 0.04 * sin(4 * a); // slightly square gate
			FEED(rr * cos(a), rr * sin(a));
		}
		for (int i = 40; i >= 0; i--)
			FEED(i / 40.0 * 0.3, 0);
		wire(orient, 0, 0, &raw[0], &raw[1]);
		for (int i = 0; i < 20; i++)
			gp_calib_update(&s, &c, raw);
#undef FEED
		gp_cal_result_t res;
		gp_calib_finish(&s, &c, raw, &cal, &res, hal_adc_channel);
		CHECK(res.group[0] == GP_CALRES_UPDATED);
		gp_group_params_t p = { cal.groups[0].radial, cal.groups[0].orient, 0, GP_SHAPE_CIRCLE, 0, 0 };
		double			  probes[4][4] = { { 1, 0, 1, 0 }, { 0, 1, 0, 1 }, { -1, 0, -1, 0 }, { 0, -1, 0, -1 } };
		for (int k = 0; k < 4; k++) {
			uint16_t a, b;
			float	 h, v;
			wire(orient, probes[k][0], probes[k][1], &a, &b);
			gp_group_compute((float)a - cal.groups[0].center_a, (float)b - cal.groups[0].center_b, &p, &h, &v);
			CHECK_NEAR(h, probes[k][2], 0.08);
			CHECK_NEAR(v, probes[k][3], 0.08);
		}
	}
}

static void test_calibration_standalone(void) {
	gp_config_t c;
	gp_config_defaults(&c);
	static gp_calib_t s;
	gp_calibration_t  cal;
	gp_calibration_defaults(&cal);
	cal.channels[3].calibrated = 1;
	cal.channels[3].min		   = 100;
	cal.channels[3].center	   = 2000;
	cal.channels[3].max		   = 3900;
	uint16_t raw[GP_MAX_AXES]  = { 2000, 2048, 2048, 2048 };
	gp_calib_start(&s, &c, raw);
	for (int i = 0; i <= 100; i++) { // axis 0 full sweep
		raw[0] = (uint16_t)(300 + i * 34);
		raw[3] = (uint16_t)(2048 + (i % 2) * 100); // axis 3 barely moves
		gp_calib_update(&s, &c, raw);
	}
	raw[0] = 2010;
	gp_cal_result_t res;
	gp_calib_finish(&s, &c, raw, &cal, &res, hal_adc_channel);
	CHECK(res.channel[0] == GP_CALRES_UPDATED);
	CHECK(cal.channels[0].min == 300 && cal.channels[0].max == 3700);
	CHECK(cal.channels[0].center == 2005); // average of start (2000) and end (2010)
	CHECK(res.channel[3] == GP_CALRES_UNCHANGED);
	CHECK(cal.channels[3].min == 100 && cal.channels[3].max == 3900);
	gp_limits_t l = limits_a();
	CHECK(gp_calibration_validate(&cal, &l, NULL) == 0);
}

static void test_radial_fill(void) {
	uint16_t bins[GP_RADIAL_BINS], out[GP_RADIAL_BINS];
	for (int b = 0; b < GP_RADIAL_BINS; b++)
		bins[b] = 1500;
	bins[3] = bins[4] = bins[5] = 0; // gap of 3: interpolated
	CHECK(gp_calib_fill_radial(bins, out));
	CHECK(out[4] == 1500);
	bins[31] = bins[0] = 0; // wrap-around gap of 2
	CHECK(gp_calib_fill_radial(bins, out));
	bins[6] = 0; // gap of 4: rejected
	CHECK(!gp_calib_fill_radial(bins, out));
}

// ---------------------------------------------------------------------------
// LEDs / HID
// ---------------------------------------------------------------------------

static void test_led_quantize(void) {
	// dithering: over 256 frames the average equals the 16-bit value
	for (uint32_t v = 0; v < 65280; v += 397) {
		uint8_t	 ctr = 0;
		uint32_t sum = 0;
		for (int i = 0; i < 256; i++) {
			gp_led_dither_next(&ctr);
			sum += gp_led_quantize((uint16_t)v, ctr, true);
		}
		CHECK_NEAR(sum / 256.0, v / 256.0, 1.0 / 256.0 + 1e-9);
	}
	CHECK(gp_led_quantize(0xFFFF, 0, true) == 255);
	CHECK(gp_led_quantize(0x12FF, 7, false) == 0x12);
	// frame period: long chains slow the refresh so frames never overlap
	CHECK(gp_led_frame_us(true, 1) == GP_LED_FRAME_US_DITHER);
	CHECK(gp_led_frame_us(true, 64) == 64 * GP_LED_US_PER_LED + GP_LED_LATCH_US);
	CHECK(gp_led_frame_us(false, 4) == GP_LED_FRAME_US_PLAIN);
}

// One axis on LEDs 1..4 with the given effect; returns red per LED.
static void effect_run(uint8_t fx, gp_axis_led_t in, float out[4]) {
	gp_config_t c;
	gp_config_defaults(&c);
	c.axes[0].led_effect  = fx;
	c.axes[0].led_mask[0] = 0x0F;
	c.axes[0].color		  = (gp_color_t){ 255, 0, 0, 0 };
	gp_rgbf_t	  acc[GP_MAX_LEDS];
	gp_axis_led_t ax[GP_MAX_AXES];
	uint32_t	  btn[2] = { 0, 0 };
	memset(acc, 0, sizeof(acc));
	memset(ax, 0, sizeof(ax));
	ax[0] = in;
	static const uint8_t one_chain[GP_MAX_LEDS] = { 0 };
	gp_led_compose_inputs(&c, 1, 0, 4, one_chain, ax, btn, acc);
	for (int i = 0; i < 4; i++)
		out[i] = acc[i].r;
}

// A 4-LED chain (pin 1) plus a single LED (pin 2), both selected: the effect
// runs on the chain only, the single LED glows as an indicator.
static void test_led_effect_per_pin(void) {
	gp_config_t c;
	gp_config_defaults(&c);
	c.axes[0].led_effect  = GP_LED_FX_DOT;
	c.axes[0].led_mask[0] = 0x1F;
	c.axes[0].color		  = (gp_color_t){ 255, 0, 0, 0 };
	static const uint8_t pins[GP_MAX_LEDS] = { 0, 0, 0, 0, 1 };
	gp_rgbf_t			 acc[GP_MAX_LEDS];
	gp_axis_led_t		 ax[GP_MAX_AXES];
	uint32_t			 btn[2] = { 0, 0 };
	memset(acc, 0, sizeof(acc));
	memset(ax, 0, sizeof(ax));
	ax[0] = (gp_axis_led_t){ 0.5f, 0.5f, true }; // 3/4 along the chain
	gp_led_compose_inputs(&c, 1, 0, 5, pins, ax, btn, acc);
	CHECK(acc[0].r == 0 && acc[1].r == 0);
	CHECK_NEAR(acc[2].r + acc[3].r, 1.0, 1e-5); // dot on the chain (pos 2.25)
	CHECK_NEAR(acc[2].r, 0.75, 1e-5);
	CHECK_NEAR(acc[4].r, 0.25, 1e-5); // indicator: glow = deflection squared
	// at rest the indicator is dark, the dot sits in the middle of the chain
	ax[0] = (gp_axis_led_t){ 0, 0, true };
	memset(acc, 0, sizeof(acc));
	gp_led_compose_inputs(&c, 1, 0, 5, pins, ax, btn, acc);
	CHECK(acc[4].r == 0);
	CHECK_NEAR(acc[1].r, 0.5, 1e-5);
	CHECK_NEAR(acc[2].r, 0.5, 1e-5);
}

static void test_led_effects(void) {
	float o[4];
	effect_run(GP_LED_FX_GLOW, (gp_axis_led_t){ 0.5f, 0.5f, true }, o);
	for (int i = 0; i < 4; i++)
		CHECK_NEAR(o[i], 0.25, 1e-5);
	// bar, centred: rest = dark, half right = LED 3, full left = LEDs 1+2
	effect_run(GP_LED_FX_BAR, (gp_axis_led_t){ 0, 0, true }, o);
	CHECK(o[0] == 0 && o[1] == 0 && o[2] == 0 && o[3] == 0);
	effect_run(GP_LED_FX_BAR, (gp_axis_led_t){ 0.5f, 0.5f, true }, o);
	CHECK(o[0] == 0 && o[1] == 0 && fabsf(o[2] - 1) < 1e-5f && o[3] == 0);
	effect_run(GP_LED_FX_BAR, (gp_axis_led_t){ 1, -1, true }, o);
	CHECK(fabsf(o[0] - 1) < 1e-5f && fabsf(o[1] - 1) < 1e-5f && o[2] == 0 && o[3] == 0);
	// bar, linear: 0.625 -> 2.5 LEDs
	effect_run(GP_LED_FX_BAR, (gp_axis_led_t){ 0.625f, 0.625f, false }, o);
	CHECK_NEAR(o[0], 1, 1e-5);
	CHECK_NEAR(o[1], 1, 1e-5);
	CHECK_NEAR(o[2], 0.5, 1e-5);
	CHECK(o[3] == 0);
	// dot: centre of 4 LEDs = half on LEDs 2 and 3; ends = one LED
	effect_run(GP_LED_FX_DOT, (gp_axis_led_t){ 0, 0, true }, o);
	CHECK(o[0] == 0 && fabsf(o[1] - 0.5f) < 1e-5f && fabsf(o[2] - 0.5f) < 1e-5f && o[3] == 0);
	effect_run(GP_LED_FX_DOT, (gp_axis_led_t){ 1, 1, true }, o);
	CHECK(o[0] == 0 && o[1] == 0 && o[2] == 0 && fabsf(o[3] - 1) < 1e-5f);
	effect_run(GP_LED_FX_DOT, (gp_axis_led_t){ 0, 0, false }, o);
	CHECK(fabsf(o[0] - 1) < 1e-5f && o[1] == 0);
	// the total brightness of the dot is constant
	for (float p = -1.0f; p <= 1.0f; p += 0.05f) {
		effect_run(GP_LED_FX_DOT, (gp_axis_led_t){ fabsf(p), p, true }, o);
		CHECK_NEAR(o[0] + o[1] + o[2] + o[3], 1.0, 1e-4);
	}
}

static void test_hid(void) {
	gp_config_t c;
	gp_config_defaults(&c);
	uint8_t	 d[GP_HID_DESC_MAX];
	uint16_t n = gp_hid_descriptor(&c, d, sizeof(d));
	CHECK(n > 0 && d[0] == 0x05 && d[n - 1] == 0xC0);
	CHECK(d[7] == c.hid_report_id);
	c.num_buttons = 64;
	c.num_axes	  = 8;
	CHECK(gp_hid_descriptor(&c, d, sizeof(d)) > 0); // largest layout fits

	gp_config_defaults(&c);
	c.num_buttons = 7; // 7 buttons + 1 padding bit, 4 axes
	uint32_t btn[2]		   = { 0xFFFFFFFFu, 0 };
	int16_t	 axes[8]	   = { 32767, -32767, 0, 1 };
	uint8_t	 r[GP_HID_REPORT_MAX];
	uint16_t len = gp_hid_report(&c, btn, axes, r);
	CHECK(len == 1 + 8);
	CHECK(r[0] == 0x7F); // padding bit cleared
	CHECK(r[1] == 0xFF && r[2] == 0x7F && r[3] == 0x01 && r[4] == 0x80);
}

// ---------------------------------------------------------------------------
// Application level (commands over the framed protocol)
// ---------------------------------------------------------------------------

typedef struct {
	uint8_t	 cmd;
	uint16_t len;
	uint8_t	 data[GP_PROTO_MAX_PAYLOAD];
	int		 got;
} reply_t;

static void on_reply(void* ctx, uint8_t cmd, const uint8_t* p, uint16_t len) {
	reply_t* r = ctx;
	r->cmd	   = cmd;
	r->len	   = len;
	memcpy(r->data, p, len);
	r->got++;
}

static void advance(uint32_t us) {
	uint32_t end = hal_host.time_us + us;
	while ((int32_t)(end - hal_host.time_us) > 0) {
		hal_host.time_us += 100;
		gp_app_task();
		if (hal_host.reboot_requested)
			return;
	}
}

static reply_t* cmd(uint8_t c, const void* payload, uint16_t len) {
	static reply_t		 r;
	static gp_frame_rx_t rx;
	static uint8_t		 buf[GP_PROTO_MAX_PAYLOAD + GP_FRAME_OVERHEAD];
	memset(&r, 0, sizeof(r));
	size_t n = gp_frame_encode(buf, c, payload, len, NULL, 0);
	gp_app_cdc_rx(buf, n);
	gp_frame_rx_init(&rx);
	gp_frame_rx_feed(&rx, hal_host.cdc_out, hal_host.cdc_out_len, on_reply, &r);
	hal_host.cdc_out_len = 0;
	return &r;
}

static void boot(bool safe) {
	hal_host_reset();
	hal_host.manual_time = true;
	hal_host.time_us	 = 1000;
	gp_boot_info_t b	 = { .reset_reason = GP_RESET_POWER_ON, .safe_mode = safe };
	gp_app_init(&b);
	advance(20000);
}

static gp_hello_t hello(void) {
	gp_hello_t h;
	reply_t*   r = cmd(GP_CMD_HELLO, NULL, 0);
	memset(&h, 0, sizeof(h));
	if (r->got && r->len == 1 + sizeof(h))
		memcpy(&h, r->data + 1, sizeof(h));
	return h;
}

static gp_config_t read_config(void) {
	gp_config_t c;
	reply_t*	r = cmd(GP_CMD_READ_CONFIG, NULL, 0);
	memset(&c, 0, sizeof(c));
	if (r->got && r->data[0] == GP_STATUS_OK && r->len == 1 + sizeof(c))
		memcpy(&c, r->data + 1, sizeof(c));
	return c;
}

static gp_telemetry_t telemetry(void) {
	gp_telemetry_t t;
	reply_t*	   r = cmd(GP_CMD_TELEMETRY, NULL, 0);
	memset(&t, 0, sizeof(t));
	if (r->got && r->len == 1 + sizeof(t))
		memcpy(&t, r->data + 1, sizeof(t));
	return t;
}

static void test_app_fresh_and_write(void) {
	hal_host_erase_flash();
	boot(false);
	gp_hello_t h = hello();
	CHECK(h.proto_version == GP_PROTO_VERSION && h.config_size == sizeof(gp_config_t));
	CHECK(h.flags & GP_HELLO_STORAGE_EMPTY);
	CHECK(!(h.flags & GP_HELLO_CONFIG_DEFAULTED));

	gp_config_t def;
	gp_config_defaults(&def);
	gp_config_t c = read_config();
	CHECK(memcmp(&c, &def, sizeof(c)) == 0);

	// invalid config -> errors, nothing stored
	c.buttons[1].pin = c.buttons[0].pin;
	reply_t* r		 = cmd(GP_CMD_VALIDATE_CONFIG, &c, sizeof(c));
	CHECK(r->got && r->data[0] == GP_STATUS_INVALID && r->data[1] >= 1);
	r = cmd(GP_CMD_WRITE_CONFIG, &c, sizeof(c));
	CHECK(r->got && r->data[0] == GP_STATUS_INVALID);
	CHECK(hal_host.writes == 0);
	r = cmd(GP_CMD_WRITE_CONFIG, &c, 10);
	CHECK(r->got && r->data[0] == GP_STATUS_BAD_LENGTH);

	// valid config -> stored, reboot, read back identical
	gp_config_defaults(&c);
	c.usb_pid = 0x1234;
	strcpy(c.usb_product, "test stick");
	c.axes[2].flags = GP_AXIS_LINEAR;
	r				= cmd(GP_CMD_WRITE_CONFIG, &c, sizeof(c));
	CHECK(r->got && r->data[0] == GP_STATUS_OK && r->data[1] == 0);
	CHECK(!hal_host.reboot_requested);
	advance(300000);
	CHECK(hal_host.reboot_requested);
	boot(false);
	gp_config_t back = read_config();
	CHECK(memcmp(&back, &c, sizeof(c)) == 0);
	h = hello();
	CHECK(!(h.flags & (GP_HELLO_STORAGE_EMPTY | GP_HELLO_CONFIG_DEFAULTED)));
	CHECK(h.storage_seq == 1);
}

static void test_app_corrupt_stored_config(void) {
	// A config that passes CRC but violates the rules (the original firmware
	// would index out of bounds with it) must be replaced by defaults.
	hal_host_reset();
	hal_host_erase_flash();
	gp_config_t		   c;
	gp_calibration_t   cal;
	gp_storage_state_t st;
	gp_config_defaults(&c);
	c.num_led_pins = 200;
	c.num_axes = 99;
	gp_calibration_defaults(&cal);
	gp_storage_load(&c, &cal, &st); // fresh state
	gp_config_defaults(&c);
	c.num_led_pins = 200;
	c.num_axes = 99;
	CHECK(gp_storage_save(&c, &cal, &st));
	boot(false);
	gp_hello_t h = hello();
	CHECK(h.flags & GP_HELLO_CONFIG_DEFAULTED);
	gp_config_t back = read_config();
	gp_config_defaults(&c);
	CHECK(memcmp(&back, &c, sizeof(c)) == 0);
	advance(100000); // runs without crashing (ASan would catch overflows)
}

static void test_app_calibration_flow(void) {
	hal_host_erase_flash();
	boot(false);
	gp_config_t c = read_config();
	c.groups[0].enabled = 1;
	CHECK(cmd(GP_CMD_WRITE_CONFIG, &c, sizeof(c))->data[0] == GP_STATUS_OK);
	advance(300000);
	boot(false);

	CHECK(cmd(GP_CMD_CAL_FINISH, NULL, 0)->data[0] == GP_STATUS_BAD_STATE);
	CHECK(cmd(GP_CMD_CAL_START, NULL, 0)->data[0] == GP_STATUS_OK);
	gp_telemetry_t t = telemetry();
	CHECK(t.state & GP_STATE_CALIBRATING);
	reply_t* r = cmd(GP_CMD_WRITE_CONFIG, &c, sizeof(c));
	CHECK(r->data[0] == GP_STATUS_BAD_STATE);

	// stick 1 (ADC 0/1), wiring variant 6; throttle on ADC 2
	for (int i = 0; i <= 50; i++) {
		wire(6, 0, i / 50.0, &hal_host.adc[0], &hal_host.adc[1]);
		advance(3000);
	}
	for (int i = 0; i <= 500; i++) {
		double a = 3.14159265 / 2 - (double)i / 400 * 2 * 3.14159265;
		wire(6, cos(a), sin(a), &hal_host.adc[0], &hal_host.adc[1]);
		hal_host.adc[2] = (uint16_t)(200 + i * 7);
		advance(3000);
	}
	wire(6, 0, 0, &hal_host.adc[0], &hal_host.adc[1]);
	hal_host.adc[2] = 2048;
	advance(200000);
	t = telemetry();
	CHECK(t.cal_group_state[0] == GP_CALGRP_DONE);
	CHECK(t.cal_channel_state[2] == GP_CALCH_OK);
	CHECK(t.cal_channel_state[3] == GP_CALCH_IDLE);

	r = cmd(GP_CMD_CAL_FINISH, NULL, 0);
	CHECK(r->data[0] == GP_STATUS_OK && r->len == 1 + sizeof(gp_cal_result_t));
	gp_cal_result_t res;
	memcpy(&res, r->data + 1, sizeof(res));
	CHECK(res.group[0] == GP_CALRES_UPDATED && res.channel[2] == GP_CALRES_UPDATED && res.channel[3] == GP_CALRES_UNCHANGED);

	// stick output: push right -> HID X +, push up -> HID Y - (HID Y points down)
	wire(6, 1, 0, &hal_host.adc[0], &hal_host.adc[1]);
	advance(100000);
	t = telemetry();
	CHECK(t.axes[0] > 30000 && abs(t.axes[1]) < 1500);
	wire(6, 0, 1, &hal_host.adc[0], &hal_host.adc[1]);
	advance(100000);
	t = telemetry();
	CHECK(t.axes[1] < -30000 && abs(t.axes[0]) < 1500);

	// calibration survives a reboot
	boot(false);
	wire(6, 1, 0, &hal_host.adc[0], &hal_host.adc[1]);
	advance(100000);
	t = telemetry();
	CHECK(t.axes[0] > 30000);
}

static void test_app_safe_mode(void) {
	hal_host_erase_flash();
	boot(false);
	gp_config_t c = read_config();
	c.usb_pid	  = 0x7777;
	CHECK(cmd(GP_CMD_WRITE_CONFIG, &c, sizeof(c))->data[0] == GP_STATUS_OK);
	advance(300000);

	boot(true);
	gp_hello_t h = hello();
	CHECK(h.flags & GP_HELLO_SAFE_MODE);
	CHECK(read_config().usb_pid != 0x7777); // defaults active
	CHECK(telemetry().state & GP_STATE_SAFE_MODE);
	// changing calibration in safe mode keeps the stored configuration
	CHECK(cmd(GP_CMD_RESET_CALIBRATION, NULL, 0)->data[0] == GP_STATUS_OK);
	boot(false);
	CHECK(read_config().usb_pid == 0x7777);
}

static void test_led_chains(void) {
	// LED pins 1 (built-in) + a chain of 4 on pin 2: LEDs are numbered 1, 2..5
	hal_host_erase_flash();
	boot(false);
	gp_config_t c = read_config();
	c.num_led_pins	 = 2;
	c.led_counts[1] = 4;
	c.brightness	= 1000;
	c.led_flags		= 0;
	c.status_led_mask[0] = 0;
	for (int i = 0; i < c.num_axes; i++)
		c.axes[i].led_mask[0] = 0;
	for (int i = 0; i < c.num_buttons; i++)
		c.buttons[i].led_mask[0] = 0;
	c.buttons[0].led_mask[0] = 1u << 4; // last LED of the chain
	c.buttons[0].color		 = (gp_color_t){ 0, 0, 255, 0 };
	CHECK(cmd(GP_CMD_WRITE_CONFIG, &c, sizeof(c))->data[0] == GP_STATUS_OK);
	boot(false);
	CHECK(hal_host.led_count[0] == 1 && hal_host.led_count[1] == 4 && hal_host.led_first[1] == 1);
	hal_host.pin_low[c.buttons[0].pin] = true;
	advance(30000);
	CHECK(hal_host.led_rgb[4][2] == 255);
	CHECK(hal_host.led_rgb[1][2] == 0 && hal_host.led_rgb[0][2] == 0);
	CHECK(telemetry().led_rgb[4][2] == 255);
	hal_host.pin_low[c.buttons[0].pin] = false;
	hal_host_erase_flash();
}

static void test_app_preview_and_sleep(void) {
	hal_host_erase_flash();
	boot(false);
	gp_config_t c = read_config();
	c.brightness  = 1000;
	c.status_led_mask[0] = 0x01;
	c.buttons[0].color.r = 255;
	c.buttons[0].color.g = 0;
	c.buttons[0].color.b = 0;
	c.buttons[0].led_mask[0] = 0x01;
	CHECK(cmd(GP_CMD_LED_PREVIEW, &c, sizeof(c))->data[0] == GP_STATUS_OK);
	CHECK(telemetry().state & GP_STATE_PREVIEW);
	hal_host.pin_low[c.buttons[0].pin] = true;
	advance(20000);
	CHECK(telemetry().led_rgb[0][0] == 255);
	advance((GP_PREVIEW_TIMEOUT_MS + 100) * 1000u);
	CHECK(!(telemetry().state & GP_STATE_PREVIEW));
	hal_host.pin_low[c.buttons[0].pin] = false;

	// host gone for > 5 s: LEDs dark
	hal_host.mounted = false;
	advance(6000000);
	CHECK(telemetry().state & GP_STATE_SLEEPING);
	for (int l = 0; l < 3; l++)
		CHECK(hal_host.led_rgb[l][0] == 0 && hal_host.led_rgb[l][1] == 0 && hal_host.led_rgb[l][2] == 0);
	hal_host.mounted   = true;
	hal_host.suspended = true;
	advance(100000);
	CHECK(telemetry().state & GP_STATE_SLEEPING);
	hal_host.suspended = false;
	advance(100000);
	CHECK(!(telemetry().state & GP_STATE_SLEEPING));
}

static void test_app_hid_rate_and_factory_reset(void) {
	hal_host_erase_flash();
	boot(false);
	uint32_t before = hal_host.reports_sent;
	advance(1000000);
	uint32_t sent = hal_host.reports_sent - before;
	CHECK(sent >= 245 && sent <= 255); // 250 Hz default

	gp_config_t c = read_config();
	CHECK(cmd(GP_CMD_WRITE_CONFIG, &c, sizeof(c))->data[0] == GP_STATUS_OK);
	advance(300000);
	boot(false);
	CHECK(!(hello().flags & GP_HELLO_STORAGE_EMPTY));
	CHECK(cmd(GP_CMD_FACTORY_RESET, NULL, 0)->data[0] == GP_STATUS_OK);
	CHECK(cmd(GP_CMD_READ_CONFIG, NULL, 0)->data[0] == GP_STATUS_BAD_STATE); // pending reboot
	advance(300000);
	CHECK(hal_host.reboot_requested);
	boot(false);
	CHECK(hello().flags & GP_HELLO_STORAGE_EMPTY);
}

static void test_app_button_reset_hold(void) {
	hal_host_erase_flash();
	boot(false);
	gp_config_t c = read_config();
	c.cal_pin	  = 8;
	CHECK(cmd(GP_CMD_WRITE_CONFIG, &c, sizeof(c))->data[0] == GP_STATUS_OK);
	advance(300000);
	boot(false);
	// hold the calibration button: calibration starts after 1 s
	hal_host.pin_low[c.cal_pin] = true;
	advance(1100000);
	CHECK(telemetry().state & GP_STATE_CALIBRATING);
	// keep holding to the reset time, then release during the warning
	advance(9000000);
	CHECK(telemetry().state & GP_STATE_RESET_PENDING);
	hal_host.pin_low[c.cal_pin] = false;
	advance(100000);
	CHECK(!(telemetry().state & GP_STATE_RESET_PENDING));
	CHECK(!hal_host.reboot_requested);
	// a short press finishes calibration
	hal_host.pin_low[c.cal_pin] = true;
	advance(100000);
	hal_host.pin_low[c.cal_pin] = false;
	advance(100000);
	CHECK(!(telemetry().state & GP_STATE_CALIBRATING));
	// holding all the way through the warning resets the device
	hal_host.pin_low[c.cal_pin] = true;
	advance(13000000);
	CHECK(hal_host.reboot_requested);
	boot(false);
	CHECK(hello().flags & GP_HELLO_STORAGE_EMPTY);
}

static void test_app_command_fuzz(void) {
	hal_host_erase_flash();
	boot(false);
	uint8_t payload[GP_PROTO_MAX_PAYLOAD];
	for (int k = 0; k < 3000; k++) {
		uint8_t c = (uint8_t)(rnd() % 0x20);
		if (c == GP_CMD_WRITE_CONFIG || c == GP_CMD_FACTORY_RESET || c == GP_CMD_REBOOT)
			continue;
		uint16_t len = (uint16_t)(rnd() % 2 ? rnd() % 64 : (rnd() % 2 ? sizeof(gp_config_t) : sizeof(gp_calibration_t)));
		for (int i = 0; i < len; i++)
			payload[i] = (uint8_t)rnd();
		if (len == sizeof(gp_config_t) && rnd() % 2) { // mostly-valid configs reach deeper code
			gp_config_t cfg;
			gp_config_defaults(&cfg);
			for (int i = 0; i < 4; i++)
				((uint8_t*)&cfg)[8 + rnd() % (sizeof(cfg) - 8)] = (uint8_t)rnd();
			memcpy(payload, &cfg, sizeof(cfg));
		}
		cmd(c, payload, len);
		advance(500);
		if (hal_host.reboot_requested)
			boot(false);
	}
	CHECK(hello().proto_version == GP_PROTO_VERSION);
}

int main(void) {
	hal_host_reset();
	RUN(test_crc32);
	RUN(test_frames);
	RUN(test_config_defaults_valid);
	RUN(test_config_invalid);
	RUN(test_config_fuzz);
	RUN(test_storage_basic);
	RUN(test_storage_power_loss);
	RUN(test_storage_seq_wrap);
	RUN(test_axis_math);
	RUN(test_calibration_orientations);
	RUN(test_calibration_incomplete);
	RUN(test_calibration_false_start);
	RUN(test_calibration_noisy_deflected_start);
	RUN(test_calibration_standalone);
	RUN(test_radial_fill);
	RUN(test_led_quantize);
	RUN(test_led_effects);
	RUN(test_led_effect_per_pin);
	RUN(test_led_chains);
	RUN(test_hid);
	RUN(test_app_fresh_and_write);
	RUN(test_app_corrupt_stored_config);
	RUN(test_app_calibration_flow);
	RUN(test_app_safe_mode);
	RUN(test_app_preview_and_sleep);
	RUN(test_app_hid_rate_and_factory_reset);
	RUN(test_app_button_reset_hold);
	RUN(test_app_command_fuzz);
	printf("\n%d checks, %d failures\n", checks, failures);
	return failures ? 1 : 0;
}
