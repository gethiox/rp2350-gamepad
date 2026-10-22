package app

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/model"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/ui"
)

// grid computes columns for cards with a minimum width.
func (a *App) grid(w, minW float32) (cols int, cardW float32) {
	gap := a.S(12)
	cols = int((w + gap) / (minW + gap))
	if cols < 1 {
		cols = 1
	}
	return cols, (w - gap*float32(cols-1)) / float32(cols)
}

func (a *App) emptyNote(x, y, w float32, text string) float32 {
	c := a.ui
	r := ui.R(x, y, w, a.S(56))
	c.RoundRect(r, a.S(ui.RadiusBox), ui.ColPanel)
	c.TextIn(ui.TBody, text, r, 0.5, ui.ColTextFaint)
	return r.Height
}

// groupOf returns the enabled stick group (draft) containing axis i, or -1.
func groupOf(d *proto.Config, i int) (int, bool) {
	for g := 0; g < proto.MaxGroups; g++ {
		gr := &d.Groups[g]
		if gr.Enabled == 1 && (int(gr.AxisH) == i || int(gr.AxisV) == i) {
			return g, int(gr.AxisH) == i
		}
	}
	return -1, false
}

// ===================================================================== axes

func (a *App) sectionAxes(x, y, w float32) float32 {
	c := a.ui
	d, b := &a.ed.Draft, a.baseView()
	y0 := y
	a.countControl(c.ID("numaxes"), x+w, y, "Active axes", &d.NumAxes, b.NumAxes, int(a.snap.Limits.PlatformAxes), proto.FNumAxes)
	y += a.sectionHeader(x, y, w, "Axes", fmt.Sprintf("Analog inputs on ADC pins (GPIO %d-%d). Axes that form a stick are combined in Sticks below.",
		a.snap.Limits.AdcFirstPin, int(a.snap.Limits.AdcFirstPin)+int(a.snap.Limits.PlatformAxes)-1))
	n := int(d.NumAxes)
	if n == 0 {
		return y - y0 + a.emptyNote(x, y, w, "No axes enabled.")
	}
	cols, cw := a.grid(w, a.S(300))
	ch := a.axisCardH()
	gap := a.S(12)
	for i := 0; i < n && i < proto.MaxAxes; i++ {
		a.axisCard(i, x+float32(i%cols)*(cw+gap), y+float32(i/cols)*(ch+gap), cw, ch)
	}
	rows := (n + cols - 1) / cols
	y += float32(rows)*(ch+gap) - gap
	return y - y0
}

// axisCardH is the height of an axis card.
func (a *App) axisCardH() float32 {
	return a.S(12+24+8+40) + 4*a.S(ui.RowH) + 3*a.S(8) + a.S(30)
}

func (a *App) axisCard(i int, x, y, w, h float32) {
	c := a.ui
	d, b := &a.ed.Draft, a.baseView()
	ax, bx := &d.Axes[i], &b.Axes[i]
	lim := &a.snap.Limits
	id := c.ID("axis", i)
	r := ui.R(x, y, w, h)
	c.Panel(r)
	p := a.S(16)
	ix, iw := x+p, w-2*p
	yy := y + a.S(12)

	// header
	c.TextIn(ui.TBodyB, fmt.Sprintf("Axis %d", i+1), ui.R(ix, yy, iw, a.S(24)), 0, ui.ColText)
	hx := ix + a.S(56)
	hx += c.Badge(hx, yy+a.S(2), usageName(ax.HidUsage), ui.ColAccent) + a.S(6)
	g, horiz := groupOf(d, i)
	if g >= 0 {
		dir := "V"
		if horiz {
			dir = "H"
		}
		c.Badge(hx, yy+a.S(2), fmt.Sprintf("Stick %d · %s", g+1, dir), ui.ColTextDim)
	}
	a.axisCalChip(i, ix+iw, yy+a.S(2))
	yy += a.S(24 + 8)

	// live bars (device state: only for axes the device already has)
	liveOK := i < int(b.NumAxes) && a.snap.TelemetryGen > 0
	a.axisBars(i, ix, yy, iw, liveOK)
	yy += a.S(40)

	rowH := a.S(ui.RowH)
	gap := a.S(8)
	half := (iw - gap) / 2
	modDot := func(key string, ly float32, mod bool, saved string, revert func()) {
		a.mark(c.ID(id, key, "m"), x+a.S(8), ly+rowH/2, mod, saved, revert)
	}

	// pin + usage
	a.pinPicker(c.ID(id, "pin"), ui.R(ix, yy, half, rowH), "Pin", roleAxis, fmt.Sprintf("Axis %d", i+1), &ax.Pin, bx.Pin,
		a.errFor(proto.FAxisPin, i))
	usages := lim.Usages()
	opts := make([]ui.Option, len(usages))
	sel := -1
	for k, u := range usages {
		opts[k] = ui.Option{Label: usageName(u)}
		if u == ax.HidUsage {
			sel = k
		}
	}
	c.DropdownP(c.ID(id, "usage"), ui.R(ix+half+gap, yy, half, rowH), "Usage", opts, sel,
		st(ax.HidUsage != bx.HidUsage, a.errFor(proto.FAxisUsage, i)), func(k int) { ax.HidUsage = usages[k] })
	modDot("pinusage", yy, ax.Pin != bx.Pin || ax.HidUsage != bx.HidUsage,
		fmt.Sprintf("%s, %s", pinName(bx.Pin), usageName(bx.HidUsage)), func() { ax.Pin, ax.HidUsage = bx.Pin, bx.HidUsage })
	yy += rowH + gap

	if g >= 0 {
		note := fmt.Sprintf("Part of Stick %d - direction, output shape and deadzone are set on the stick.", g+1)
		c.Paragraph(ui.TSmall, note, ix, yy+a.S(4), iw, ui.ColTextFaint)
		yy += 2 * (rowH + gap)
	} else {
		// mode + invert
		mode := 0
		if ax.Flags&proto.AxisLinear != 0 {
			mode = 1
		}
		segW := iw - a.S(104)
		modeMod := ax.Flags&proto.AxisLinear != bx.Flags&proto.AxisLinear
		if c.Segmented(c.ID(id, "mode"), ui.R(ix, yy, segW, rowH), []string{"Centred", "Linear"}, &mode, st(modeMod, "")) {
			ax.Flags = ax.Flags&^proto.AxisLinear | uint8(mode)*proto.AxisLinear
		}
		if c.MouseIn(ui.R(ix, yy, segW, rowH)) {
			c.Tip(c.ID(id, "modetip"), "Centred: a stick or pedal that rests in the middle.\nLinear: a throttle or slider that rests at one end.")
		}
		inv := ax.Flags&proto.AxisInvert != 0
		invMod := ax.Flags&proto.AxisInvert != bx.Flags&proto.AxisInvert
		c.TextIn(ui.TSmall, "Invert", ui.R(ix+segW+a.S(14), yy, a.S(50), rowH), 0, ui.ColTextDim)
		if c.Switch(c.ID(id, "inv"), ui.R(ix+iw-a.S(38), yy, a.S(38), rowH), &inv, st(invMod, "")) {
			ax.Flags ^= proto.AxisInvert
		}
		modDot("modeinv", yy, modeMod || invMod, "", func() {
			m := uint8(proto.AxisLinear | proto.AxisInvert)
			ax.Flags = ax.Flags&^m | bx.Flags&m
		})
		yy += rowH + gap

		// deadzone
		custom := 0
		if ax.Flags&proto.AxisCustomDz != 0 {
			custom = 1
		}
		dzMod := ax.Flags&proto.AxisCustomDz != bx.Flags&proto.AxisCustomDz
		segW = a.S(132)
		if c.Segmented(c.ID(id, "dzmode"), ui.R(ix, yy, segW, rowH), []string{"Global dz", "Custom"}, &custom, st(dzMod, "")) {
			ax.Flags = ax.Flags&^proto.AxisCustomDz | uint8(custom)*proto.AxisCustomDz
		}
		modDot("dz", yy, dzMod || custom == 1 && (ax.CenterDz != bx.CenterDz || ax.OuterDz != bx.OuterDz), "", func() {
			ax.Flags = ax.Flags&^proto.AxisCustomDz | bx.Flags&proto.AxisCustomDz
			ax.CenterDz, ax.OuterDz = bx.CenterDz, bx.OuterDz
		})
		fx := ix + segW + gap
		fw := (ix + iw - fx - gap) / 2
		lo, hi := "centre", "edge"
		if ax.Flags&proto.AxisLinear != 0 {
			lo, hi = "low end", "high end"
		}
		if custom == 1 {
			mn, mx := rangeOf(lim, proto.FAxisCenterDz)
			numU16(c, c.ID(id, "cdz"), ui.R(fx, yy, fw, rowH), &ax.CenterDz, bx.CenterDz, mn, mx, 5, 1, "%", a.errFor(proto.FAxisCenterDz, i))
			mn, mx = rangeOf(lim, proto.FAxisOuterDz)
			numU16(c, c.ID(id, "odz"), ui.R(fx+fw+gap, yy, fw, rowH), &ax.OuterDz, bx.OuterDz, mn, mx, 5, 1, "%", a.errFor(proto.FAxisOuterDz, i))
			if c.MouseIn(ui.R(fx, yy, fw*2+gap, rowH)) {
				c.Tip(c.ID(id, "dztip"), fmt.Sprintf("Left: %s deadzone, right: %s deadzone, in percent of travel.", lo, hi))
			}
		} else {
			c.TextIn(ui.TSmall, c.Ellipsize(ui.TSmall, fmt.Sprintf("%s %s · %s %s", lo, pct(int(d.CenterDz)), hi, pct(int(d.OuterDz))), iw-segW-gap),
				ui.R(fx, yy, iw-segW-gap, rowH), 0, ui.ColTextFaint)
		}
		yy += rowH + gap
	}

	// LEDs, effect + colour
	colW, fxW := a.S(56), a.S(84)
	chipsW := iw - a.S(40) - fxW - colW - 2*gap
	c.TextIn(ui.TSmall, "LEDs", ui.R(ix, yy, a.S(40), rowH), 0, ui.ColTextDim)
	a.ledChips(c.ID(id, "leds"), ix+a.S(40), yy+rowH/2, chipsW, &ax.LedMask, bx.LedMask, d, a.errFor(proto.FAxisLedMask, i))
	fxOpts := []ui.Option{{Label: "Glow"}, {Label: "Bar"}, {Label: "Dot"}}
	fxR := ui.R(ix+iw-colW-gap-fxW, yy, fxW, rowH)
	c.Dropdown(c.ID(id, "fx"), fxR, fxOpts, int(ax.LedEffect), st(ax.LedEffect != bx.LedEffect, a.errFor(proto.FAxisLedEffect, i)),
		func(k int) { ax.LedEffect = uint8(k) })
	if c.MouseIn(fxR) {
		c.Tip(c.ID(id, "fxtip"), "How this axis lights its LEDs:\n"+
			"Glow: all of them brighten as the axis moves.\n"+
			"Bar: a level meter; a centred axis fills outward from the middle.\n"+
			"Dot: a point that slides along them.\n"+
			"Bar and Dot run along each LED pin's chain separately; a single\n"+
			"selected LED on a pin glows as a classic indicator.")
	}
	modDot("leds", yy, ax.LedMask != bx.LedMask || ax.Color != bx.Color || ax.LedEffect != bx.LedEffect, "", func() {
		ax.LedMask, ax.Color, ax.LedEffect = bx.LedMask, bx.Color, bx.LedEffect
	})
	a.colorPick(c.ID(id, "col"), ui.R(ix+iw-colW, yy, colW, rowH), &ax.Color, bx.Color)

	// first error of this axis
	if e := a.firstErr(i, proto.FAxisPin, proto.FAxisUsage, proto.FAxisFlags, proto.FAxisCenterDz, proto.FAxisOuterDz,
		proto.FAxisLedMask, proto.FAxisLedEffect, proto.FAxisColor); e != "" {
		c.TextIn(ui.TSmall, c.Ellipsize(ui.TSmall, e, iw), ui.R(ix, y+h-a.S(24), iw, a.S(16)), 0, ui.ColBad)
		c.RoundRectLine(r, a.S(ui.RadiusBox), 1, ui.Fade(ui.ColBad, 0.6))
	}
}

func (a *App) firstErr(index int, fields ...int) string {
	for _, f := range fields {
		if e := a.errFor(uint8(f), index); e != "" {
			return e
		}
	}
	return ""
}

// axisCalChip shows the calibration state of an axis (right aligned at rx).
func (a *App) axisCalChip(i int, rx, y float32) {
	c := a.ui
	d, b := &a.ed.Draft, a.baseView()
	var text string
	col := ui.ColTextDim
	switch {
	case a.calibrating() && i < int(b.NumAxes):
		switch a.snap.Telemetry.CalChannelState[i] {
		case proto.CalChOk:
			text, col = "range captured", ui.ColGood
		case proto.CalChMoving:
			text, col = "keep moving…", ui.ColCal
		default:
			text, col = "move me", ui.ColCal
		}
	case i >= int(b.NumAxes) || d.Axes[i].Pin != b.Axes[i].Pin:
		text, col = "calibrate after saving", ui.ColTextFaint
	default:
		ch := int(d.Axes[i].Pin) - int(a.snap.Limits.AdcFirstPin)
		if ch >= 0 && ch < proto.MaxAxes && a.snap.Calibration.Channels[ch].Calibrated == 1 {
			text, col = "calibrated", ui.ColGood
		} else {
			text, col = "not calibrated", ui.ColWarn
		}
	}
	w := c.BadgeWidth(text)
	c.Badge(rx-w, y, text, col)
}

// axisBars draws the raw position (with calibration range and deadzones)
// and the output the device reports.
func (a *App) axisBars(i int, x, y, w float32, live bool) {
	c := a.ui
	d := &a.ed.Draft
	tel := &a.snap.Telemetry
	h := a.S(14)
	track := ui.R(x, y, w-a.S(64), h)
	c.RoundRect(track, a.S(4), ui.ColField)
	pos := func(v uint16) float32 { return track.X + track.Width*float32(v)/4095 }
	if !live {
		c.TextIn(ui.TSmall, "live data after saving", track, 0.5, ui.ColTextFaint)
		return
	}
	mn, cen, mx := tel.CalMin[i], tel.CalCenter[i], tel.CalMax[i]
	calCol := ui.Fade(ui.ColAccent, 0.22)
	if a.calibrating() {
		calCol = ui.Fade(ui.ColCal, 0.35)
	}
	if mx > mn {
		c.RoundRect(ui.R(pos(mn), y, pos(mx)-pos(mn), h), a.S(4), calCol)
	}
	g, _ := groupOf(d, i)
	ax := &d.Axes[i]
	if !a.calibrating() && g < 0 && mx > mn {
		cdz, odz := d.CenterDz, d.OuterDz
		if ax.Flags&proto.AxisCustomDz != 0 {
			cdz, odz = ax.CenterDz, ax.OuterDz
		}
		dzc := ui.Fade(ui.ColModified, 0.28)
		if ax.Flags&proto.AxisLinear != 0 {
			span := float32(mx - mn)
			lo := float32(mn) + span*float32(cdz)/1000
			hi := float32(mx) - span*float32(odz)/1000
			rl.DrawRectangleRec(ui.R(pos(mn), y, pos(uint16(lo))-pos(mn), h), dzc)
			rl.DrawRectangleRec(ui.R(pos(uint16(hi)), y, pos(mx)-pos(uint16(hi)), h), dzc)
		} else {
			l := float32(cen) - float32(cen-mn)*float32(cdz)/1000
			rr := float32(cen) + float32(mx-cen)*float32(cdz)/1000
			rl.DrawRectangleRec(ui.R(pos(uint16(l)), y, pos(uint16(rr))-pos(uint16(l)), h), dzc)
			ol := float32(mn) + float32(cen-mn)*float32(odz)/1000
			or := float32(mx) - float32(mx-cen)*float32(odz)/1000
			rl.DrawRectangleRec(ui.R(pos(mn), y, pos(uint16(ol))-pos(mn), h), dzc)
			rl.DrawRectangleRec(ui.R(pos(uint16(or)), y, pos(mx)-pos(uint16(or)), h), dzc)
		}
	}
	if ax.Flags&proto.AxisLinear == 0 || g >= 0 {
		rl.DrawRectangleRec(ui.R(pos(cen)-a.S(0.75), y, a.S(1.5), h), ui.Fade(ui.ColText, 0.45))
	}
	px := pos(tel.Raw[i])
	rl.DrawRectangleRec(ui.R(px-a.S(1.5), y-a.S(3), a.S(3), h+a.S(6)), ui.ColText)
	c.Text(ui.TMonoS, fmt.Sprintf("%4d", tel.Raw[i]), track.X+track.Width+a.S(8), y+a.S(1), ui.ColTextDim)

	// output bar (-1..1 as reported over USB)
	oy := y + h + a.S(8)
	ot := ui.R(track.X, oy, track.Width, a.S(6))
	c.RoundRect(ot, a.S(3), ui.ColField)
	v := float32(tel.Axes[i]) / 32767
	mid := ot.X + ot.Width/2
	if ax.Flags&proto.AxisLinear != 0 && g < 0 {
		t := (v + 1) / 2
		if ax.Flags&proto.AxisInvert != 0 {
			t = 1 - t
		}
		c.RoundRect(ui.R(ot.X, oy, ot.Width*t, ot.Height), a.S(3), ui.ColAccent)
	} else if v >= 0 {
		rl.DrawRectangleRec(ui.R(mid, oy, ot.Width/2*v, ot.Height), ui.ColAccent)
	} else {
		rl.DrawRectangleRec(ui.R(mid+ot.Width/2*v, oy, -ot.Width/2*v, ot.Height), ui.ColAccent)
	}
	c.Text(ui.TMonoS, fmt.Sprintf("%+.2f", v), track.X+track.Width+a.S(8), oy-a.S(4), ui.ColAccent)
}

// ==================================================================== sticks

func (a *App) sectionSticks(x, y, w float32) float32 {
	c := a.ui
	d := &a.ed.Draft
	y0 := y
	y += a.sectionHeader(x, y, w, "Sticks", "Two axes combined into one thumbstick with a round (or square) output. "+
		"Calibration detects which axis is which and their direction.")
	var shown []int
	for g := 0; g < proto.MaxGroups; g++ {
		if d.Groups[g].Enabled == 1 {
			shown = append(shown, g)
		}
	}
	// "Add stick" if two active axes are free
	free := a.freeAxes()
	slot := -1
	for g := 0; g < proto.MaxGroups; g++ {
		if d.Groups[g].Enabled == 0 {
			slot = g
			break
		}
	}
	addR := ui.R(x+w-a.S(120), y0-a.S(4), a.S(120), a.S(ui.RowH))
	if c.IconButton(c.ID("addstick"), addR, ui.IconPlus, "Add stick", ui.BtnSecondary, len(free) < 2 || slot < 0) {
		gr := &d.Groups[slot]
		gr.Enabled, gr.AxisH, gr.AxisV = 1, uint8(free[0]), uint8(free[1])
	}
	if len(shown) == 0 {
		return y - y0 + a.emptyNote(x, y, w, "No sticks. Axes work independently.")
	}
	cols, cw := a.grid(w, a.S(330))
	ch := a.S(214)
	gap := a.S(12)
	for k, g := range shown {
		a.stickCard(g, x+float32(k%cols)*(cw+gap), y+float32(k/cols)*(ch+gap), cw, ch)
	}
	rows := (len(shown) + cols - 1) / cols
	return y - y0 + float32(rows)*(ch+gap) - gap
}

func (a *App) freeAxes() []int {
	d := &a.ed.Draft
	var out []int
	for i := 0; i < int(d.NumAxes); i++ {
		if g, _ := groupOf(d, i); g < 0 {
			out = append(out, i)
		}
	}
	return out
}

// stickCal finds the device calibration for a stick made of two pins.
func (a *App) stickCal(pinH, pinV uint8) *proto.CalGroup {
	for k := range a.snap.Calibration.Groups {
		r := &a.snap.Calibration.Groups[k]
		if r.Calibrated == 1 && (r.PinA == pinH && r.PinB == pinV || r.PinA == pinV && r.PinB == pinH) {
			return r
		}
	}
	return nil
}

func (a *App) stickCard(g int, x, y, w, h float32) {
	c := a.ui
	d, b := &a.ed.Draft, a.baseView()
	gr, bg := &d.Groups[g], &b.Groups[g]
	id := c.ID("stick", g)
	r := ui.R(x, y, w, h)
	c.Panel(r)
	p := a.S(14)
	ix, iw := x+p, w-2*p
	yy := y + a.S(12)
	c.Text(ui.TBodyB, fmt.Sprintf("Stick %d", g+1), ix, yy, ui.ColText)
	if bg.Enabled == 0 {
		c.Badge(ix+a.S(64), yy, "new", ui.ColModified)
	}

	// calibration status
	var text string
	col := ui.ColWarn
	switch {
	case a.calibrating() && bg.Enabled == 1:
		tel := &a.snap.Telemetry
		switch tel.CalGroupState[g] {
		case proto.CalGrpDone:
			text, col = "turn complete", ui.ColGood
		case proto.CalGrpRotating:
			text, col = fmt.Sprintf("rotating %d°", tel.CalGroupSweep[g]), ui.ColCal
		case proto.CalGrpUp:
			text, col = "now rotate clockwise", ui.ColCal
		default:
			text, col = "push up", ui.ColCal
		}
	case bg.Enabled == 0 || gr.AxisH != bg.AxisH || gr.AxisV != bg.AxisV:
		text, col = "calibrate after saving", ui.ColTextFaint
	case a.stickCal(d.Axes[gr.AxisH].Pin, d.Axes[gr.AxisV].Pin) != nil:
		text, col = "calibrated", ui.ColGood
	default:
		text = "not calibrated"
	}
	bw := c.BadgeWidth(text)
	rmR := ui.R(ix+iw-a.S(26), yy-a.S(4), a.S(26), a.S(26))
	c.Badge(rmR.X-bw-a.S(8), yy, text, col)
	if c.IconButton(c.ID(id, "rm"), rmR, ui.IconCross, "", ui.BtnGhost, false) {
		gr.Enabled = 0
	}
	if c.MouseIn(rmR) {
		c.Tip(c.ID(id, "rm"), "Remove this stick (the axes become independent again)")
	}
	yy += a.S(32)

	lw := a.S(78)
	rowH := a.S(ui.RowH)
	half := (iw - a.S(12)) / 2
	label := func(t string, lx, ly float32) { c.TextIn(ui.TSmall, t, ui.R(lx, ly, lw, rowH), 0, ui.ColTextFaint) }
	axisOpts := func() []ui.Option {
		o := make([]ui.Option, d.NumAxes)
		for i := range o {
			o[i] = ui.Option{Label: fmt.Sprintf("Axis %d (%s)", i+1, usageName(d.Axes[i].HidUsage))}
			if og, _ := groupOf(d, i); og >= 0 && og != g {
				o[i].Note = fmt.Sprintf("Stick %d", og+1)
			}
		}
		return o
	}()
	errAxes := a.errFor(proto.FGroupAxes, g)
	label("Horizontal", ix, yy)
	c.Dropdown(c.ID(id, "h"), ui.R(ix+lw, yy, half-lw, rowH), axisOpts, int(gr.AxisH), st(gr.AxisH != bg.AxisH, errAxes),
		func(k int) { gr.AxisH = uint8(k) })
	label("Vertical", ix+half+a.S(12), yy)
	c.Dropdown(c.ID(id, "v"), ui.R(ix+half+a.S(12)+lw, yy, half-lw, rowH), axisOpts, int(gr.AxisV), st(gr.AxisV != bg.AxisV, errAxes),
		func(k int) { gr.AxisV = uint8(k) })
	a.mark(c.ID(id, "axm"), ix-a.S(7), yy+rowH/2, gr.AxisH != bg.AxisH || gr.AxisV != bg.AxisV, "", func() { gr.AxisH, gr.AxisV = bg.AxisH, bg.AxisV })
	yy += rowH + a.S(8)

	label("Output", ix, yy)
	shape := int(gr.Shape)
	if c.Segmented(c.ID(id, "shape"), ui.R(ix+lw, yy, a.S(150), rowH), []string{"Round", "Square"}, &shape, st(gr.Shape != bg.Shape, "")) {
		gr.Shape = uint8(shape)
	}
	if c.MouseIn(ui.R(ix+lw, yy, a.S(150), rowH)) {
		c.Tip(c.ID(id, "shapetip"), "Round: full deflection is reached on a circle.\nSquare: diagonals reach the corners (both axes at 100 %), like many older games expect.")
	}
	a.mark(c.ID(id, "shm"), ix-a.S(7), yy+rowH/2, gr.Shape != bg.Shape, "", func() { gr.Shape = bg.Shape })
	yy += rowH + a.S(8)

	label("Deadzone", ix, yy)
	custom := 0
	if gr.Flags&proto.GroupCustomDz != 0 {
		custom = 1
	}
	if c.Segmented(c.ID(id, "dzm"), ui.R(ix+lw, yy, a.S(130), rowH), []string{"Global", "Custom"}, &custom,
		st(gr.Flags&proto.GroupCustomDz != bg.Flags&proto.GroupCustomDz, "")) {
		gr.Flags = gr.Flags&^proto.GroupCustomDz | uint8(custom)*proto.GroupCustomDz
	}
	fx := ix + lw + a.S(140)
	fw := (ix + iw - fx - a.S(8)) / 2
	if custom == 1 {
		lim := &a.snap.Limits
		mn, mx := rangeOf(lim, proto.FGroupCenterDz)
		numU16(c, c.ID(id, "cdz"), ui.R(fx, yy, fw, rowH), &gr.CenterDz, bg.CenterDz, mn, mx, 5, 1, "%", a.errFor(proto.FGroupCenterDz, g))
		mn, mx = rangeOf(lim, proto.FGroupOuterDz)
		numU16(c, c.ID(id, "odz"), ui.R(fx+fw+a.S(8), yy, fw, rowH), &gr.OuterDz, bg.OuterDz, mn, mx, 5, 1, "%", a.errFor(proto.FGroupOuterDz, g))
	} else {
		c.TextIn(ui.TSmall, fmt.Sprintf("centre %s · edge %s", pct(int(d.CenterDz)), pct(int(d.OuterDz))), ui.R(fx, yy, fw*2, rowH), 0, ui.ColTextFaint)
	}
	a.mark(c.ID(id, "dzmk"), ix-a.S(7), yy+rowH/2, gr.Flags&proto.GroupCustomDz != bg.Flags&proto.GroupCustomDz ||
		gr.CenterDz != bg.CenterDz || gr.OuterDz != bg.OuterDz, "", func() {
		gr.Flags = gr.Flags&^proto.GroupCustomDz | bg.Flags&proto.GroupCustomDz
		gr.CenterDz, gr.OuterDz = bg.CenterDz, bg.OuterDz
	})
	yy += rowH + a.S(8)

	label("Adjust", ix, yy)
	ax := ix + lw
	for k, it := range []struct {
		bit  uint8
		name string
	}{{proto.GroupSwap, "Swap"}, {proto.GroupInvertH, "Flip H"}, {proto.GroupInvertV, "Flip V"}} {
		on := gr.Flags&it.bit != 0
		cid := c.ID(id, "adj", k)
		if c.Checkbox(cid, ui.R(ax, yy, a.S(80), rowH), it.name, &on, st(gr.Flags&it.bit != bg.Flags&it.bit, "")) {
			gr.Flags ^= it.bit
		}
		ax += a.S(84)
	}
	if c.MouseIn(ui.R(ix+lw, yy, a.S(252), rowH)) {
		c.Tip(c.ID(id, "adjtip"), "Calibration detects the orientation automatically. Use these only if the stick still moves the wrong way afterwards.")
	}
	a.mark(c.ID(id, "adjm"), ix-a.S(7), yy+rowH/2, (gr.Flags^bg.Flags)&(proto.GroupSwap|proto.GroupInvertH|proto.GroupInvertV) != 0, "", func() {
		m := uint8(proto.GroupSwap | proto.GroupInvertH | proto.GroupInvertV)
		gr.Flags = gr.Flags&^m | bg.Flags&m
	})
	if e := a.firstErr(g, proto.FGroupAxes, proto.FGroupShape, proto.FGroupFlags, proto.FGroupCenterDz, proto.FGroupOuterDz); e != "" {
		c.TextIn(ui.TSmall, c.Ellipsize(ui.TSmall, e, iw), ui.R(ix, y+h-a.S(22), iw, a.S(16)), 0, ui.ColBad)
		c.RoundRectLine(r, a.S(ui.RadiusBox), 1, ui.Fade(ui.ColBad, 0.6))
	}
}

// =================================================================== buttons

func (a *App) sectionButtons(x, y, w float32) float32 {
	c := a.ui
	d, b := &a.ed.Draft, a.baseView()
	y0 := y
	a.countControl(c.ID("numbtn"), x+w, y, "Active buttons", &d.NumButtons, b.NumButtons, int(a.snap.Limits.MaxButtons), proto.FNumButtons)
	y += a.sectionHeader(x, y, w, "Buttons", "Digital inputs. Wire each button between its GPIO and GND (internal pull-ups are used).")
	n := int(d.NumButtons)
	if n == 0 {
		return y - y0 + a.emptyNote(x, y, w, "No buttons enabled.")
	}
	cols, cw := a.grid(w, a.S(214))
	ch := a.S(88)
	gap := a.S(10)
	for i := 0; i < n && i < proto.MaxButtons; i++ {
		a.buttonCard(i, x+float32(i%cols)*(cw+gap), y+float32(i/cols)*(ch+gap), cw, ch)
	}
	rows := (n + cols - 1) / cols
	return y - y0 + float32(rows)*(ch+gap) - gap
}

func (a *App) buttonCard(i int, x, y, w, h float32) {
	c := a.ui
	d, b := &a.ed.Draft, a.baseView()
	bt, bb := &d.Buttons[i], &b.Buttons[i]
	id := c.ID("btn", i)
	r := ui.R(x, y, w, h)
	c.Panel(r)
	p := a.S(12)
	ix, iw := x+p, w-2*p
	rowH := a.S(ui.RowH)
	yy := y + a.S(10)

	pressed := i < int(b.NumButtons) && a.snap.TelemetryGen > 0 && a.snap.Telemetry.Button(i)
	dot := ui.ColField
	if pressed {
		dot = ui.ColGood
	}
	c.Circle(ix+a.S(7), yy+rowH/2, a.S(6), dot)
	c.TextIn(ui.TBodyB, fmt.Sprintf("Button %d", i+1), ui.R(ix+a.S(20), yy, iw, rowH), 0, ui.ColText)
	pw := min(a.S(112), iw*0.5)
	a.pinPicker(c.ID(id, "pin"), ui.R(ix+iw-pw, yy, pw, rowH), "", roleButton, fmt.Sprintf("Button %d", i+1), &bt.Pin, bb.Pin,
		a.errFor(proto.FButtonPin, i))
	a.mark(c.ID(id, "pm"), ix+iw-pw-a.S(10), yy+rowH/2, bt.Pin != bb.Pin, pinName(bb.Pin), func() { bt.Pin = bb.Pin })
	yy += rowH + a.S(8)

	sw := a.S(48)
	c.TextIn(ui.TSmall, "LEDs", ui.R(ix, yy, a.S(40), rowH), 0, ui.ColTextDim)
	a.ledChips(c.ID(id, "leds"), ix+a.S(40), yy+rowH/2, iw-a.S(40)-sw-a.S(20), &bt.LedMask, bb.LedMask, d, a.errFor(proto.FButtonLedMask, i))
	a.colorPick(c.ID(id, "col"), ui.R(ix+iw-sw, yy, sw, rowH), &bt.Color, bb.Color)
	a.mark(c.ID(id, "lm"), ix+iw-sw-a.S(10), yy+rowH/2, bt.LedMask != bb.LedMask || bt.Color != bb.Color, "", func() {
		bt.LedMask, bt.Color = bb.LedMask, bb.Color
	})
	if e := a.firstErr(i, proto.FButtonPin, proto.FButtonLedMask, proto.FButtonFlags, proto.FButtonColor); e != "" {
		c.RoundRectLine(r, a.S(ui.RadiusBox), 1, ui.Fade(ui.ColBad, 0.7))
		if c.MouseIn(r) {
			c.Tip(c.ID(id, "err"), e)
		}
	}
}

// ====================================================================== LEDs

func (a *App) sectionLeds(x, y, w float32) float32 {
	c := a.ui
	d, b := &a.ed.Draft, a.baseView()
	lim := &a.snap.Limits
	y0 := y
	// Masks follow their physical LED when chains get longer or shorter.
	layout := model.LayoutOf(d)
	defer func() { model.RemapLedMasks(d, layout) }()

	a.countControl(c.ID("numleds"), x+w, y, "LED pins", &d.NumLedPins, b.NumLedPins, int(lim.PlatformLedPins), proto.FNumLedPins)
	desc := "WS2812 / NeoPixel LEDs. Each LED pin drives one LED or a chain of them; LEDs are numbered across the pins in order. " +
		"Colours of inputs add up on each LED."
	if a.snap.Telemetry.State&proto.StatePreview != 0 {
		desc += " Unsaved colour and brightness changes are previewed on the device right now."
	}
	y += a.sectionHeader(x, y, w, "LEDs", desc)
	f := a.beginForm("leds", x, y, w)
	ctlW := f.w - f.labelW
	live := a.snap.TelemetryGen > 0

	first, baseFirst := 0, 0
	for i := 0; i < int(d.NumLedPins) && i < proto.MaxLedPins; i++ {
		i := i
		lid := c.ID("led", i)
		if d.LedCounts[i] == 0 {
			d.LedCounts[i] = 1
		}
		builtin := d.LedPins[i] == lim.BuiltinLedPin && lim.BuiltinLedPin != proto.PinNone
		if builtin {
			d.LedCounts[i] = 1
		}
		mod := d.LedPins[i] != b.LedPins[i] || d.LedCounts[i] != b.LedCounts[i]
		saved := fmt.Sprintf("%s, %d LED", pinName(b.LedPins[i]), b.LedCounts[i])
		if b.LedCounts[i] != 1 {
			saved += "s"
		}
		if i >= int(b.NumLedPins) {
			saved = ""
		}
		myFirst, myBaseFirst := first, baseFirst
		f.row(lid, fmt.Sprintf("LED pin %d", i+1), "", mod, saved, func() {
			d.LedPins[i], d.LedCounts[i] = b.LedPins[i], b.LedCounts[i]
		}, a.firstErr(i, proto.FLedPin, proto.FLedCount), ctlW, func(rr rl.Rectangle) {
			pw := a.S(150)
			a.pinPicker(c.ID(lid, "pin"), ui.R(rr.X, rr.Y, pw, rr.Height), "", roleLed, fmt.Sprintf("LED pin %d", i+1), &d.LedPins[i], b.LedPins[i],
				a.errFor(proto.FLedPin, i))
			cx := rr.X + pw + a.S(14)
			if builtin {
				c.TextIn(ui.TSmall, "built-in LED", ui.R(cx, rr.Y, a.S(130), rr.Height), 0, ui.ColTextDim)
			} else {
				c.TextIn(ui.TSmall, "Chain", ui.R(cx, rr.Y, a.S(44), rr.Height), 0, ui.ColTextDim)
				others := d.LedTotal() - int(d.LedCounts[i])
				mx := max(1, min(int(lim.Ranges[proto.FLedCount].Max), int(lim.MaxLeds)-others))
				numU8(c, c.ID(lid, "count"), ui.R(cx+a.S(44), rr.Y, a.S(86), rr.Height), &d.LedCounts[i], b.LedCounts[i], 1, mx, 1, "",
					a.errFor(proto.FLedCount, i))
				if c.MouseIn(ui.R(cx, rr.Y, a.S(130), rr.Height)) {
					c.Tip(c.ID(lid, "ctip"), "Number of LEDs chained on this pin (DIN to DOUT).\nLong chains refresh a little slower: about 30 µs per LED.")
				}
			}
			cx += a.S(146)
			// live colours (only while the device uses the same layout for this pin)
			if live && i < int(b.NumLedPins) && b.LedPins[i] == d.LedPins[i] && b.LedCounts[i] == d.LedCounts[i] {
				n := int(b.LedCounts[i])
				step := a.S(22)
				fit := int((rr.X + rr.Width - cx) / step)
				for k := 0; k < n && k < fit; k++ {
					lc := a.snap.Telemetry.LedRgb[myBaseFirst+k]
					px := cx + float32(k)*step + a.S(9)
					c.Circle(px, rr.Y+rr.Height/2, a.S(8), rl.Color{R: lc[0], G: lc[1], B: lc[2], A: 255})
					c.Ring(px, rr.Y+rr.Height/2, a.S(8), a.S(1), ui.ColBorderHi)
					if c.MouseIn(ui.R(px-a.S(9), rr.Y, a.S(18), rr.Height)) {
						c.Tip(c.ID(lid, "live", k), fmt.Sprintf("LED %d", myFirst+k+1))
					}
				}
				if n > fit && fit > 0 {
					c.TextIn(ui.TSmall, fmt.Sprintf("+%d", n-fit), ui.R(cx+float32(fit)*step, rr.Y, a.S(40), rr.Height), 0, ui.ColTextFaint)
				}
			}
		})
		first += int(d.LedCounts[i])
		if i < int(b.NumLedPins) {
			baseFirst += int(b.LedCounts[i])
		}
	}

	if d.LedTotal() > 0 {
		f.row(c.ID("statusleds"), "Status lights", "These LEDs show the device state: a slow white breathing when idle, "+
			"blue while waiting for the computer, magenta while calibrating and red in safe mode.",
			d.StatusLedMask != b.StatusLedMask, ledSummary(b.StatusLedMask, d.LedTotal()),
			func() { d.StatusLedMask = b.StatusLedMask }, a.errFor(proto.FStatusLedMask, -1), ctlW, func(rr rl.Rectangle) {
				a.ledChips(c.ID("statusleds", "chips"), rr.X, rr.Y+rr.Height/2, min(rr.Width, a.S(420)), &d.StatusLedMask, b.StatusLedMask, d,
					a.errFor(proto.FStatusLedMask, -1))
			})
	}
	ctlW = min(a.S(360), f.w-f.labelW)

	pctRow := func(key, label, help string, v *uint16, bv uint16, field int) {
		mn, mx := rangeOf(lim, field)
		f.row(c.ID(key), label, help, *v != bv, pct(int(bv)), func() { *v = bv }, a.errFor(uint8(field), -1), ctlW, func(rr rl.Rectangle) {
			fv := float32(*v)
			if c.Slider(c.ID(key, "s"), ui.R(rr.X, rr.Y, rr.Width-a.S(100), rr.Height), &fv, float32(mn), float32(mx), st(*v != bv, "")) {
				*v = uint16(math.Round(float64(fv)/5) * 5)
			}
			numU16(c, c.ID(key, "n"), ui.R(rr.X+rr.Width-a.S(88), rr.Y, a.S(88), rr.Height), v, bv, mn, mx, 10, 1, "%", "")
		})
	}
	pctRow("bright", "Brightness", "Overall LED brightness.", &d.Brightness, b.Brightness, proto.FBrightness)
	pctRow("sbright", "Idle status glow", "Brightness of the slow breathing on status LEDs while idle.", &d.StatusBrightness, b.StatusBrightness, proto.FStatusBrightness)

	flagRow := func(key, label, help string, bit uint8) {
		on := d.LedFlags&bit != 0
		bon := b.LedFlags&bit != 0
		f.row(c.ID(key), label, help, on != bon, fmt.Sprint(bon), func() { d.LedFlags = d.LedFlags&^bit | b.LedFlags&bit }, "", ctlW,
			func(rr rl.Rectangle) {
				if c.Switch(c.ID(key, "sw"), rr, &on, st(on != bon, "")) {
					d.LedFlags ^= bit
				}
			})
	}
	flagRow("dither", "Smooth dimming", "Temporal dithering: the LEDs alternate quickly between neighbouring brightness levels, "+
		"giving 256x finer steps than the LEDs' own 8-bit levels.", proto.LedDither)
	flagRow("keepon", "Stay lit while the PC sleeps", "Normally the LEDs turn off and the device rests when the computer sleeps or is shut down.", proto.LedKeepOnSleep)
	return a.endForm("leds", &f, y) - y0
}

// ========================================================= input processing

func (a *App) sectionProcessing(x, y, w float32) float32 {
	c := a.ui
	d, b := &a.ed.Draft, a.baseView()
	lim := &a.snap.Limits
	y0 := y
	y += a.sectionHeader(x, y, w, "Input processing", "How inputs are sampled and filtered. Defaults suit most hardware.")
	f := a.beginForm("input", x, y, w)
	ctlW := min(a.S(360), f.w-f.labelW)

	mn, mx := rangeOf(lim, proto.FPollRate)
	f.row(c.ID("poll"), "Sampling rate", "How often axes and buttons are read.", d.PollRateHz != b.PollRateHz, fmt.Sprint(b.PollRateHz, " Hz"),
		func() { d.PollRateHz = b.PollRateHz }, a.errFor(proto.FPollRate, -1), a.S(150), func(rr rl.Rectangle) {
			numU16(c, c.ID("poll", "n"), rr, &d.PollRateHz, b.PollRateHz, mn, mx, 100, 0, "Hz", "")
		})

	mn, mx = rangeOf(lim, proto.FSmoothingSamples)
	window := float64(d.SmoothingSamples) / math.Max(1, float64(d.PollRateHz)) * 1000
	mode := int(d.SmoothingMode)
	f.row(c.ID("smooth"), "Smoothing", fmt.Sprintf("Averages the last %d samples (%.0f ms). More = calmer but slower; 'Weighted' reacts fastest.",
		d.SmoothingSamples, window), d.SmoothingSamples != b.SmoothingSamples || d.SmoothingMode != b.SmoothingMode, "",
		func() { d.SmoothingSamples, d.SmoothingMode = b.SmoothingSamples, b.SmoothingMode },
		a.firstErr(-1, proto.FSmoothingSamples, proto.FSmoothingMode), ctlW, func(rr rl.Rectangle) {
			numU8(c, c.ID("smooth", "n"), ui.R(rr.X, rr.Y, a.S(100), rr.Height), &d.SmoothingSamples, b.SmoothingSamples, mn, mx, 1, "", "")
			if c.Segmented(c.ID("smooth", "m"), ui.R(rr.X+a.S(110), rr.Y, rr.Width-a.S(110), rr.Height),
				[]string{"Average", "Weighted", "Triangle"}, &mode, st(d.SmoothingMode != b.SmoothingMode, "")) {
				d.SmoothingMode = uint8(mode)
			}
		})

	mn, mx = rangeOf(lim, proto.FDebounce)
	f.row(c.ID("deb"), "Button debounce", "Ignores contact bounce for this long after a button changes. The first edge is reported instantly.",
		d.DebounceMs != b.DebounceMs, fmt.Sprint(b.DebounceMs, " ms"), func() { d.DebounceMs = b.DebounceMs },
		a.errFor(proto.FDebounce, -1), a.S(150), func(rr rl.Rectangle) {
			numU8(c, c.ID("deb", "n"), rr, &d.DebounceMs, b.DebounceMs, mn, mx, 1, "ms", "")
		})

	dzRow := func(key, label, help string, v *uint16, bv uint16, field int) {
		mn, mx := rangeOf(lim, field)
		f.row(c.ID(key), label, help, *v != bv, pct(int(bv)), func() { *v = bv }, a.errFor(uint8(field), -1), ctlW, func(rr rl.Rectangle) {
			fv := float32(*v)
			if c.Slider(c.ID(key, "s"), ui.R(rr.X, rr.Y, rr.Width-a.S(100), rr.Height), &fv, float32(mn), float32(mx), st(*v != bv, "")) {
				*v = uint16(math.Round(float64(fv)/5) * 5)
			}
			numU16(c, c.ID(key, "n"), ui.R(rr.X+rr.Width-a.S(88), rr.Y, a.S(88), rr.Height), v, bv, mn, mx, 5, 1, "%", "")
		})
	}
	dzRow("cdz", "Centre deadzone", "Default for all axes and sticks: ignores small movements around the centre (or the low end of linear axes).",
		&d.CenterDz, b.CenterDz, proto.FCenterDz)
	dzRow("odz", "Edge deadzone", "Reaches 100 % slightly before the physical end of travel.", &d.OuterDz, b.OuterDz, proto.FOuterDz)
	return a.endForm("input", &f, y) - y0
}

// ============================================================= device & USB

func (a *App) sectionDevice(x, y, w float32) float32 {
	c := a.ui
	d, b := &a.ed.Draft, a.baseView()
	lim := &a.snap.Limits
	y0 := y
	y += a.sectionHeader(x, y, w, "Device & USB", "How the device presents itself to the computer, and the calibration button.")
	f := a.beginForm("usb", x, y, w)
	ctlW := min(a.S(360), f.w-f.labelW)

	strRow := func(key, label, help string, v *proto.CStr, bv proto.CStr, field int, placeholder string) {
		f.row(c.ID(key), label, help, *v != bv, `"`+bv.String()+`"`, func() { *v = bv }, a.errFor(uint8(field), -1), ctlW, func(rr rl.Rectangle) {
			s, done := c.TextField(c.ID(key, "t"), rr, v.String(), ui.TextOpts{MaxLen: proto.StrLen - 1, Placeholder: placeholder,
				Filter: func(r rune) bool { return r >= 0x20 && r <= 0x7E }}, st(*v != bv, a.errFor(uint8(field), -1)))
			if done {
				v.SetString(s)
			}
		})
	}
	strRow("prod", "Device name", "Shown by Windows/Linux/macOS and in games.", &d.UsbProduct, b.UsbProduct, proto.FUsbProduct, "")
	strRow("manu", "Manufacturer", "", &d.UsbManufacturer, b.UsbManufacturer, proto.FUsbManufacturer, "")
	chip := ""
	for _, v := range a.snap.Hello.ChipID {
		chip += fmt.Sprintf("%02X", v)
	}
	strRow("serial", "Serial number", "Leave empty to use the chip's unique ID ("+chip+"). Use different serials if you connect several devices.",
		&d.UsbSerial, b.UsbSerial, proto.FUsbSerial, "automatic (chip ID)")

	idMod := d.UsbVid != b.UsbVid || d.UsbPid != b.UsbPid
	f.row(c.ID("vidpid"), "USB vendor / product ID",
		"Changing these makes the computer see a different device (games may need re-binding). The configurator remembers the new ID automatically.",
		idMod, fmt.Sprintf("%04X:%04X", b.UsbVid, b.UsbPid), func() { d.UsbVid, d.UsbPid = b.UsbVid, b.UsbPid },
		a.firstErr(-1, proto.FUsbVid, proto.FUsbPid), a.S(220), func(rr rl.Rectangle) {
			hw := (rr.Width - a.S(20)) / 2
			c.HexField(c.ID("vid"), ui.R(rr.X, rr.Y, hw, rr.Height), &d.UsbVid, st(d.UsbVid != b.UsbVid, a.errFor(proto.FUsbVid, -1)))
			c.TextIn(ui.TBody, ":", ui.R(rr.X+hw, rr.Y, a.S(20), rr.Height), 0.5, ui.ColTextDim)
			c.HexField(c.ID("pid"), ui.R(rr.X+hw+a.S(20), rr.Y, hw, rr.Height), &d.UsbPid, st(d.UsbPid != b.UsbPid, a.errFor(proto.FUsbPid, -1)))
		})

	mn, mx := rangeOf(lim, proto.FHidRate)
	interval := 1000.0 / math.Max(1, float64(d.HidRateHz))
	f.row(c.ID("hidrate"), "USB report rate", fmt.Sprintf("Reports per second sent to the computer (one every %.1f ms). The computer polls in whole milliseconds.", interval),
		d.HidRateHz != b.HidRateHz, fmt.Sprint(b.HidRateHz, " Hz"), func() { d.HidRateHz = b.HidRateHz }, a.errFor(proto.FHidRate, -1), a.S(150),
		func(rr rl.Rectangle) {
			numU16(c, c.ID("hidrate", "n"), rr, &d.HidRateHz, b.HidRateHz, mn, mx, 50, 0, "Hz", "")
		})

	dt := int(d.HidDeviceType)
	f.row(c.ID("devtype"), "Device type", "Some games and Steam treat these differently. 'Joystick' is often better for flight simulators.",
		d.HidDeviceType != b.HidDeviceType, "", func() { d.HidDeviceType = b.HidDeviceType }, "", a.S(220), func(rr rl.Rectangle) {
			if c.Segmented(c.ID("devtype", "s"), rr, []string{"Gamepad", "Joystick"}, &dt, st(d.HidDeviceType != b.HidDeviceType, "")) {
				d.HidDeviceType = uint8(dt)
			}
		})

	mn, mx = rangeOf(lim, proto.FHidReportID)
	f.row(c.ID("repid"), "HID report ID", "Advanced. Only change if some software requires a specific value.",
		d.HidReportID != b.HidReportID, fmt.Sprint(b.HidReportID), func() { d.HidReportID = b.HidReportID }, a.errFor(proto.FHidReportID, -1), a.S(150),
		func(rr rl.Rectangle) {
			numU8(c, c.ID("repid", "n"), rr, &d.HidReportID, b.HidReportID, mn, mx, 1, "", "")
		})

	c.Divider(f.x+a.S(4), f.y, f.w-a.S(8))
	f.y += a.S(14)

	f.row(c.ID("calpin"), "Calibration button", "Hold to start calibration, press again to finish. Optional - calibration also works from this window.",
		d.CalPin != b.CalPin, pinName(b.CalPin), func() { d.CalPin = b.CalPin }, a.errFor(proto.FCalPin, -1), a.S(170), func(rr rl.Rectangle) {
			a.pinPicker(c.ID("calpin", "p"), rr, "", roleCal, "calibration button", &d.CalPin, b.CalPin, a.errFor(proto.FCalPin, -1))
		})
	if d.CalPin != proto.PinNone {
		mn, mx = rangeOf(lim, proto.FCalHold)
		f.row(c.ID("calhold"), "Hold to calibrate", "", d.CalHoldMs != b.CalHoldMs, fmt.Sprint(b.CalHoldMs, " ms"), func() { d.CalHoldMs = b.CalHoldMs },
			a.errFor(proto.FCalHold, -1), a.S(150), func(rr rl.Rectangle) {
				numU16(c, c.ID("calhold", "n"), rr, &d.CalHoldMs, b.CalHoldMs, mn, mx, 100, 0, "ms", "")
			})
		_, mx = rangeOf(lim, proto.FResetHold)
		f.row(c.ID("rsthold"), "Hold for factory reset", "Keep holding this long to erase all settings (LEDs blink red for 2 s first; release to cancel). 0 = disabled.",
			d.ResetHoldMs != b.ResetHoldMs, fmt.Sprint(b.ResetHoldMs, " ms"), func() { d.ResetHoldMs = b.ResetHoldMs },
			a.errFor(proto.FResetHold, -1), a.S(150), func(rr rl.Rectangle) {
				numU16(c, c.ID("rsthold", "n"), rr, &d.ResetHoldMs, b.ResetHoldMs, 0, mx, 1000, 0, "ms", "")
			})
	}
	return a.endForm("usb", &f, y) - y0
}

// --- per-section change / error counts for the navigation --------------------

func (a *App) countErrs(fields ...int) int {
	n := 0
	for _, e := range a.errs {
		for _, f := range fields {
			if int(e.Field) == f {
				n++
			}
		}
	}
	return n
}

func (a *App) countAxes() (int, int) {
	d, b := &a.ed.Draft, a.baseView()
	n := 0
	if d.NumAxes != b.NumAxes {
		n++
	}
	for i := 0; i < proto.MaxAxes; i++ {
		if d.Axes[i] != b.Axes[i] {
			n++
		}
	}
	return n, a.countErrs(proto.FNumAxes, proto.FAxisPin, proto.FAxisUsage, proto.FAxisFlags, proto.FAxisCenterDz, proto.FAxisOuterDz, proto.FAxisLedMask, proto.FAxisLedEffect, proto.FAxisColor)
}

func (a *App) countSticks() (int, int) {
	d, b := &a.ed.Draft, a.baseView()
	n := 0
	for g := 0; g < proto.MaxGroups; g++ {
		if d.Groups[g] != b.Groups[g] {
			n++
		}
	}
	return n, a.countErrs(proto.FGroupEnabled, proto.FGroupAxes, proto.FGroupShape, proto.FGroupFlags, proto.FGroupCenterDz, proto.FGroupOuterDz)
}

func (a *App) countButtons() (int, int) {
	d, b := &a.ed.Draft, a.baseView()
	n := 0
	if d.NumButtons != b.NumButtons {
		n++
	}
	for i := 0; i < proto.MaxButtons; i++ {
		if d.Buttons[i] != b.Buttons[i] {
			n++
		}
	}
	return n, a.countErrs(proto.FNumButtons, proto.FButtonPin, proto.FButtonFlags, proto.FButtonLedMask, proto.FButtonColor)
}

func (a *App) countLeds() (int, int) {
	d, b := &a.ed.Draft, a.baseView()
	n := 0
	for _, x := range []bool{d.NumLedPins != b.NumLedPins, d.LedPins != b.LedPins, d.LedCounts != b.LedCounts, d.StatusLedMask != b.StatusLedMask,
		d.Brightness != b.Brightness, d.StatusBrightness != b.StatusBrightness, d.LedFlags != b.LedFlags} {
		if x {
			n++
		}
	}
	return n, a.countErrs(proto.FNumLedPins, proto.FLedPin, proto.FLedCount, proto.FStatusLedMask, proto.FLedFlags, proto.FBrightness, proto.FStatusBrightness)
}

func (a *App) countProcessing() (int, int) {
	d, b := &a.ed.Draft, a.baseView()
	n := 0
	for _, x := range []bool{d.PollRateHz != b.PollRateHz, d.SmoothingSamples != b.SmoothingSamples, d.SmoothingMode != b.SmoothingMode,
		d.DebounceMs != b.DebounceMs, d.CenterDz != b.CenterDz, d.OuterDz != b.OuterDz} {
		if x {
			n++
		}
	}
	return n, a.countErrs(proto.FPollRate, proto.FSmoothingSamples, proto.FSmoothingMode, proto.FDebounce, proto.FCenterDz, proto.FOuterDz)
}

func (a *App) countDevice() (int, int) {
	d, b := &a.ed.Draft, a.baseView()
	n := 0
	for _, x := range []bool{d.UsbProduct != b.UsbProduct, d.UsbManufacturer != b.UsbManufacturer, d.UsbSerial != b.UsbSerial,
		d.UsbVid != b.UsbVid, d.UsbPid != b.UsbPid, d.HidRateHz != b.HidRateHz, d.HidDeviceType != b.HidDeviceType,
		d.HidReportID != b.HidReportID, d.CalPin != b.CalPin, d.CalHoldMs != b.CalHoldMs, d.ResetHoldMs != b.ResetHoldMs} {
		if x {
			n++
		}
	}
	return n, a.countErrs(proto.FHeader, proto.FUsbVid, proto.FUsbPid, proto.FUsbManufacturer, proto.FUsbProduct, proto.FUsbSerial,
		proto.FHidRate, proto.FHidReportID, proto.FHidDeviceType, proto.FCalPin, proto.FCalHold, proto.FResetHold)
}
