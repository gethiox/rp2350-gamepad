// SPDX-License-Identifier: MIT
#include "config.h"

#include <stdarg.h>
#include <stdio.h>
#include <string.h>

#include "hal.h"

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

#define DEFAULT_VID 0xACDC
#define DEFAULT_PID 0x2137

static void set_color(gp_color_t* c, uint8_t r, uint8_t g, uint8_t b) {
	c->r		= r;
	c->g		= g;
	c->b		= b;
	c->reserved = 0;
}

void gp_config_defaults(gp_config_t* c) {
	memset(c, 0, sizeof(*c));
	c->magic   = GP_CONFIG_MAGIC;
	c->version = GP_CONFIG_VERSION;
	c->size	   = sizeof(gp_config_t);

	c->usb_vid = DEFAULT_VID;
	c->usb_pid = DEFAULT_PID;
	strcpy(c->usb_manufacturer, "rp2350");
	strcpy(c->usb_product, "gamepad");
	c->usb_serial[0]   = 0; // chip unique ID
	c->hid_rate_hz	   = 250;
	c->hid_report_id   = 1;
	c->hid_device_type = GP_HID_GAMEPAD;

	c->poll_rate_hz		 = 1000;
	c->smoothing_samples = 16;
	c->smoothing_mode	 = GP_SMOOTH_LINEAR;
	c->debounce_ms		 = 4;
	c->num_axes			 = 4;
	c->num_buttons		 = 8;
	c->num_led_pins		 = 4;
	c->center_dz		 = 50;
	c->outer_dz			 = 50;

	c->cal_pin		 = GP_PIN_NONE;
	c->cal_hold_ms	 = 1000;
	c->reset_hold_ms = 10000;

	c->brightness		 = 50;
	c->status_brightness = 250;
	static const uint8_t led_pins[GP_MAX_LED_PINS] = { 22, 23, 24, 25, GP_PIN_NONE, GP_PIN_NONE, GP_PIN_NONE, GP_PIN_NONE };
	memcpy(c->led_pins, led_pins, sizeof(led_pins));
	for (int i = 0; i < GP_MAX_LED_PINS; i++)
		c->led_counts[i] = 1;
	c->status_led_mask[0] = 0x03;
	c->led_flags		  = GP_LED_DITHER;

	for (int g = 0; g < GP_MAX_GROUPS; g++) {
		c->groups[g].enabled = g < 2; // sticks 1 and 2 from axes 1+2 and 3+4
		c->groups[g].axis_h	 = (uint8_t)(g * 2);
		c->groups[g].axis_v	 = (uint8_t)(g * 2 + 1);
		c->groups[g].shape	 = GP_SHAPE_CIRCLE;
	}

	static const uint8_t axis_pins[4]			= { 26, 27, 28, 29 };
	static const uint8_t usages[GP_MAX_AXES]	= { GP_USAGE_X, GP_USAGE_Y, GP_USAGE_Z, GP_USAGE_RX,
												GP_USAGE_RY, GP_USAGE_RZ, GP_USAGE_SLIDER, GP_USAGE_DIAL };
	static const uint8_t axis_leds[4]			= { 0x04, 0x04, 0x08, 0x08 };
	for (int i = 0; i < GP_MAX_AXES; i++) {
		gp_axis_cfg_t* a = &c->axes[i];
		a->pin			 = i < 4 ? axis_pins[i] : GP_PIN_NONE;
		a->hid_usage	 = usages[i];
		a->flags		 = i < 4 ? GP_AXIS_INVERT : 0; // matches the behaviour of the original firmware
		a->led_effect	 = GP_LED_FX_GLOW;
		a->led_mask[0]	 = i < 4 ? axis_leds[i] : 0;
		a->center_dz	 = 50;
		a->outer_dz		 = 50;
		if (i % 2 == 0)
			set_color(&a->color, 255, 0, 0);
		else
			set_color(&a->color, 0, 255, 0);
	}

	// Two stick clicks (blue on LEDs 3/4), two red and four sky-blue buttons on LED 2.
	static const uint8_t btn_pins[8] = { 14, 15, 0, 1, 2, 3, 4, 5 };
	static const uint8_t btn_leds[8] = { 0x04, 0x08, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02 };
	for (int i = 0; i < GP_MAX_BUTTONS; i++) {
		gp_button_cfg_t* b = &c->buttons[i];
		b->pin			   = i < 8 ? btn_pins[i] : GP_PIN_NONE;
		b->led_mask[0]	   = i < 8 ? btn_leds[i] : 0;
		if (i == 2 || i == 3)
			set_color(&b->color, 255, 0, 0);
		else if (i >= 4 && i < 8)
			set_color(&b->color, 0, 159, 255);
		else
			set_color(&b->color, 0, 0, 255);
	}
}

void gp_calibration_defaults(gp_calibration_t* cal) {
	memset(cal, 0, sizeof(*cal));
	cal->magic	 = GP_CAL_MAGIC;
	cal->version = GP_CAL_VERSION;
	cal->size	 = sizeof(gp_calibration_t);
	for (int i = 0; i < GP_MAX_AXES; i++) {
		cal->channels[i].min	= 0;
		cal->channels[i].center = 2048;
		cal->channels[i].max	= GP_ADC_MAX;
	}
	for (int g = 0; g < GP_MAX_GROUPS; g++) {
		cal->groups[g].pin_a	= GP_PIN_NONE;
		cal->groups[g].pin_b	= GP_PIN_NONE;
		cal->groups[g].center_a = 2048;
		cal->groups[g].center_b = 2048;
		for (int b = 0; b < GP_RADIAL_BINS; b++)
			cal->groups[g].radial[b] = 2047;
	}
}

// ---------------------------------------------------------------------------
// Limits
// ---------------------------------------------------------------------------

static const uint8_t allowed_usages[] = {
	GP_USAGE_X, GP_USAGE_Y, GP_USAGE_Z, GP_USAGE_RX, GP_USAGE_RY, GP_USAGE_RZ, GP_USAGE_SLIDER, GP_USAGE_DIAL,
};

bool gp_usage_allowed(uint8_t usage) {
	for (unsigned i = 0; i < sizeof(allowed_usages); i++)
		if (allowed_usages[i] == usage)
			return true;
	return false;
}

static void range(gp_limits_t* l, int field, uint16_t min, uint16_t max) {
	l->ranges[field].min = min;
	l->ranges[field].max = max;
}

void gp_limits_build(gp_limits_t* l) {
	memset(l, 0, sizeof(*l));
	l->max_axes	   = GP_MAX_AXES;
	l->max_buttons = GP_MAX_BUTTONS;
	l->max_leds	   = GP_MAX_LEDS;
	l->max_led_pins = GP_MAX_LED_PINS;
	l->max_groups  = GP_MAX_GROUPS;
	l->radial_bins = GP_RADIAL_BINS;
	l->str_len	   = GP_STR_LEN;
	l->dz_sum_max  = GP_DZ_SUM_MAX;
	hal_describe_platform(l); // gpio_count, platform_axes/leds, pin masks
	if (l->platform_axes > GP_MAX_AXES)
		l->platform_axes = GP_MAX_AXES;
	if (l->platform_led_pins > GP_MAX_LED_PINS)
		l->platform_led_pins = GP_MAX_LED_PINS;

	l->num_hid_usages = sizeof(allowed_usages);
	memcpy(l->hid_usages, allowed_usages, sizeof(allowed_usages));

	range(l, GP_F_USB_VID, 0x0001, 0xFFFE);
	range(l, GP_F_USB_PID, 0x0001, 0xFFFE);
	range(l, GP_F_HID_RATE, 10, 1000);
	range(l, GP_F_HID_REPORT_ID, 1, 255);
	range(l, GP_F_HID_DEVICE_TYPE, 0, 1);
	range(l, GP_F_POLL_RATE, 100, 2000);
	range(l, GP_F_SMOOTHING_SAMPLES, 1, 128);
	range(l, GP_F_SMOOTHING_MODE, 0, 2);
	range(l, GP_F_DEBOUNCE, 0, 50);
	range(l, GP_F_CENTER_DZ, 0, 500);
	range(l, GP_F_OUTER_DZ, 0, 500);
	range(l, GP_F_NUM_AXES, 0, l->platform_axes);
	range(l, GP_F_NUM_BUTTONS, 0, GP_MAX_BUTTONS);
	range(l, GP_F_NUM_LED_PINS, 0, l->platform_led_pins);
	range(l, GP_F_LED_COUNT, 1, GP_MAX_LEDS);
	range(l, GP_F_AXIS_LED_EFFECT, 0, GP_LED_FX_MAX);
	range(l, GP_F_CAL_HOLD, 200, 10000);
	range(l, GP_F_RESET_HOLD, 0, 60000);
	range(l, GP_F_BRIGHTNESS, 0, 1000);
	range(l, GP_F_STATUS_BRIGHTNESS, 0, 1000);
	range(l, GP_F_AXIS_CENTER_DZ, 0, 500);
	range(l, GP_F_AXIS_OUTER_DZ, 0, 500);
	range(l, GP_F_GROUP_CENTER_DZ, 0, 500);
	range(l, GP_F_GROUP_OUTER_DZ, 0, 500);
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

typedef struct {
	gp_error_list_t* out;
	int				 count;
} vctx_t;

#if defined(__GNUC__)
__attribute__((format(printf, 4, 5)))
#endif
static void
err(vctx_t* v, uint8_t field, uint8_t index, const char* fmt, ...) {
	v->count++;
	if (!v->out)
		return;
	if (v->out->count >= GP_MAX_ERRORS) {
		v->out->truncated = 1;
		return;
	}
	gp_error_t* e = &v->out->errors[v->out->count++];
	memset(e, 0, sizeof(*e));
	e->field = field;
	e->index = index;
	va_list ap;
	va_start(ap, fmt);
	vsnprintf(e->message, sizeof(e->message), fmt, ap);
	va_end(ap);
}

static bool in_range(const gp_limits_t* l, int field, uint32_t v) {
	return v >= l->ranges[field].min && v <= l->ranges[field].max;
}

#define CHECK_RANGE(v, l, field, value, name)                                                                     \
	do {                                                                                                           \
		if (!in_range(l, field, value))                                                                            \
			err(v, field, 0xFF, "%s must be %u..%u", name, (unsigned)(l)->ranges[field].min, (unsigned)(l)->ranges[field].max); \
	} while (0)

static void check_string(vctx_t* v, uint8_t field, const char* s, const char* name, bool allow_empty) {
	size_t n = strnlen(s, GP_STR_LEN);
	if (n >= GP_STR_LEN) {
		err(v, field, 0xFF, "%s is too long (max %d characters)", name, GP_STR_LEN - 1);
		return;
	}
	if (n == 0 && !allow_empty)
		err(v, field, 0xFF, "%s must not be empty", name);
	for (size_t i = 0; i < n; i++) {
		if ((unsigned char)s[i] < 0x20 || (unsigned char)s[i] > 0x7E) {
			err(v, field, 0xFF, "%s may only contain printable ASCII", name);
			return;
		}
	}
	for (size_t i = n; i < GP_STR_LEN; i++) {
		if (s[i] != 0) {
			err(v, field, 0xFF, "%s has data after its end", name);
			return;
		}
	}
}

// GPIO ownership for conflict detection
typedef struct {
	char	what[20];
	uint8_t used;
} owner_t;

static void describe(char* out, size_t n, const char* kind, int index) {
	if (index < 0)
		snprintf(out, n, "%s", kind);
	else
		snprintf(out, n, "%s %d", kind, index + 1);
}

static void claim_pin(vctx_t* v, owner_t* owners, const gp_limits_t* l, uint8_t pin, const char* kind, int index,
					  uint8_t field, const uint32_t allowed[2], const char* role) {
	char me[20];
	describe(me, sizeof(me), kind, index);
	uint8_t idx = index < 0 ? 0xFF : (uint8_t)index;
	if (pin == GP_PIN_NONE) {
		err(v, field, idx, "%s needs a GPIO pin", me);
		return;
	}
	if (pin >= l->gpio_count || !gp_mask_has(allowed, pin)) {
		err(v, field, idx, "%s: GPIO %u can't be used %s", me, pin, role);
		return;
	}
	if (owners[pin].used) {
		err(v, field, idx, "%s: GPIO %u is already used by %s", me, pin, owners[pin].what);
		return;
	}
	owners[pin].used = 1;
	memcpy(owners[pin].what, me, sizeof(me));
}

static void check_color(vctx_t* v, uint8_t field, uint8_t idx, const gp_color_t* c, const char* who) {
	if (c->reserved != 0)
		err(v, field, idx, "%s: colour has a non-zero reserved byte", who);
}

static void check_dz(vctx_t* v, const gp_limits_t* l, uint8_t f_center, uint8_t f_outer, uint8_t idx,
					 uint16_t center, uint16_t outer, const char* who) {
	if (!in_range(l, f_center, center))
		err(v, f_center, idx, "%s: centre deadzone must be %u..%u %%", who, l->ranges[f_center].min / 10,
			l->ranges[f_center].max / 10);
	if (!in_range(l, f_outer, outer))
		err(v, f_outer, idx, "%s: outer deadzone must be %u..%u %%", who, l->ranges[f_outer].min / 10,
			l->ranges[f_outer].max / 10);
	if ((uint32_t)center + outer > l->dz_sum_max)
		err(v, f_outer, idx, "%s: centre + outer deadzone must not exceed %u %%", who, l->dz_sum_max / 10);
}

int gp_config_validate(const gp_config_t* c, const gp_limits_t* l, gp_error_list_t* out) {
	vctx_t v = { out, 0 };
	if (out)
		memset(out, 0, sizeof(*out));

	if (c->magic != GP_CONFIG_MAGIC || c->version != GP_CONFIG_VERSION || c->size != sizeof(gp_config_t)) {
		err(&v, GP_F_HEADER, 0xFF, "Configuration format %u is not supported (expected %u)", c->version,
			GP_CONFIG_VERSION);
		goto done; // nothing else can be trusted
	}

	// --- USB ---
	CHECK_RANGE(&v, l, GP_F_USB_VID, c->usb_vid, "USB vendor ID");
	CHECK_RANGE(&v, l, GP_F_USB_PID, c->usb_pid, "USB product ID");
	check_string(&v, GP_F_USB_MANUFACTURER, c->usb_manufacturer, "Manufacturer name", false);
	check_string(&v, GP_F_USB_PRODUCT, c->usb_product, "Product name", false);
	check_string(&v, GP_F_USB_SERIAL, c->usb_serial, "Serial number", true);
	CHECK_RANGE(&v, l, GP_F_HID_RATE, c->hid_rate_hz, "USB report rate (Hz)");
	CHECK_RANGE(&v, l, GP_F_HID_REPORT_ID, c->hid_report_id, "HID report ID");
	CHECK_RANGE(&v, l, GP_F_HID_DEVICE_TYPE, c->hid_device_type, "Device type");

	// --- processing ---
	CHECK_RANGE(&v, l, GP_F_POLL_RATE, c->poll_rate_hz, "Input poll rate (Hz)");
	CHECK_RANGE(&v, l, GP_F_SMOOTHING_SAMPLES, c->smoothing_samples, "Smoothing samples");
	CHECK_RANGE(&v, l, GP_F_SMOOTHING_MODE, c->smoothing_mode, "Smoothing mode");
	CHECK_RANGE(&v, l, GP_F_DEBOUNCE, c->debounce_ms, "Button debounce (ms)");
	check_dz(&v, l, GP_F_CENTER_DZ, GP_F_OUTER_DZ, 0xFF, c->center_dz, c->outer_dz, "Global");

	bool counts_ok = true;
	if (!in_range(l, GP_F_NUM_AXES, c->num_axes)) {
		err(&v, GP_F_NUM_AXES, 0xFF, "This chip supports at most %u axes", l->ranges[GP_F_NUM_AXES].max);
		counts_ok = false;
	}
	if (!in_range(l, GP_F_NUM_BUTTONS, c->num_buttons)) {
		err(&v, GP_F_NUM_BUTTONS, 0xFF, "At most %u buttons are supported", l->ranges[GP_F_NUM_BUTTONS].max);
		counts_ok = false;
	}
	if (!in_range(l, GP_F_NUM_LED_PINS, c->num_led_pins)) {
		err(&v, GP_F_NUM_LED_PINS, 0xFF, "At most %u LED pins are supported", l->ranges[GP_F_NUM_LED_PINS].max);
		counts_ok = false;
	}

	// --- button timing ---
	CHECK_RANGE(&v, l, GP_F_CAL_HOLD, c->cal_hold_ms, "Calibration hold time (ms)");
	if (c->reset_hold_ms != 0) {
		if (c->reset_hold_ms < (uint32_t)c->cal_hold_ms + GP_RESET_HOLD_GAP || !in_range(l, GP_F_RESET_HOLD, c->reset_hold_ms))
			err(&v, GP_F_RESET_HOLD, 0xFF, "Factory reset hold must be 0 (off) or %u..%u ms",
				(unsigned)(c->cal_hold_ms + GP_RESET_HOLD_GAP), l->ranges[GP_F_RESET_HOLD].max);
	}

	// --- LEDs ---
	CHECK_RANGE(&v, l, GP_F_BRIGHTNESS, c->brightness, "LED brightness");
	CHECK_RANGE(&v, l, GP_F_STATUS_BRIGHTNESS, c->status_brightness, "Status LED brightness");
	if (c->led_flags & ~GP_LED_FLAGS_ALL)
		err(&v, GP_F_LED_FLAGS, 0xFF, "Unknown LED option bits");
	if (c->reserved0 || c->reserved1 || c->reserved2 || c->reserved3)
		err(&v, GP_F_HEADER, 0xFF, "Reserved fields must be zero");

	if (!counts_ok)
		goto done; // per-item checks would index out of range

	// LED chains: LEDs are numbered across the pins in order.
	unsigned total_leds = 0;
	for (int i = 0; i < c->num_led_pins; i++) {
		uint8_t n = c->led_counts[i];
		if (!in_range(l, GP_F_LED_COUNT, n)) {
			err(&v, GP_F_LED_COUNT, (uint8_t)i, "LED pin %d: chain length must be %u..%u", i + 1,
				l->ranges[GP_F_LED_COUNT].min, l->ranges[GP_F_LED_COUNT].max);
			continue;
		}
		if (c->led_pins[i] == l->builtin_led_pin && n != 1)
			err(&v, GP_F_LED_COUNT, (uint8_t)i, "LED pin %d: the built-in LED is a single LED", i + 1);
		if (total_leds + n > GP_MAX_LEDS)
			err(&v, GP_F_LED_COUNT, (uint8_t)i, "At most %u LEDs in total are supported", GP_MAX_LEDS);
		total_leds += n;
	}
	if (total_leds > GP_MAX_LEDS)
		total_leds = GP_MAX_LEDS;
	uint32_t led_all[2] = { total_leds >= 32 ? 0xFFFFFFFFu : ((1u << total_leds) - 1u),
							total_leds >= 64 ? 0xFFFFFFFFu : (total_leds > 32 ? ((1u << (total_leds - 32)) - 1u) : 0u) };
#define MASK_OUTSIDE(m) (((m)[0] & ~led_all[0]) || ((m)[1] & ~led_all[1]))
	if (MASK_OUTSIDE(c->status_led_mask))
		err(&v, GP_F_STATUS_LED_MASK, 0xFF, "Status LEDs refer to an LED that does not exist");

	// --- pin ownership ---
	static owner_t owners[64]; // static: keeps the firmware stack small (not re-entrant)
	memset(owners, 0, sizeof(owners));
	char who[16];

	if (c->cal_pin != GP_PIN_NONE)
		claim_pin(&v, owners, l, c->cal_pin, "Calibration button", -1, GP_F_CAL_PIN, l->cal_pins, "for the calibration button");

	for (int i = 0; i < c->num_led_pins; i++)
		claim_pin(&v, owners, l, c->led_pins[i], "LED pin", i, GP_F_LED_PIN, l->led_pins, "for LEDs");

	// --- axes ---
	static uint8_t usage_count[256];
	memset(usage_count, 0, sizeof(usage_count));
	int sliders			 = 0;
	for (int i = 0; i < c->num_axes; i++) {
		const gp_axis_cfg_t* a = &c->axes[i];
		snprintf(who, sizeof(who), "Axis %d", i + 1);
		claim_pin(&v, owners, l, a->pin, "Axis", i, GP_F_AXIS_PIN, l->adc_pins, "for an axis (needs an ADC pin)");
		if (!gp_usage_allowed(a->hid_usage)) {
			err(&v, GP_F_AXIS_USAGE, (uint8_t)i, "%s: unsupported HID usage 0x%02X", who, a->hid_usage);
		} else if (a->hid_usage == GP_USAGE_SLIDER || a->hid_usage == GP_USAGE_DIAL) {
			if (++sliders > 2)
				err(&v, GP_F_AXIS_USAGE, (uint8_t)i, "%s: at most two Slider/Dial axes are supported", who);
		} else if (usage_count[a->hid_usage]++) {
			err(&v, GP_F_AXIS_USAGE, (uint8_t)i, "%s: HID usage is already used by another axis", who);
		}
		if (a->flags & ~GP_AXIS_FLAGS_ALL)
			err(&v, GP_F_AXIS_FLAGS, (uint8_t)i, "%s: unknown option bits", who);
		if (a->flags & GP_AXIS_CUSTOM_DZ)
			check_dz(&v, l, GP_F_AXIS_CENTER_DZ, GP_F_AXIS_OUTER_DZ, (uint8_t)i, a->center_dz, a->outer_dz, who);
		if (MASK_OUTSIDE(a->led_mask))
			err(&v, GP_F_AXIS_LED_MASK, (uint8_t)i, "%s: lights an LED that does not exist", who);
		if (a->led_effect > GP_LED_FX_MAX)
			err(&v, GP_F_AXIS_LED_EFFECT, (uint8_t)i, "%s: unknown LED effect", who);
		check_color(&v, GP_F_AXIS_COLOR, (uint8_t)i, &a->color, who);
	}

	// --- buttons ---
	for (int i = 0; i < c->num_buttons; i++) {
		const gp_button_cfg_t* b = &c->buttons[i];
		snprintf(who, sizeof(who), "Button %d", i + 1);
		claim_pin(&v, owners, l, b->pin, "Button", i, GP_F_BUTTON_PIN, l->button_pins, "for a button");
		if (b->flags || b->reserved)
			err(&v, GP_F_BUTTON_FLAGS, (uint8_t)i, "%s: unknown option bits", who);
		if (MASK_OUTSIDE(b->led_mask))
			err(&v, GP_F_BUTTON_LED_MASK, (uint8_t)i, "%s: lights an LED that does not exist", who);
		check_color(&v, GP_F_BUTTON_COLOR, (uint8_t)i, &b->color, who);
	}

	// --- stick groups ---
	uint8_t axis_group[GP_MAX_AXES];
	memset(axis_group, 0xFF, sizeof(axis_group));
	for (int g = 0; g < GP_MAX_GROUPS; g++) {
		const gp_group_cfg_t* gr = &c->groups[g];
		snprintf(who, sizeof(who), "Stick %d", g + 1);
		if (gr->enabled > 1) {
			err(&v, GP_F_GROUP_ENABLED, (uint8_t)g, "%s: invalid enabled value", who);
			continue;
		}
		if (gr->reserved || gr->reserved2)
			err(&v, GP_F_GROUP_FLAGS, (uint8_t)g, "%s: reserved fields must be zero", who);
		if (gr->axis_h >= GP_MAX_AXES || gr->axis_v >= GP_MAX_AXES) {
			err(&v, GP_F_GROUP_AXES, (uint8_t)g, "%s: axis index out of range", who);
			continue;
		}
		if (!gr->enabled)
			continue;
		if (gr->axis_h >= c->num_axes || gr->axis_v >= c->num_axes)
			err(&v, GP_F_GROUP_AXES, (uint8_t)g, "%s uses an axis that is not enabled", who);
		else if (gr->axis_h == gr->axis_v)
			err(&v, GP_F_GROUP_AXES, (uint8_t)g, "%s needs two different axes", who);
		else {
			const uint8_t ax[2] = { gr->axis_h, gr->axis_v };
			for (int k = 0; k < 2; k++) {
				if (axis_group[ax[k]] != 0xFF)
					err(&v, GP_F_GROUP_AXES, (uint8_t)g, "%s: Axis %u is already part of Stick %u", who, ax[k] + 1,
						axis_group[ax[k]] + 1);
				else
					axis_group[ax[k]] = (uint8_t)g;
			}
		}
		if (gr->shape > GP_SHAPE_SQUARE)
			err(&v, GP_F_GROUP_SHAPE, (uint8_t)g, "%s: unknown output shape", who);
		if (gr->flags & ~GP_GROUP_FLAGS_ALL)
			err(&v, GP_F_GROUP_FLAGS, (uint8_t)g, "%s: unknown option bits", who);
		if (gr->flags & GP_GROUP_CUSTOM_DZ)
			check_dz(&v, l, GP_F_GROUP_CENTER_DZ, GP_F_GROUP_OUTER_DZ, (uint8_t)g, gr->center_dz, gr->outer_dz, who);
	}

#undef MASK_OUTSIDE
done:
	if (out)
		out->status = v.count ? GP_STATUS_INVALID : GP_STATUS_OK;
	return v.count;
}

unsigned gp_led_total(const gp_config_t* c) {
	unsigned n = 0;
	for (int i = 0; i < c->num_led_pins && i < GP_MAX_LED_PINS; i++)
		n += c->led_counts[i];
	return n > GP_MAX_LEDS ? GP_MAX_LEDS : n;
}

int gp_calibration_validate(const gp_calibration_t* cal, const gp_limits_t* l, gp_error_list_t* out) {
	vctx_t v = { out, 0 };
	if (out)
		memset(out, 0, sizeof(*out));

	if (cal->magic != GP_CAL_MAGIC || cal->version != GP_CAL_VERSION || cal->size != sizeof(gp_calibration_t)) {
		err(&v, GP_F_HEADER, 0xFF, "Calibration format %u is not supported (expected %u)", cal->version, GP_CAL_VERSION);
		goto done;
	}
	for (int i = 0; i < GP_MAX_AXES; i++) {
		const gp_cal_channel_t* ch = &cal->channels[i];
		if (ch->calibrated > 1 || ch->reserved) {
			err(&v, GP_F_CAL_CHANNEL, (uint8_t)i, "ADC channel %d: invalid flags", i);
			continue;
		}
		if (!ch->calibrated)
			continue;
		if (ch->max > GP_ADC_MAX || !(ch->min < ch->center && ch->center < ch->max) ||
			ch->max - ch->min < GP_CAL_IMPORT_SPAN)
			err(&v, GP_F_CAL_CHANNEL, (uint8_t)i, "ADC channel %d: min/centre/max are inconsistent", i);
	}
	for (int g = 0; g < GP_MAX_GROUPS; g++) {
		const gp_cal_group_t* gr = &cal->groups[g];
		if (gr->calibrated > 1) {
			err(&v, GP_F_CAL_GROUP, (uint8_t)g, "Stick %d: invalid flags", g + 1);
			continue;
		}
		if (!gr->calibrated)
			continue;
		if (gr->pin_a >= 64 || gr->pin_b >= 64 || !gp_mask_has(l->adc_pins, gr->pin_a) ||
			!gp_mask_has(l->adc_pins, gr->pin_b) || gr->pin_a == gr->pin_b)
			err(&v, GP_F_CAL_GROUP, (uint8_t)g, "Stick %d: invalid pins", g + 1);
		if (gr->orient & ~GP_ORIENT_ALL)
			err(&v, GP_F_CAL_GROUP, (uint8_t)g, "Stick %d: invalid orientation", g + 1);
		if (gr->center_a > GP_ADC_MAX || gr->center_b > GP_ADC_MAX)
			err(&v, GP_F_CAL_GROUP, (uint8_t)g, "Stick %d: invalid centre", g + 1);
		for (int b = 0; b < GP_RADIAL_BINS; b++) {
			if (gr->radial[b] < GP_RADIAL_MIN || gr->radial[b] > GP_RADIAL_MAX) {
				err(&v, GP_F_CAL_GROUP, (uint8_t)g, "Stick %d: invalid radial map", g + 1);
				break;
			}
		}
	}
done:
	if (out)
		out->status = v.count ? GP_STATUS_INVALID : GP_STATUS_OK;
	return v.count;
}
