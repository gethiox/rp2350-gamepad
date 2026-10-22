// SPDX-License-Identifier: MIT
//
// main.c - RP2350 entry point: watchdog / crash detection, USB, main loop.

#include "hardware/watchdog.h"
#include "pico/stdlib.h"
#include "tusb.h"

#include "app.h"
#include "usb_descriptors.h"

#define WATCHDOG_MS			  1000
#define CRASH_SCRATCH_MAGIC	  0x47504352u // marks scratch[1] as our crash counter
#define CRASH_COUNTER_CLEAR_MS 10000		// running this long = boot was healthy

// Watchdog scratch registers 0..3 survive a watchdog reset but are cleared on
// power-up; the SDK uses 4..7 for its own reboot bookkeeping.
static gp_boot_info_t detect_boot(void) {
	gp_boot_info_t b = { 0 };
	if (watchdog_enable_caused_reboot())
		b.reset_reason = GP_RESET_WATCHDOG; // the watchdog fired: hang or crash
	else if (watchdog_caused_reboot())
		b.reset_reason = GP_RESET_REQUESTED; // our own watchdog_reboot()
	else
		b.reset_reason = GP_RESET_POWER_ON;

	uint32_t count = (watchdog_hw->scratch[0] == CRASH_SCRATCH_MAGIC) ? watchdog_hw->scratch[1] : 0;
	count		   = (b.reset_reason == GP_RESET_WATCHDOG) ? count + 1 : 0;
	if (count > 255)
		count = 255;
	watchdog_hw->scratch[0] = CRASH_SCRATCH_MAGIC;
	watchdog_hw->scratch[1] = count;
	b.crash_count			= (uint8_t)count;
	b.safe_mode				= count >= GP_SAFE_MODE_CRASHES;
	return b;
}

int main(void) {
	gp_boot_info_t boot = detect_boot();
	watchdog_enable(WATCHDOG_MS, true); // paused while a debugger halts the core

	gp_app_init(&boot);
	usb_descriptors_init(gp_app_config());
	tusb_init();

	bool	 crash_cleared = boot.crash_count == 0;
	uint32_t start		   = time_us_32();
	uint8_t	 buf[256];

	while (1) {
		tud_task();
		uint32_t budget = 8; // bound the work per iteration
		while (tud_cdc_available() && budget--) {
			uint32_t n = tud_cdc_read(buf, sizeof(buf));
			if (!n)
				break;
			gp_app_cdc_rx(buf, n);
		}
		gp_app_task();
		watchdog_update();

		if (!crash_cleared && time_us_32() - start > CRASH_COUNTER_CLEAR_MS * 1000u) {
			watchdog_hw->scratch[1] = 0;
			crash_cleared			= true;
		}
		if (gp_app_sleeping())
			sleep_us(500); // nothing to do while the host sleeps
	}
}
