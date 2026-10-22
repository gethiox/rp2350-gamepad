// SPDX-License-Identifier: MIT
//
// hid.h - HID report descriptor and report packing.
//
// Report layout (after the report ID byte added by the USB stack):
//   ceil(num_buttons / 8) bytes of button bits (button 1 = bit 0)
//   num_axes x int16 little endian, logical range -32767..32767

#ifndef GP_HID_H
#define GP_HID_H

#include <stdint.h>

#include "gp_protocol.h"

#define GP_HID_DESC_MAX	  96
#define GP_HID_REPORT_MAX (GP_MAX_BUTTONS / 8 + GP_MAX_AXES * 2)

uint16_t gp_hid_descriptor(const gp_config_t* cfg, uint8_t* out, uint16_t cap);
uint16_t gp_hid_report(const gp_config_t* cfg, const uint32_t buttons[2], const int16_t* axes, uint8_t* out);

#endif
