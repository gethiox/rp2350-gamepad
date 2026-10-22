package ui

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

// Modal draws a dimmed backdrop and a centred dialog of the given size
// (logical units are already applied by the caller). Must be called from a
// function deferred at LayerModal. Returns the content rectangle and
// whether the close button / Escape was used.
func (c *Ctx) Modal(id ID, title string, w, h float32) (rl.Rectangle, bool) {
	rl.DrawRectangleRec(R(0, 0, c.W, c.H), Fade(ColBg, 0.72))
	w = min32(w, c.W-c.S(32))
	h = min32(h, c.H-c.S(32))
	r := R((c.W-w)/2, (c.H-h)/2, w, h)
	c.Occlude(r)
	c.Shadow(r, c.S(12))
	c.RoundRect(r, c.S(12), ColPanel)
	c.RoundRectLine(r, c.S(12), 1, ColBorderHi)
	c.Text(TTitle, title, r.X+c.S(20), r.Y+c.S(18), ColText)
	cb := R(r.X+r.Width-c.S(44), r.Y+c.S(12), c.S(32), c.S(32))
	closed := c.IconButton(c.ID(id, "close"), cb, IconCross, "", BtnGhost, false)
	if c.Key(rl.KeyEscape) && c.Focus == 0 && c.popup == 0 {
		closed = true
	}
	return R(r.X+c.S(20), r.Y+c.S(58), r.Width-c.S(40), r.Height-c.S(78)), closed
}

// MenuItem is an entry of a popup menu.
type MenuItem struct {
	Label    string
	Icon     Icon
	Danger   bool
	Disabled bool
	Divider  bool // draw a separator above this item
}

// Menu is a button that opens a list of actions; onPick receives the index.
func (c *Ctx) Menu(id ID, r rl.Rectangle, icon Icon, label string, items []MenuItem, onPick func(int)) {
	hover, clicked := c.Interact(id, r)
	if clicked {
		c.togglePopup(id)
	}
	open := c.popupOpen(id)
	bg := Fade(ColField, 0)
	if hover || open {
		bg = ColFieldHov
	}
	c.RoundRect(r, c.S(RadiusCtl), bg)
	if label == "" {
		c.DrawIcon(icon, r.X+r.Width/2, r.Y+r.Height/2, c.S(16), ColTextDim)
	} else {
		c.DrawIcon(icon, r.X+c.S(16), r.Y+r.Height/2, c.S(15), ColTextDim)
		c.TextIn(TBodyB, label, R(r.X+c.S(30), r.Y, r.Width, r.Height), 0, ColText)
	}
	if !open {
		return
	}
	c.keepPopup(id)
	rowH := c.S(32)
	w := c.S(220)
	for _, it := range items {
		tw, _ := c.Measure(TBody, it.Label)
		w = max32(w, tw+c.S(56))
	}
	h := c.S(8)
	for _, it := range items {
		h += rowH
		if it.Divider {
			h += c.S(9)
		}
	}
	anchor := r
	pr := c.placePopup(anchor, w, h)
	if pr.X+pr.Width > r.X+r.Width { // right-align to the button
		pr.X = max32(c.S(8), r.X+r.Width-w)
	}
	c.Defer(c.popupLayer(), func() {
		c.withoutClip(func() {
			c.Occlude(pr)
			c.Shadow(pr, c.S(8))
			c.RoundRect(pr, c.S(8), ColPanelHi)
			c.RoundRectLine(pr, c.S(8), 1, ColBorderHi)
			y := pr.Y + c.S(4)
			for i, it := range items {
				if it.Divider {
					c.Divider(pr.X+c.S(10), y+c.S(4), pr.Width-c.S(20))
					y += c.S(9)
				}
				ir := R(pr.X+c.S(4), y, pr.Width-c.S(8), rowH)
				y += rowH
				ih, ic := false, false
				if !it.Disabled {
					ih, ic = c.Interact(c.ID(id, "item", i), ir)
				}
				if ih {
					c.RoundRect(ir, c.S(5), ColFieldHov)
				}
				col := ColText
				if it.Danger {
					col = ColBad
				}
				if it.Disabled {
					col = ColTextFaint
				}
				if it.Icon != IconNone {
					c.DrawIcon(it.Icon, ir.X+c.S(16), ir.Y+rowH/2, c.S(14), col)
				}
				c.TextIn(TBody, it.Label, R(ir.X+c.S(34), ir.Y, ir.Width, rowH), 0, col)
				if ic {
					c.popup, c.popupNext = 0, 0
					if onPick != nil {
						onPick(i)
					}
				}
			}
			if c.Pressed && !rl.CheckCollisionPointRec(c.Mouse, pr) && !rl.CheckCollisionPointRec(c.Mouse, r) {
				c.popup, c.popupNext = 0, 0
			}
			if c.Key(rl.KeyEscape) {
				c.popup, c.popupNext = 0, 0
			}
		})
	})
}

// Spinner draws a small activity indicator.
func (c *Ctx) Spinner(x, y, r float32, col rl.Color) {
	a := float32(c.Time*300) - float32(int(c.Time*300)/360*360)
	rl.DrawRing(rl.Vector2{X: x, Y: y}, r-c.S(2), r, a, a+270, 24, col)
}
