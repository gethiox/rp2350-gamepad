// SPDX-License-Identifier: MIT
//
// usb_descriptors.c - composite CDC (configuration port) + HID (gamepad).
// Descriptors are generated once at boot from the active configuration.

#include <string.h>

#include "tusb.h"

#include "app.h"
#include "config.h"
#include "hal.h"
#include "hid.h"
#include "usb_descriptors.h"

#ifndef GP_FW_BCD
#define GP_FW_BCD 0x0100
#endif

enum { ITF_NUM_CDC = 0, ITF_NUM_CDC_DATA, ITF_NUM_HID, ITF_NUM_TOTAL };

#define EPNUM_CDC_NOTIF 0x81
#define EPNUM_CDC_OUT	0x02
#define EPNUM_CDC_IN	0x82
#define EPNUM_HID		0x83

enum { STR_LANGID = 0, STR_MANUFACTURER, STR_PRODUCT, STR_SERIAL, STR_CDC, STR_HID, STR_COUNT };

#define CONFIG_TOTAL_LEN (TUD_CONFIG_DESC_LEN + TUD_CDC_DESC_LEN + TUD_HID_DESC_LEN)

static tusb_desc_device_t desc_device;
static uint8_t			  desc_config[CONFIG_TOTAL_LEN];
static uint8_t			  desc_hid[GP_HID_DESC_MAX];
static char				  serial[GP_STR_LEN];
static const gp_config_t* cfg;

void usb_descriptors_init(const gp_config_t* config) {
	cfg = config;

	// The CDC function uses an Interface Association Descriptor, so the
	// device must declare the Miscellaneous / IAD class triple.
	desc_device = (tusb_desc_device_t){
		.bLength			= sizeof(tusb_desc_device_t),
		.bDescriptorType	= TUSB_DESC_DEVICE,
		.bcdUSB				= 0x0200,
		.bDeviceClass		= TUSB_CLASS_MISC,
		.bDeviceSubClass	= MISC_SUBCLASS_COMMON,
		.bDeviceProtocol	= MISC_PROTOCOL_IAD,
		.bMaxPacketSize0	= CFG_TUD_ENDPOINT0_SIZE,
		.idVendor			= cfg->usb_vid,
		.idProduct			= cfg->usb_pid,
		.bcdDevice			= GP_FW_BCD,
		.iManufacturer		= STR_MANUFACTURER,
		.iProduct			= STR_PRODUCT,
		.iSerialNumber		= STR_SERIAL,
		.bNumConfigurations = 1,
	};

	uint16_t hid_len = gp_hid_descriptor(cfg, desc_hid, sizeof(desc_hid));

	uint32_t interval = 1000u / cfg->hid_rate_hz; // full speed: bInterval in ms
	if (interval < 1)
		interval = 1;
	if (interval > 255)
		interval = 255;

	// WS2812 LEDs can draw up to ~60 mA each at full white.
	uint32_t power_ma = 100u + 60u * gp_led_total(cfg);
	if (power_ma > 500)
		power_ma = 500;

	// No remote wakeup: the device follows the host to sleep and never wakes it.
	const uint8_t tmp[] = {
		TUD_CONFIG_DESCRIPTOR(1, ITF_NUM_TOTAL, 0, CONFIG_TOTAL_LEN, 0, power_ma),
		TUD_CDC_DESCRIPTOR(ITF_NUM_CDC, STR_CDC, EPNUM_CDC_NOTIF, 8, EPNUM_CDC_OUT, EPNUM_CDC_IN, 64),
		TUD_HID_DESCRIPTOR(ITF_NUM_HID, STR_HID, HID_ITF_PROTOCOL_NONE, hid_len, EPNUM_HID, CFG_TUD_HID_EP_BUFSIZE,
						   interval),
	};
	_Static_assert(sizeof(tmp) == CONFIG_TOTAL_LEN, "config descriptor size");
	memcpy(desc_config, tmp, sizeof(tmp));

	if (cfg->usb_serial[0]) {
		memcpy(serial, cfg->usb_serial, sizeof(serial)); // validated: NUL terminated
		serial[sizeof(serial) - 1] = 0;
	} else {
		uint8_t id[8];
		hal_chip_id(id);
		static const char hex[] = "0123456789ABCDEF";
		for (int i = 0; i < 8; i++) {
			serial[2 * i]	  = hex[id[i] >> 4];
			serial[2 * i + 1] = hex[id[i] & 0xF];
		}
		serial[16] = 0;
	}
}

uint8_t const* tud_descriptor_device_cb(void) { return (uint8_t const*)&desc_device; }
uint8_t const* tud_descriptor_configuration_cb(uint8_t index) {
	(void)index;
	return desc_config;
}
uint8_t const* tud_hid_descriptor_report_cb(uint8_t instance) {
	(void)instance;
	return desc_hid;
}

static uint16_t desc_str[GP_STR_LEN + 1];

uint16_t const* tud_descriptor_string_cb(uint8_t index, uint16_t langid) {
	(void)langid;
	const char* s;
	size_t		n;
	switch (index) {
	case STR_LANGID:
		desc_str[1] = 0x0409; // English (US)
		desc_str[0] = (uint16_t)((TUSB_DESC_STRING << 8) | 4);
		return desc_str;
	case STR_MANUFACTURER:
		s = cfg->usb_manufacturer;
		break;
	case STR_PRODUCT:
		s = cfg->usb_product;
		break;
	case STR_SERIAL:
		s = serial;
		break;
	case STR_CDC:
		s = "Configuration";
		break;
	case STR_HID:
		s = cfg->usb_product;
		break;
	default:
		return NULL;
	}
	n = strnlen(s, GP_STR_LEN - 1);
	for (size_t i = 0; i < n; i++)
		desc_str[1 + i] = (uint8_t)s[i];
	desc_str[0] = (uint16_t)((TUSB_DESC_STRING << 8) | (2 * n + 2));
	return desc_str;
}

// HID GET/SET_REPORT over the control pipe are not used.
uint16_t tud_hid_get_report_cb(uint8_t itf, uint8_t report_id, hid_report_type_t report_type, uint8_t* buffer,
							   uint16_t reqlen) {
	(void)itf;
	(void)report_id;
	(void)report_type;
	(void)buffer;
	(void)reqlen;
	return 0;
}

void tud_hid_set_report_cb(uint8_t itf, uint8_t report_id, hid_report_type_t report_type, uint8_t const* buffer,
						   uint16_t bufsize) {
	(void)itf;
	(void)report_id;
	(void)report_type;
	(void)buffer;
	(void)bufsize;
}
