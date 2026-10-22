// Command rp2350-gamepad-configurator is the desktop configuration tool for
// the rp2350-gamepad firmware.
package main

import (
	"flag"
	"fmt"
	"runtime"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/gethiox/rp2350-gamepad/configurator/assets"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/app"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/device"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/logbuf"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/settings"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/ui"
)

func init() {
	// raylib must run on the main OS thread.
	runtime.LockOSThread()
}

func main() {
	sim := flag.String("simulator", "", "developer option: path to rp2350-gamepad-sim instead of real hardware")
	simArgs := flag.String("simulator-args", "", "extra arguments for the simulator")
	capture := flag.String("screenshot", "", "developer option: save a screenshot to this file and exit")
	captureAfter := flag.Duration("screenshot-after", 3*time.Second, "delay before -screenshot")
	size := flag.String("size", "", "developer option: window size WxH")
	version := flag.Bool("version", false, "print the version and exit")
	demo := flag.String("demo", "", "developer option: scripted UI actions for screenshots")
	flag.Parse()
	if *version {
		fmt.Println("rp2350-gamepad-configurator", app.Version)
		return
	}

	logbuf.Default.OpenFile()
	defer logbuf.Default.Close()
	logbuf.Infof("rp2350-gamepad configurator %s (%s/%s)", app.Version, runtime.GOOS, runtime.GOARCH)

	set, err := settings.Load()
	if err != nil {
		logbuf.Warnf("%v", err)
	}

	var finder device.Finder = device.SerialFinder{}
	if *sim != "" {
		args := strings.Fields(*simArgs)
		finder = &device.SimFinder{Binary: *sim, Args: args}
		logbuf.Infof("Using the firmware simulator %s %s", *sim, *simArgs)
	}
	mgr := device.NewManager(finder, set.DeviceIDs)
	mgr.Start()
	defer mgr.Stop()

	w, h := 1280, 720
	if set.WindowW >= 800 && set.WindowH >= 500 {
		w, h = set.WindowW, set.WindowH
	}
	if *size != "" {
		fmt.Sscanf(*size, "%dx%d", &w, &h)
	}

	flags := uint32(rl.FlagWindowResizable | rl.FlagMsaa4xHint | rl.FlagVsyncHint)
	if runtime.GOOS == "darwin" {
		flags |= rl.FlagWindowHighdpi
	}
	rl.SetConfigFlags(flags)
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.InitWindow(int32(w), int32(h), "rp2350-gamepad configurator")
	defer rl.CloseWindow()
	rl.SetExitKey(0) // Escape closes dialogs, not the app
	rl.SetWindowIcon(*appIcon())
	rl.SetTargetFPS(60)

	fonts := ui.NewFonts(assets.DosisSemiBold, assets.DosisBold, assets.SpaceMonoBold)
	ctx := ui.NewCtx(fonts)
	a := app.New(mgr, set, ctx)
	a.SetDemo(*demo)

	start := time.Now()
	for a.Frame() {
		if *capture != "" && time.Since(start) > *captureAfter {
			rl.TakeScreenshot(*capture)
			break
		}
		if rl.IsWindowFocused() {
			rl.SetTargetFPS(int32(targetFPS(set.MaxFPS)))
		} else {
			rl.SetTargetFPS(20)
		}
	}
	if *size == "" && !rl.IsWindowMaximized() && !rl.IsWindowFullscreen() {
		set.WindowW, set.WindowH = rl.GetScreenWidth(), rl.GetScreenHeight()
		if err := set.Save(); err != nil {
			logbuf.Warnf("Could not save settings: %v", err)
		}
	}
}

// targetFPS resolves the "max frame rate" setting (0 = display refresh rate).
func targetFPS(setting int) int {
	if setting > 0 {
		return setting
	}
	if hz := rl.GetMonitorRefreshRate(rl.GetCurrentMonitor()); hz >= 30 {
		return hz
	}
	return 60
}

// appIcon draws the window icon (a small joystick) procedurally.
func appIcon() *rl.Image {
	const n = 64
	img := rl.GenImageColor(n, n, rl.Color{})
	bg := rl.Color{R: 0x16, G: 0x1A, B: 0x20, A: 255}
	acc := rl.Color{R: 0x5B, G: 0x9C, B: 0xFF, A: 255}
	rl.ImageDrawCircle(img, n/2, n/2, n/2-1, bg)
	rl.ImageDrawRectangle(img, 14, 42, 36, 9, acc)
	for t := 0; t <= 20; t++ { // shaft
		x := 32 + t*7/20
		y := 42 - t
		rl.ImageDrawRectangle(img, int32(x-2), int32(y), 5, 1, acc)
	}
	rl.ImageDrawCircle(img, 40, 20, 8, acc)
	return img
}
