# SPDX-License-Identifier: MIT
# Fails if protocol/layout.json does not match the current C structures.
execute_process(COMMAND ${DUMP} OUTPUT_VARIABLE actual RESULT_VARIABLE rc)
if(NOT rc EQUAL 0)
    message(FATAL_ERROR "layout_dump failed")
endif()
file(READ ${EXPECTED} expected)
if(NOT actual STREQUAL expected)
    message(FATAL_ERROR "protocol/layout.json is out of date. Regenerate it with:\n"
                        "  ${DUMP} > protocol/layout.json\n"
                        "and update the Go mirror structures in configurator/internal/proto.")
endif()
