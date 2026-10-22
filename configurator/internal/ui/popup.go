package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// --- popup bookkeeping ---

func (c *Ctx) popupOpen(id ID) bool { return c.popup == id }

func (c *Ctx) keepPopup(id ID) { c.popupNext = id }

func (c *Ctx) togglePopup(id ID) {
	if c.popup == id {
		c.popup = 0
	} else {
		c.popup = id
	}
}

// OpenPopup opens the popup of the widget with the given id.
func (c *Ctx) OpenPopup(id ID) { c.popup, c.popupNext = id, id }

// ClosePopups closes whatever popup is open.
func (c *Ctx) ClosePopups() { c.popup, c.popupNext = 0, 0 }

func (c *Ctx) popupLayer() int {
	if c.layer >= LayerModal {
		return LayerModalP
	}
	return LayerPopup
}

// placePopup positions a popup of size (w, h) next to anchor, inside the window.
func (c *Ctx) placePopup(anchor rl.Rectangle, w, h float32) rl.Rectangle {
	x := anchor.X
	y := anchor.Y + anchor.Height + c.S(4)
	if y+h > c.H-c.S(8) && anchor.Y-h-c.S(4) > c.S(8) {
		y = anchor.Y - h - c.S(4)
	}
	if y+h > c.H-c.S(8) {
		y = c.H - c.S(8) - h
	}
	if x+w > c.W-c.S(8) {
		x = c.W - c.S(8) - w
	}
	return R(max32(c.S(8), x), max32(c.S(8), y), w, h)
}

// --- dropdown ---

type Option struct {
	Label    string
	Note     string // dimmed text on the right (e.g. "used by Button 3")
	Disabled bool
	Swatch   *rl.Color
}

// Dropdown selects one option. The list is drawn in a popup layer after
// the base layer, so the choice is delivered through onSelect.
func (c *Ctx) Dropdown(id ID, r rl.Rectangle, opts []Option, sel int, st FieldState, onSelect func(int)) {
	c.DropdownP(id, r, "", opts, sel, st, onSelect)
}

// DropdownP is a Dropdown with a dim caption inside the closed button.
func (c *Ctx) DropdownP(id ID, r rl.Rectangle, prefix string, opts []Option, sel int, st FieldState, onSelect func(int)) {
	hover, clicked := false, false
	if !st.Disabled {
		hover, clicked = c.Interact(id, r)
	}
	open := c.popupOpen(id)
	if clicked {
		c.togglePopup(id)
		open = c.popupOpen(id)
	}
	bg, border := c.fieldColors(st, hover, open)
	if open {
		border = ColAccent
	}
	rad := c.S(RadiusCtl)
	c.RoundRect(r, rad, bg)
	c.RoundRectLine(r, rad, 1, border)
	label := "—"
	if sel >= 0 && sel < len(opts) {
		label = opts[sel].Label
	}
	col := ColText
	if st.Disabled {
		col = ColTextFaint
	}
	inner := R(r.X+c.S(9), r.Y, r.Width-c.S(30), r.Height)
	if prefix != "" {
		pw, _ := c.Measure(TSmall, prefix)
		c.TextIn(TSmall, prefix, inner, 0, ColTextFaint)
		inner.X += pw + c.S(7)
		inner.Width -= pw + c.S(7)
	}
	if sel >= 0 && sel < len(opts) && opts[sel].Swatch != nil {
		sw := c.S(10)
		c.Circle(inner.X+sw/2, r.Y+r.Height/2, sw/2, *opts[sel].Swatch)
		inner.X += sw + c.S(6)
		inner.Width -= sw + c.S(6)
	}
	c.TextIn(TBody, c.Ellipsize(TBody, label, inner.Width), inner, 0, col)
	c.DrawIcon(IconChevronDown, r.X+r.Width-c.S(14), r.Y+r.Height/2, c.S(10), ColTextDim)

	if !open {
		return
	}
	c.keepPopup(id)
	rowH := c.S(28)
	w := max32(r.Width, c.S(170))
	for _, o := range opts {
		lw, _ := c.Measure(TBody, o.Label)
		nw, _ := c.Measure(TSmall, o.Note)
		w = max32(w, lw+nw+c.S(48))
	}
	w = min32(w, c.S(360))
	h := min32(float32(len(opts))*rowH+c.S(8), c.S(320))
	pr := c.placePopup(r, w, h)
	scrollID := c.ID(id, "scroll")
	c.Defer(c.popupLayer(), func() {
		c.withoutClip(func() {
			c.Occlude(pr)
			c.Shadow(pr, c.S(8))
			c.RoundRect(pr, c.S(8), ColPanelHi)
			c.RoundRectLine(pr, c.S(8), 1, ColBorderHi)
			content := float32(len(opts))*rowH + c.S(8)
			sa := c.BeginScroll(scrollID, Inset(pr, c.S(1)), content)
			if !sa.didInit {
				// scroll so the selected item is visible when opening
				if sel >= 0 {
					sa.Offset = clamp32(float32(sel)*rowH-pr.Height/2+rowH, 0, max32(0, content-pr.Height))
				}
				sa.didInit = true
			}
			y := pr.Y + c.S(4) - sa.Offset
			for i, o := range opts {
				ir := R(pr.X+c.S(4), y, pr.Width-c.S(8)-sa.BarW, rowH)
				y += rowH
				if ir.Y+ir.Height < pr.Y || ir.Y > pr.Y+pr.Height {
					continue
				}
				iid := c.ID(id, "opt", i)
				ih, ic := false, false
				if !o.Disabled {
					ih, ic = c.Interact(iid, ir)
				}
				if i == sel {
					c.RoundRect(ir, c.S(5), ColAccentDim)
				} else if ih {
					c.RoundRect(ir, c.S(5), ColFieldHov)
				}
				col := ColText
				if o.Disabled {
					col = ColTextFaint
				}
				tx := ir.X + c.S(8)
				if o.Swatch != nil {
					c.Circle(tx+c.S(5), ir.Y+rowH/2, c.S(5), *o.Swatch)
					tx += c.S(16)
				}
				c.TextIn(TBody, o.Label, R(tx, ir.Y, ir.Width, rowH), 0, col)
				if o.Note != "" {
					c.TextIn(TSmall, o.Note, R(ir.X, ir.Y, ir.Width-c.S(8), rowH), 1, ColTextFaint)
				}
				if ic {
					if sel != i && onSelect != nil {
						onSelect(i)
					}
					c.popup, c.popupNext = 0, 0
				}
			}
			c.EndScroll(sa)
			if c.Pressed && !rl.CheckCollisionPointRec(c.Mouse, pr) && !rl.CheckCollisionPointRec(c.Mouse, r) {
				c.popup, c.popupNext = 0, 0
			}
			if c.Key(rl.KeyEscape) {
				c.popup, c.popupNext = 0, 0
			}
		})
	})
}

// --- colour ---

// ColorSwatch shows a colour; clicking opens a picker. onChange is called
// whenever the colour changes (also while dragging, for live preview).
func (c *Ctx) ColorSwatch(id ID, r rl.Rectangle, col rl.Color, st FieldState, onChange func(rl.Color)) {
	hover, clicked := false, false
	if !st.Disabled {
		hover, clicked = c.Interact(id, r)
	}
	if clicked {
		c.togglePopup(id)
	}
	open := c.popupOpen(id)
	rad := c.S(RadiusCtl)
	_, border := c.fieldColors(st, hover, open)
	if open {
		border = ColAccent
	}
	c.RoundRect(r, rad, ColField)
	sw := Inset(r, c.S(4))
	c.RoundRect(sw, rad-c.S(2), col)
	c.RoundRectLine(r, rad, 1, border)
	if !open {
		return
	}
	c.keepPopup(id)
	w, h := c.S(248), c.S(286)
	pr := c.placePopup(r, w, h)
	c.Defer(c.popupLayer(), func() {
		c.withoutClip(func() {
			c.Occlude(pr)
			c.Shadow(pr, c.S(8))
			c.RoundRect(pr, c.S(8), ColPanelHi)
			c.RoundRectLine(pr, c.S(8), 1, ColBorderHi)
			cur := col
			if c.colorPicker(id, Inset(pr, c.S(12)), &cur) && onChange != nil {
				onChange(cur)
			}
			if c.Pressed && !rl.CheckCollisionPointRec(c.Mouse, pr) && !rl.CheckCollisionPointRec(c.Mouse, r) {
				c.popup, c.popupNext = 0, 0
			}
			if c.Key(rl.KeyEscape) && c.Focus == 0 {
				c.popup, c.popupNext = 0, 0
			}
		})
	})
}

// Presets offered in the colour picker.
var ColorPresets = []rl.Color{
	hex(0xFF0000), hex(0xFF6000), hex(0xFFC800), hex(0x00FF00), hex(0x00FFA0),
	hex(0x00A0FF), hex(0x0000FF), hex(0xAA00FF), hex(0xFF008C), hex(0xFFFFFF),
}

type hsv struct{ h, s, v float32 }

// per-picker HSV state, so the hue survives at zero saturation/value
var pickerHSV = map[ID]hsv{}

func toHSV(col rl.Color) hsv {
	v := rl.ColorToHSV(col)
	return hsv{v.X, v.Y, v.Z}
}

func (c *Ctx) colorPicker(id ID, r rl.Rectangle, col *rl.Color) bool {
	cur, ok := pickerHSV[id]
	if !ok || rl.ColorFromHSV(cur.h, cur.s, cur.v) != *col {
		n := toHSV(*col)
		if ok && (n.s == 0 || n.v == 0) {
			n.h = cur.h
		}
		cur = n
	}
	changed := false
	// saturation / value square
	sq := R(r.X, r.Y, r.Width, c.S(150))
	hue := rl.ColorFromHSV(cur.h, 1, 1)
	rl.DrawRectangleGradientH(int32(sq.X), int32(sq.Y), int32(sq.Width), int32(sq.Height), rl.White, hue)
	rl.DrawRectangleGradientV(int32(sq.X), int32(sq.Y), int32(sq.Width), int32(sq.Height), rl.Color{A: 0}, rl.Black)
	sid := c.ID(id, "sv")
	c.Interact(sid, sq)
	if c.Pressing(sid) {
		cur.s = clamp32((c.Mouse.X-sq.X)/sq.Width, 0, 1)
		cur.v = 1 - clamp32((c.Mouse.Y-sq.Y)/sq.Height, 0, 1)
		changed = true
		c.Cursor(rl.MouseCursorCrosshair)
	}
	mx, my := sq.X+cur.s*sq.Width, sq.Y+(1-cur.v)*sq.Height
	c.Ring(mx, my, c.S(6), c.S(2), rl.White)
	c.Ring(mx, my, c.S(7.5), c.S(1), Fade(rl.Black, 0.6))

	// hue bar
	hb := R(r.X, sq.Y+sq.Height+c.S(10), r.Width, c.S(14))
	segW := hb.Width / 6
	for i := 0; i < 6; i++ {
		a := rl.ColorFromHSV(float32(i)*60, 1, 1)
		b := rl.ColorFromHSV(float32(i+1)*60, 1, 1)
		x0 := int32(math.Floor(float64(hb.X + float32(i)*segW)))
		x1 := int32(math.Ceil(float64(hb.X + float32(i+1)*segW)))
		rl.DrawRectangleGradientH(x0, int32(hb.Y), x1-x0, int32(hb.Height), a, b)
	}
	hid := c.ID(id, "hue")
	c.Interact(hid, R(hb.X, hb.Y-c.S(4), hb.Width, hb.Height+c.S(8)))
	if c.Pressing(hid) {
		cur.h = clamp32((c.Mouse.X-hb.X)/hb.Width, 0, 0.9999) * 360
		changed = true
	}
	hx := hb.X + cur.h/360*hb.Width
	c.RoundRectLine(R(hx-c.S(3), hb.Y-c.S(3), c.S(6), hb.Height+c.S(6)), c.S(2), c.S(2), rl.White)

	// presets
	py := hb.Y + hb.Height + c.S(12)
	ps := (r.Width - c.S(4)*float32(len(ColorPresets)-1)) / float32(len(ColorPresets))
	for i, p := range ColorPresets {
		pr := R(r.X+float32(i)*(ps+c.S(4)), py, ps, ps)
		pid := c.ID(id, "preset", i)
		h, clicked := c.Interact(pid, pr)
		c.RoundRect(pr, c.S(4), p)
		if h {
			c.RoundRectLine(pr, c.S(4), c.S(2), rl.White)
		}
		if clicked {
			cur = toHSV(p)
			changed = true
		}
	}

	// hex + rgb
	fy := py + ps + c.S(12)
	nc := rl.ColorFromHSV(cur.h, cur.s, cur.v)
	hexS, done := c.TextField(c.ID(id, "hex"), R(r.X, fy, c.S(84), c.S(RowH)), fmt.Sprintf("%02X%02X%02X", nc.R, nc.G, nc.B),
		TextOpts{MaxLen: 6, Mono: true, Filter: func(r rune) bool { return strings.ContainsRune("0123456789abcdefABCDEF", r) }},
		FieldState{})
	if done {
		if v, err := strconv.ParseUint(hexS, 16, 32); err == nil && len(hexS) == 6 {
			cur = toHSV(rl.Color{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255})
			changed = true
		}
	}
	c.Text(TMonoS, fmt.Sprintf("R %3d  G %3d  B %3d", nc.R, nc.G, nc.B), r.X+c.S(94), fy+c.S(9), ColTextDim)

	pickerHSV[id] = cur
	if changed {
		n := rl.ColorFromHSV(cur.h, cur.s, cur.v)
		n.A = 255
		if n != *col {
			*col = n
			return true
		}
	}
	return false
}

// --- scroll areas ---

type ScrollArea struct {
	id      ID
	r       rl.Rectangle
	content float32
	Offset  float32
	BarW    float32
	didInit bool
}

var scrollState = map[ID]*ScrollArea{}

// BeginScroll starts a vertically scrolling, clipped region. Draw content
// at y - sa.Offset; call EndScroll afterwards.
func (c *Ctx) BeginScroll(id ID, r rl.Rectangle, contentH float32) *ScrollArea {
	sa, ok := scrollState[id]
	if !ok {
		sa = &ScrollArea{id: id}
		scrollState[id] = sa
	}
	sa.r, sa.content = r, contentH
	maxOff := max32(0, contentH-r.Height)
	if w := c.UseWheel(r); w != 0 {
		sa.Offset -= w * c.S(48)
	}
	sa.Offset = clamp32(sa.Offset, 0, maxOff)
	sa.BarW = 0
	if maxOff > 0 {
		sa.BarW = c.S(10)
	}
	c.PushClip(r)
	return sa
}

func (c *Ctx) EndScroll(sa *ScrollArea) {
	c.PopClip()
	maxOff := max32(0, sa.content-sa.r.Height)
	if maxOff <= 0 {
		return
	}
	r := sa.r
	track := R(r.X+r.Width-c.S(7), r.Y+c.S(3), c.S(4), r.Height-c.S(6))
	th := max32(c.S(24), track.Height*r.Height/sa.content)
	ty := track.Y + (track.Height-th)*(sa.Offset/maxOff)
	thumb := R(track.X-c.S(2), ty, c.S(8), th)
	bid := c.ID(sa.id, "bar")
	hover, _ := c.Interact(bid, R(track.X-c.S(4), track.Y, c.S(12), track.Height))
	if c.Pressing(bid) {
		t := clamp32((c.Mouse.Y-track.Y-th/2)/(track.Height-th), 0, 1)
		sa.Offset = t * maxOff
	}
	col := Fade(ColTextFaint, 0.5)
	if hover || c.Pressing(bid) {
		col = ColTextDim
	}
	c.RoundRect(R(track.X, ty, track.Width, th), c.S(2), col)
	_ = thumb
}

// ScrollTo adjusts an area so that y (content coordinates) is visible.
func (c *Ctx) ScrollTo(id ID, y float32) {
	if sa, ok := scrollState[id]; ok {
		sa.Offset = max32(0, y)
	}
}

// --- generic popover ---

// TogglePopup opens the popup of id, or closes it if it is open.
func (c *Ctx) TogglePopup(id ID) { c.togglePopup(id) }

// PopupOpen reports whether the popup of id is open.
func (c *Ctx) PopupOpen(id ID) bool { return c.popupOpen(id) }

// Popover draws a floating panel of size (w, h) next to anchor while the
// popup of id is open. body draws the content (in the popup layer, after
// the rest of the frame). A click outside the panel and the anchor, or
// Escape, closes it.
func (c *Ctx) Popover(id ID, anchor rl.Rectangle, w, h float32, body func(r rl.Rectangle)) {
	if !c.popupOpen(id) {
		return
	}
	c.keepPopup(id)
	pr := c.placePopup(anchor, w, h)
	c.Defer(c.popupLayer(), func() {
		c.withoutClip(func() {
			c.Occlude(pr)
			c.Shadow(pr, c.S(8))
			c.RoundRect(pr, c.S(8), ColPanelHi)
			c.RoundRectLine(pr, c.S(8), 1, ColBorderHi)
			body(pr)
			if c.Pressed && !rl.CheckCollisionPointRec(c.Mouse, pr) && !rl.CheckCollisionPointRec(c.Mouse, anchor) {
				c.popup, c.popupNext = 0, 0
			}
			if c.Key(rl.KeyEscape) && c.Focus == 0 {
				c.popup, c.popupNext = 0, 0
			}
		})
	})
}
