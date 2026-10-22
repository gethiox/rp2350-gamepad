// SPDX-License-Identifier: MIT
#ifndef GP_USB_DESCRIPTORS_H
#define GP_USB_DESCRIPTORS_H

#include "gp_protocol.h"

// Builds all USB descriptors from the active configuration. Must run before tusb_init().
void usb_descriptors_init(const gp_config_t* cfg);

#endif
