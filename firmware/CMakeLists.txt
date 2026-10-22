# SPDX-License-Identifier: MIT
#
# rp2350-gamepad firmware.
#
#   cmake -S firmware -B build -G Ninja -DCMAKE_BUILD_TYPE=Release
#   ninja -C build            -> build/rp2350-gamepad.uf2
#
# Requires PICO_SDK_PATH (Pico SDK 2.2.0) and an arm-none-eabi toolchain.
# See docs/building.md. For bit-identical (reproducible) builds use
# scripts/build-firmware.sh, which pins the toolchain and SOURCE_DATE_EPOCH.

cmake_minimum_required(VERSION 3.13)

set(PICO_BOARD rp2350_generic CACHE STRING "Board")
set(PICO_BOARD_HEADER_DIRS ${CMAKE_CURRENT_LIST_DIR}/boards)
set(PICO_PLATFORM rp2350-arm-s CACHE STRING "Platform")

include(pico_sdk_import.cmake)

project(rp2350_gamepad C CXX ASM)
set(CMAKE_C_STANDARD 11)

# Reproducible builds: no absolute paths in the binary (also for SDK code).
get_filename_component(GP_TOOLCHAIN_ROOT "${CMAKE_C_COMPILER}" DIRECTORY)
get_filename_component(GP_TOOLCHAIN_ROOT "${GP_TOOLCHAIN_ROOT}" DIRECTORY)
add_compile_options(
    -ffile-prefix-map=${GP_TOOLCHAIN_ROOT}=toolchain
    -ffile-prefix-map=${CMAKE_CURRENT_LIST_DIR}=firmware
    -ffile-prefix-map=${PICO_SDK_PATH}=pico-sdk
    -ffile-prefix-map=${CMAKE_BINARY_DIR}=build)

if(PICO_SDK_VERSION_STRING VERSION_LESS "2.2.0")
    message(FATAL_ERROR "Pico SDK 2.2.0 or newer is required (found ${PICO_SDK_VERSION_STRING})")
endif()

pico_sdk_init()

# --- version -------------------------------------------------------------
# GP_VERSION can be given explicitly (CI passes the release tag); otherwise
# it is derived from git.
if(NOT GP_VERSION)
    execute_process(
        COMMAND git describe --tags --always --dirty
        WORKING_DIRECTORY ${CMAKE_CURRENT_LIST_DIR}
        OUTPUT_VARIABLE GP_VERSION OUTPUT_STRIP_TRAILING_WHITESPACE ERROR_QUIET)
    if(NOT GP_VERSION)
        set(GP_VERSION "dev")
    endif()
endif()
# USB bcdDevice = major.minor in BCD (0.0 for untagged builds)
set(GP_FW_BCD "0x0000")
if(GP_VERSION MATCHES "^v?([0-9]+)\\.([0-9]+)")
    math(EXPR _bcd "((${CMAKE_MATCH_1} / 10) << 12) | ((${CMAKE_MATCH_1} % 10) << 8) | ((${CMAKE_MATCH_2} / 10) << 4) | (${CMAKE_MATCH_2} % 10)" OUTPUT_FORMAT HEXADECIMAL)
    set(GP_FW_BCD ${_bcd})
endif()
message(STATUS "rp2350-gamepad firmware version: ${GP_VERSION} (bcdDevice ${GP_FW_BCD})")

# --- storage location ----------------------------------------------------
# Two 4 KiB sectors at the end of the first 2 MiB of flash. Must never
# overlap the firmware image (checked after linking).
set(GP_STORAGE_OFFSET "0x1FE000" CACHE STRING "Flash offset of the two settings sectors")

# --- firmware ------------------------------------------------------------
set(CORE ${CMAKE_CURRENT_LIST_DIR}/src/core)
set(PLAT ${CMAKE_CURRENT_LIST_DIR}/src/rp2350)

add_executable(rp2350-gamepad
    ${CORE}/app.c
    ${CORE}/axis.c
    ${CORE}/calib.c
    ${CORE}/config.c
    ${CORE}/crc32.c
    ${CORE}/frame.c
    ${CORE}/hid.c
    ${CORE}/input.c
    ${CORE}/leds.c
    ${CORE}/storage.c
    ${PLAT}/hal_rp2350.c
    ${PLAT}/main.c
    ${PLAT}/usb_descriptors.c
)

pico_generate_pio_header(rp2350-gamepad ${PLAT}/ws2812.pio)
target_include_directories(rp2350-gamepad PRIVATE ${CORE} ${PLAT})

target_compile_definitions(rp2350-gamepad PRIVATE
    GP_FW_VERSION="${GP_VERSION}"
    GP_FW_BCD=${GP_FW_BCD}
    GP_STORAGE_OFFSET=${GP_STORAGE_OFFSET}
    GP_BOARD_NAME="RP2350"
    PICO_STACK_SIZE=0x1000
    # No stdio at all: GPIO 0/1 stay free for buttons.
    PICO_STDIO_UART=0
)

# Our own sources build warning-free; keep it that way.
set_source_files_properties(
    ${CORE}/app.c ${CORE}/axis.c ${CORE}/calib.c ${CORE}/config.c ${CORE}/crc32.c ${CORE}/frame.c
    ${CORE}/hid.c ${CORE}/input.c ${CORE}/leds.c ${CORE}/storage.c
    ${PLAT}/hal_rp2350.c ${PLAT}/main.c ${PLAT}/usb_descriptors.c
    PROPERTIES COMPILE_OPTIONS "-Wall;-Wextra;-Wno-unused-parameter;-Werror")



target_link_libraries(rp2350-gamepad PRIVATE
    pico_stdlib
    pico_unique_id
    hardware_adc
    hardware_dma
    hardware_flash
    hardware_pio
    hardware_sync
    hardware_watchdog
    tinyusb_device
)

pico_enable_stdio_uart(rp2350-gamepad 0)
pico_enable_stdio_usb(rp2350-gamepad 0)

pico_set_program_name(rp2350-gamepad "rp2350-gamepad")
pico_set_program_version(rp2350-gamepad "${GP_VERSION}")
pico_set_program_url(rp2350-gamepad "https://github.com/gethiox/rp2350-gamepad")

pico_add_extra_outputs(rp2350-gamepad)

# Fail the build if the image would overlap the settings storage.
add_custom_command(TARGET rp2350-gamepad POST_BUILD
    COMMAND ${CMAKE_COMMAND} -DBIN=$<TARGET_FILE_DIR:rp2350-gamepad>/rp2350-gamepad.bin
            -DLIMIT=${GP_STORAGE_OFFSET} -P ${CMAKE_CURRENT_LIST_DIR}/cmake/check_size.cmake
    VERBATIM)

# --- recovery image ------------------------------------------------------
# reset-settings.uf2 only contains erased (0xFF) blocks for the settings
# sectors. Copying it to the BOOTSEL drive wipes stored configuration and
# calibration without touching the firmware.
find_package(Python3 REQUIRED COMPONENTS Interpreter)
add_custom_command(
    OUTPUT ${CMAKE_CURRENT_BINARY_DIR}/reset-settings.uf2
    COMMAND ${Python3_EXECUTABLE} ${CMAKE_CURRENT_LIST_DIR}/../scripts/make_reset_uf2.py
            --offset ${GP_STORAGE_OFFSET} --size 0x2000
            --out ${CMAKE_CURRENT_BINARY_DIR}/reset-settings.uf2
    DEPENDS ${CMAKE_CURRENT_LIST_DIR}/../scripts/make_reset_uf2.py
    VERBATIM)
add_custom_target(reset-settings ALL DEPENDS ${CMAKE_CURRENT_BINARY_DIR}/reset-settings.uf2)
