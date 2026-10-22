# SPDX-License-Identifier: MIT
# Fails if the firmware binary reaches into the settings storage area.
file(SIZE "${BIN}" size)
math(EXPR limit "${LIMIT}" OUTPUT_FORMAT DECIMAL)
if(size GREATER_EQUAL limit)
    message(FATAL_ERROR "Firmware image (${size} bytes) overlaps settings storage at ${LIMIT}")
endif()
math(EXPR pct "100 * ${size} / ${limit}")
message(STATUS "Firmware image: ${size} bytes (${pct}% of the space before settings storage)")
