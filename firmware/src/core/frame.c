// SPDX-License-Identifier: MIT
#include "frame.h"

#include <string.h>

#include "crc32.h"

void gp_frame_rx_init(gp_frame_rx_t* rx) { memset(rx, 0, sizeof(*rx)); }

static void consume(gp_frame_rx_t* rx, uint16_t n) {
	if (n >= rx->len) {
		rx->len = 0;
		return;
	}
	memmove(rx->buf, rx->buf + n, rx->len - n);
	rx->len -= n;
}

static uint32_t frame_crc(uint8_t cmd, uint16_t len, const uint8_t* payload) {
	uint8_t head[3] = { cmd, (uint8_t)(len & 0xFF), (uint8_t)(len >> 8) };
	return gp_crc32_update(gp_crc32(head, 3), payload, len);
}

static void parse(gp_frame_rx_t* rx, gp_frame_handler_t handler, void* ctx) {
	while (rx->len >= GP_FRAME_OVERHEAD) {
		if (rx->buf[0] != GP_FRAME_SYNC0 || rx->buf[1] != GP_FRAME_SYNC1) {
			// Skip to the next possible sync byte.
			const uint8_t* p = memchr(rx->buf + 1, GP_FRAME_SYNC0, rx->len - 1);
			uint16_t	   n = p ? (uint16_t)(p - rx->buf) : rx->len;
			rx->dropped += n;
			consume(rx, n);
			continue;
		}
		uint16_t len = (uint16_t)(rx->buf[3] | (rx->buf[4] << 8));
		if (len > GP_PROTO_MAX_PAYLOAD) {
			rx->dropped++;
			consume(rx, 1);
			continue;
		}
		uint16_t total = (uint16_t)(len + GP_FRAME_OVERHEAD);
		if (rx->len < total)
			return; // wait for the rest
		uint32_t crc;
		memcpy(&crc, rx->buf + 5 + len, 4);
		if (crc != frame_crc(rx->buf[2], len, rx->buf + 5)) {
			rx->crc_errors++;
			rx->dropped++;
			consume(rx, 1); // could be a false sync; rescan from the next byte
			continue;
		}
		handler(ctx, rx->buf[2], rx->buf + 5, len);
		consume(rx, total);
	}
}

void gp_frame_rx_feed(gp_frame_rx_t* rx, const uint8_t* data, size_t len, gp_frame_handler_t handler, void* ctx) {
	while (len > 0) {
		size_t space = sizeof(rx->buf) - rx->len;
		if (space == 0) {
			// Cannot happen with a well-formed stream (buffer holds two max
			// frames); recover by dropping the oldest byte.
			rx->dropped++;
			consume(rx, 1);
			continue;
		}
		size_t n = len < space ? len : space;
		memcpy(rx->buf + rx->len, data, n);
		rx->len = (uint16_t)(rx->len + n);
		data += n;
		len -= n;
		parse(rx, handler, ctx);
	}
}

size_t gp_frame_encode(uint8_t* out, uint8_t cmd, const void* a, uint16_t alen, const void* b, uint16_t blen) {
	uint16_t len = (uint16_t)(alen + blen);
	out[0]		 = GP_FRAME_SYNC0;
	out[1]		 = GP_FRAME_SYNC1;
	out[2]		 = cmd;
	out[3]		 = (uint8_t)(len & 0xFF);
	out[4]		 = (uint8_t)(len >> 8);
	if (alen)
		memcpy(out + 5, a, alen);
	if (blen)
		memcpy(out + 5 + alen, b, blen);
	uint32_t crc = frame_crc(cmd, len, out + 5);
	memcpy(out + 5 + len, &crc, 4);
	return (size_t)len + GP_FRAME_OVERHEAD;
}
