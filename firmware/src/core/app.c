// SPDX-License-Identifier: MIT
#include "app.h"

#include <math.h>
#include <string.h>

#include "axis.h"
#include "calib.h"
#include "config.h"
#include "frame.h"
#include "hal.h"
#include "hid.h"
#include "input.h"
#include "leds.h"
#include "storage.h"

#ifndef GP_FW_VERSION
#define GP_FW_VERSION "dev"
#endif

#define RESET_WARN_MS	 2000 // blinking period before a button factory reset
#define REBOOT_DELAY_MS	 150  // lets the USB stack deliver the response first

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

static gp_boot_info_t	  boot;
static gp_config_t		  cfg;		   // active configuration (applied at boot)
static gp_config_t		  persist_cfg; // configuration to store alongside calibration changes
static gp_calibration_t	  cal;		   // active calibration (changes live)
static gp_limits_t		  limits;
static gp_storage_state_t storage;
static uint16_t			  hello_flags;

// inputs
static gp_filter_t	 filter;
static gp_debounce_t btn_db[GP_MAX_BUTTONS];
static gp_debounce_t cal_db;
static uint16_t		 raw_now[GP_MAX_AXES];
static uint16_t		 raw_f[GP_MAX_AXES];
static float		 axis_out[GP_MAX_AXES]; // HID orientation, -1..1
static gp_axis_led_t axis_led[GP_MAX_AXES]; // what each axis shows on its LEDs
static int16_t		 hid_axes[GP_MAX_AXES];
static uint32_t		 buttons[2];
static bool			 inputs_valid;

typedef struct {
	bool				  on;
	uint8_t				  slot_a, slot_b; // axis slots feeding dx / dy
	uint16_t			  center_a, center_b;
	const gp_cal_group_t* rec; // NULL = uncalibrated
} group_rt_t;
static group_rt_t groups[GP_MAX_GROUPS];

// calibration
static gp_calib_t calib;

// calibration / reset button
static bool		cal_btn_prev;
static uint32_t cal_btn_since;
static bool		cal_btn_consumed;
static bool		cal_btn_began_in_cal;
static bool		reset_warn;
static uint32_t reset_warn_since;

// LEDs
static uint8_t		  dither_counter;
static uint8_t		  led_total;   // LEDs on all pins
static uint8_t		  led_longest; // longest chain
static uint8_t		  led_pin_of[GP_MAX_LEDS]; // LED pin of each LED
static gp_config_t	  preview_cfg;
static bool			  preview_on;
static uint32_t		  preview_since;
static uint8_t		  led_display[GP_MAX_LEDS][4];

// timing
static uint32_t now_us;
static uint32_t last_poll, last_hid, last_led;
static uint64_t uptime_us;
static uint32_t last_task_us;
static uint32_t loop_max_cur, loop_max_last;
static uint32_t stats_since;
static uint16_t hid_count_cur, hid_count_last;

// sleep
static uint32_t last_awake;
static bool		sleeping;

// pending reboot / reset
enum { PENDING_NONE, PENDING_REBOOT, PENDING_FACTORY_RESET };
static int		pending;
static uint32_t pending_at;

// protocol
static gp_frame_rx_t	rx;
static uint8_t			tx_buf[GP_PROTO_MAX_PAYLOAD + GP_FRAME_OVERHEAD];
static gp_config_t		scratch_cfg;
static gp_calibration_t scratch_cal;
static gp_error_list_t	errs;
static uint32_t			tx_dropped;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

static inline bool elapsed(uint32_t since, uint32_t us) { return (uint32_t)(now_us - since) >= us; }

static bool calibrating(void) { return calib.active; }

static void schedule(int what) {
	pending	   = what;
	pending_at = now_us;
}

static int adc_channel_of(uint8_t pin) { return hal_adc_channel(pin); }

static const gp_cal_channel_t* channel_cal(uint8_t pin) {
	int ch = hal_adc_channel(pin);
	return (ch >= 0 && ch < GP_MAX_AXES) ? &cal.channels[ch] : NULL;
}

static uint8_t slot_of_pin(uint8_t pin, uint8_t a, uint8_t b) { return cfg.axes[a].pin == pin ? a : b; }

// Binds every enabled stick group to its calibration record (if any).
static void resolve_groups(void) {
	for (int g = 0; g < GP_MAX_GROUPS; g++) {
		group_rt_t*			  rt = &groups[g];
		const gp_group_cfg_t* gc = &cfg.groups[g];
		memset(rt, 0, sizeof(*rt));
		if (!gc->enabled || gc->axis_h >= cfg.num_axes || gc->axis_v >= cfg.num_axes || gc->axis_h == gc->axis_v)
			continue;
		rt->on		  = true;
		uint8_t pin_h = cfg.axes[gc->axis_h].pin, pin_v = cfg.axes[gc->axis_v].pin;
		for (int k = 0; k < GP_MAX_GROUPS && !rt->rec; k++) {
			const gp_cal_group_t* r = &cal.groups[(g + k) % GP_MAX_GROUPS];
			if (r->calibrated &&
				((r->pin_a == pin_h && r->pin_b == pin_v) || (r->pin_a == pin_v && r->pin_b == pin_h)))
				rt->rec = r;
		}
		if (rt->rec) {
			rt->slot_a	 = slot_of_pin(rt->rec->pin_a, gc->axis_h, gc->axis_v);
			rt->slot_b	 = slot_of_pin(rt->rec->pin_b, gc->axis_h, gc->axis_v);
			rt->center_a = rt->rec->center_a;
			rt->center_b = rt->rec->center_b;
		} else {
			// Uncalibrated stick: identity orientation, centre from the
			// per-axis calibration if available.
			rt->slot_a	 = gc->axis_h;
			rt->slot_b	 = gc->axis_v;
			rt->center_a = gp_channel_span(channel_cal(pin_h)).center;
			rt->center_b = gp_channel_span(channel_cal(pin_v)).center;
		}
	}
}

static void init_inputs(void) {
	gp_filter_init(&filter, cfg.smoothing_samples, cfg.smoothing_mode);
	for (int i = 0; i < cfg.num_axes; i++)
		hal_adc_init(cfg.axes[i].pin);
	uint32_t db = (uint32_t)cfg.debounce_ms * 1000u;
	for (int i = 0; i < cfg.num_buttons; i++) {
		hal_button_init(cfg.buttons[i].pin);
		gp_debounce_init(&btn_db[i], now_us, db);
	}
	if (cfg.cal_pin != GP_PIN_NONE)
		hal_button_init(cfg.cal_pin);
	gp_debounce_init(&cal_db, now_us, db);
	led_total = led_longest = 0;
	for (int i = 0; i < cfg.num_led_pins; i++) {
		hal_led_init((uint8_t)i, cfg.led_pins[i], cfg.led_counts[i]);
		if (cfg.led_counts[i] > led_longest)
			led_longest = cfg.led_counts[i];
	}
	led_total	   = (uint8_t)gp_led_total(&cfg);
	for (int i = 0, l = 0; i < cfg.num_led_pins; i++)
		for (int k = 0; k < cfg.led_counts[i] && l < GP_MAX_LEDS; k++)
			led_pin_of[l++] = (uint8_t)i;
	dither_counter = 0;
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

// Every piece of runtime state is reset here so that a re-init (simulated
// reboot in host tests) behaves exactly like a fresh boot.
static void reset_state(void) {
	memset(&filter, 0, sizeof(filter));
	memset(btn_db, 0, sizeof(btn_db));
	memset(&cal_db, 0, sizeof(cal_db));
	memset(raw_now, 0, sizeof(raw_now));
	memset(raw_f, 0, sizeof(raw_f));
	memset(axis_out, 0, sizeof(axis_out));
	memset(axis_led, 0, sizeof(axis_led));
	memset(hid_axes, 0, sizeof(hid_axes));
	memset(buttons, 0, sizeof(buttons));
	inputs_valid = false;
	memset(groups, 0, sizeof(groups));
	memset(&calib, 0, sizeof(calib));
	cal_btn_prev = cal_btn_consumed = cal_btn_began_in_cal = reset_warn = false;
	cal_btn_since = reset_warn_since = 0;
	memset(&preview_cfg, 0, sizeof(preview_cfg));
	preview_on	  = false;
	preview_since = 0;
	memset(led_display, 0, sizeof(led_display));
	uptime_us	  = 0;
	loop_max_cur = loop_max_last = 0;
	hid_count_cur = hid_count_last = 0;
	sleeping					   = false;
	pending						   = PENDING_NONE;
	pending_at					   = 0;
	tx_dropped					   = 0;
}

void gp_app_init(const gp_boot_info_t* b) {
	reset_state();
	boot   = *b;
	now_us = hal_time_us();
	gp_limits_build(&limits);
	gp_storage_load(&cfg, &cal, &storage);
	persist_cfg = cfg;

	hello_flags = 0;
	if (boot.simulator)
		hello_flags |= GP_HELLO_SIMULATOR;
	if (storage.empty)
		hello_flags |= GP_HELLO_STORAGE_EMPTY;
	if (storage.recovered)
		hello_flags |= GP_HELLO_STORAGE_RECOVERED;

	// Stored data is validated against this firmware's rules; anything that
	// does not pass is replaced by defaults so the device always boots.
	if (!storage.config_loaded || gp_config_validate(&cfg, &limits, NULL) != 0) {
		if (!storage.empty)
			hello_flags |= GP_HELLO_CONFIG_DEFAULTED;
		gp_config_defaults(&cfg);
		persist_cfg = cfg;
	}
	if (!storage.cal_loaded || gp_calibration_validate(&cal, &limits, NULL) != 0) {
		if (!storage.empty)
			hello_flags |= GP_HELLO_CAL_DEFAULTED;
		gp_calibration_defaults(&cal);
	}
	if (boot.safe_mode) {
		// Repeated crashes: run with defaults but keep what is stored, so
		// saving calibration does not silently replace the stored config.
		hello_flags |= GP_HELLO_SAFE_MODE;
		gp_config_defaults(&cfg);
	}
	// Defaults themselves must be valid for this platform; if not (e.g. a
	// board with fewer pins), fall back to a minimal empty configuration.
	if (gp_config_validate(&cfg, &limits, NULL) != 0) {
		cfg.num_axes = cfg.num_buttons = cfg.num_led_pins = 0;
		cfg.cal_pin									  = GP_PIN_NONE;
		cfg.status_led_mask[0] = cfg.status_led_mask[1] = 0;
		for (int g = 0; g < GP_MAX_GROUPS; g++)
			cfg.groups[g].enabled = 0;
		if (!boot.safe_mode)
			persist_cfg = cfg;
	}

	gp_frame_rx_init(&rx);
	init_inputs();
	resolve_groups();
	last_poll = last_hid = last_led = stats_since = last_task_us = now_us;
	last_awake												   = now_us;
}

const gp_config_t* gp_app_config(void) { return &cfg; }
bool			   gp_app_sleeping(void) { return sleeping && !(cfg.led_flags & GP_LED_KEEP_ON_SLEEP); }
bool			   gp_app_calibrating(void) { return calib.active; }

// ---------------------------------------------------------------------------
// Input processing
// ---------------------------------------------------------------------------

static void process_axes(void) {
	bool grouped[GP_MAX_AXES] = { false };
	for (int g = 0; g < GP_MAX_GROUPS; g++) {
		group_rt_t* rt = &groups[g];
		if (!rt->on)
			continue;
		const gp_group_cfg_t* gc = &cfg.groups[g];
		bool				  custom = gc->flags & GP_GROUP_CUSTOM_DZ;
		gp_group_params_t	  p		 = {
				   .radial	   = rt->rec ? rt->rec->radial : NULL,
				   .orient	   = rt->rec ? rt->rec->orient : 0,
				   .user_flags = gc->flags,
				   .shape	   = gc->shape,
				   .center_dz  = (custom ? gc->center_dz : cfg.center_dz) * 0.001f,
				   .outer_dz   = (custom ? gc->outer_dz : cfg.outer_dz) * 0.001f,
		   };
		float dx = (float)raw_f[rt->slot_a] - (float)rt->center_a;
		float dy = (float)raw_f[rt->slot_b] - (float)rt->center_b;
		float h, v;
		gp_group_compute(dx, dy, &p, &h, &v);
		axis_out[gc->axis_h] = h;
		axis_out[gc->axis_v] = -v; // HID: positive Y points down
		axis_led[gc->axis_h] = (gp_axis_led_t){ fabsf(h), h, true };
		axis_led[gc->axis_v] = (gp_axis_led_t){ fabsf(v), v, true };
		grouped[gc->axis_h] = grouped[gc->axis_v] = true;
	}
	for (int i = 0; i < cfg.num_axes; i++) {
		if (grouped[i])
			continue;
		const gp_axis_cfg_t* a		= &cfg.axes[i];
		bool				 custom = a->flags & GP_AXIS_CUSTOM_DZ;
		float				 cdz	= (custom ? a->center_dz : cfg.center_dz) * 0.001f;
		float				 odz	= (custom ? a->outer_dz : cfg.outer_dz) * 0.001f;
		gp_span_t			 s		= gp_channel_span(channel_cal(a->pin));
		float				 v;
		bool				 linear = a->flags & GP_AXIS_LINEAR;
		v	= linear ? gp_axis_linear(raw_f[i], s, cdz, odz) : gp_axis_centered(raw_f[i], s, cdz, odz);
		axis_out[i] = (a->flags & GP_AXIS_INVERT) ? -v : v;
		if (linear) {
			float p		= (axis_out[i] + 1.0f) * 0.5f; // 0..1 in the output direction
			axis_led[i] = (gp_axis_led_t){ (v + 1.0f) * 0.5f, p, false };
		} else {
			axis_led[i] = (gp_axis_led_t){ fabsf(v), axis_out[i], true };
		}
	}
	for (int i = 0; i < cfg.num_axes; i++)
		hid_axes[i] = gp_axis_to_hid(axis_out[i]);
}

static void poll_inputs(void) {
	for (int i = 0; i < cfg.num_axes; i++)
		raw_now[i] = hal_adc_read(cfg.axes[i].pin);
	gp_filter_step(&filter, raw_now, raw_f, cfg.num_axes);

	uint32_t db = (uint32_t)cfg.debounce_ms * 1000u;
	buttons[0] = buttons[1] = 0;
	for (int i = 0; i < cfg.num_buttons; i++)
		if (gp_debounce_update(&btn_db[i], hal_button_pressed(cfg.buttons[i].pin), now_us, db))
			buttons[i / 32] |= 1u << (i % 32);
	if (cfg.cal_pin != GP_PIN_NONE)
		gp_debounce_update(&cal_db, hal_button_pressed(cfg.cal_pin), now_us, db);

	if (calibrating()) {
		gp_calib_update(&calib, &cfg, raw_f);
		memset(axis_out, 0, sizeof(axis_out));
		memset(axis_led, 0, sizeof(axis_led));
		memset(hid_axes, 0, sizeof(hid_axes)); // games see a centred device while calibrating
	} else {
		process_axes();
	}
	inputs_valid = true;
}

// ---------------------------------------------------------------------------
// Calibration control
// ---------------------------------------------------------------------------

static void cal_start(void) {
	if (calibrating())
		return;
	gp_calib_start(&calib, &cfg, raw_f);
}

// Returns a GP_STATUS_* code.
static uint8_t cal_finish(gp_cal_result_t* res) {
	gp_cal_result_t dummy;
	if (!res)
		res = &dummy;
	if (!calibrating())
		return GP_STATUS_BAD_STATE;
	gp_calib_finish(&calib, &cfg, raw_f, &cal, res, adc_channel_of);
	resolve_groups();
	hal_watchdog_feed();
	if (!gp_storage_save(&persist_cfg, &cal, &storage))
		return GP_STATUS_STORAGE; // stays active in RAM until reboot
	return GP_STATUS_OK;
}

static void cal_button(void) {
	if (cfg.cal_pin == GP_PIN_NONE)
		return;
	bool pressed = cal_db.state;
	if (pressed && !cal_btn_prev) {
		cal_btn_since		 = now_us;
		cal_btn_consumed	 = false;
		cal_btn_began_in_cal = calibrating();
	}
	if (pressed) {
		uint32_t held_ms = (now_us - cal_btn_since) / 1000u;
		if (!calibrating() && !cal_btn_consumed && held_ms >= cfg.cal_hold_ms) {
			cal_start();
			cal_btn_consumed = true;
		}
		if (cfg.reset_hold_ms && held_ms >= cfg.reset_hold_ms && !reset_warn) {
			reset_warn		 = true;
			reset_warn_since = now_us;
			cal_btn_consumed = true;
		}
		if (reset_warn && elapsed(reset_warn_since, RESET_WARN_MS * 1000u) && pending == PENDING_NONE) {
			gp_calib_cancel(&calib);
			schedule(PENDING_FACTORY_RESET);
		}
	} else if (cal_btn_prev) {
		if (reset_warn) {
			reset_warn = false; // released during the warning: cancelled
		} else if (cal_btn_began_in_cal && !cal_btn_consumed && calibrating()) {
			cal_finish(NULL);
		}
	}
	cal_btn_prev = pressed;
}

// ---------------------------------------------------------------------------
// LEDs
// ---------------------------------------------------------------------------

static const gp_config_t* led_look(void) { return preview_on ? &preview_cfg : &cfg; }

static void led_frame(void) {
	const gp_config_t* look = led_look();
	static gp_rgbf_t   acc[GP_MAX_LEDS];
	memset(acc, 0, sizeof(acc));
	int n = led_total;

	bool dark = gp_app_sleeping();
	if (reset_warn) {
		float on = ((now_us / 125000u) & 1u) ? 1.0f : 0.0f;
		for (int l = 0; l < n; l++)
			acc[l].r = on;
	} else if (!dark) {
		gp_rgbf_t st = { 0, 0, 0 };
		if (hello_flags & GP_HELLO_SAFE_MODE) {
			st.r = gp_led_breathe(now_us, 2000000);
		} else if (!hal_usb_mounted()) {
			st.b = gp_led_breathe(now_us, 2000000);
		} else if (calibrating()) {
			float f = gp_led_breathe(now_us, 600000);
			st.r = st.b = f;
		} else {
			float f = gp_led_breathe(now_us, 2000000) * look->status_brightness * 0.001f;
			st.r = st.g = st.b = f;
		}
		for (int l = 0; l < n; l++) {
			if ((look->status_led_mask[l / 32] >> (l % 32)) & 1u) {
				acc[l].r += st.r;
				acc[l].g += st.g;
				acc[l].b += st.b;
			}
		}
		if (!calibrating() && inputs_valid)
			gp_led_compose_inputs(look, cfg.num_axes, cfg.num_buttons, n, led_pin_of, axis_led, buttons, acc);
	}

	float bright = look->brightness * 0.001f;
	bool  dither = look->led_flags & GP_LED_DITHER;
	gp_led_dither_next(&dither_counter);
	static uint8_t out[GP_MAX_LEDS][3];
	for (int l = 0; l < n; l++) {
		float c[3] = { acc[l].r, acc[l].g, acc[l].b };
		for (int k = 0; k < 3; k++) {
			float v = c[k] > 1.0f ? 1.0f : (c[k] > 0.0f ? c[k] : 0.0f);
			led_display[l][k] = (uint8_t)(v * 255.0f + 0.5f);
			out[l][k]		  = gp_led_quantize((uint16_t)(v * bright * 65535.0f), dither_counter, dither);
		}
		led_display[l][3] = 0;
	}
	int first = 0;
	for (int p = 0; p < cfg.num_led_pins; p++) {
		hal_led_write((uint8_t)p, (const uint8_t(*)[3])&out[first], cfg.led_counts[p]);
		first += cfg.led_counts[p];
	}
}

// ---------------------------------------------------------------------------
// Protocol
// ---------------------------------------------------------------------------

static void respond(uint8_t cmd, uint8_t status, const void* body, uint16_t len) {
	if (len > GP_PROTO_MAX_PAYLOAD - 1)
		len = 0;
	size_t total = (size_t)len + 1 + GP_FRAME_OVERHEAD;
	if (hal_cdc_write_available() < total) {
		tx_dropped++; // host will time out and retry
		return;
	}
	size_t n = gp_frame_encode(tx_buf, cmd, &status, 1, body, len);
	hal_cdc_write(tx_buf, n);
	hal_cdc_flush();
}

static void respond_errors(uint8_t cmd, const gp_error_list_t* e) {
	// gp_error_list_t starts with the status byte, so it is sent as-is.
	uint16_t len = (uint16_t)(4 + e->count * sizeof(gp_error_t));
	if (hal_cdc_write_available() < (size_t)len + GP_FRAME_OVERHEAD) {
		tx_dropped++;
		return;
	}
	size_t n = gp_frame_encode(tx_buf, cmd, e, len, NULL, 0);
	hal_cdc_write(tx_buf, n);
	hal_cdc_flush();
}

static void single_error(uint8_t status, const char* msg) {
	memset(&errs, 0, sizeof(errs));
	errs.status = status;
	errs.count	= 1;
	errs.errors[0].index = 0xFF;
	strncpy(errs.errors[0].message, msg, GP_ERROR_MSG_LEN - 1);
}

static void build_hello(gp_hello_t* h) {
	memset(h, 0, sizeof(*h));
	h->proto_version  = GP_PROTO_VERSION;
	h->config_version = GP_CONFIG_VERSION;
	h->config_size	  = sizeof(gp_config_t);
	h->cal_version	  = GP_CAL_VERSION;
	h->cal_size		  = sizeof(gp_calibration_t);
	h->telemetry_size = sizeof(gp_telemetry_t);
	h->limits_size	  = sizeof(gp_limits_t);
	h->flags		  = hello_flags;
	h->storage_seq	  = storage.seq;
	hal_chip_id(h->chip_id);
	h->reset_reason = boot.reset_reason;
	h->crash_count	= boot.crash_count;
	strncpy(h->fw_version, GP_FW_VERSION, GP_STR_LEN - 1);
	strncpy(h->board, hal_board_name(), GP_STR_LEN - 1);
}

static void build_telemetry(gp_telemetry_t* t) {
	memset(t, 0, sizeof(*t));
	t->uptime_ms  = (uint32_t)(uptime_us / 1000u);
	t->buttons[0] = buttons[0];
	t->buttons[1] = buttons[1];
	for (int i = 0; i < cfg.num_axes; i++) {
		t->axes[i]	  = hid_axes[i];
		t->raw[i]	  = raw_f[i];
		t->raw_now[i] = raw_now[i];
		gp_span_t s	  = gp_channel_span(channel_cal(cfg.axes[i].pin));
		t->cal_min[i] = s.min;
		t->cal_center[i] = s.center;
		t->cal_max[i]	 = s.max;
	}
	if (calibrating())
		gp_calib_telemetry(&calib, &cfg, t);
	memcpy(t->led_rgb, led_display, sizeof(t->led_rgb));
	t->state = (uint8_t)((calibrating() ? GP_STATE_CALIBRATING : 0) | (hal_usb_mounted() ? GP_STATE_USB_MOUNTED : 0) |
						 (sleeping ? GP_STATE_SLEEPING : 0) | (preview_on ? GP_STATE_PREVIEW : 0) |
						 ((hello_flags & GP_HELLO_SAFE_MODE) ? GP_STATE_SAFE_MODE : 0) |
						 (cal_db.state ? GP_STATE_CAL_BUTTON : 0) | (reset_warn ? GP_STATE_RESET_PENDING : 0));
	t->loop_max_us		 = (uint16_t)(loop_max_last > 65535 ? 65535 : loop_max_last);
	t->hid_reports_per_s = hid_count_last;
}

static bool payload_is(uint16_t len, size_t want, uint8_t cmd) {
	if (len == want)
		return true;
	respond(cmd, GP_STATUS_BAD_LENGTH, NULL, 0);
	return false;
}

static void handle_frame(void* ctx, uint8_t cmd, const uint8_t* p, uint16_t len) {
	(void)ctx;
	if (pending != PENDING_NONE && cmd != GP_CMD_HELLO && cmd != GP_CMD_TELEMETRY) {
		respond(cmd, GP_STATUS_BAD_STATE, NULL, 0); // about to reboot
		return;
	}
	switch (cmd) {
	case GP_CMD_HELLO: {
		gp_hello_t h;
		build_hello(&h);
		respond(cmd, GP_STATUS_OK, &h, sizeof(h));
	} break;

	case GP_CMD_GET_LIMITS:
		respond(cmd, GP_STATUS_OK, &limits, sizeof(limits));
		break;

	case GP_CMD_READ_CONFIG:
		respond(cmd, GP_STATUS_OK, &cfg, sizeof(cfg));
		break;

	case GP_CMD_READ_CALIBRATION:
		respond(cmd, GP_STATUS_OK, &cal, sizeof(cal));
		break;

	case GP_CMD_VALIDATE_CONFIG:
		if (!payload_is(len, sizeof(gp_config_t), cmd))
			break;
		memcpy(&scratch_cfg, p, sizeof(scratch_cfg));
		gp_config_validate(&scratch_cfg, &limits, &errs);
		respond_errors(cmd, &errs);
		break;

	case GP_CMD_WRITE_CONFIG:
		if (!payload_is(len, sizeof(gp_config_t), cmd))
			break;
		if (calibrating()) {
			single_error(GP_STATUS_BAD_STATE, "Finish or cancel calibration first");
			respond_errors(cmd, &errs);
			break;
		}
		memcpy(&scratch_cfg, p, sizeof(scratch_cfg));
		if (gp_config_validate(&scratch_cfg, &limits, &errs) != 0) {
			respond_errors(cmd, &errs);
			break;
		}
		hal_watchdog_feed();
		if (!gp_storage_save(&scratch_cfg, &cal, &storage)) {
			single_error(GP_STATUS_STORAGE, "Writing to flash failed; previous settings kept");
			respond_errors(cmd, &errs);
			break;
		}
		persist_cfg = scratch_cfg;
		respond_errors(cmd, &errs); // status OK, no errors
		schedule(PENDING_REBOOT);	// new USB descriptors need re-enumeration
		break;

	case GP_CMD_WRITE_CALIBRATION:
		if (!payload_is(len, sizeof(gp_calibration_t), cmd))
			break;
		if (calibrating()) {
			single_error(GP_STATUS_BAD_STATE, "Finish or cancel calibration first");
			respond_errors(cmd, &errs);
			break;
		}
		memcpy(&scratch_cal, p, sizeof(scratch_cal));
		if (gp_calibration_validate(&scratch_cal, &limits, &errs) != 0) {
			respond_errors(cmd, &errs);
			break;
		}
		hal_watchdog_feed();
		if (!gp_storage_save(&persist_cfg, &scratch_cal, &storage)) {
			single_error(GP_STATUS_STORAGE, "Writing to flash failed; previous calibration kept");
			respond_errors(cmd, &errs);
			break;
		}
		cal = scratch_cal;
		resolve_groups();
		respond_errors(cmd, &errs);
		break;

	case GP_CMD_RESET_CALIBRATION: {
		if (calibrating()) {
			respond(cmd, GP_STATUS_BAD_STATE, NULL, 0);
			break;
		}
		gp_calibration_defaults(&scratch_cal);
		hal_watchdog_feed();
		if (!gp_storage_save(&persist_cfg, &scratch_cal, &storage)) {
			respond(cmd, GP_STATUS_STORAGE, NULL, 0);
			break;
		}
		cal = scratch_cal;
		resolve_groups();
		respond(cmd, GP_STATUS_OK, NULL, 0);
	} break;

	case GP_CMD_TELEMETRY: {
		gp_telemetry_t t;
		build_telemetry(&t);
		respond(cmd, GP_STATUS_OK, &t, sizeof(t));
	} break;

	case GP_CMD_CAL_START:
		if (!inputs_valid || gp_app_sleeping()) {
			respond(cmd, GP_STATUS_BAD_STATE, NULL, 0);
			break;
		}
		cal_start();
		respond(cmd, GP_STATUS_OK, NULL, 0);
		break;

	case GP_CMD_CAL_FINISH: {
		gp_cal_result_t res;
		memset(&res, 0, sizeof(res));
		uint8_t st = cal_finish(&res);
		respond(cmd, st, &res, sizeof(res));
	} break;

	case GP_CMD_CAL_CANCEL:
		gp_calib_cancel(&calib);
		respond(cmd, GP_STATUS_OK, NULL, 0);
		break;

	case GP_CMD_LED_PREVIEW:
		if (!payload_is(len, sizeof(gp_config_t), cmd))
			break;
		memcpy(&preview_cfg, p, sizeof(preview_cfg));
		// Only the look is taken from the preview; clamp what could misbehave.
		if (preview_cfg.brightness > 1000)
			preview_cfg.brightness = 1000;
		if (preview_cfg.status_brightness > 1000)
			preview_cfg.status_brightness = 1000;
		preview_on	  = true;
		preview_since = now_us;
		respond(cmd, GP_STATUS_OK, NULL, 0);
		break;

	case GP_CMD_LED_PREVIEW_STOP:
		preview_on = false;
		respond(cmd, GP_STATUS_OK, NULL, 0);
		break;

	case GP_CMD_FACTORY_RESET:
		gp_calib_cancel(&calib);
		respond(cmd, GP_STATUS_OK, NULL, 0);
		schedule(PENDING_FACTORY_RESET);
		break;

	case GP_CMD_REBOOT:
		respond(cmd, GP_STATUS_OK, NULL, 0);
		schedule(PENDING_REBOOT);
		break;

#ifdef GP_HOST_BUILD
	case GP_CMD_SIM_INPUT:
		if (!payload_is(len, sizeof(gp_sim_input_t), cmd))
			break;
		{
			gp_sim_input_t in;
			memcpy(&in, p, sizeof(in));
			hal_sim_input(&in);
		}
		respond(cmd, GP_STATUS_OK, NULL, 0);
		break;
#endif

	default:
		respond(cmd, GP_STATUS_UNKNOWN_CMD, NULL, 0);
		break;
	}
}

void gp_app_cdc_rx(const uint8_t* data, size_t len) { gp_frame_rx_feed(&rx, data, len, handle_frame, NULL); }

// ---------------------------------------------------------------------------
// Main loop step
// ---------------------------------------------------------------------------

static void update_sleep(void) {
	bool awake = hal_usb_mounted() && !hal_usb_suspended();
	if (awake)
		last_awake = now_us;
	bool was = sleeping;
	sleeping = hal_usb_suspended() || (!hal_usb_mounted() && elapsed(last_awake, GP_SLEEP_UNMOUNTED_MS * 1000u));
	if (was && !sleeping)
		gp_filter_reseed(&filter); // history is stale after a pause
}

void gp_app_task(void) {
	now_us			  = hal_time_us();
	uint32_t loop_dt = now_us - last_task_us;
	uptime_us += loop_dt;
	last_task_us = now_us;
	if (loop_dt > loop_max_cur)
		loop_max_cur = loop_dt;
	if (elapsed(stats_since, 1000000u)) {
		stats_since += 1000000u;
		if (elapsed(stats_since, 1000000u))
			stats_since = now_us;
		loop_max_last  = loop_max_cur;
		loop_max_cur   = 0;
		hid_count_last = hid_count_cur;
		hid_count_cur  = 0;
	}

	if (pending != PENDING_NONE && elapsed(pending_at, REBOOT_DELAY_MS * 1000u)) {
		if (pending == PENDING_FACTORY_RESET) {
			hal_watchdog_feed();
			gp_storage_erase_all(&storage);
		}
		hal_reboot();
		return;
	}

	if (preview_on && elapsed(preview_since, GP_PREVIEW_TIMEOUT_MS * 1000u))
		preview_on = false;

	update_sleep();
	bool paused = gp_app_sleeping();

	// Inputs
	uint32_t poll_us = 1000000u / cfg.poll_rate_hz;
	if (elapsed(last_poll, poll_us)) {
		last_poll += poll_us;
		if (elapsed(last_poll, 4 * poll_us))
			last_poll = now_us; // fell behind (e.g. flash write): don't burst
		if (!paused) {
			poll_inputs();
			cal_button();
		}
	}

	// USB report
	uint32_t hid_us = 1000000u / cfg.hid_rate_hz;
	if (elapsed(last_hid, hid_us)) {
		last_hid += hid_us;
		if (elapsed(last_hid, 4 * hid_us))
			last_hid = now_us;
		if (inputs_valid && hal_usb_mounted() && !hal_usb_suspended() && hal_hid_ready()) {
			uint8_t	 rep[GP_HID_REPORT_MAX];
			uint16_t n = gp_hid_report(&cfg, buttons, hid_axes, rep);
			if (n && hal_hid_send(cfg.hid_report_id, rep, n))
				hid_count_cur++;
		}
	}

	// LEDs
	if (led_total > 0) {
		bool	 dark	= paused && !reset_warn;
		uint32_t led_us = dark ? 50000u : gp_led_frame_us(led_look()->led_flags & GP_LED_DITHER, led_longest);
		if (elapsed(last_led, led_us)) {
			last_led = now_us;
			led_frame();
		}
	}
}
