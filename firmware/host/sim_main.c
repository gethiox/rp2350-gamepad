// SPDX-License-Identifier: MIT
//
// sim_main.c - runs the firmware core on a PC.
//
// The simulator speaks the device protocol on stdin/stdout, so the
// configurator (and its integration tests) can talk to the real firmware
// logic without hardware. A reboot (after saving a configuration) exits with
// status 3; the caller restarts it, which mirrors a USB re-enumeration.
//
//   rp2350-gamepad-sim [--flash FILE] [--rp2350b] [--manual] [--orient N]
//
//   --flash FILE  persist the emulated flash (settings survive restarts)
//   --rp2350b     emulate the 48-pin RP2350B (8 ADC channels)
//   --manual      no generated motion; inputs only via GP_CMD_SIM_INPUT
//   --orient N    0..7: how stick 1 is wired (bit0 swap, bit1 flip a, bit2 flip b)

#define _POSIX_C_SOURCE 200809L
#include <errno.h>
#include <fcntl.h>
#include <math.h>
#include <poll.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

#include "app.h"
#include "hal_host.h"

#define PI 3.14159265358979323846

static const char* flash_path;
static bool		   auto_motion = true;
static int		   orient	   = 5; // swapped + mirrored by default: exercises calibration

static void save_flash(void) {
	if (!flash_path)
		return;
	char tmp[4096];
	snprintf(tmp, sizeof(tmp), "%s.tmp", flash_path);
	FILE* f = fopen(tmp, "wb");
	if (!f)
		return;
	fwrite(hal_host.flash, 1, sizeof(hal_host.flash), f);
	fclose(f);
	rename(tmp, flash_path);
}

static void load_flash(void) {
	hal_host_erase_flash();
	if (!flash_path)
		return;
	FILE* f = fopen(flash_path, "rb");
	if (!f)
		return;
	if (fread(hal_host.flash, 1, sizeof(hal_host.flash), f) != sizeof(hal_host.flash))
		hal_host_erase_flash();
	fclose(f);
}

static void flush_out(void) {
	size_t off = 0;
	while (off < hal_host.cdc_out_len) {
		ssize_t n = write(STDOUT_FILENO, hal_host.cdc_out + off, hal_host.cdc_out_len - off);
		if (n < 0) {
			if (errno == EINTR || errno == EAGAIN)
				continue;
			hal_host.cdc_out_len = 0; // reader gone: drop output, keep running
			return;
		}
		off += (size_t)n;
	}
	hal_host.cdc_out_len = 0;
}

static void on_reboot(void) {
	flush_out();
	save_flash();
	exit(3);
}

static void on_sim_input(const gp_sim_input_t* in) {
	auto_motion = in->auto_motion != 0;
	if (!auto_motion) {
		for (int i = 0; i < 8; i++)
			hal_host.adc[i] = in->raw[i];
		for (int p = 0; p < 64; p++)
			hal_host.pin_low[p] = (in->buttons[p / 32] >> (p % 32)) & 1u;
	}
	hal_host.mounted   = in->usb_state != 2;
	hal_host.suspended = in->usb_state == 1;
}

static uint16_t to_adc(double v) { // v in -1..1 -> ADC counts
	double r = 2048.0 + v * 1700.0;
	r += (double)(rand() % 7 - 3);
	if (r < 0)
		r = 0;
	if (r > 4095)
		r = 4095;
	return (uint16_t)r;
}

// Generated inputs. Normally a repeating 8 second script; while a
// calibration runs, a script that performs what a user would do: rest, push
// stick 1 up, rotate it clockwise 1.25 turns, let go, sweep the other axes.
static double cal_t0 = -1;

static void generate(double t) {
	if (gp_app_calibrating()) {
		if (cal_t0 < 0)
			cal_t0 = t;
		double ct = t - cal_t0;
		double ph = ct < 0.6 ? 0.5 : (ct < 5.6 ? ct - 0.1 : 0.5); // rest .. gesture .. rest
		t		  = ph;
	} else {
		cal_t0 = -1;
	}
	double ph = fmod(t, 8.0);
	double ux = 0, uy = 0;
	if (ph < 1.0) {
		// rest
	} else if (ph < 1.5) {
		uy = (ph - 1.0) / 0.5;
	} else if (ph < 5.0) {
		double a = PI / 2 - (ph - 1.5) / 3.5 * 2.5 * PI; // clockwise
		// slightly square gate: superellipse radius
		double c = cos(a), s = sin(a);
		double r = 1.0 / pow(pow(fabs(c), 3) + pow(fabs(s), 3), 1.0 / 3.0);
		r		 = r > 1.12 ? 1.12 : r;
		ux		 = c * r * 0.95;
		uy		 = s * r * 0.95;
	} else if (ph < 5.5) {
		double k = 1.0 - (ph - 5.0) / 0.5;
		double a = PI / 2 - 2.5 * PI;
		ux		 = cos(a) * k;
		uy		 = sin(a) * k;
	} else {
		double k = (ph - 5.5) / 2.5 * 2 * PI;
		ux		 = 0.45 * sin(k);
		uy		 = 0.3 * sin(2 * k);
	}
	// Wire stick 1 according to --orient.
	double a = ux, b = uy;
	if (orient & 1) {
		a = uy;
		b = ux;
	}
	if (orient & 2)
		a = -a;
	if (orient & 4)
		b = -b;
	hal_host.adc[0] = to_adc(a);
	hal_host.adc[1] = to_adc(b);
	// Axis 3: throttle sweeping its full range, axis 4: slow wobble.
	double tri		= fabs(fmod(t / 4.0, 2.0) - 1.0) * 2.0 - 1.0;
	hal_host.adc[2] = to_adc(tri * 1.15);
	hal_host.adc[3] = to_adc(0.6 * sin(t * 0.7));
	for (int ch = 4; ch < 8; ch++)
		hal_host.adc[ch] = to_adc(0.8 * sin(t * (0.3 + 0.1 * ch)));
	// Buttons: walk through GPIO 0..25, 300 ms each. The calibration
	// button (if one is set) is skipped: a short press there would finish
	// a calibration started from the configurator.
	memset(hal_host.pin_low, 0, sizeof(hal_host.pin_low));
	int step = (int)(t / 0.3);
	int pin	 = step % 30;
	if ((step / 30) % 2 == 0 && pin < 26 && pin != gp_app_config()->cal_pin)
		hal_host.pin_low[pin] = true;
}

static double now_s(void) {
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return (double)ts.tv_sec + (double)ts.tv_nsec * 1e-9;
}

int main(int argc, char** argv) {
	signal(SIGPIPE, SIG_IGN); // the configurator may close the pipe at any time
	hal_host.rp2350b = false;
	for (int i = 1; i < argc; i++) {
		if (!strcmp(argv[i], "--flash") && i + 1 < argc)
			flash_path = argv[++i];
		else if (!strcmp(argv[i], "--rp2350b"))
			hal_host.rp2350b = true;
		else if (!strcmp(argv[i], "--manual"))
			auto_motion = false;
		else if (!strcmp(argv[i], "--orient") && i + 1 < argc)
			orient = atoi(argv[++i]) & 7;
		else {
			fprintf(stderr, "usage: %s [--flash FILE] [--rp2350b] [--manual] [--orient N]\n", argv[0]);
			return 2;
		}
	}
	hal_host_reset();
	load_flash();
	hal_host.on_reboot		 = on_reboot;
	hal_host.on_flash_change = save_flash;
	hal_host.on_sim_input	 = on_sim_input;

	gp_boot_info_t boot = { .reset_reason = GP_RESET_POWER_ON, .simulator = true };
	gp_app_init(&boot);

	fcntl(STDIN_FILENO, F_SETFL, fcntl(STDIN_FILENO, F_GETFL) | O_NONBLOCK);
	double	t0		 = now_s();
	double	eof_at	 = 0; // like real hardware, keep running briefly after the host closes the port
	uint8_t buf[4096];
	for (;;) {
		if (eof_at == 0) {
			struct pollfd pfd = { .fd = STDIN_FILENO, .events = POLLIN };
			poll(&pfd, 1, 0);
			if (pfd.revents & (POLLIN | POLLHUP)) {
				ssize_t n = read(STDIN_FILENO, buf, sizeof(buf));
				if (n == 0)
					eof_at = now_s(); // port closed: pending actions (reset, reboot) still complete
				if (n > 0)
					gp_app_cdc_rx(buf, (size_t)n);
			}
		} else if (now_s() - eof_at > 1.0) {
			save_flash();
			return 0;
		}
		if (auto_motion)
			generate(now_s() - t0);
		gp_app_task();
		flush_out();
		struct timespec ts = { 0, 200000 }; // 0.2 ms
		nanosleep(&ts, NULL);
	}
}
