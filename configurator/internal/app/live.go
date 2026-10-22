package app

import (
	"fmt"
	"math"
	"sort"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/device"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/ui"
)

func (a *App) livePanel(r rl.Rectangle) {
	c := a.ui
	rl.DrawRectangleRec(r, ui.Mix(ui.ColBg, ui.ColPanel, 0.45))
	rl.DrawRectangleRec(ui.R(r.X, r.Y, 1, r.Height), ui.Fade(ui.ColBorder, 0.8))
	sid := c.ID("live")
	sa := c.BeginScroll(sid, r, a.liveH)
	x := r.X + a.S(14)
	w := r.Width - a.S(28) - sa.BarW*0.5
	y := r.Y + a.S(14) - sa.Offset
	start := y
	y += a.statusCard(x, y, w) + a.S(12)
	y += a.calCard(x, y, w) + a.S(12)
	y += a.sticksLive(x, y, w)
	y += a.inputsCard(x, y, w) + a.S(12)
	a.liveH = y - start + a.S(14)
	c.EndScroll(sa)
}

func (a *App) card(x, y, w, h float32, title string) float32 {
	c := a.ui
	c.Panel(ui.R(x, y, w, h))
	c.Caption(title, x+a.S(14), y+a.S(14), ui.ColTextFaint)
	return y + a.S(36)
}

func (a *App) kv(x, y, w float32, k, v string, col rl.Color) float32 {
	c := a.ui
	c.Text(ui.TSmall, k, x, y, ui.ColTextFaint)
	vw, _ := c.Measure(ui.TBody, v)
	v = c.Ellipsize(ui.TBody, v, w*0.62)
	vw, _ = c.Measure(ui.TBody, v)
	c.Text(ui.TBody, v, x+w-vw, y-a.S(1), col)
	return a.S(22)
}

func (a *App) statusCard(x, y, w float32) float32 {
	c := a.ui
	s := &a.snap
	t := &s.Telemetry
	h := a.S(36) + 5*a.S(22) + a.S(12)
	warn := ""
	switch {
	case s.Hello.Flags&proto.HelloSafeMode != 0:
		warn = "Safe mode: stored settings are not applied."
	case t.State&proto.StateSleeping != 0:
		warn = "The computer is asleep - LEDs and inputs are paused."
	case s.Hello.ResetReason == proto.ResetWatchdog:
		warn = "Recovered from a crash or hang (watchdog)."
	}
	if warn != "" {
		h += c.ParagraphHeight(ui.TSmall, warn, w-a.S(28)) + a.S(6)
	}
	yy := a.card(x, y, w, h, "Device")
	ix, iw := x+a.S(14), w-a.S(28)
	stale := s.State == device.Ready && time.Since(s.TelemetryTime) > 600*time.Millisecond
	state, col := "Connected", ui.ColGood
	if s.State == device.Rebooting {
		state, col = "Restarting…", ui.ColWarn
	} else if stale {
		state, col = "Not responding", ui.ColWarn
	}
	bw := c.BadgeWidth(state)
	c.Badge(x+w-a.S(14)-bw, y+a.S(10), state, col)

	serial := s.Config.UsbSerial.String()
	if serial == "" {
		for _, v := range s.Hello.ChipID {
			serial += fmt.Sprintf("%02X", v)
		}
	}
	up := time.Duration(t.UptimeMs) * time.Millisecond
	yy += a.kv(ix, yy, iw, "Firmware", s.Hello.FwVersion.String(), ui.ColText)
	yy += a.kv(ix, yy, iw, "Board", s.Hello.Board.String(), ui.ColText)
	yy += a.kv(ix, yy, iw, "Serial", serial, ui.ColText)
	yy += a.kv(ix, yy, iw, "Running for", up.Truncate(time.Second).String(), ui.ColText)
	rate := fmt.Sprintf("%d reports/s", t.HidReportsPerS)
	if t.State&proto.StateUsbMounted == 0 {
		rate = "not enumerated"
	}
	yy += a.kv(ix, yy, iw, "USB", rate, ui.ColText)
	if warn != "" {
		c.Paragraph(ui.TSmall, warn, ix, yy+a.S(4), iw, ui.ColWarn)
	}
	return h
}

// --- calibration -------------------------------------------------------------

func (a *App) calCard(x, y, w float32) float32 {
	c := a.ui
	ix, iw := x+a.S(14), w-a.S(28)
	b := &a.ed.Base
	t := &a.snap.Telemetry
	if !a.calibrating() {
		text := "Calibrate after changing hardware or if an axis does not reach its full range. " +
			"Only the axes and sticks you move are updated."
		h := a.S(36) + c.ParagraphHeight(ui.TSmall, text, iw) + a.S(14) + a.S(34) + a.S(14)
		res := ""
		if a.lastCal != nil && a.ui.Time-a.lastCalT < 120 {
			res = a.calSummary(a.lastCal)
			h += c.ParagraphHeight(ui.TSmall, res, iw) + a.S(8)
		}
		yy := a.card(x, y, w, h, "Calibration")
		yy += c.Paragraph(ui.TSmall, text, ix, yy, iw, ui.ColTextDim) + a.S(14)
		if res != "" {
			yy += c.Paragraph(ui.TSmall, res, ix, yy, iw, ui.ColGood) + a.S(8)
		}
		dis := a.busy() || a.snap.State != device.Ready || t.State&proto.StateSleeping != 0
		if c.Button(c.ID("calstart"), ui.R(ix, yy, iw, a.S(34)), "Start calibration", ui.BtnCal, dis) {
			a.mgr.Do(device.Op{Kind: device.OpCalStart})
		}
		return h
	}

	// calibrating: steps and live progress
	steps := "1. Move every axis from end to end.\n2. Sticks: push UP, then rotate CLOCKWISE along the edge for one full turn.\n3. Let go and press Finish."
	h := a.S(36) + c.ParagraphHeight(ui.TSmall, steps, iw) + a.S(10)
	nItems := int(b.NumAxes)
	for g := 0; g < proto.MaxGroups; g++ {
		if b.Groups[g].Enabled == 1 {
			nItems++
		}
	}
	h += float32(nItems)*a.S(24) + a.S(10) + a.S(34) + a.S(14)
	yy := a.card(x, y, w, h, "Calibration")
	bw := c.BadgeWidth("in progress")
	c.Badge(x+w-a.S(14)-bw, y+a.S(10), "in progress", ui.ColCal)
	yy += c.Paragraph(ui.TSmall, steps, ix, yy, iw, ui.ColText) + a.S(10)
	for g := 0; g < proto.MaxGroups; g++ {
		if b.Groups[g].Enabled != 1 {
			continue
		}
		st := t.CalGroupState[g]
		frac := float32(t.CalGroupSweep[g]) / 350
		var label string
		col := ui.ColCal
		switch st {
		case proto.CalGrpDone:
			label, col, frac = "complete", ui.ColGood, 1
		case proto.CalGrpRotating:
			label = fmt.Sprintf("%d° of 360°", t.CalGroupSweep[g])
		case proto.CalGrpUp:
			label = "up captured - rotate clockwise"
		default:
			label, frac = "push up first", 0
		}
		a.progressRow(ix, yy, iw, fmt.Sprintf("Stick %d", g+1), label, frac, col)
		yy += a.S(24)
	}
	for i := 0; i < int(b.NumAxes); i++ {
		if g, _ := groupOf(b, i); g >= 0 {
			// grouped axes are reported through their stick, but their
			// standalone range is captured as well
		}
		span := float32(t.CalMax[i]) - float32(t.CalMin[i])
		frac := span / 512
		label, col := "move from end to end", ui.ColCal
		switch t.CalChannelState[i] {
		case proto.CalChOk:
			label, col = fmt.Sprintf("range %d-%d", t.CalMin[i], t.CalMax[i]), ui.ColGood
		case proto.CalChMoving:
			label = "keep moving"
		default:
			label = "not moved - keeps its calibration"
			col = ui.ColTextFaint
		}
		a.progressRow(ix, yy, iw, fmt.Sprintf("Axis %d", i+1), label, frac, col)
		yy += a.S(24)
	}
	yy += a.S(10)
	bw2 := (iw - a.S(10)) / 2
	if c.Button(c.ID("calfinish"), ui.R(ix, yy, bw2, a.S(34)), "Finish & save", ui.BtnPrimary, a.busy()) {
		a.mgr.Do(device.Op{Kind: device.OpCalFinish})
	}
	if c.Button(c.ID("calcancel"), ui.R(ix+bw2+a.S(10), yy, bw2, a.S(34)), "Cancel", ui.BtnSecondary, a.busy()) {
		a.mgr.Do(device.Op{Kind: device.OpCalCancel})
	}
	return h
}

func (a *App) progressRow(x, y, w float32, name, label string, frac float32, col rl.Color) {
	c := a.ui
	frac = clampf(frac, 0, 1)
	icon := ui.IconDot
	if frac >= 1 && col == ui.ColGood {
		icon = ui.IconCheck
	}
	c.DrawIcon(icon, x+a.S(7), y+a.S(9), a.S(13), col)
	c.Text(ui.TBody, name, x+a.S(20), y, ui.ColText)
	c.Text(ui.TSmall, label, x+a.S(84), y+a.S(2), col)
	bar := ui.R(x+w-a.S(60), y+a.S(7), a.S(60), a.S(5))
	c.RoundRect(bar, a.S(2.5), ui.ColField)
	c.RoundRect(ui.R(bar.X, bar.Y, bar.Width*frac, bar.Height), a.S(2.5), col)
}

func (a *App) calSummary(r *proto.CalResult) string {
	var upd, fail []string
	b := &a.ed.Base
	for i := 0; i < int(b.NumAxes); i++ {
		if r.Channel[i] == proto.CalResUpdated {
			upd = append(upd, fmt.Sprintf("Axis %d", i+1))
		}
	}
	for g := 0; g < proto.MaxGroups; g++ {
		switch r.Group[g] {
		case proto.CalResUpdated:
			upd = append(upd, fmt.Sprintf("Stick %d", g+1))
		case proto.CalResFailed:
			fail = append(fail, fmt.Sprintf("Stick %d", g+1))
		}
	}
	s := "Last calibration updated: "
	if len(upd) == 0 {
		s += "nothing (no input was moved enough)."
	} else {
		s += join(upd) + "."
	}
	if len(fail) > 0 {
		s += " Incomplete (now uncalibrated): " + join(fail) + " - push up, then rotate clockwise a full turn."
	}
	return s
}

func join(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}

// --- sticks --------------------------------------------------------------------

func (a *App) sticksLive(x, y, w float32) float32 {
	b := &a.ed.Base
	total := float32(0)
	for g := 0; g < proto.MaxGroups; g++ {
		gr := &b.Groups[g]
		if gr.Enabled != 1 || int(gr.AxisH) >= int(b.NumAxes) || int(gr.AxisV) >= int(b.NumAxes) {
			continue
		}
		h := a.stickViz(g, x, y+total, w)
		total += h + a.S(12)
	}
	return total
}

func (a *App) stickViz(g int, x, y, w float32) float32 {
	c := a.ui
	b, d := &a.ed.Base, &a.ed.Draft
	gr := &b.Groups[g]
	tel := &a.snap.Telemetry
	tr := &a.sticks[g]
	now := a.ui.Time
	if a.snap.TelemetryGen != tr.lastGen {
		tr.lastGen = a.snap.TelemetryGen
		hv := float32(tel.Axes[gr.AxisH]) / 32767
		vv := -float32(tel.Axes[gr.AxisV]) / 32767
		tr.add(hv, vv, now, gr.Shape == proto.ShapeSquare)
	}
	size := min(w-a.S(28), a.S(230))
	metricsH := a.S(4 * 22)
	h := a.S(36) + size + a.S(12) + metricsH + a.S(10)
	yy := a.card(x, y, w, h, fmt.Sprintf("Stick %d", g+1))
	rb := ui.R(x+w-a.S(14)-a.S(64), y+a.S(8), a.S(64), a.S(24))
	if c.Button(c.ID("trailreset", g), rb, "Reset", ui.BtnGhost, false) {
		tr.reset()
	}
	px := x + (w-size)/2
	plot := ui.R(px, yy, size, size)
	cx, cy := plot.X+size/2, plot.Y+size/2
	rad := size/2 - a.S(8)
	c.RoundRect(plot, a.S(8), ui.Mix(ui.ColBg, ui.ColPanel, 0.3))
	// guides
	c.Line(plot.X+a.S(6), cy, plot.X+size-a.S(6), cy, 1, ui.Fade(ui.ColBorder, 0.9))
	c.Line(cx, plot.Y+a.S(6), cx, plot.Y+size-a.S(6), 1, ui.Fade(ui.ColBorder, 0.9))
	if gr.Shape == proto.ShapeSquare {
		rl.DrawRectangleLinesEx(ui.R(cx-rad, cy-rad, 2*rad, 2*rad), 1, ui.Fade(ui.ColTextFaint, 0.5))
	} else {
		c.Ring(cx, cy, rad, 1, ui.Fade(ui.ColTextFaint, 0.5))
	}
	// deadzones (draft values: shows the effect before saving)
	dg := &d.Groups[g]
	cdz, odz := d.CenterDz, d.OuterDz
	if dg.Flags&proto.GroupCustomDz != 0 {
		cdz, odz = dg.CenterDz, dg.OuterDz
	}
	if cdz > 0 {
		c.Circle(cx, cy, rad*float32(cdz)/1000, ui.Fade(ui.ColModified, 0.18))
	}
	_ = odz
	// measured edge (only between neighbouring valid bins)
	pts, ok, _ := tr.edge(now)
	for i := 0; i < envBins; i++ {
		j := (i + 1) % envBins
		if ok[i] && ok[j] {
			c.Line(cx+pts[i].X*rad, cy-pts[i].Y*rad, cx+pts[j].X*rad, cy-pts[j].Y*rad, a.S(1.5), ui.Fade(ui.ColAccent, 0.55))
		}
	}
	// trail
	n := len(tr.pts)
	for i, p := range tr.pts {
		age := float32(now - p.t)
		alpha := clampf(1-age/2.0, 0, 1) * float32(i+1) / float32(n)
		if alpha <= 0.02 {
			continue
		}
		c.Circle(cx+p.h*rad, cy-p.v*rad, a.S(1.6), ui.Fade(ui.ColAccent, alpha*0.9))
	}
	if n > 0 {
		p := tr.pts[n-1]
		c.Circle(cx+p.h*rad, cy-p.v*rad, a.S(5), ui.ColText)
		c.Text(ui.TMonoS, fmt.Sprintf("H %+.2f  V %+.2f", p.h, p.v), plot.X+a.S(8), plot.Y+size-a.S(18), ui.ColTextDim)
	}
	yy += size + a.S(12)

	// metrics
	ix, iw := x+a.S(14), w-a.S(28)
	m := tr.metrics(now)
	if m.coverage < 0.75 {
		c.Paragraph(ui.TSmall, "Rotate the stick along its edge a few times to measure how round its output is.", ix, yy, iw, ui.ColTextFaint)
		return h
	}
	yy += a.kv(ix, yy, iw, "Edge coverage", fmt.Sprintf("%.0f %%", m.coverage*100), ui.ColText)
	col := ui.ColGood
	if m.roundness < 0.9 {
		col = ui.ColWarn
	}
	shapeName := "Roundness (min / max radius)"
	if gr.Shape == proto.ShapeSquare {
		shapeName = "Squareness (min / max edge)"
	}
	yy += a.kv(ix, yy, iw, shapeName, fmt.Sprintf("%.0f %%", m.roundness*100), col)
	col = ui.ColGood
	if m.full < 0.9 {
		col = ui.ColWarn
	}
	yy += a.kv(ix, yy, iw, "Reaches full deflection", fmt.Sprintf("%.0f %% of directions", m.full*100), col)
	a.kv(ix, yy, iw, "Average edge radius", fmt.Sprintf("%.0f %%", m.mean*100), ui.ColText)
	return h
}

// --- buttons & LEDs overview -------------------------------------------------------

func (a *App) inputsCard(x, y, w float32) float32 {
	c := a.ui
	b := &a.ed.Base
	t := &a.snap.Telemetry
	ix, iw := x+a.S(14), w-a.S(28)
	cell := a.S(26)
	gap := a.S(5)
	cols := int((iw + gap) / (cell + gap))
	if cols < 1 {
		cols = 1
	}
	nb := int(b.NumButtons)
	rows := (nb + cols - 1) / cols
	h := a.S(36) + float32(rows)*(cell+gap) + a.S(10)
	// LEDs: one row per LED pin, wrapped
	nl := b.LedTotal()
	ld := a.S(20) // LED pitch
	perRow := max(1, int(iw/ld))
	ledRows := 0
	for p := 0; p < int(b.NumLedPins) && p < proto.MaxLedPins; p++ {
		ledRows += (int(b.LedCounts[p]) + perRow - 1) / perRow
	}
	if nl > 0 {
		h += a.S(28) + float32(ledRows)*ld + a.S(8)
	}
	yy := a.card(x, y, w, h, "Buttons")
	for i := 0; i < nb; i++ {
		r := ui.R(ix+float32(i%cols)*(cell+gap), yy+float32(i/cols)*(cell+gap), cell, cell)
		on := t.Button(i)
		bg, fg := ui.ColField, ui.ColTextFaint
		if on {
			bg, fg = ui.ColGood, ui.ColBg
		}
		c.RoundRect(r, a.S(5), bg)
		c.TextIn(ui.TMonoS, fmt.Sprint(i+1), r, 0.5, fg)
	}
	yy += float32(rows)*(cell+gap) + a.S(10)
	if nl > 0 {
		c.Caption("LEDs", ix, yy, ui.ColTextFaint)
		if t.State&proto.StatePreview != 0 {
			c.TextIn(ui.TSmall, "preview", ui.R(ix, yy-a.S(4), iw, a.S(24)), 1, ui.ColModified)
		}
		yy += a.S(24)
		l := 0
		for p := 0; p < int(b.NumLedPins) && p < proto.MaxLedPins; p++ {
			for k := 0; k < int(b.LedCounts[p]) && l < proto.MaxLeds; k++ {
				if k > 0 && k%perRow == 0 {
					yy += ld
				}
				lc := t.LedRgb[l]
				col := rl.Color{R: lc[0], G: lc[1], B: lc[2], A: 255}
				lx := ix + float32(k%perRow)*ld + ld/2
				cy := yy + ld/2
				c.Circle(lx, cy, ld*0.45, ui.Fade(col, 0.25))
				c.Circle(lx, cy, ld*0.32, col)
				c.Ring(lx, cy, ld*0.32, a.S(1), ui.ColBorderHi)
				if c.MouseIn(ui.R(lx-ld/2, yy, ld, ld)) {
					c.Tip(c.ID("liveled", l), fmt.Sprintf("LED %d (LED pin %d)", l+1, p+1))
				}
				l++
			}
			yy += ld
		}
	}
	return h
}

// --- stick trail & roundness metrics ---------------------------------------------

type trailPt struct {
	h, v float32
	t    float64
}

const envBins = 72
const envWindow = 8.0 // seconds an edge measurement stays valid

// stickTrail keeps recent positions (for the fading trail) and, per angle
// bin, the furthest recent point (the measured edge of the output).
// For round sticks "furthest" is the Euclidean radius; for square output
// it is max(|h|, |v|), so a perfect square scores 100 % as well.
type stickTrail struct {
	pts     []trailPt
	lastGen uint64
	square  bool
	env     [envBins]struct {
		h, v, n float32
		t       float64
	}
}

func (s *stickTrail) reset() {
	s.pts = nil
	for i := range s.env {
		s.env[i].n, s.env[i].t = 0, 0
	}
}

func (s *stickTrail) norm(h, v float32) float32 {
	if s.square {
		return max(float32(math.Abs(float64(h))), float32(math.Abs(float64(v))))
	}
	return float32(math.Hypot(float64(h), float64(v)))
}

func (s *stickTrail) add(h, v float32, now float64, square bool) {
	if square != s.square {
		s.reset()
		s.square = square
	}
	s.pts = append(s.pts, trailPt{h, v, now})
	if len(s.pts) > 240 {
		s.pts = s.pts[len(s.pts)-240:]
	}
	n := s.norm(h, v)
	if n < 0.35 {
		return // only samples near the edge describe its shape
	}
	ang := math.Atan2(float64(v), float64(h))
	bin := int((ang + math.Pi) / (2 * math.Pi) * envBins)
	if bin >= envBins {
		bin = envBins - 1
	}
	e := &s.env[bin]
	if now-e.t > envWindow || n > e.n {
		e.h, e.v, e.n, e.t = h, v, n, now
	}
}

// edge returns, per bin, the measured edge point and whether it is valid.
// Bins whose radius is far below the median only saw the stick passing
// through (not along the rim) and are ignored.
func (s *stickTrail) edge(now float64) ([envBins]rl.Vector2, [envBins]bool, int) {
	var pts [envBins]rl.Vector2
	var ok [envBins]bool
	var ns []float32
	for _, e := range s.env {
		if e.n > 0 && now-e.t <= envWindow {
			ns = append(ns, e.n)
		}
	}
	if len(ns) == 0 {
		return pts, ok, 0
	}
	sort.Slice(ns, func(i, j int) bool { return ns[i] < ns[j] })
	med := ns[len(ns)/2]
	cnt := 0
	for i, e := range s.env {
		if e.n > 0 && now-e.t <= envWindow && e.n >= 0.8*med {
			pts[i], ok[i] = rl.Vector2{X: e.h, Y: e.v}, true
			cnt++
		}
	}
	return pts, ok, cnt
}

type stickMetrics struct {
	coverage, roundness, full, mean float32
}

func (s *stickTrail) metrics(now float64) stickMetrics {
	var m stickMetrics
	cnt := 0
	lo, hi, sum := float32(10), float32(0), float32(0)
	full := 0
	_, ok, _ := s.edge(now)
	for i, e := range s.env {
		if !ok[i] {
			continue
		}
		cnt++
		r := min(e.n, 1)
		lo, hi, sum = min(lo, r), max(hi, r), sum+r
		if r >= 0.98 {
			full++
		}
	}
	m.coverage = float32(cnt) / envBins
	if cnt > 0 && hi > 0 {
		m.roundness = lo / hi
		m.mean = sum / float32(cnt)
		m.full = float32(full) / float32(cnt)
	}
	return m
}
