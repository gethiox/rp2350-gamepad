// SPDX-License-Identifier: MIT
#include "hid.h"

#include <string.h>

uint16_t gp_hid_descriptor(const gp_config_t* cfg, uint8_t* out, uint16_t cap) {
	uint8_t d[GP_HID_DESC_MAX];
	int		n = 0;
#define B(x) d[n++] = (uint8_t)(x)
	B(0x05);
	B(0x01); // Usage Page (Generic Desktop)
	B(0x09);
	B(cfg->hid_device_type == GP_HID_JOYSTICK ? 0x04 : 0x05); // Usage (Joystick / Game Pad)
	B(0xA1);
	B(0x01); // Collection (Application)
	B(0x85);
	B(cfg->hid_report_id); // Report ID

	if (cfg->num_buttons > 0) {
		B(0x05);
		B(0x09); // Usage Page (Button)
		B(0x19);
		B(0x01); // Usage Minimum (1)
		B(0x29);
		B(cfg->num_buttons); // Usage Maximum
		B(0x15);
		B(0x00); // Logical Minimum (0)
		B(0x25);
		B(0x01); // Logical Maximum (1)
		B(0x75);
		B(0x01); // Report Size (1)
		B(0x95);
		B(cfg->num_buttons); // Report Count
		B(0x81);
		B(0x02); // Input (Data, Var, Abs)
		uint8_t pad = (uint8_t)((8 - (cfg->num_buttons % 8)) % 8);
		if (pad) {
			B(0x75);
			B(0x01); // Report Size (1)
			B(0x95);
			B(pad); // Report Count
			B(0x81);
			B(0x03); // Input (Const)
		}
	}
	if (cfg->num_axes > 0) {
		B(0x05);
		B(0x01); // Usage Page (Generic Desktop)
		B(0x16);
		B(0x01);
		B(0x80); // Logical Minimum (-32767)
		B(0x26);
		B(0xFF);
		B(0x7F); // Logical Maximum (32767)
		for (int i = 0; i < cfg->num_axes; i++) {
			B(0x09);
			B(cfg->axes[i].hid_usage); // Usage
		}
		B(0x75);
		B(0x10); // Report Size (16)
		B(0x95);
		B(cfg->num_axes); // Report Count
		B(0x81);
		B(0x02); // Input (Data, Var, Abs)
	}
	B(0xC0); // End Collection
#undef B
	if (n > cap)
		return 0;
	memcpy(out, d, (size_t)n);
	return (uint16_t)n;
}

uint16_t gp_hid_report(const gp_config_t* cfg, const uint32_t buttons[2], const int16_t* axes, uint8_t* out) {
	uint16_t n		   = 0;
	int		 btn_bytes = (cfg->num_buttons + 7) / 8;
	for (int i = 0; i < btn_bytes; i++) {
		uint32_t word = buttons[i / 4];
		out[n++]	  = (uint8_t)(word >> (8 * (i % 4)));
	}
	// Clear the padding bits of the last button byte.
	if (cfg->num_buttons % 8)
		out[n - 1] &= (uint8_t)((1u << (cfg->num_buttons % 8)) - 1u);
	for (int i = 0; i < cfg->num_axes; i++) {
		uint16_t v = (uint16_t)axes[i];
		out[n++]   = (uint8_t)(v & 0xFF);
		out[n++]   = (uint8_t)(v >> 8);
	}
	return n;
}
