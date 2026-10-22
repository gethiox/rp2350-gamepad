// SPDX-License-Identifier: MIT
//
// gp_protocol.h - the binary contract between the firmware and the configurator.
//
// Everything in this header is sent over the wire or stored in flash as raw
// little-endian bytes. The layout is fixed explicitly (no implicit padding);
// static asserts at the bottom pin every size, and the host-side layout dump
// (firmware/host/layout_dump.c) lets the Go configurator verify that its
// mirror structures match byte-for-byte.
//
// Compatibility rules:
//  * Any change to a struct below MUST bump the matching *_VERSION constant.
//  * Old stored versions are either migrated (see storage.c) or replaced
//    by defaults; they are never interpreted with a newer layout.
//  * GP_PROTO_VERSION is bumped whenever commands or payloads change.

#ifndef GP_PROTOCOL_H
#define GP_PROTOCOL_H

#include <stdint.h>

// ---------------------------------------------------------------------------
// Capacity of the data structures (schema maxima, not platform limits).
// The platform limits (how many ADC pins exist etc.) are reported separately
// in gp_limits_t so the same schema can serve RP2350A and RP2350B builds.
// ---------------------------------------------------------------------------
#define GP_MAX_AXES	   8
#define GP_MAX_BUTTONS 64
#define GP_MAX_LED_PINS 8  // WS2812 data pins
#define GP_MAX_LEDS	   64 // LEDs on all pins together (chains)
#define GP_MAX_GROUPS  4
#define GP_RADIAL_BINS 32
#define GP_STR_LEN	   32 // including the terminating NUL
#define GP_ADC_MAX	   4095
#define GP_PIN_NONE	   0xFF

#define GP_PROTO_VERSION  1
#define GP_CONFIG_MAGIC	  0x43475052u // "RPGC"
#define GP_CONFIG_VERSION 1
#define GP_CAL_MAGIC	  0x41435052u // "RPCA"
#define GP_CAL_VERSION	  1

// ---------------------------------------------------------------------------
// Configuration (persisted, written by the configurator)
// ---------------------------------------------------------------------------

typedef struct {
	uint8_t r, g, b;
	uint8_t reserved; // must be 0
} gp_color_t;

// Axis flags
#define GP_AXIS_INVERT	  0x01 // flip the output direction
#define GP_AXIS_LINEAR	  0x02 // throttle/slider: min..max, no centre
#define GP_AXIS_CUSTOM_DZ 0x04 // use the per-axis deadzones below
#define GP_AXIS_FLAGS_ALL 0x07

// HID usages (Generic Desktop page) allowed for axes
#define GP_USAGE_X		0x30
#define GP_USAGE_Y		0x31
#define GP_USAGE_Z		0x32
#define GP_USAGE_RX		0x33
#define GP_USAGE_RY		0x34
#define GP_USAGE_RZ		0x35
#define GP_USAGE_SLIDER 0x36
#define GP_USAGE_DIAL	0x37

// LED masks are 64-bit (two words, [0] = LEDs 1..32). LEDs are numbered in
// pin order and, within a pin, in chain order.

// Axis LED effects (how an axis drives the LEDs in its mask)
#define GP_LED_FX_GLOW 0 // every LED brightens with the deflection
#define GP_LED_FX_BAR  1 // level meter along the LEDs (centred axes fill from the middle)
#define GP_LED_FX_DOT  2 // a point moves along the LEDs, blended between neighbours
#define GP_LED_FX_MAX  2

typedef struct {
	uint8_t	   pin;		   // ADC capable GPIO
	uint8_t	   hid_usage;  // GP_USAGE_*
	uint8_t	   flags;	   // GP_AXIS_*
	uint8_t	   led_effect; // GP_LED_FX_*
	uint16_t   center_dz;  // permille, only with GP_AXIS_CUSTOM_DZ
	uint16_t   outer_dz;   // permille, only with GP_AXIS_CUSTOM_DZ
	gp_color_t color;
	uint32_t   led_mask[2]; // LEDs that react to this axis
} gp_axis_cfg_t;

typedef struct {
	uint8_t	   pin;		 // any GPIO, active low (button to GND)
	uint8_t	   flags;	 // reserved, must be 0
	uint16_t   reserved; // must be 0
	gp_color_t color;
	uint32_t   led_mask[2]; // LEDs that light while pressed
} gp_button_cfg_t;

// Stick group flags (user adjustments applied on top of calibration)
#define GP_GROUP_SWAP	   0x01
#define GP_GROUP_INVERT_H  0x02
#define GP_GROUP_INVERT_V  0x04
#define GP_GROUP_CUSTOM_DZ 0x08
#define GP_GROUP_FLAGS_ALL 0x0F

#define GP_SHAPE_CIRCLE 0
#define GP_SHAPE_SQUARE 1

typedef struct {
	uint8_t	 enabled;	// 0/1
	uint8_t	 axis_h;	// axis index that reports the horizontal output
	uint8_t	 axis_v;	// axis index that reports the vertical output
	uint8_t	 shape;		// GP_SHAPE_*
	uint8_t	 flags;		// GP_GROUP_*
	uint8_t	 reserved;	// must be 0
	uint16_t center_dz; // permille, only with GP_GROUP_CUSTOM_DZ
	uint16_t outer_dz;	// permille, only with GP_GROUP_CUSTOM_DZ
	uint16_t reserved2; // must be 0
} gp_group_cfg_t;

// LED flags
#define GP_LED_DITHER		0x01 // temporal dithering for finer brightness steps
#define GP_LED_KEEP_ON_SLEEP 0x02 // keep LEDs running while the host sleeps / is off
#define GP_LED_FLAGS_ALL	0x03

#define GP_SMOOTH_SIMPLE   0
#define GP_SMOOTH_LINEAR   1 // linearly weighted moving average
#define GP_SMOOTH_TRIANGLE 2

#define GP_HID_GAMEPAD	0
#define GP_HID_JOYSTICK 1

typedef struct {
	uint32_t magic;	  // GP_CONFIG_MAGIC
	uint16_t version; // GP_CONFIG_VERSION
	uint16_t size;	  // sizeof(gp_config_t)

	// USB identity
	uint16_t usb_vid;
	uint16_t usb_pid;
	char	 usb_manufacturer[GP_STR_LEN];
	char	 usb_product[GP_STR_LEN];
	char	 usb_serial[GP_STR_LEN]; // empty = chip unique ID
	uint16_t hid_rate_hz;
	uint8_t	 hid_report_id;
	uint8_t	 hid_device_type; // GP_HID_*

	// Input processing
	uint16_t poll_rate_hz;
	uint8_t	 smoothing_samples;
	uint8_t	 smoothing_mode; // GP_SMOOTH_*
	uint8_t	 debounce_ms;
	uint8_t	 num_axes;
	uint8_t	 num_buttons;
	uint8_t	 num_led_pins;
	uint16_t center_dz; // permille, global default
	uint16_t outer_dz;	// permille, global default

	// Calibration / reset button
	uint8_t	 cal_pin; // GPIO or GP_PIN_NONE
	uint8_t	 reserved0;
	uint16_t cal_hold_ms;	// hold to start calibration
	uint16_t reset_hold_ms; // hold to factory reset, 0 = disabled

	// LEDs (WS2812, a chain of led_counts[i] LEDs on each pin)
	uint16_t brightness;		// permille
	uint16_t status_brightness; // permille, idle "breathing" of status LEDs
	uint8_t	 led_flags; // GP_LED_*
	uint8_t	 reserved1;
	uint8_t	 led_pins[GP_MAX_LED_PINS];
	uint8_t	 led_counts[GP_MAX_LED_PINS]; // LEDs in the chain (built-in LED pin: 1)
	uint32_t status_led_mask[2];
	uint16_t reserved2;
	uint16_t reserved3;

	gp_group_cfg_t	groups[GP_MAX_GROUPS];
	gp_axis_cfg_t	axes[GP_MAX_AXES];
	gp_button_cfg_t buttons[GP_MAX_BUTTONS];
} gp_config_t;

// ---------------------------------------------------------------------------
// Calibration (persisted, produced by the device)
// ---------------------------------------------------------------------------

// One record per ADC channel (pin - limits.adc_first_pin), independent of
// which axis slot uses the pin, so re-ordering axes keeps calibration.
typedef struct {
	uint8_t	 calibrated; // 0/1
	uint8_t	 reserved;
	uint16_t min, center, max;
} gp_cal_channel_t;

#define GP_ORIENT_SWAP	   0x01
#define GP_ORIENT_INVERT_H 0x02
#define GP_ORIENT_INVERT_V 0x04
#define GP_ORIENT_ALL	   0x07

// Stick calibration. Raw frame: dx comes from pin_a, dy from pin_b.
// radial[] holds the maximum deflection per raw angle bin
// (bin b covers atan2(dy,dx) in [-pi + b*2pi/N, -pi + (b+1)*2pi/N)).
typedef struct {
	uint8_t	 calibrated; // 0/1
	uint8_t	 pin_a;
	uint8_t	 pin_b;
	uint8_t	 orient; // GP_ORIENT_*, maps raw frame to (horizontal, up)
	uint16_t center_a;
	uint16_t center_b;
	uint16_t radial[GP_RADIAL_BINS];
} gp_cal_group_t;

typedef struct {
	uint32_t		 magic;	  // GP_CAL_MAGIC
	uint16_t		 version; // GP_CAL_VERSION
	uint16_t		 size;	  // sizeof(gp_calibration_t)
	gp_cal_channel_t channels[GP_MAX_AXES];
	gp_cal_group_t	 groups[GP_MAX_GROUPS];
} gp_calibration_t;

// ---------------------------------------------------------------------------
// Wire protocol
//
// Frame: 0xAA 0x55 | cmd (1) | len (2, LE) | payload (len) | crc32 (4, LE)
// crc32 = IEEE 802.3 CRC over cmd, len bytes and payload.
// Every response uses the request's command code; the first payload byte of
// a response is a GP_STATUS_* code.
// ---------------------------------------------------------------------------
#define GP_FRAME_SYNC0		 0xAA
#define GP_FRAME_SYNC1		 0x55
#define GP_FRAME_OVERHEAD	 9
#define GP_PROTO_MAX_PAYLOAD 2048

enum {
	GP_CMD_HELLO			 = 0x01, // -> status, gp_hello_t
	GP_CMD_GET_LIMITS		 = 0x02, // -> status, gp_limits_t
	GP_CMD_READ_CONFIG		 = 0x03, // -> status, gp_config_t (active)
	GP_CMD_VALIDATE_CONFIG	 = 0x04, // gp_config_t -> error list (no side effects)
	GP_CMD_WRITE_CONFIG		 = 0x05, // gp_config_t -> error list; on OK saves and reboots
	GP_CMD_READ_CALIBRATION	 = 0x06, // -> status, gp_calibration_t (active)
	GP_CMD_WRITE_CALIBRATION = 0x07, // gp_calibration_t -> error list; applied live and saved
	GP_CMD_TELEMETRY		 = 0x08, // -> status, gp_telemetry_t
	GP_CMD_CAL_START		 = 0x09, // -> status
	GP_CMD_CAL_FINISH		 = 0x0A, // -> status, gp_cal_result_t (saved)
	GP_CMD_CAL_CANCEL		 = 0x0B, // -> status (nothing changes)
	GP_CMD_LED_PREVIEW		 = 0x0C, // gp_config_t -> status; LED-only preview, expires after GP_PREVIEW_TIMEOUT_MS
	GP_CMD_LED_PREVIEW_STOP	 = 0x0D, // -> status
	GP_CMD_FACTORY_RESET	 = 0x0E, // -> status, then erases storage and reboots
	GP_CMD_REBOOT			 = 0x0F, // -> status, then reboots
	GP_CMD_RESET_CALIBRATION = 0x10, // -> status; calibration back to defaults (saved)
	GP_CMD_SIM_INPUT		 = 0x70, // simulator only: gp_sim_input_t -> status
};

enum {
	GP_STATUS_OK		  = 0,
	GP_STATUS_INVALID	  = 1, // validation failed, error list follows
	GP_STATUS_BAD_LENGTH  = 2, // payload size does not match
	GP_STATUS_BAD_STATE	  = 3, // not allowed right now (e.g. during calibration)
	GP_STATUS_STORAGE	  = 4, // flash write or verification failed
	GP_STATUS_UNKNOWN_CMD = 5,
};

#define GP_PREVIEW_TIMEOUT_MS 1500

// Hello flags
#define GP_HELLO_SAFE_MODE		  0x0001 // repeated crashes: stored config not applied
#define GP_HELLO_CONFIG_DEFAULTED 0x0002 // stored config missing/invalid -> defaults
#define GP_HELLO_CAL_DEFAULTED	  0x0004 // stored calibration missing/invalid -> defaults
#define GP_HELLO_STORAGE_EMPTY	  0x0008 // no valid storage slot found
#define GP_HELLO_STORAGE_RECOVERED 0x0010 // one slot was damaged, the other was used
#define GP_HELLO_SIMULATOR		  0x8000

#define GP_RESET_POWER_ON 0
#define GP_RESET_WATCHDOG 1 // watchdog timeout (hang or crash)
#define GP_RESET_REQUESTED 2 // reboot after save / command

typedef struct {
	uint16_t proto_version;
	uint16_t config_version;
	uint16_t config_size;
	uint16_t cal_version;
	uint16_t cal_size;
	uint16_t telemetry_size;
	uint16_t limits_size;
	uint16_t flags; // GP_HELLO_*
	uint32_t storage_seq;
	uint8_t	 chip_id[8];
	uint8_t	 reset_reason; // GP_RESET_*
	uint8_t	 crash_count;  // consecutive watchdog resets
	uint16_t reserved;
	char	 fw_version[GP_STR_LEN];
	char	 board[GP_STR_LEN];
} gp_hello_t;

// Field identifiers used by error reports and the range table.
enum {
	GP_F_NONE = 0,
	GP_F_HEADER,
	GP_F_USB_VID,
	GP_F_USB_PID,
	GP_F_USB_MANUFACTURER,
	GP_F_USB_PRODUCT,
	GP_F_USB_SERIAL,
	GP_F_HID_RATE,
	GP_F_HID_REPORT_ID,
	GP_F_HID_DEVICE_TYPE,
	GP_F_POLL_RATE, // 10
	GP_F_SMOOTHING_SAMPLES,
	GP_F_SMOOTHING_MODE,
	GP_F_DEBOUNCE,
	GP_F_CENTER_DZ,
	GP_F_OUTER_DZ,
	GP_F_NUM_AXES,
	GP_F_NUM_BUTTONS,
	GP_F_NUM_LED_PINS,
	GP_F_CAL_PIN,
	GP_F_CAL_HOLD, // 20
	GP_F_RESET_HOLD,
	GP_F_BRIGHTNESS,
	GP_F_STATUS_BRIGHTNESS,
	GP_F_LED_PIN,
	GP_F_STATUS_LED_MASK,
	GP_F_LED_FLAGS,
	GP_F_AXIS_PIN,
	GP_F_AXIS_USAGE,
	GP_F_AXIS_FLAGS,
	GP_F_AXIS_CENTER_DZ, // 30
	GP_F_AXIS_OUTER_DZ,
	GP_F_AXIS_LED_MASK,
	GP_F_AXIS_COLOR,
	GP_F_BUTTON_PIN,
	GP_F_BUTTON_FLAGS,
	GP_F_BUTTON_LED_MASK,
	GP_F_BUTTON_COLOR,
	GP_F_GROUP_ENABLED,
	GP_F_GROUP_AXES,
	GP_F_GROUP_SHAPE, // 40
	GP_F_GROUP_FLAGS,
	GP_F_GROUP_CENTER_DZ,
	GP_F_GROUP_OUTER_DZ,
	GP_F_CAL_CHANNEL,
	GP_F_CAL_GROUP,
	GP_F_LED_COUNT,
	GP_F_AXIS_LED_EFFECT,
	GP_F_COUNT
};

typedef struct {
	uint16_t min, max;
} gp_range_t;

typedef struct {
	uint8_t	   max_axes; // schema capacity
	uint8_t	   max_buttons;
	uint8_t	   max_leds;	 // LEDs on all pins together
	uint8_t	   max_groups;
	uint8_t	   platform_axes;	 // ADC channels available on this chip
	uint8_t	   platform_led_pins; // LED data pins this build can drive
	uint8_t	   gpio_count;
	uint8_t	   adc_first_pin;
	uint8_t	   radial_bins;
	uint8_t	   str_len;
	uint8_t	   num_hid_usages;
	uint8_t	   max_led_pins;
	uint16_t   dz_sum_max;		// centre + outer deadzone must not exceed this (permille)
	uint8_t	   builtin_led_pin; // GPIO of the board's own LED (always a single LED), GP_PIN_NONE if none
	uint8_t	   reserved1;
	uint32_t   button_pins[2]; // 64-bit GPIO masks, [0] = GPIO 0..31
	uint32_t   led_pins[2];
	uint32_t   adc_pins[2];
	uint32_t   cal_pins[2];
	uint8_t	   hid_usages[12];
	gp_range_t ranges[GP_F_COUNT]; // numeric ranges per field (0,0 = n/a)
} gp_limits_t;

#define GP_MAX_ERRORS	   16
#define GP_ERROR_MSG_LEN 60

typedef struct {
	uint8_t field; // GP_F_*
	uint8_t index; // array index (axis/button/LED pin/group), 0xFF if n/a
	uint8_t reserved[2];
	char	message[GP_ERROR_MSG_LEN];
} gp_error_t;

// Response payload of VALIDATE/WRITE commands:
// status (1) | count (1) | truncated (1) | reserved (1) | gp_error_t[count]
typedef struct {
	uint8_t	   status;
	uint8_t	   count;
	uint8_t	   truncated;
	uint8_t	   reserved;
	gp_error_t errors[GP_MAX_ERRORS];
} gp_error_list_t;

// Telemetry state bits
#define GP_STATE_CALIBRATING   0x01
#define GP_STATE_USB_MOUNTED   0x02
#define GP_STATE_SLEEPING	   0x04
#define GP_STATE_PREVIEW	   0x08
#define GP_STATE_SAFE_MODE	   0x10
#define GP_STATE_CAL_BUTTON	   0x20
#define GP_STATE_RESET_PENDING 0x40

// Calibration progress per channel / group
#define GP_CALCH_IDLE	0 // not moved (yet)
#define GP_CALCH_MOVING 1 // moved, range still too small
#define GP_CALCH_OK		2 // enough range captured

#define GP_CALGRP_IDLE	   0 // not touched
#define GP_CALGRP_UP	   1 // initial "up" push captured
#define GP_CALGRP_ROTATING 2 // rotating, see sweep
#define GP_CALGRP_DONE	   3 // full turn captured with direction

typedef struct {
	uint32_t uptime_ms;
	uint32_t buttons[2];
	int16_t	 axes[GP_MAX_AXES];		// output as reported over HID (-32767..32767)
	uint16_t raw[GP_MAX_AXES];		// filtered ADC values
	uint16_t raw_now[GP_MAX_AXES];	// latest unfiltered ADC sample
	uint16_t cal_min[GP_MAX_AXES];	// during calibration: captured range,
	uint16_t cal_center[GP_MAX_AXES]; // otherwise: effective calibration
	uint16_t cal_max[GP_MAX_AXES];
	uint8_t	 led_rgb[GP_MAX_LEDS][4]; // LED colour before global brightness (r,g,b,0)
	uint16_t cal_group_sweep[GP_MAX_GROUPS]; // degrees rotated
	uint8_t	 cal_channel_state[GP_MAX_AXES]; // GP_CALCH_* per axis slot
	uint8_t	 cal_group_state[GP_MAX_GROUPS];
	uint8_t	 cal_group_coverage[GP_MAX_GROUPS]; // radial bins captured
	uint8_t	 state;							   // GP_STATE_*
	uint8_t	 reserved;
	uint16_t loop_max_us;		// slowest main loop iteration in the last second
	uint16_t hid_reports_per_s; // HID reports actually sent in the last second
	uint16_t reserved2;
} gp_telemetry_t;

// Calibration result codes
#define GP_CALRES_UNCHANGED 0 // not touched, previous calibration kept
#define GP_CALRES_UPDATED	1
#define GP_CALRES_FAILED	2 // touched but incomplete -> marked uncalibrated

typedef struct {
	uint8_t channel[GP_MAX_AXES]; // per axis slot
	uint8_t group[GP_MAX_GROUPS];
} gp_cal_result_t;

// Simulator only: override inputs
typedef struct {
	uint16_t raw[GP_MAX_AXES];
	uint32_t buttons[2];
	uint8_t	 cal_button;
	uint8_t	 auto_motion; // 1 = simulator generates motion itself
	uint8_t	 usb_state;	  // 0 = mounted, 1 = suspended, 2 = unplugged
	uint8_t	 reserved;
} gp_sim_input_t;

// ---------------------------------------------------------------------------
// Layout checks
// ---------------------------------------------------------------------------
_Static_assert(sizeof(gp_color_t) == 4, "gp_color_t");
_Static_assert(sizeof(gp_axis_cfg_t) == 20, "gp_axis_cfg_t");
_Static_assert(sizeof(gp_button_cfg_t) == 16, "gp_button_cfg_t");
_Static_assert(sizeof(gp_group_cfg_t) == 12, "gp_group_cfg_t");
_Static_assert(sizeof(gp_config_t) == 1396, "gp_config_t");
_Static_assert(sizeof(gp_cal_channel_t) == 8, "gp_cal_channel_t");
_Static_assert(sizeof(gp_cal_group_t) == 72, "gp_cal_group_t");
_Static_assert(sizeof(gp_calibration_t) == 360, "gp_calibration_t");
_Static_assert(sizeof(gp_hello_t) == 96, "gp_hello_t");
_Static_assert(sizeof(gp_error_t) == 64, "gp_error_t");
_Static_assert(sizeof(gp_telemetry_t) == 396, "gp_telemetry_t");
_Static_assert(sizeof(gp_cal_result_t) == 12, "gp_cal_result_t");
_Static_assert(sizeof(gp_sim_input_t) == 28, "gp_sim_input_t");
_Static_assert(sizeof(gp_limits_t) == 60 + 4 * GP_F_COUNT, "gp_limits_t");
_Static_assert(sizeof(gp_error_list_t) <= GP_PROTO_MAX_PAYLOAD, "error list fits a frame");
_Static_assert(1 + sizeof(gp_config_t) <= GP_PROTO_MAX_PAYLOAD, "config fits a frame");

#endif
