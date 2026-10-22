// SPDX-License-Identifier: MIT
//
// frame.h - byte-stream framing (see gp_protocol.h for the frame format).

#ifndef GP_FRAME_H
#define GP_FRAME_H

#include <stddef.h>
#include <stdint.h>

#include "gp_protocol.h"

#define GP_RX_BUF_SIZE (2 * (GP_PROTO_MAX_PAYLOAD + GP_FRAME_OVERHEAD))

typedef void (*gp_frame_handler_t)(void* ctx, uint8_t cmd, const uint8_t* payload, uint16_t len);

typedef struct {
	uint8_t	 buf[GP_RX_BUF_SIZE];
	uint16_t len;
	uint32_t crc_errors; // diagnostics
	uint32_t dropped;	 // bytes discarded while resynchronising
} gp_frame_rx_t;

void gp_frame_rx_init(gp_frame_rx_t* rx);

// Appends bytes and dispatches every complete, CRC-valid frame.
// Garbage (e.g. modem probes from the OS) is skipped byte by byte, so a
// valid frame following junk is never lost.
void gp_frame_rx_feed(gp_frame_rx_t* rx, const uint8_t* data, size_t len, gp_frame_handler_t handler, void* ctx);

// Encodes a frame into `out` (must hold len + GP_FRAME_OVERHEAD bytes).
// Payload is given as up to two parts (status byte + struct, typically).
size_t gp_frame_encode(uint8_t* out, uint8_t cmd, const void* a, uint16_t alen, const void* b, uint16_t blen);

#endif
