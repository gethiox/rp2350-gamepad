package ui

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func R(x, y, w, h float32) rl.Rectangle { return rl.Rectangle{X: x, Y: y, Width: w, Height: h} }

// Inset shrinks r by d on every side.
func Inset(r rl.Rectangle, d float32) rl.Rectangle {
	return rl.Rectangle{X: r.X + d, Y: r.Y + d, Width: r.Width - 2*d, Height: r.Height - 2*d}
}

func roundness(r rl.Rectangle, radius float32) float32 {
	m := min32(r.Width, r.Height)
	if m <= 0 {
		return 0
	}
	return clamp32(2*radius/m, 0, 1)
}

func segs(radius float32) int32 {
	s := int32(radius / 1.5)
	if s < 4 {
		s = 4
	}
	if s > 16 {
		s = 16
	}
	return s
}

func (c *Ctx) RoundRect(r rl.Rectangle, radius float32, col rl.Color) {
	if radius <= 0.5 {
		rl.DrawRectangleRec(r, col)
		return
	}
	rl.DrawRectangleRounded(r, roundness(r, radius), segs(radius), col)
}

func (c *Ctx) RoundRectLine(r rl.Rectangle, radius, thick float32, col rl.Color) {
	rl.DrawRectangleRoundedLinesEx(r, roundness(r, radius), segs(radius), thick, col)
}

// Shadow draws a soft drop shadow under a floating element.
func (c *Ctx) Shadow(r rl.Rectangle, radius float32) {
	for i := 3; i >= 1; i-- {
		d := float32(i) * c.S(3)
		rr := R(r.X-d*0.5, r.Y+d*0.6, r.Width+d, r.Height+d)
		c.RoundRect(rr, radius+d, Fade(ColShadow, 0.35/float32(i)))
	}
}

// Panel draws a card.
func (c *Ctx) Panel(r rl.Rectangle) {
	c.RoundRect(r, c.S(RadiusBox), ColPanel)
	c.RoundRectLine(r, c.S(RadiusBox), 1, Fade(ColBorder, 0.8))
}

func (c *Ctx) Line(x0, y0, x1, y1, thick float32, col rl.Color) {
	rl.DrawLineEx(rl.Vector2{X: x0, Y: y0}, rl.Vector2{X: x1, Y: y1}, thick, col)
}

func (c *Ctx) Circle(x, y, r float32, col rl.Color) {
	rl.DrawCircleV(rl.Vector2{X: x, Y: y}, r, col)
}

func (c *Ctx) Ring(x, y, r, thick float32, col rl.Color) {
	rl.DrawRing(rl.Vector2{X: x, Y: y}, r-thick/2, r+thick/2, 0, 360, int32(clamp32(r, 24, 96)), col)
}

// --- icons (vector, so they stay sharp at every scale) ---

type Icon int

const (
	IconNone Icon = iota
	IconCheck
	IconCross
	IconWarn
	IconChevronDown
	IconChevronUp
	IconMenu
	IconPlus
	IconGear
	IconUndo
	IconInfo
	IconStick
	IconDot
	IconSave
	IconFolder
	IconCopy
)

// DrawIcon draws icon centred at (x, y) with the given size.
func (c *Ctx) DrawIcon(ic Icon, x, y, size float32, col rl.Color) {
	s := size / 2
	t := max32(1.4, size/9)
	switch ic {
	case IconCheck:
		c.Line(x-s*0.7, y, x-s*0.15, y+s*0.55, t*1.15, col)
		c.Line(x-s*0.15, y+s*0.55, x+s*0.75, y-s*0.55, t*1.15, col)
	case IconCross:
		c.Line(x-s*0.6, y-s*0.6, x+s*0.6, y+s*0.6, t, col)
		c.Line(x+s*0.6, y-s*0.6, x-s*0.6, y+s*0.6, t, col)
	case IconWarn:
		a := rl.Vector2{X: x, Y: y - s*0.85}
		b := rl.Vector2{X: x - s*0.95, Y: y + s*0.8}
		d := rl.Vector2{X: x + s*0.95, Y: y + s*0.8}
		rl.DrawTriangle(a, b, d, col)
		c.Line(x, y-s*0.3, x, y+s*0.25, t, ColBg)
		c.Circle(x, y+s*0.52, t*0.6, ColBg)
	case IconInfo:
		c.Circle(x, y, s*0.9, col)
		c.Line(x, y-s*0.05, x, y+s*0.5, t, ColBg)
		c.Circle(x, y-s*0.4, t*0.6, ColBg)
	case IconChevronDown:
		c.Line(x-s*0.5, y-s*0.2, x, y+s*0.3, t, col)
		c.Line(x, y+s*0.3, x+s*0.5, y-s*0.2, t, col)
	case IconChevronUp:
		c.Line(x-s*0.5, y+s*0.2, x, y-s*0.3, t, col)
		c.Line(x, y-s*0.3, x+s*0.5, y+s*0.2, t, col)
	case IconMenu:
		for i := -1; i <= 1; i++ {
			c.Circle(x+float32(i)*s*0.6, y, t*0.9, col)
		}
	case IconPlus:
		c.Line(x-s*0.6, y, x+s*0.6, y, t, col)
		c.Line(x, y-s*0.6, x, y+s*0.6, t, col)
	case IconGear:
		c.Ring(x, y, s*0.55, t*1.4, col)
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			cx, cy := float32(math.Cos(a)), float32(math.Sin(a))
			c.Line(x+cx*s*0.6, y+cy*s*0.6, x+cx*s*0.92, y+cy*s*0.92, t*1.6, col)
		}
	case IconUndo:
		rl.DrawRing(rl.Vector2{X: x, Y: y + s*0.1}, s*0.55-t/2, s*0.55+t/2, 200, 450, 16, col)
		ax, ay := x-s*0.55, y+s*0.1
		rl.DrawTriangle(rl.Vector2{X: ax - s*0.35, Y: ay - s*0.05}, rl.Vector2{X: ax + s*0.35, Y: ay - s*0.05},
			rl.Vector2{X: ax, Y: ay + s*0.4}, col)
	case IconStick:
		c.RoundRect(R(x-s*0.85, y+s*0.45, s*1.7, s*0.45), s*0.15, col)
		c.Line(x, y+s*0.45, x+s*0.35, y-s*0.45, t*1.3, col)
		c.Circle(x+s*0.38, y-s*0.55, s*0.32, col)
	case IconDot:
		c.Circle(x, y, s*0.45, col)
	case IconSave:
		c.Line(x, y-s*0.75, x, y+s*0.25, t, col)
		c.Line(x-s*0.45, y-s*0.2, x, y+s*0.25, t, col)
		c.Line(x+s*0.45, y-s*0.2, x, y+s*0.25, t, col)
		c.Line(x-s*0.75, y+s*0.75, x+s*0.75, y+s*0.75, t, col)
	case IconFolder:
		c.RoundRect(R(x-s*0.85, y-s*0.45, s*1.7, s*1.2), s*0.15, col)
		c.RoundRect(R(x-s*0.85, y-s*0.7, s*0.8, s*0.4), s*0.12, col)
	case IconCopy:
		c.RoundRectLine(R(x-s*0.75, y-s*0.45, s*1.05, s*1.2), s*0.15, t, col)
		c.RoundRectLine(R(x-s*0.3, y-s*0.8, s*1.05, s*1.2), s*0.15, t, col)
	}
}
