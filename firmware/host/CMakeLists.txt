# SPDX-License-Identifier: MIT
#
# Host build of the firmware core: unit tests, simulator and layout dump.
#
#   cmake -S firmware/host -B build-host -DCMAKE_BUILD_TYPE=Debug
#   cmake --build build-host
#   ctest --test-dir build-host --output-on-failure
#
# Needs only a C compiler (GCC or Clang) - no Pico SDK.

cmake_minimum_required(VERSION 3.13)
project(rp2350_gamepad_host C)
set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)

option(GP_SANITIZE "Build with AddressSanitizer + UndefinedBehaviorSanitizer" ON)

set(CORE ${CMAKE_CURRENT_LIST_DIR}/../src/core)

add_library(gp_core STATIC
    ${CORE}/app.c ${CORE}/axis.c ${CORE}/calib.c ${CORE}/config.c ${CORE}/crc32.c
    ${CORE}/frame.c ${CORE}/hid.c ${CORE}/input.c ${CORE}/leds.c ${CORE}/storage.c
    ${CMAKE_CURRENT_LIST_DIR}/hal_host.c)
target_include_directories(gp_core PUBLIC ${CORE} ${CMAKE_CURRENT_LIST_DIR})
target_compile_definitions(gp_core PUBLIC GP_HOST_BUILD=1 GP_FW_VERSION="sim")
target_compile_options(gp_core PRIVATE -Wall -Wextra -Wno-unused-parameter -Werror)
target_link_libraries(gp_core PUBLIC m)

if(GP_SANITIZE AND NOT WIN32)
    target_compile_options(gp_core PUBLIC -fsanitize=address,undefined -fno-sanitize-recover=all -fno-omit-frame-pointer)
    target_link_options(gp_core PUBLIC -fsanitize=address,undefined)
endif()

add_executable(gp_tests tests/test_main.c)
target_link_libraries(gp_tests PRIVATE gp_core)

add_executable(rp2350-gamepad-sim sim_main.c)
target_link_libraries(rp2350-gamepad-sim PRIVATE gp_core)

add_executable(layout_dump layout_dump.c)
target_include_directories(layout_dump PRIVATE ${CORE})

enable_testing()
add_test(NAME firmware_core COMMAND gp_tests)
add_test(NAME layout_up_to_date
    COMMAND ${CMAKE_COMMAND} -DDUMP=$<TARGET_FILE:layout_dump>
            -DEXPECTED=${CMAKE_CURRENT_LIST_DIR}/../../protocol/layout.json
            -P ${CMAKE_CURRENT_LIST_DIR}/compare_layout.cmake)
