// SPDX-License-Identifier: MIT
//
// storage.h - power-loss safe persistence of configuration + calibration.
//
// Two flash sectors ("slots") are used alternately. Each save writes a
// complete record (header + config + calibration) with a sequence number
// and CRCs into the slot that does NOT hold the newest valid record, then
// reads it back to verify. If power is lost mid-write the previous record
// is still intact in the other slot and is used on the next boot.

#ifndef GP_STORAGE_H
#define GP_STORAGE_H

#include <stdbool.h>
#include <stdint.h>

#include "gp_protocol.h"

#define GP_STORAGE_MAGIC  0x53475052u // "RPGS"
#define GP_STORAGE_FORMAT 1

typedef struct {
	uint32_t magic;
	uint16_t format;
	uint16_t header_size;
	uint32_t seq;
	uint16_t config_version;
	uint16_t config_size;
	uint16_t cal_version;
	uint16_t cal_size;
	uint32_t payload_crc; // over config + calibration bytes
	uint32_t reserved;
	uint32_t header_crc; // over all preceding header bytes
} gp_storage_header_t;

_Static_assert(sizeof(gp_storage_header_t) == 32, "storage header");

typedef struct {
	bool	 config_loaded; // stored config was valid and used
	bool	 cal_loaded;
	bool	 empty;			// no valid slot at all
	bool	 recovered;		// newest slot was damaged, older one used
	uint32_t seq;
	int		 slot; // slot holding the active record, -1 if none
} gp_storage_state_t;

// Loads into cfg/cal. Anything missing or unusable is replaced by defaults.
// The caller still validates the result against the platform limits.
void gp_storage_load(gp_config_t* cfg, gp_calibration_t* cal, gp_storage_state_t* st);

// Writes a new record. Never touches the slot holding the current record.
bool gp_storage_save(const gp_config_t* cfg, const gp_calibration_t* cal, gp_storage_state_t* st);

// Erases both slots (factory reset).
bool gp_storage_erase_all(gp_storage_state_t* st);

#endif
