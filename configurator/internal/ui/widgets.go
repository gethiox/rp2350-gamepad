package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// FieldState decorates an input: unsaved change and/or validation error.
type FieldState struct {
	Modified bool
	Error    bool
	Disabled bool
}

func (c *Ctx) fieldColors(st FieldState, hover, active bool) (bg, border rl.Color) {
	bg = ColField
	border = Fade(ColBorder, 0.9)
	if st.Disabled {
		return Fade(ColField, 0.5), Fade(ColBorder, 0.5)
	}
	if hover {
		bg = ColFieldHov
	}
	if active {
		bg = ColFieldAct
	}
	if st.Modified {
		border = Fade(ColModified, 0.75)
	}
	if st.Error {
		border = ColBad
	}
	return
}

// --- buttons ---

type ButtonKind int

const (
	BtnSecondary ButtonKind = iota
	BtnPrimary
	BtnDanger
	BtnGhost
	BtnCal
)

// Button draws a push button; returns true when clicked.
func (c *Ctx) Button(id ID, r rl.Rectangle, label string, kind ButtonKind, disabled bool) bool {
	return c.IconButton(id, r, IconNone, label, kind, disabled)
}

func (c *Ctx) IconButton(id ID, r rl.Rectangle, icon Icon, label string, kind ButtonKind, disabled bool) bool {
	hover, clicked := false, false
	if !disabled {
		hover, clicked = c.Interact(id, r)
	}
	pressed := c.Pressing(id) && hover
	var bg, fg, border rl.Color
	switch kind {
	case BtnPrimary:
		bg, fg, border = ColAccent, hex(0x0B1220), ColAccent
	case BtnDanger:
		bg, fg, border = Fade(ColBad, 0.16), ColBad, Fade(ColBad, 0.55)
	case BtnCal:
		bg, fg, border = Fade(ColCal, 0.18), ColCal, Fade(ColCal, 0.55)
	case BtnGhost:
		bg, fg, border = Fade(ColField, 0), ColTextDim, Fade(ColBorder, 0)
	default:
		bg, fg, border = ColField, ColText, ColBorder
	}
	if hover {
		switch kind {
		case BtnPrimary:
			bg = Mix(ColAccent, ColText, 0.15)
		case BtnGhost:
			bg, fg = ColFieldHov, ColText
		default:
			bg = Mix(bg, ColText, 0.07)
		}
	}
	if pressed {
		bg = Mix(bg, ColBg, 0.2)
	}
	if disabled {
		bg, fg, border = Fade(bg, 0.35), Fade(fg, 0.4), Fade(border, 0.3)
	}
	rad := c.S(RadiusCtl)
	c.RoundRect(r, rad, bg)
	if border.A > 0 {
		c.RoundRectLine(r, rad, 1, border)
	}
	st := TBodyB
	tw, _ := c.Measure(st, label)
	is := c.S(15)
	gap := c.S(7)
	total := tw
	if icon != IconNone {
		total += is
		if label != "" {
			total += gap
		}
	}
	x := r.X + (r.Width-total)/2
	if icon != IconNone {
		c.DrawIcon(icon, x+is/2, r.Y+r.Height/2, is, fg)
		x += is + gap
	}
	if label != "" {
		c.TextIn(st, label, R(x, r.Y, tw, r.Height), 0, fg)
	}
	return clicked
}

// --- switch / checkbox ---

// Switch is an on/off toggle. Returns true when toggled.
func (c *Ctx) Switch(id ID, r rl.Rectangle, on *bool, st FieldState) bool {
	h := c.S(20)
	w := c.S(36)
	tr := R(r.X, r.Y+(r.Height-h)/2, w, h)
	hover, clicked := false, false
	if !st.Disabled {
		hover, clicked = c.Interact(id, R(r.X, r.Y, w, r.Height))
	}
	if clicked {
		*on = !*on
	}
	bg := ColField
	if *on {
		bg = ColAccent
	}
	if hover {
		bg = Mix(bg, ColText, 0.1)
	}
	if st.Disabled {
		bg = Fade(bg, 0.4)
	}
	c.RoundRect(tr, h/2, bg)
	border := Fade(ColBorderHi, 0.8)
	if st.Modified {
		border = ColModified
	}
	if st.Error {
		border = ColBad
	}
	c.RoundRectLine(tr, h/2, 1, border)
	k := h - c.S(6)
	kx := tr.X + c.S(3)
	if *on {
		kx = tr.X + tr.Width - c.S(3) - k
	}
	kc := ColTextDim
	if *on {
		kc = hex(0x0B1220)
	}
	if st.Disabled {
		kc = Fade(kc, 0.5)
	}
	c.Circle(kx+k/2, tr.Y+tr.Height/2, k/2, kc)
	return clicked
}

// Checkbox with a label to its right. Returns true when toggled.
func (c *Ctx) Checkbox(id ID, r rl.Rectangle, label string, on *bool, st FieldState) bool {
	box := c.S(18)
	br := R(r.X, r.Y+(r.Height-box)/2, box, box)
	tw, _ := c.Measure(TBody, label)
	hit := R(r.X, r.Y, box+c.S(8)+tw, r.Height)
	hover, clicked := false, false
	if !st.Disabled {
		hover, clicked = c.Interact(id, hit)
	}
	if clicked {
		*on = !*on
	}
	bg, border := c.fieldColors(st, hover, false)
	if *on {
		bg = ColAccent
		if !st.Modified && !st.Error {
			border = ColAccent
		}
	}
	c.RoundRect(br, c.S(4), bg)
	c.RoundRectLine(br, c.S(4), 1, border)
	if *on {
		c.DrawIcon(IconCheck, br.X+box/2, br.Y+box/2, box*0.75, hex(0x0B1220))
	}
	col := ColText
	if st.Disabled {
		col = ColTextFaint
	}
	c.TextIn(TBody, label, R(br.X+box+c.S(8), r.Y, tw, r.Height), 0, col)
	return clicked
}

// --- segmented control ---

// Segmented picks one of several options. Returns true when changed.
func (c *Ctx) Segmented(id ID, r rl.Rectangle, options []string, sel *int, st FieldState) bool {
	n := len(options)
	if n == 0 {
		return false
	}
	rad := c.S(RadiusCtl)
	c.RoundRect(r, rad, ColField)
	border := Fade(ColBorder, 0.9)
	if st.Modified {
		border = Fade(ColModified, 0.75)
	}
	if st.Error {
		border = ColBad
	}
	w := r.Width / float32(n)
	changed := false
	for i, o := range options {
		cr := R(r.X+float32(i)*w, r.Y, w, r.Height)
		hover, clicked := false, false
		if !st.Disabled {
			hover, clicked = c.Interact(c.ID(id, i), cr)
		}
		if clicked && *sel != i {
			*sel = i
			changed = true
		}
		col := ColTextDim
		if i == *sel {
			c.RoundRect(Inset(cr, c.S(2)), rad-c.S(1), ColAccentDim)
			col = ColText
		} else if hover {
			c.RoundRect(Inset(cr, c.S(2)), rad-c.S(1), ColFieldHov)
			col = ColText
		}
		if st.Disabled {
			col = Fade(col, 0.5)
		}
		c.TextIn(TBody, c.Ellipsize(TBody, o, w-c.S(8)), cr, 0.5, col)
	}
	c.RoundRectLine(r, rad, 1, border)
	return changed
}

// --- slider ---

// Slider edits v in [min, max]. Returns true while the value changes.
func (c *Ctx) Slider(id ID, r rl.Rectangle, v *float32, min, max float32, st FieldState) bool {
	th := c.S(4)
	track := R(r.X, r.Y+(r.Height-th)/2, r.Width, th)
	hover := false
	if !st.Disabled {
		hover, _ = c.Interact(id, R(r.X-c.S(6), r.Y, r.Width+c.S(12), r.Height))
	}
	changed := false
	if c.Pressing(id) && !st.Disabled {
		t := clamp32((c.Mouse.X-r.X)/r.Width, 0, 1)
		nv := min + t*(max-min)
		if nv != *v {
			*v = nv
			changed = true
		}
		c.Cursor(rl.MouseCursorResizeEW)
	}
	t := float32(0)
	if max > min {
		t = clamp32((*v-min)/(max-min), 0, 1)
	}
	c.RoundRect(track, th/2, ColField)
	fill := ColAccent
	if st.Modified {
		fill = ColModified
	}
	if st.Disabled {
		fill = Fade(fill, 0.4)
	}
	c.RoundRect(R(track.X, track.Y, track.Width*t, th), th/2, fill)
	kr := c.S(7)
	if hover || c.Pressing(id) {
		kr = c.S(8)
	}
	kx := track.X + track.Width*t
	c.Circle(kx, track.Y+th/2, kr, ColText)
	if st.Error {
		c.Ring(kx, track.Y+th/2, kr, c.S(2), ColBad)
	}
	return changed
}

// --- text input ---

type editState struct {
	id       ID
	buf      []rune
	caret    int
	selAll   bool
	original string
	// a commit that happened outside the widget's own frame (click away)
	pendingID  ID
	pendingVal string
	pending    bool
}

func (c *Ctx) beginEdit(id ID, value string) {
	if c.Focus != 0 && c.Focus != id {
		c.commitEdit()
	}
	c.Focus = id
	c.edit.id = id
	c.edit.buf = []rune(value)
	c.edit.caret = len(c.edit.buf)
	c.edit.selAll = true
	c.edit.original = value
}

func (c *Ctx) commitEdit() {
	if c.Focus == 0 {
		return
	}
	c.edit.pendingID = c.Focus
	c.edit.pendingVal = string(c.edit.buf)
	c.edit.pending = true
	c.Focus = 0
}

// Editing reports whether a text field currently has keyboard focus.
func (c *Ctx) Editing() bool { return c.Focus != 0 }

type TextOpts struct {
	MaxLen      int
	Filter      func(rune) bool
	Placeholder string
	Align       float32
	Mono        bool
	Suffix      string
	RightPad    float32 // space reserved on the right (e.g. stepper buttons)
}

// TextField edits a string; returns (new value, true) once the edit is
// committed (Enter, Tab or clicking elsewhere). Escape cancels.
func (c *Ctx) TextField(id ID, r rl.Rectangle, value string, o TextOpts, st FieldState) (string, bool) {
	if c.edit.pending && c.edit.pendingID == id {
		c.edit.pending = false
		if c.edit.pendingVal != value {
			return c.edit.pendingVal, true
		}
	}
	hover := false
	if !st.Disabled {
		hover, _ = c.Interact(id, r)
		if hover {
			c.Cursor(rl.MouseCursorIBeam)
		}
	}
	focused := c.Focus == id
	if hover && c.Pressed && !focused && !st.Disabled {
		c.beginEdit(id, value)
		focused = true
	}

	result, done := value, false
	if focused {
		e := &c.edit
		insert := func(rs []rune) {
			if e.selAll {
				e.buf, e.caret, e.selAll = e.buf[:0], 0, false
			}
			for _, ch := range rs {
				if o.Filter != nil && !o.Filter(ch) {
					continue
				}
				if o.MaxLen > 0 && len(e.buf) >= o.MaxLen {
					break
				}
				e.buf = append(e.buf[:e.caret], append([]rune{ch}, e.buf[e.caret:]...)...)
				e.caret++
			}
		}
		if len(c.Chars) > 0 && !c.Ctrl {
			insert(c.Chars)
		}
		switch {
		case c.Key(rl.KeyBackspace):
			if e.selAll {
				e.buf, e.caret, e.selAll = e.buf[:0], 0, false
			} else if e.caret > 0 {
				e.buf = append(e.buf[:e.caret-1], e.buf[e.caret:]...)
				e.caret--
			}
		case c.Key(rl.KeyDelete):
			if e.selAll {
				e.buf, e.caret, e.selAll = e.buf[:0], 0, false
			} else if e.caret < len(e.buf) {
				e.buf = append(e.buf[:e.caret], e.buf[e.caret+1:]...)
			}
		case c.Key(rl.KeyLeft):
			e.selAll = false
			if e.caret > 0 {
				e.caret--
			}
		case c.Key(rl.KeyRight):
			e.selAll = false
			if e.caret < len(e.buf) {
				e.caret++
			}
		case c.Key(rl.KeyHome):
			e.selAll, e.caret = false, 0
		case c.Key(rl.KeyEnd):
			e.selAll, e.caret = false, len(e.buf)
		case c.Ctrl && c.Key(rl.KeyA):
			e.selAll = true
		case c.Ctrl && c.Key(rl.KeyC):
			rl.SetClipboardText(string(e.buf))
		case c.Ctrl && c.Key(rl.KeyV):
			insert([]rune(strings.TrimSpace(rl.GetClipboardText())))
		case c.Key(rl.KeyEscape):
			c.Focus = 0
			focused = false
		case c.Key(rl.KeyEnter), c.Key(rl.KeyKpEnter), c.Key(rl.KeyTab):
			c.Focus = 0
			focused = false
			result, done = string(e.buf), string(e.buf) != value
		}
		if hover && c.Pressed && focused && !c.DoubleClick {
			// click positions the caret
			e.selAll = false
			e.caret = len(e.buf)
			x := r.X + c.S(8)
			for i := range e.buf {
				w, _ := c.Measure(c.textStyle(o), string(e.buf[:i+1]))
				if x+w > c.Mouse.X {
					e.caret = i
					break
				}
			}
		}
		if hover && c.DoubleClick {
			e.selAll = true
		}
	}

	bg, border := c.fieldColors(st, hover, focused)
	if focused {
		border = ColAccent
	}
	rad := c.S(RadiusCtl)
	c.RoundRect(r, rad, bg)
	c.RoundRectLine(r, rad, 1, border)

	ts := c.textStyle(o)
	inner := R(r.X+c.S(8), r.Y, r.Width-c.S(16)-o.RightPad, r.Height)
	suffixW := float32(0)
	if o.Suffix != "" {
		suffixW, _ = c.Measure(TSmall, o.Suffix)
		c.TextIn(TSmall, o.Suffix, R(inner.X+inner.Width-suffixW, r.Y, suffixW, r.Height), 0, ColTextFaint)
		inner.Width -= suffixW + c.S(4)
	}
	col := ColText
	if st.Disabled {
		col = ColTextFaint
	}
	c.PushClip(inner)
	if focused {
		txt := string(c.edit.buf)
		tw, th := c.Measure(ts, txt)
		cw, _ := c.Measure(ts, string(c.edit.buf[:c.edit.caret]))
		x := inner.X
		if o.Align > 0 && tw < inner.Width {
			x = inner.X + (inner.Width-tw)*o.Align
		}
		if cw > inner.Width-c.S(2) {
			x -= cw - inner.Width + c.S(2)
		}
		ty := r.Y + (r.Height-th)/2
		if c.edit.selAll && len(c.edit.buf) > 0 {
			rl.DrawRectangleRec(R(x-c.S(1), ty, tw+c.S(2), th), Fade(ColAccent, 0.35))
		}
		c.Text(ts, txt, x, ty, col)
		if math.Mod(c.Time, 1.0) < 0.6 {
			rl.DrawRectangleRec(R(x+cw, ty+c.S(1), max32(1, c.S(1.5)), th-c.S(2)), ColAccent)
		}
	} else if value == "" && o.Placeholder != "" {
		c.TextIn(TBody, o.Placeholder, inner, o.Align, ColTextFaint)
	} else {
		c.TextIn(ts, value, inner, o.Align, col)
	}
	c.PopClip()
	return result, done
}

func (c *Ctx) textStyle(o TextOpts) TextStyle {
	if o.Mono {
		return TMono
	}
	return TBody
}

// NumberField edits an integer with optional stepper buttons, the mouse
// wheel (while focused or hovered with Ctrl) and up/down keys. With
// decimals > 0 the stored integer is shown divided by 10^decimals (e.g.
// permille shown as percent with one decimal). Returns true on change.
func (c *Ctx) NumberField(id ID, r rl.Rectangle, v *int, min, max, step, decimals int, suffix string, st FieldState) bool {
	changed := false
	set := func(n int) {
		if n < min {
			n = min
		}
		if n > max {
			n = max
		}
		if n != *v {
			*v = n
			changed = true
		}
	}
	div := math.Pow(10, float64(decimals))
	format := func(n int) string {
		if decimals == 0 {
			return strconv.Itoa(n)
		}
		return strconv.FormatFloat(float64(n)/div, 'f', decimals, 64)
	}
	bw := c.S(20)
	s, done := c.TextField(id, r, format(*v), TextOpts{MaxLen: 8, Mono: true, Suffix: suffix, RightPad: bw - c.S(4),
		Filter: func(r rune) bool { return r >= '0' && r <= '9' || r == '-' || (decimals > 0 && (r == '.' || r == ',')) }}, st)
	// separator between the value and the steppers
	rl.DrawRectangleRec(R(r.X+r.Width-bw, r.Y+c.S(5), 1, r.Height-c.S(10)), Fade(ColBorder, 0.9))
	if done {
		s = strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			set(int(math.Round(f * div)))
		}
	}
	// steppers
	up := R(r.X+r.Width-bw, r.Y, bw, r.Height/2)
	dn := R(r.X+r.Width-bw, r.Y+r.Height/2, bw, r.Height/2)
	for i, br := range []rl.Rectangle{up, dn} {
		bid := c.ID(id, "step", i)
		hover, clicked := false, false
		if !st.Disabled {
			hover, clicked = c.Interact(bid, br)
		}
		col := ColTextFaint
		if hover {
			col = ColText
		}
		ic := IconChevronUp
		if i == 1 {
			ic = IconChevronDown
		}
		c.DrawIcon(ic, br.X+bw/2, br.Y+br.Height/2+c.S(float32(1-2*i)), c.S(9), col)
		if clicked {
			if i == 0 {
				set(*v + step)
			} else {
				set(*v - step)
			}
		}
	}
	if !st.Disabled && (c.Focus == id || (c.Ctrl && c.MouseIn(r))) {
		if w := c.UseWheel(r); w != 0 {
			set(*v + int(w)*step)
		}
	}
	if c.Focus == id && (c.Key(rl.KeyUp) || c.Key(rl.KeyDown)) {
		if c.Key(rl.KeyUp) {
			set(*v + step)
		} else {
			set(*v - step)
		}
		c.edit.buf = []rune(format(*v))
		c.edit.caret, c.edit.selAll = len(c.edit.buf), true
	}
	return changed
}

// HexField edits a 16-bit value as four hex digits.
func (c *Ctx) HexField(id ID, r rl.Rectangle, v *uint16, st FieldState) bool {
	s, done := c.TextField(id, r, fmt.Sprintf("%04X", *v), TextOpts{MaxLen: 4, Mono: true, Align: 0,
		Filter: func(r rune) bool { return strings.ContainsRune("0123456789abcdefABCDEF", r) }}, st)
	if done {
		if n, err := strconv.ParseUint(s, 16, 16); err == nil && uint16(n) != *v {
			*v = uint16(n)
			return true
		}
	}
	return false
}

// --- badges / markers ---

// BadgeWidth returns the width Badge would use.
func (c *Ctx) BadgeWidth(text string) float32 {
	tw, _ := c.Measure(TSmall, text)
	return tw + c.S(14)
}

// Badge draws a small pill and returns its width.
func (c *Ctx) Badge(x, y float32, text string, col rl.Color) float32 {
	tw, th := c.Measure(TSmall, text)
	h := th + c.S(6)
	r := R(x, y, tw+c.S(14), h)
	c.RoundRect(r, h/2, Fade(col, 0.16))
	c.TextIn(TSmall, text, r, 0.5, col)
	return r.Width
}

// ModMark draws the "unsaved change" marker at (x, centre y). Clicking it
// reverts the field. Returns true when the user asked to revert.
func (c *Ctx) ModMark(id ID, x, cy float32, modified bool, saved string) bool {
	if !modified {
		return false
	}
	s := c.S(14)
	r := R(x-s/2, cy-s/2, s, s)
	hover, clicked := c.Interact(id, r)
	if hover {
		c.Circle(x, cy, c.S(7), Fade(ColModified, 0.25))
		c.DrawIcon(IconUndo, x, cy, c.S(11), ColModified)
		c.Tip(id, "Unsaved change. Saved on the device: "+saved+"\nClick to undo.")
	} else {
		c.Circle(x, cy, c.S(3.5), ColModified)
	}
	return clicked
}

// Divider draws a horizontal rule.
func (c *Ctx) Divider(x, y, w float32) {
	rl.DrawRectangleRec(R(x, y, w, max32(1, c.S(1))), Fade(ColBorder, 0.7))
}

// Caption draws an upper-case section caption.
func (c *Ctx) Caption(text string, x, y float32, col rl.Color) {
	c.Text(TLabel, strings.ToUpper(text), x, y, col)
}
