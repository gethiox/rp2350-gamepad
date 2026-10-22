// SPDX-License-Identifier: MIT
#ifndef GP_CRC32_H
#define GP_CRC32_H

#include <stddef.h>
#include <stdint.h>

// IEEE 802.3 CRC-32 (same as zlib / Go hash/crc32.ChecksumIEEE).
uint32_t gp_crc32_update(uint32_t crc, const void* data, size_t len);
static inline uint32_t gp_crc32(const void* data, size_t len) { return gp_crc32_update(0, data, len); }

#endif
