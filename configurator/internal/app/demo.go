package app

import (
	"fmt"
	"strings"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/device"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/logbuf"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/model"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
)

// SetDemo schedules developer actions (used to produce documentation
// screenshots against the simulator): comma separated list of
// nav:<section>, log, dialog:<settings|import|about|reset>, edit, sticks,
// calibrate, menu, color, pins.
func (a *App) SetDemo(spec string) {
	if spec != "" {
		a.demoSteps = strings.Split(spec, ",")
	}
}

func (a *App) runDemo() {
	if len(a.demoSteps) == 0 || !a.ready() || a.snap.TelemetryGen < 30 {
		return
	}
	if a.demoAt == 0 {
		a.demoAt = a.ui.Time
	}
	if a.ui.Time < a.demoWait || a.ui.Time-a.demoAt < 0.3 {
		return
	}
	a.demoAt = a.ui.Time
	step := a.demoSteps[0]
	a.demoSteps = a.demoSteps[1:]
	c := a.ui
	d := &a.ed.Draft
	switch {
	case strings.HasPrefix(step, "nav:"):
		a.navTarget = strings.TrimPrefix(step, "nav:")
	case step == "log":
		a.logOpen, a.logH = true, 230
	case step == "dialog:settings":
		a.dialog = dlgSettings
	case step == "dialog:import":
		a.dialog = dlgImport
	case step == "dialog:about":
		a.dialog = dlgAbout
	case step == "dialog:reset":
		a.ask("Factory reset?", "Erases the saved configuration AND calibration on the device and restarts it with defaults. "+
			"Unsaved changes in this window are discarded too. Consider exporting a backup first.", "Erase everything", true, func() {})
	case step == "sticks":
		d.Groups[0].Enabled = 1
	case step == "tweak":
		d.Brightness = 700
		d.Axes[2].Flags = proto.AxisLinear
		d.Axes[2].Color = proto.Color{R: 255, G: 150, B: 0}
	case step == "edit":
		d.Brightness = 800
		d.Axes[0].Color = proto.Color{R: 255, G: 120}
		d.Axes[2].Flags = proto.AxisLinear | proto.AxisCustomDz
		d.Axes[2].CenterDz = 30
		d.UsbProduct.SetString("my stick")
		d.Buttons[2].Pin = d.Buttons[0].Pin // conflict -> device reports an error
	case strings.HasPrefix(step, "wait:"):
		var sec float64
		fmt.Sscanf(strings.TrimPrefix(step, "wait:"), "%g", &sec)
		a.demoWait = a.ui.Time + sec
	case step == "save":
		a.save()
		a.demoWait = a.ui.Time + 2
	case step == "finish":
		a.mgr.Do(device.Op{Kind: device.OpCalFinish})
	case step == "big":
		// RP2350B layout: 8 axes on GPIO 40-47, 36 buttons, 4 LEDs, 2 sticks
		lim := &a.snap.Limits
		d.NumAxes = lim.PlatformAxes
		for i := 0; i < int(d.NumAxes); i++ {
			d.Axes[i].Pin = lim.AdcFirstPin + uint8(i)
		}
		d.NumLedPins = 4
		for i := 0; i < 4; i++ {
			d.LedPins[i] = uint8(20 + i)
			d.LedCounts[i] = 1
		}
		d.LedCounts[3] = 12 // a 12-LED strip on the 4th pin
		d.CalPin = 24
		d.NumButtons = 0
		for p := 0; p < int(lim.GpioCount) && d.NumButtons < 36; p++ {
			if p >= 20 && p <= 24 || lim.AdcPinOK(p) {
				continue
			}
			d.Buttons[d.NumButtons].Pin = uint8(p)
			d.Buttons[d.NumButtons].LedMask = proto.LedMaskOf(1 << uint(p%15))
			d.Buttons[d.NumButtons].Color = proto.Color{R: uint8(p * 37), G: uint8(255 - p*20), B: 200}
			d.NumButtons++
		}
		d.Groups[0].Enabled, d.Groups[1].Enabled = 1, 1
	case step == "calibrate":
		a.mgr.Do(device.Op{Kind: device.OpCalStart})
	case step == "menu":
		c.OpenPopup(c.ID("mainmenu"))
	case step == "color":
		c.OpenPopup(c.ID(c.ID("axis", 0), "col"))
	case step == "chain":
		// LED pin 2 becomes a chain of 4; sticks drive it as a dot / bar
		old := model.LayoutOf(d)
		d.LedCounts[1] = 4
		model.RemapLedMasks(d, old)
		d.Axes[0].LedEffect, d.Axes[0].LedMask = proto.LedFxDot, proto.LedMaskOf(0b11110)
		d.Axes[1].LedEffect, d.Axes[1].LedMask = proto.LedFxBar, proto.LedMaskOf(0b11110)
	case step == "ledpop":
		c.OpenPopup(c.ID(c.ID("btn", 0), "leds"))
	case step == "logsel":
		es, _ := logbuf.Default.Snapshot()
		if len(es) > 3 {
			a.logSelA, a.logSelB = logPos{es[1].ID, 14}, logPos{es[3].ID, 40}
			a.logFollow = true
		}
	case step == "pins":
		c.OpenPopup(c.ID(c.ID("btn", 1), "pin"))
	}
}
