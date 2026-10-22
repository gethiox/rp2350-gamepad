// SPDX-License-Identifier: MIT
//
// hal_rp2350.c - hardware implementation of hal.h for the RP2350.

#include <string.h>

#include "hardware/adc.h"
#include "hardware/dma.h"
#include "hardware/flash.h"
#include "hardware/gpio.h"
#include "hardware/pio.h"
#include "hardware/sync.h"
#include "hardware/watchdog.h"
#include "pico/stdlib.h"
#include "pico/unique_id.h"
#include "tusb.h"

#include "hal.h"
#include "ws2812.pio.h"

#ifndef GP_STORAGE_OFFSET
#error "GP_STORAGE_OFFSET must be defined by the build"
#endif
#ifndef GP_BOARD_NAME
#define GP_BOARD_NAME "RP2350"
#endif
// GPIOs a board wires to on-board parts and that must not be offered.
#ifndef GP_BOARD_RESERVED_PINS
#define GP_BOARD_RESERVED_PINS 0ull
#endif

// GPIO of the board's own WS2812 (a single LED). GP_PIN_NONE if there is none.
#ifndef GP_BOARD_BUILTIN_LED_PIN
#define GP_BOARD_BUILTIN_LED_PIN GP_PIN_NONE
#endif

#define LEDS_PER_PIO 4

// ---------------------------------------------------------------------------
// Platform description
// ---------------------------------------------------------------------------

static void set_mask(uint32_t m[2], uint64_t v) {
	m[0] = (uint32_t)v;
	m[1] = (uint32_t)(v >> 32);
}

void hal_describe_platform(gp_limits_t* l) {
	l->gpio_count	 = NUM_BANK0_GPIOS;
	l->adc_first_pin = ADC_BASE_PIN;
	l->platform_axes = NUM_ADC_CHANNELS - 1; // last channel is the temperature sensor
	l->platform_led_pins = 2 * LEDS_PER_PIO;
	l->builtin_led_pin	 = GP_BOARD_BUILTIN_LED_PIN;

	uint64_t all = (NUM_BANK0_GPIOS >= 64) ? ~0ull : ((1ull << NUM_BANK0_GPIOS) - 1ull);
	all &= ~(uint64_t)GP_BOARD_RESERVED_PINS;
	uint64_t adc = ((1ull << (NUM_ADC_CHANNELS - 1)) - 1ull) << ADC_BASE_PIN;
	set_mask(l->button_pins, all);
	set_mask(l->led_pins, all);
	set_mask(l->cal_pins, all);
	set_mask(l->adc_pins, adc & all);
}

const char* hal_board_name(void) { return GP_BOARD_NAME; }

void hal_chip_id(uint8_t out[8]) {
	pico_unique_board_id_t id;
	pico_get_unique_board_id(&id);
	memcpy(out, id.id, 8);
}

uint32_t hal_time_us(void) { return time_us_32(); }

// ---------------------------------------------------------------------------
// GPIO / ADC
// ---------------------------------------------------------------------------

static bool adc_ready;

void hal_button_init(uint8_t pin) {
	gpio_init(pin);
	gpio_set_dir(pin, GPIO_IN);
	gpio_pull_up(pin);
}

bool hal_button_pressed(uint8_t pin) { return !gpio_get(pin); }

int hal_adc_channel(uint8_t pin) {
	int ch = (int)pin - ADC_BASE_PIN;
	return (ch >= 0 && ch < (int)NUM_ADC_CHANNELS - 1) ? ch : -1;
}

void hal_adc_init(uint8_t pin) {
	if (!adc_ready) {
		adc_init();
		adc_ready = true;
	}
	if (hal_adc_channel(pin) >= 0)
		adc_gpio_init(pin);
}

uint16_t hal_adc_read(uint8_t pin) {
	int ch = hal_adc_channel(pin);
	if (ch < 0)
		return 2048;
	adc_select_input((uint)ch);
	return adc_read();
}

// ---------------------------------------------------------------------------
// LEDs: a WS2812 chain per pin, one PIO state machine and one DMA channel
// per pin. A frame is copied into a per-pin buffer and streamed by DMA, so
// long chains never block the main loop.
// ---------------------------------------------------------------------------

typedef struct {
	PIO		 pio;
	uint	 sm;
	int		 dma;
	uint8_t	 count;
	bool	 ok;
	uint32_t buf[GP_MAX_LEDS];
} led_out_t;

static led_out_t led_out[GP_MAX_LED_PINS];
static int		 prog_offset[2] = { -1, -1 };

void hal_led_init(uint8_t index, uint8_t pin, uint8_t count) {
	if (index >= GP_MAX_LED_PINS || count == 0 || count > GP_MAX_LEDS)
		return;
	led_out_t* o	   = &led_out[index];
	int		   pio_idx = index / LEDS_PER_PIO;
	PIO		   pio	   = pio_idx == 0 ? pio0 : pio1;
	if (prog_offset[pio_idx] < 0) {
		if (!pio_can_add_program(pio, &ws2812_program))
			return;
		prog_offset[pio_idx] = (int)pio_add_program(pio, &ws2812_program);
	}
	uint sm = index % LEDS_PER_PIO;
	pio_sm_claim(pio, sm);
	ws2812_program_init(pio, sm, (uint)prog_offset[pio_idx], pin, 800000.0f);

	int dma = dma_claim_unused_channel(false);
	if (dma < 0)
		return;
	dma_channel_config dc = dma_channel_get_default_config((uint)dma);
	channel_config_set_transfer_data_size(&dc, DMA_SIZE_32);
	channel_config_set_read_increment(&dc, true);
	channel_config_set_write_increment(&dc, false);
	channel_config_set_dreq(&dc, pio_get_dreq(pio, sm, true));
	dma_channel_configure((uint)dma, &dc, &pio->txf[sm], o->buf, 0, false);

	o->pio	 = pio;
	o->sm	 = sm;
	o->dma	 = dma;
	o->count = count;
	o->ok	 = true;
}

void hal_led_write(uint8_t index, const uint8_t (*rgb)[3], uint8_t count) {
	if (index >= GP_MAX_LED_PINS)
		return;
	led_out_t* o = &led_out[index];
	if (!o->ok || count != o->count || dma_channel_is_busy((uint)o->dma))
		return; // previous frame still streaming: skip this one
	for (unsigned i = 0; i < count; i++) {
		uint32_t grb = ((uint32_t)rgb[i][1] << 16) | ((uint32_t)rgb[i][0] << 8) | rgb[i][2];
		o->buf[i]	 = grb << 8u;
	}
	dma_channel_transfer_from_buffer_now((uint)o->dma, o->buf, count);
}

// ---------------------------------------------------------------------------
// Storage
// ---------------------------------------------------------------------------

static uint32_t slot_offset(uint8_t slot) { return GP_STORAGE_OFFSET + (uint32_t)slot * FLASH_SECTOR_SIZE; }

const uint8_t* hal_storage_read(uint8_t slot) { return (const uint8_t*)(XIP_BASE + slot_offset(slot)); }

bool hal_storage_write(uint8_t slot, const uint8_t* data, size_t len) {
	if (slot > 1 || len > FLASH_SECTOR_SIZE || (len % FLASH_PAGE_SIZE) != 0)
		return false;
	watchdog_update(); // a sector erase can take a few hundred ms
	uint32_t ints = save_and_disable_interrupts();
	flash_range_erase(slot_offset(slot), FLASH_SECTOR_SIZE);
	flash_range_program(slot_offset(slot), data, len);
	restore_interrupts(ints);
	watchdog_update();
	return true; // the caller verifies the contents
}

bool hal_storage_erase(uint8_t slot) {
	if (slot > 1)
		return false;
	watchdog_update();
	uint32_t ints = save_and_disable_interrupts();
	flash_range_erase(slot_offset(slot), FLASH_SECTOR_SIZE);
	restore_interrupts(ints);
	watchdog_update();
	const uint8_t* p = hal_storage_read(slot);
	for (int i = 0; i < 32; i++)
		if (p[i] != 0xFF)
			return false;
	return true;
}

// ---------------------------------------------------------------------------
// USB
// ---------------------------------------------------------------------------

bool hal_usb_mounted(void) { return tud_mounted(); }
bool hal_usb_suspended(void) { return tud_suspended(); }
bool hal_hid_ready(void) { return tud_hid_ready(); }
bool hal_hid_send(uint8_t id, const uint8_t* data, uint16_t len) { return tud_hid_report(id, data, len); }

size_t hal_cdc_write_available(void) { return tud_cdc_write_available(); }
size_t hal_cdc_write(const uint8_t* data, size_t len) { return tud_cdc_write(data, (uint32_t)len); }
void   hal_cdc_flush(void) { tud_cdc_write_flush(); }

// ---------------------------------------------------------------------------
// System
// ---------------------------------------------------------------------------

void hal_watchdog_feed(void) { watchdog_update(); }

void hal_reboot(void) {
	watchdog_reboot(0, 0, 0);
	while (1)
		tight_loop_contents();
}
