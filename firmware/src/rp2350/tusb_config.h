// SPDX-License-Identifier: MIT
#ifndef TUSB_CONFIG_H
#define TUSB_CONFIG_H

#define CFG_TUSB_RHPORT0_MODE  OPT_MODE_DEVICE
#define CFG_TUD_ENDPOINT0_SIZE 64

#define CFG_TUD_HID 1
#define CFG_TUD_CDC 1

#define CFG_TUD_HID_EP_BUFSIZE 64

// Software FIFOs (RAM, not USB DPRAM). TX holds the largest response
// (error list, ~1 KiB) in one piece; responses are only written whole.
#define CFG_TUD_CDC_RX_BUFSIZE 1024
#define CFG_TUD_CDC_TX_BUFSIZE 2048
#define CFG_TUD_CDC_EP_BUFSIZE 64

#endif
