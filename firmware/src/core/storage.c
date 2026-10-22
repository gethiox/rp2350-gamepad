// SPDX-License-Identifier: MIT
#include "storage.h"

#include <stddef.h>
#include <string.h>

#include "config.h"
#include "crc32.h"
#include "hal.h"

#define RECORD_SIZE (sizeof(gp_storage_header_t) + sizeof(gp_config_t) + sizeof(gp_calibration_t))
#define PROGRAM_SIZE ((RECORD_SIZE + 255u) & ~255u) // flash page multiple

_Static_assert(PROGRAM_SIZE <= HAL_STORAGE_SECTOR_SIZE, "record fits a sector");

static uint8_t record_buf[PROGRAM_SIZE]; // static: keeps the stack small

static uint32_t header_crc(const gp_storage_header_t* h) {
	return gp_crc32(h, offsetof(gp_storage_header_t, header_crc));
}

// Returns true if the slot holds a structurally valid record.
static bool slot_valid(const uint8_t* p, gp_storage_header_t* out) {
	gp_storage_header_t h;
	memcpy(&h, p, sizeof(h));
	if (h.magic != GP_STORAGE_MAGIC || h.header_size != sizeof(h) || h.header_crc != header_crc(&h))
		return false;
	if (h.format != GP_STORAGE_FORMAT)
		return false; // future: migrate older storage formats here
	uint32_t payload = (uint32_t)h.config_size + h.cal_size;
	if (payload + sizeof(h) > HAL_STORAGE_SECTOR_SIZE)
		return false;
	if (gp_crc32(p + sizeof(h), payload) != h.payload_crc)
		return false;
	*out = h;
	return true;
}

// Newer sequence number with wrap-around.
static bool seq_newer(uint32_t a, uint32_t b) { return (int32_t)(a - b) > 0; }

void gp_storage_load(gp_config_t* cfg, gp_calibration_t* cal, gp_storage_state_t* st) {
	memset(st, 0, sizeof(*st));
	st->slot = -1;
	gp_config_defaults(cfg);
	gp_calibration_defaults(cal);

	gp_storage_header_t h[2];
	bool				valid[2];
	bool				present[2];
	for (int s = 0; s < 2; s++) {
		const uint8_t* p = hal_storage_read((uint8_t)s);
		valid[s]		 = slot_valid(p, &h[s]);
		uint32_t magic;
		memcpy(&magic, p, 4);
		present[s] = magic == GP_STORAGE_MAGIC; // something was written there
	}

	int use = -1;
	if (valid[0] && valid[1])
		use = seq_newer(h[1].seq, h[0].seq) ? 1 : 0;
	else if (valid[0])
		use = 0;
	else if (valid[1])
		use = 1;

	if (use < 0) {
		st->empty = true;
		return;
	}
	// A damaged slot that was written after the one we use means a save was
	// interrupted; report it so the configurator can tell the user.
	int other	  = 1 - use;
	st->recovered = present[other] && !valid[other];
	st->slot	  = use;
	st->seq		  = h[use].seq;

	const uint8_t* p = hal_storage_read((uint8_t)use) + sizeof(gp_storage_header_t);
	// Only records of the exact current layout are used. Older layouts would
	// be migrated here once a second version exists.
	if (h[use].config_version == GP_CONFIG_VERSION && h[use].config_size == sizeof(gp_config_t)) {
		memcpy(cfg, p, sizeof(gp_config_t));
		st->config_loaded = true;
	}
	p += h[use].config_size;
	if (h[use].cal_version == GP_CAL_VERSION && h[use].cal_size == sizeof(gp_calibration_t)) {
		memcpy(cal, p, sizeof(gp_calibration_t));
		st->cal_loaded = true;
	}
}

bool gp_storage_save(const gp_config_t* cfg, const gp_calibration_t* cal, gp_storage_state_t* st) {
	memset(record_buf, 0xFF, sizeof(record_buf));
	gp_storage_header_t h;
	memset(&h, 0, sizeof(h));
	h.magic			 = GP_STORAGE_MAGIC;
	h.format		 = GP_STORAGE_FORMAT;
	h.header_size	 = sizeof(h);
	h.seq			 = st->slot >= 0 ? st->seq + 1 : 1;
	h.config_version = GP_CONFIG_VERSION;
	h.config_size	 = sizeof(gp_config_t);
	h.cal_version	 = GP_CAL_VERSION;
	h.cal_size		 = sizeof(gp_calibration_t);

	uint8_t* payload = record_buf + sizeof(h);
	memcpy(payload, cfg, sizeof(gp_config_t));
	memcpy(payload + sizeof(gp_config_t), cal, sizeof(gp_calibration_t));
	h.payload_crc = gp_crc32(payload, sizeof(gp_config_t) + sizeof(gp_calibration_t));
	h.header_crc  = header_crc(&h);
	memcpy(record_buf, &h, sizeof(h));

	int target = st->slot == 0 ? 1 : 0;
	if (!hal_storage_write((uint8_t)target, record_buf, sizeof(record_buf)))
		return false;

	// Verify what actually landed in flash.
	gp_storage_header_t check;
	const uint8_t*		p = hal_storage_read((uint8_t)target);
	if (!slot_valid(p, &check) || check.seq != h.seq || memcmp(p, record_buf, RECORD_SIZE) != 0)
		return false;

	st->slot	  = target;
	st->seq		  = h.seq;
	st->empty	  = false;
	st->recovered = false;
	return true;
}

bool gp_storage_erase_all(gp_storage_state_t* st) {
	bool ok = hal_storage_erase(0);
	ok		= hal_storage_erase(1) && ok;
	memset(st, 0, sizeof(*st));
	st->slot  = -1;
	st->empty = true;
	return ok;
}
