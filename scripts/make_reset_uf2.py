#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Create a UF2 file that erases the rp2350-gamepad settings area.

The file contains only 0xFF-filled blocks covering the two settings
sectors, using the RP2350 "absolute" UF2 family so the boot ROM writes them
at their absolute flash address. Copying it to the BOOTSEL drive therefore
resets configuration and calibration while leaving the firmware in place.
"""
import argparse
import struct

UF2_MAGIC0 = 0x0A324655
UF2_MAGIC1 = 0x9E5D5157
UF2_MAGIC_END = 0x0AB16F30
UF2_FLAG_FAMILY = 0x00002000
FAMILY_ABSOLUTE = 0xE48BFF57
XIP_BASE = 0x10000000
PAYLOAD = 256


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--offset", required=True, help="flash offset of the settings area (e.g. 0x1FE000)")
    ap.add_argument("--size", required=True, help="size in bytes (e.g. 0x2000)")
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    offset = int(a.offset, 0)
    size = int(a.size, 0)
    if offset % 4096 or size % 4096:
        raise SystemExit("offset and size must be 4 KiB aligned")

    count = size // PAYLOAD
    blocks = []
    for n in range(count):
        addr = XIP_BASE + offset + n * PAYLOAD
        header = struct.pack("<8I", UF2_MAGIC0, UF2_MAGIC1, UF2_FLAG_FAMILY, addr, PAYLOAD, n, count, FAMILY_ABSOLUTE)
        data = b"\xff" * PAYLOAD + b"\x00" * (476 - PAYLOAD)
        blocks.append(header + data + struct.pack("<I", UF2_MAGIC_END))
    with open(a.out, "wb") as f:
        f.write(b"".join(blocks))


if __name__ == "__main__":
    main()
