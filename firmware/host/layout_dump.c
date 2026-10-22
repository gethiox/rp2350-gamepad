// SPDX-License-Identifier: MIT
//
// layout_dump.c - prints the binary layout of every wire/storage structure
// and all protocol constants as JSON. The output is committed as
// protocol/layout.json; CI regenerates it and fails on any difference, and
// the Go configurator's tests check its own structures against it.

#include <stddef.h>
#include <stdio.h>

#include "gp_protocol.h"

static int first_struct = 1, first_field;

static void begin(const char* name, size_t size) {
	printf("%s\n    \"%s\": {\"size\": %zu, \"fields\": [", first_struct ? "" : ",", name, size);
	first_struct = 0;
	first_field	 = 1;
}
static void field(const char* name, size_t off, size_t size) {
	printf("%s\n      {\"name\": \"%s\", \"offset\": %zu, \"size\": %zu}", first_field ? "" : ",", name, off, size);
	first_field = 0;
}
static void end(void) { printf("\n    ]}"); }

#define S(T) begin(#T, sizeof(T))
#define F(T, f) field(#f, offsetof(T, f), sizeof(((T*)0)->f))

static int first_const = 1;
static void constant(const char* name, long long v) {
	printf("%s\n    \"%s\": %lld", first_const ? "" : ",", name, v);
	first_const = 0;
}
#define C(x) constant(#x, (long long)(x))

int main(void) {
	printf("{\n  \"structs\": {");

	S(gp_color_t);
	F(gp_color_t, r); F(gp_color_t, g); F(gp_color_t, b); F(gp_color_t, reserved);
	end();

	S(gp_axis_cfg_t);
	F(gp_axis_cfg_t, pin); F(gp_axis_cfg_t, hid_usage); F(gp_axis_cfg_t, flags); F(gp_axis_cfg_t, led_effect);
	F(gp_axis_cfg_t, center_dz); F(gp_axis_cfg_t, outer_dz); F(gp_axis_cfg_t, color); F(gp_axis_cfg_t, led_mask);
	end();

	S(gp_button_cfg_t);
	F(gp_button_cfg_t, pin); F(gp_button_cfg_t, flags); F(gp_button_cfg_t, reserved);
	F(gp_button_cfg_t, color); F(gp_button_cfg_t, led_mask);
	end();

	S(gp_group_cfg_t);
	F(gp_group_cfg_t, enabled); F(gp_group_cfg_t, axis_h); F(gp_group_cfg_t, axis_v); F(gp_group_cfg_t, shape);
	F(gp_group_cfg_t, flags); F(gp_group_cfg_t, reserved); F(gp_group_cfg_t, center_dz); F(gp_group_cfg_t, outer_dz);
	F(gp_group_cfg_t, reserved2);
	end();

	S(gp_config_t);
	F(gp_config_t, magic); F(gp_config_t, version); F(gp_config_t, size);
	F(gp_config_t, usb_vid); F(gp_config_t, usb_pid); F(gp_config_t, usb_manufacturer); F(gp_config_t, usb_product);
	F(gp_config_t, usb_serial); F(gp_config_t, hid_rate_hz); F(gp_config_t, hid_report_id);
	F(gp_config_t, hid_device_type); F(gp_config_t, poll_rate_hz); F(gp_config_t, smoothing_samples);
	F(gp_config_t, smoothing_mode); F(gp_config_t, debounce_ms); F(gp_config_t, num_axes); F(gp_config_t, num_buttons);
	F(gp_config_t, num_led_pins); F(gp_config_t, center_dz); F(gp_config_t, outer_dz); F(gp_config_t, cal_pin);
	F(gp_config_t, reserved0); F(gp_config_t, cal_hold_ms); F(gp_config_t, reset_hold_ms); F(gp_config_t, brightness);
	F(gp_config_t, status_brightness); F(gp_config_t, led_flags); F(gp_config_t, reserved1); F(gp_config_t, led_pins);
	F(gp_config_t, led_counts); F(gp_config_t, status_led_mask); F(gp_config_t, reserved2); F(gp_config_t, reserved3);
	F(gp_config_t, groups);
	F(gp_config_t, axes); F(gp_config_t, buttons);
	end();

	S(gp_cal_channel_t);
	F(gp_cal_channel_t, calibrated); F(gp_cal_channel_t, reserved); F(gp_cal_channel_t, min);
	F(gp_cal_channel_t, center); F(gp_cal_channel_t, max);
	end();

	S(gp_cal_group_t);
	F(gp_cal_group_t, calibrated); F(gp_cal_group_t, pin_a); F(gp_cal_group_t, pin_b); F(gp_cal_group_t, orient);
	F(gp_cal_group_t, center_a); F(gp_cal_group_t, center_b); F(gp_cal_group_t, radial);
	end();

	S(gp_calibration_t);
	F(gp_calibration_t, magic); F(gp_calibration_t, version); F(gp_calibration_t, size);
	F(gp_calibration_t, channels); F(gp_calibration_t, groups);
	end();

	S(gp_hello_t);
	F(gp_hello_t, proto_version); F(gp_hello_t, config_version); F(gp_hello_t, config_size);
	F(gp_hello_t, cal_version); F(gp_hello_t, cal_size); F(gp_hello_t, telemetry_size); F(gp_hello_t, limits_size);
	F(gp_hello_t, flags); F(gp_hello_t, storage_seq); F(gp_hello_t, chip_id); F(gp_hello_t, reset_reason);
	F(gp_hello_t, crash_count); F(gp_hello_t, reserved); F(gp_hello_t, fw_version); F(gp_hello_t, board);
	end();

	S(gp_range_t);
	F(gp_range_t, min); F(gp_range_t, max);
	end();

	S(gp_limits_t);
	F(gp_limits_t, max_axes); F(gp_limits_t, max_buttons); F(gp_limits_t, max_leds); F(gp_limits_t, max_groups);
	F(gp_limits_t, platform_axes); F(gp_limits_t, platform_led_pins); F(gp_limits_t, gpio_count);
	F(gp_limits_t, adc_first_pin); F(gp_limits_t, radial_bins); F(gp_limits_t, str_len);
	F(gp_limits_t, num_hid_usages); F(gp_limits_t, max_led_pins); F(gp_limits_t, dz_sum_max);
	F(gp_limits_t, builtin_led_pin); F(gp_limits_t, reserved1);
	F(gp_limits_t, button_pins); F(gp_limits_t, led_pins); F(gp_limits_t, adc_pins); F(gp_limits_t, cal_pins);
	F(gp_limits_t, hid_usages); F(gp_limits_t, ranges);
	end();

	S(gp_error_t);
	F(gp_error_t, field); F(gp_error_t, index); F(gp_error_t, reserved); F(gp_error_t, message);
	end();

	S(gp_telemetry_t);
	F(gp_telemetry_t, uptime_ms); F(gp_telemetry_t, buttons); F(gp_telemetry_t, axes); F(gp_telemetry_t, raw);
	F(gp_telemetry_t, raw_now); F(gp_telemetry_t, cal_min); F(gp_telemetry_t, cal_center); F(gp_telemetry_t, cal_max);
	F(gp_telemetry_t, led_rgb); F(gp_telemetry_t, cal_group_sweep); F(gp_telemetry_t, cal_channel_state);
	F(gp_telemetry_t, cal_group_state); F(gp_telemetry_t, cal_group_coverage); F(gp_telemetry_t, state);
	F(gp_telemetry_t, reserved); F(gp_telemetry_t, loop_max_us); F(gp_telemetry_t, hid_reports_per_s);
	F(gp_telemetry_t, reserved2);
	end();

	S(gp_cal_result_t);
	F(gp_cal_result_t, channel); F(gp_cal_result_t, group);
	end();

	S(gp_sim_input_t);
	F(gp_sim_input_t, raw); F(gp_sim_input_t, buttons); F(gp_sim_input_t, cal_button); F(gp_sim_input_t, auto_motion);
	F(gp_sim_input_t, usb_state); F(gp_sim_input_t, reserved);
	end();

	printf("\n  },\n  \"constants\": {");
	C(GP_MAX_AXES); C(GP_MAX_BUTTONS); C(GP_MAX_LED_PINS); C(GP_MAX_LEDS); C(GP_MAX_GROUPS); C(GP_RADIAL_BINS); C(GP_STR_LEN);
	C(GP_ADC_MAX); C(GP_PIN_NONE);
	C(GP_PROTO_VERSION); C(GP_CONFIG_MAGIC); C(GP_CONFIG_VERSION); C(GP_CAL_MAGIC); C(GP_CAL_VERSION);
	C(GP_AXIS_INVERT); C(GP_AXIS_LINEAR); C(GP_AXIS_CUSTOM_DZ);
	C(GP_USAGE_X); C(GP_USAGE_Y); C(GP_USAGE_Z); C(GP_USAGE_RX); C(GP_USAGE_RY); C(GP_USAGE_RZ);
	C(GP_USAGE_SLIDER); C(GP_USAGE_DIAL);
	C(GP_GROUP_SWAP); C(GP_GROUP_INVERT_H); C(GP_GROUP_INVERT_V); C(GP_GROUP_CUSTOM_DZ);
	C(GP_SHAPE_CIRCLE); C(GP_SHAPE_SQUARE);
	C(GP_LED_DITHER); C(GP_LED_KEEP_ON_SLEEP);
	C(GP_LED_FX_GLOW); C(GP_LED_FX_BAR); C(GP_LED_FX_DOT);
	C(GP_SMOOTH_SIMPLE); C(GP_SMOOTH_LINEAR); C(GP_SMOOTH_TRIANGLE);
	C(GP_HID_GAMEPAD); C(GP_HID_JOYSTICK);
	C(GP_ORIENT_SWAP); C(GP_ORIENT_INVERT_H); C(GP_ORIENT_INVERT_V);
	C(GP_FRAME_SYNC0); C(GP_FRAME_SYNC1); C(GP_FRAME_OVERHEAD); C(GP_PROTO_MAX_PAYLOAD);
	C(GP_CMD_HELLO); C(GP_CMD_GET_LIMITS); C(GP_CMD_READ_CONFIG); C(GP_CMD_VALIDATE_CONFIG); C(GP_CMD_WRITE_CONFIG);
	C(GP_CMD_READ_CALIBRATION); C(GP_CMD_WRITE_CALIBRATION); C(GP_CMD_TELEMETRY); C(GP_CMD_CAL_START);
	C(GP_CMD_CAL_FINISH); C(GP_CMD_CAL_CANCEL); C(GP_CMD_LED_PREVIEW); C(GP_CMD_LED_PREVIEW_STOP);
	C(GP_CMD_FACTORY_RESET); C(GP_CMD_REBOOT); C(GP_CMD_RESET_CALIBRATION); C(GP_CMD_SIM_INPUT);
	C(GP_STATUS_OK); C(GP_STATUS_INVALID); C(GP_STATUS_BAD_LENGTH); C(GP_STATUS_BAD_STATE); C(GP_STATUS_STORAGE);
	C(GP_STATUS_UNKNOWN_CMD);
	C(GP_PREVIEW_TIMEOUT_MS);
	C(GP_HELLO_SAFE_MODE); C(GP_HELLO_CONFIG_DEFAULTED); C(GP_HELLO_CAL_DEFAULTED); C(GP_HELLO_STORAGE_EMPTY);
	C(GP_HELLO_STORAGE_RECOVERED); C(GP_HELLO_SIMULATOR);
	C(GP_RESET_POWER_ON); C(GP_RESET_WATCHDOG); C(GP_RESET_REQUESTED);
	C(GP_F_NONE); C(GP_F_HEADER); C(GP_F_USB_VID); C(GP_F_USB_PID); C(GP_F_USB_MANUFACTURER); C(GP_F_USB_PRODUCT);
	C(GP_F_USB_SERIAL); C(GP_F_HID_RATE); C(GP_F_HID_REPORT_ID); C(GP_F_HID_DEVICE_TYPE); C(GP_F_POLL_RATE);
	C(GP_F_SMOOTHING_SAMPLES); C(GP_F_SMOOTHING_MODE); C(GP_F_DEBOUNCE); C(GP_F_CENTER_DZ); C(GP_F_OUTER_DZ);
	C(GP_F_NUM_AXES); C(GP_F_NUM_BUTTONS); C(GP_F_NUM_LED_PINS); C(GP_F_CAL_PIN); C(GP_F_CAL_HOLD); C(GP_F_RESET_HOLD);
	C(GP_F_BRIGHTNESS); C(GP_F_STATUS_BRIGHTNESS); C(GP_F_LED_PIN); C(GP_F_STATUS_LED_MASK); C(GP_F_LED_FLAGS);
	C(GP_F_AXIS_PIN); C(GP_F_AXIS_USAGE); C(GP_F_AXIS_FLAGS); C(GP_F_AXIS_CENTER_DZ); C(GP_F_AXIS_OUTER_DZ);
	C(GP_F_AXIS_LED_MASK); C(GP_F_AXIS_COLOR); C(GP_F_BUTTON_PIN); C(GP_F_BUTTON_FLAGS); C(GP_F_BUTTON_LED_MASK);
	C(GP_F_BUTTON_COLOR); C(GP_F_GROUP_ENABLED); C(GP_F_GROUP_AXES); C(GP_F_GROUP_SHAPE); C(GP_F_GROUP_FLAGS);
	C(GP_F_GROUP_CENTER_DZ); C(GP_F_GROUP_OUTER_DZ); C(GP_F_CAL_CHANNEL); C(GP_F_CAL_GROUP); C(GP_F_LED_COUNT);
	C(GP_F_AXIS_LED_EFFECT); C(GP_F_COUNT);
	C(GP_MAX_ERRORS); C(GP_ERROR_MSG_LEN);
	C(GP_STATE_CALIBRATING); C(GP_STATE_USB_MOUNTED); C(GP_STATE_SLEEPING); C(GP_STATE_PREVIEW);
	C(GP_STATE_SAFE_MODE); C(GP_STATE_CAL_BUTTON); C(GP_STATE_RESET_PENDING);
	C(GP_CALCH_IDLE); C(GP_CALCH_MOVING); C(GP_CALCH_OK);
	C(GP_CALGRP_IDLE); C(GP_CALGRP_UP); C(GP_CALGRP_ROTATING); C(GP_CALGRP_DONE);
	C(GP_CALRES_UNCHANGED); C(GP_CALRES_UPDATED); C(GP_CALRES_FAILED);
	printf("\n  }\n}\n");
	return 0;
}
