// SPDX-License-Identifier: MIT
//
// hal_host.c - HAL implementation for unit tests and the simulator.

#define _POSIX_C_SOURCE 200809L
#include "hal_host.h"

#include <string.h>
#include <time.h>

#include "hal.h"

hal_host_t hal_host;

void hal_host_reset(void) {
	uint8_t flash[2][4096];
	memcpy(flash, hal_host.flash, sizeof(flash));
	bool b = hal_host.rp2350b;
	void (*rb)(void)				  = hal_host.on_reboot;
	void (*fc)(void)				  = hal_host.on_flash_change;
	void (*si)(const gp_sim_input_t*) = hal_host.on_sim_input;
	memset(&hal_host, 0, sizeof(hal_host));
	memcpy(hal_host.flash, flash, sizeof(flash));
	hal_host.rp2350b		  = b;
	hal_host.on_reboot		  = rb;
	hal_host.on_flash_change  = fc;
	hal_host.on_sim_input	  = si;
	hal_host.mounted		  = true;
	hal_host.hid_ready		  = true;
	hal_host.cdc_space		  = 2048;
	hal_host.fail_write_after = -1;
	for (int i = 0; i < 8; i++)
		hal_host.adc[i] = 2048;
}

void hal_host_erase_flash(void) { memset(hal_host.flash, 0xFF, sizeof(hal_host.flash)); }

// --- platform ---

void hal_describe_platform(gp_limits_t* l) {
	int		 gpios = hal_host.rp2350b ? 48 : 30;
	int		 first = hal_host.rp2350b ? 40 : 26;
	int		 nadc  = hal_host.rp2350b ? 8 : 4;
	uint64_t all   = (1ull << gpios) - 1ull;
	uint64_t adc   = ((1ull << nadc) - 1ull) << first;
	l->gpio_count	 = (uint8_t)gpios;
	l->adc_first_pin = (uint8_t)first;
	l->platform_axes = (uint8_t)nadc;
	l->platform_led_pins = GP_MAX_LED_PINS;
	l->builtin_led_pin	 = 22;
	l->button_pins[0] = (uint32_t)all;
	l->button_pins[1] = (uint32_t)(all >> 32);
	memcpy(l->led_pins, l->button_pins, sizeof(l->led_pins));
	memcpy(l->cal_pins, l->button_pins, sizeof(l->cal_pins));
	l->adc_pins[0] = (uint32_t)adc;
	l->adc_pins[1] = (uint32_t)(adc >> 32);
}

const char* hal_board_name(void) { return hal_host.rp2350b ? "Simulator (RP2350B)" : "Simulator (RP2350A)"; }

void hal_chip_id(uint8_t out[8]) {
	static const uint8_t id[8] = { 0xE6, 0x61, 0x38, 0x50, 0x83, 0x2B, 0x47, 0x2F };
	memcpy(out, id, 8);
}

uint32_t hal_time_us(void) {
	if (hal_host.manual_time)
		return hal_host.time_us;
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return (uint32_t)((uint64_t)ts.tv_sec * 1000000u + (uint64_t)ts.tv_nsec / 1000u);
}

// --- GPIO / ADC ---

void hal_button_init(uint8_t pin) { (void)pin; }
bool hal_button_pressed(uint8_t pin) { return pin < 64 && hal_host.pin_low[pin]; }

int hal_adc_channel(uint8_t pin) {
	int first = hal_host.rp2350b ? 40 : 26;
	int n	  = hal_host.rp2350b ? 8 : 4;
	int ch	  = (int)pin - first;
	return (ch >= 0 && ch < n) ? ch : -1;
}

void	 hal_adc_init(uint8_t pin) { (void)pin; }
uint16_t hal_adc_read(uint8_t pin) {
	int ch = hal_adc_channel(pin);
	return ch < 0 ? 2048 : hal_host.adc[ch];
}

// --- LEDs ---

void hal_led_init(uint8_t index, uint8_t pin, uint8_t count) {
	if (index >= GP_MAX_LED_PINS)
		return;
	hal_host.led_pin[index]	  = pin;
	hal_host.led_count[index] = count;
	hal_host.led_first[index] = index == 0 ? 0 : (uint8_t)(hal_host.led_first[index - 1] + hal_host.led_count[index - 1]);
}

void hal_led_write(uint8_t index, const uint8_t (*rgb)[3], uint8_t count) {
	if (index >= GP_MAX_LED_PINS || count != hal_host.led_count[index])
		return;
	unsigned first = hal_host.led_first[index];
	for (unsigned i = 0; i < count && first + i < GP_MAX_LEDS; i++)
		memcpy(hal_host.led_rgb[first + i], rgb[i], 3);
	hal_host.led_frames++;
}

// --- storage ---

const uint8_t* hal_storage_read(uint8_t slot) { return hal_host.flash[slot & 1]; }

bool hal_storage_write(uint8_t slot, const uint8_t* data, size_t len) {
	if (slot > 1 || len > HAL_STORAGE_SECTOR_SIZE)
		return false;
	hal_host.writes++;
	memset(hal_host.flash[slot], 0xFF, HAL_STORAGE_SECTOR_SIZE);
	size_t n = len;
	if (hal_host.fail_write_after >= 0) {
		if ((size_t)hal_host.fail_write_after < n)
			n = (size_t)hal_host.fail_write_after;
		hal_host.fail_write_after = -1;
	}
	memcpy(hal_host.flash[slot], data, n);
	if (hal_host.on_flash_change)
		hal_host.on_flash_change();
	return true;
}

bool hal_storage_erase(uint8_t slot) {
	if (slot > 1 || hal_host.erase_fails)
		return false;
	memset(hal_host.flash[slot], 0xFF, HAL_STORAGE_SECTOR_SIZE);
	if (hal_host.on_flash_change)
		hal_host.on_flash_change();
	return true;
}

// --- USB ---

bool hal_usb_mounted(void) { return hal_host.mounted; }
bool hal_usb_suspended(void) { return hal_host.suspended; }
bool hal_hid_ready(void) { return hal_host.hid_ready; }

bool hal_hid_send(uint8_t id, const uint8_t* data, uint16_t len) {
	if (len > sizeof(hal_host.last_report))
		return false;
	memcpy(hal_host.last_report, data, len);
	hal_host.last_report_len = len;
	hal_host.last_report_id	 = id;
	hal_host.reports_sent++;
	return true;
}

size_t hal_cdc_write_available(void) {
	size_t room = sizeof(hal_host.cdc_out) - hal_host.cdc_out_len;
	return hal_host.cdc_space < room ? hal_host.cdc_space : room;
}

size_t hal_cdc_write(const uint8_t* data, size_t len) {
	size_t n = hal_cdc_write_available();
	if (len < n)
		n = len;
	memcpy(hal_host.cdc_out + hal_host.cdc_out_len, data, n);
	hal_host.cdc_out_len += n;
	return n;
}

void hal_cdc_flush(void) {}

// --- system ---

void hal_watchdog_feed(void) {}

void hal_reboot(void) {
	hal_host.reboot_requested = true;
	if (hal_host.on_reboot)
		hal_host.on_reboot();
}

void hal_sim_input(const gp_sim_input_t* in) {
	if (hal_host.on_sim_input)
		hal_host.on_sim_input(in);
}
