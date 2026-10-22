package app

import (
	"fmt"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/ui"
)

var usageNames = map[uint8]string{
	proto.UsageX: "X", proto.UsageY: "Y", proto.UsageZ: "Z", proto.UsageRx: "Rx", proto.UsageRy: "Ry",
	proto.UsageRz: "Rz", proto.UsageSlider: "Slider", proto.UsageDial: "Dial",
}

func usageName(u uint8) string {
	if n, ok := usageNames[u]; ok {
		return n
	}
	return fmt.Sprintf("0x%02X", u)
}

func pinName(p uint8) string {
	if p == proto.PinNone {
		return "none"
	}
	return fmt.Sprintf("GPIO %d", p)
}

func st(mod bool, err string) ui.FieldState { return ui.FieldState{Modified: mod, Error: err != ""} }

func pct(permille int) string {
	if permille%10 == 0 {
		return fmt.Sprintf("%d %%", permille/10)
	}
	return fmt.Sprintf("%.1f %%", float64(permille)/10)
}

// --- pin pickers ---------------------------------------------------------------

type pinRole int

const (
	roleAxis pinRole = iota
	roleButton
	roleLed
	roleCal
)

// owners maps every GPIO used by the draft to a description.
func (a *App) owners() map[uint8]string {
	d := &a.ed.Draft
	m := map[uint8]string{}
	if d.CalPin != proto.PinNone {
		m[d.CalPin] = "calibration button"
	}
	for i := 0; i < int(d.NumLedPins) && i < proto.MaxLedPins; i++ {
		m[d.LedPins[i]] = fmt.Sprintf("LED pin %d", i+1)
	}
	for i := 0; i < int(d.NumAxes) && i < proto.MaxAxes; i++ {
		m[d.Axes[i].Pin] = fmt.Sprintf("Axis %d", i+1)
	}
	for i := 0; i < int(d.NumButtons) && i < proto.MaxButtons; i++ {
		m[d.Buttons[i].Pin] = fmt.Sprintf("Button %d", i+1)
	}
	return m
}

// pinPicker draws a GPIO dropdown. self is the owner label of the edited
// item (its own pin is not shown as "used").
func (a *App) pinPicker(id ui.ID, r rl.Rectangle, prefix string, role pinRole, self string, value *uint8, base uint8, err string) {
	lim := &a.snap.Limits
	own := a.owners()
	var opts []ui.Option
	var pins []uint8
	if role == roleCal {
		opts = append(opts, ui.Option{Label: "None"})
		pins = append(pins, proto.PinNone)
	}
	for p := 0; p < int(lim.GpioCount); p++ {
		ok := false
		switch role {
		case roleAxis:
			ok = lim.AdcPinOK(p)
		case roleButton:
			ok = lim.ButtonPinOK(p)
		case roleLed:
			ok = lim.LedPinOK(p)
		case roleCal:
			ok = lim.CalPinOK(p)
		}
		if role == roleAxis && !ok {
			continue // only ADC pins make sense for axes
		}
		o := ui.Option{Label: fmt.Sprintf("GPIO %d", p), Disabled: !ok}
		if who, used := own[uint8(p)]; used && who != self {
			o.Note = who
		}
		opts = append(opts, o)
		pins = append(pins, uint8(p))
	}
	sel := -1
	for i, p := range pins {
		if p == *value {
			sel = i
		}
	}
	if sel < 0 && *value != proto.PinNone {
		opts = append(opts, ui.Option{Label: pinName(*value) + " (invalid)", Disabled: true})
		pins = append(pins, *value)
		sel = len(pins) - 1
	}
	a.ui.DropdownP(id, r, prefix, opts, sel, st(*value != base, err), func(i int) { *value = pins[i] })
}

// --- small composite controls -------------------------------------------------

// ledChips toggles which LEDs react to an input.
// ledChips edits an LED mask. When all LEDs fit into maxW it shows one chip
// per LED (a small gap separates LED pins); otherwise a button that opens
// the chips in a popup, one row per LED pin. Returns the width used.
func (a *App) ledChips(id ui.ID, x, cy, maxW float32, mask *proto.LedMask, base proto.LedMask, d *proto.Config, err string) float32 {
	c := a.ui
	dd := a.S(22)
	gap := a.S(5)
	ggap := a.S(7) // extra space between LED pins
	total := d.LedTotal()
	if total == 0 {
		c.TextIn(ui.TSmall, "no LEDs", ui.R(x, cy-dd/2, a.S(80), dd), 0, ui.ColTextFaint)
		return a.S(60)
	}
	pins := 0
	for p := 0; p < int(d.NumLedPins); p++ {
		if d.LedCounts[p] > 0 {
			pins++
		}
	}
	inlineW := float32(total)*(dd+gap) - gap + float32(pins-1)*ggap
	if inlineW <= maxW {
		lx := x
		l := 0
		for p := 0; p < int(d.NumLedPins) && p < proto.MaxLedPins; p++ {
			for k := 0; k < int(d.LedCounts[p]) && l < proto.MaxLeds; k++ {
				a.ledChip(id, ui.R(lx, cy-dd/2, dd, dd), l, mask, base, d, err)
				lx += dd + gap
				l++
			}
			lx += ggap
		}
		return inlineW
	}

	// summary button + popup
	r := ui.R(x, cy-a.S(ui.RowH)/2, maxW, a.S(ui.RowH))
	hover, clicked := c.Interact(id, r)
	if clicked {
		c.TogglePopup(id)
	}
	open := c.PopupOpen(id)
	border := ui.ColBorder
	switch {
	case err != "":
		border = ui.ColBad
	case *mask != base:
		border = ui.ColModified
	case open:
		border = ui.ColAccent
	case hover:
		border = ui.ColBorderHi
	}
	rad := a.S(ui.RadiusCtl)
	c.RoundRect(r, rad, ui.ColField)
	c.RoundRectLine(r, rad, 1, border)
	c.TextIn(ui.TSmall, c.Ellipsize(ui.TSmall, ledSummary(*mask, total), r.Width-a.S(30)), ui.R(r.X+a.S(9), r.Y, r.Width, r.Height), 0, ui.ColText)
	c.DrawIcon(ui.IconChevronDown, r.X+r.Width-a.S(14), cy, a.S(10), ui.ColTextDim)

	perRow := 4
	for p := 0; p < int(d.NumLedPins) && p < proto.MaxLedPins; p++ {
		perRow = max(perRow, min(16, int(d.LedCounts[p])))
	}
	labelW := a.S(112)
	pw := a.S(24) + labelW + float32(perRow)*(dd+gap)
	rows := 0
	for p := 0; p < int(d.NumLedPins); p++ {
		rows += (int(d.LedCounts[p]) + perRow - 1) / perRow
	}
	ph := a.S(20) + float32(rows)*(dd+a.S(8))
	c.Popover(id, r, pw, ph, func(pr rl.Rectangle) {
		yy := pr.Y + a.S(10)
		l := 0
		for p := 0; p < int(d.NumLedPins) && p < proto.MaxLedPins; p++ {
			n := int(d.LedCounts[p])
			label := fmt.Sprintf("Pin %d · GPIO %d", p+1, d.LedPins[p])
			c.TextIn(ui.TSmall, label, ui.R(pr.X+a.S(12), yy, labelW, dd), 0, ui.ColTextDim)
			for k := 0; k < n && l < proto.MaxLeds; k++ {
				if k > 0 && k%perRow == 0 {
					yy += dd + a.S(8)
				}
				cx := pr.X + a.S(12) + labelW + float32(k%perRow)*(dd+gap)
				a.ledChip(id, ui.R(cx, yy, dd, dd), l, mask, base, d, err)
				l++
			}
			yy += dd + a.S(8)
		}
	})
	return maxW
}

// ledChip is one toggleable LED (number l, 0-based) of an LED mask.
func (a *App) ledChip(id ui.ID, r rl.Rectangle, l int, mask *proto.LedMask, base proto.LedMask, d *proto.Config, err string) {
	c := a.ui
	on := mask.Has(l)
	hover, clicked := c.Interact(c.ID(id, l), r)
	if clicked {
		mask.Set(l, !on)
		on = !on
	}
	bg, fg := ui.ColField, ui.ColTextDim
	if on {
		bg, fg = ui.ColAccentDim, ui.ColText
	}
	if hover {
		bg = ui.Mix(bg, ui.ColText, 0.1)
	}
	cx, cy, rad := r.X+r.Width/2, r.Y+r.Height/2, r.Width/2
	c.Circle(cx, cy, rad, bg)
	ring := ui.Fade(ui.ColBorderHi, 0.8)
	if mask.Has(l) != base.Has(l) {
		ring = ui.ColModified
	}
	if err != "" {
		ring = ui.ColBad
	}
	c.Ring(cx, cy, rad, a.S(1.2), ring)
	c.TextIn(ui.TSmall, fmt.Sprint(l+1), r, 0.5, fg)
	if hover {
		p, k, _ := d.LedLocation(l)
		where := fmt.Sprintf("LED pin %d (GPIO %d)", p+1, d.LedPins[p])
		if d.LedCounts[p] > 1 {
			where = fmt.Sprintf("LED %d of the chain on LED pin %d (GPIO %d)", k+1, p+1, d.LedPins[p])
		}
		c.Tip(c.ID(id, l), fmt.Sprintf("LED %d: %s", l+1, where))
	}
}

// ledSummary describes a mask as "LEDs 1, 3-6" (numbers are 1-based).
func ledSummary(m proto.LedMask, total int) string {
	var parts []string
	for l := 0; l < total; l++ {
		if !m.Has(l) {
			continue
		}
		e := l
		for e+1 < total && m.Has(e+1) {
			e++
		}
		if e > l {
			parts = append(parts, fmt.Sprintf("%d-%d", l+1, e+1))
		} else {
			parts = append(parts, fmt.Sprint(l+1))
		}
		l = e
	}
	switch {
	case len(parts) == 0:
		return "No LEDs"
	case len(parts) == 1 && !strings.Contains(parts[0], "-"):
		return "LED " + parts[0]
	}
	return "LEDs " + strings.Join(parts, ", ")
}

func toRL(col proto.Color) rl.Color { return rl.Color{R: col.R, G: col.G, B: col.B, A: 255} }

func (a *App) colorPick(id ui.ID, r rl.Rectangle, col *proto.Color, base proto.Color) {
	a.ui.ColorSwatch(id, r, toRL(*col), st(*col != base, ""), func(n rl.Color) {
		*col = proto.Color{R: n.R, G: n.G, B: n.B}
	})
}

// mark draws the unsaved-change dot; clicking reverts.
func (a *App) mark(id ui.ID, x, cy float32, modified bool, saved string, revert func()) {
	if a.ui.ModMark(id, x, cy, modified, saved) && revert != nil {
		revert()
	}
}

// number edits an unsigned integer field through an int.
func numU16(c *ui.Ctx, id ui.ID, r rl.Rectangle, v *uint16, base uint16, min, max, step, dec int, suffix, err string) {
	n := int(*v)
	if c.NumberField(id, r, &n, min, max, step, dec, suffix, st(*v != base, err)) {
		*v = uint16(n)
	}
}

func numU8(c *ui.Ctx, id ui.ID, r rl.Rectangle, v *uint8, base uint8, min, max, step int, suffix, err string) {
	n := int(*v)
	if c.NumberField(id, r, &n, min, max, step, 0, suffix, st(*v != base, err)) {
		*v = uint8(n)
	}
}

func rangeOf(l *proto.Limits, f int) (int, int) {
	r := l.Ranges[f]
	return int(r.Min), int(r.Max)
}

// --- form layout for list-style sections ---------------------------------------

type form struct {
	a      *App
	x, y   float32
	w      float32
	labelW float32
}

// row draws a labelled control row and advances the cursor. help (dim) and
// err (red) are wrapped below the control.
func (f *form) row(id ui.ID, label, help string, mod bool, saved string, revert func(), err string, ctlW float32,
	draw func(r rl.Rectangle)) {
	a, c := f.a, f.a.ui
	h := a.S(ui.RowH)
	cy := f.y + h/2
	a.mark(c.ID(id, "mod"), f.x+a.S(8), cy, mod, saved, revert)
	c.TextIn(ui.TBody, label, ui.R(f.x+a.S(20), f.y, f.labelW-a.S(24), h), 0, ui.ColTextDim)
	cx := f.x + f.labelW
	if ctlW <= 0 || ctlW > f.w-f.labelW {
		ctlW = f.w - f.labelW
	}
	draw(ui.R(cx, f.y, ctlW, h))
	f.y += h + a.S(4)
	tw := f.w - f.labelW
	if err != "" {
		f.y += c.Paragraph(ui.TSmall, err, cx, f.y, tw, ui.ColBad) + a.S(2)
	}
	if help != "" {
		f.y += c.Paragraph(ui.TSmall, help, cx, f.y, tw, ui.ColTextFaint) + a.S(2)
	}
	f.y += a.S(8)
}

// sectionHeader draws the title + description; returns the height used.
func (a *App) sectionHeader(x, y, w float32, title, desc string) float32 {
	c := a.ui
	c.Text(ui.THeading, title, x, y, ui.ColText)
	h := a.S(26)
	if desc != "" {
		h += c.Paragraph(ui.TSmall, desc, x, y+h, w-a.S(240), ui.ColTextFaint)
	}
	return h + a.S(10)
}

// countControl is the "N active" stepper in a section header.
func (a *App) countControl(id ui.ID, right, y float32, label string, v *uint8, base uint8, max int, field int) {
	c := a.ui
	fw := a.S(84)
	r := ui.R(right-fw, y-a.S(3), fw, a.S(ui.RowH))
	err := a.errFor(uint8(field), -1)
	numU8(c, id, r, v, base, 0, max, 1, "", err)
	lw, _ := c.Measure(ui.TBody, label)
	c.TextIn(ui.TBody, label, ui.R(r.X-lw-a.S(10), r.Y, lw, r.Height), 0, ui.ColTextDim)
	a.mark(c.ID(id, "mod"), r.X-lw-a.S(22), r.Y+r.Height/2, *v != base, fmt.Sprint(base), func() { *v = base })
}

// beginForm draws the card behind a form section (sized from the previous
// frame) and returns the form cursor.
func (a *App) beginForm(key string, x, y, w float32) form {
	h := a.formH[key]
	if h > 0 {
		a.ui.Panel(ui.R(x, y, w, h))
	}
	return form{a: a, x: x + a.S(8), y: y + a.S(16), w: w - a.S(24), labelW: a.S(214)}
}

// endForm records the form height; returns the y below the card.
func (a *App) endForm(key string, f *form, top float32) float32 {
	h := f.y - top + a.S(6)
	a.formH[key] = h
	return top + h
}
