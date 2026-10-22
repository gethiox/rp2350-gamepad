// Package ui is a small immediate-mode widget toolkit on top of raylib,
// made for the configurator: a calm dark theme, crisp text at any scale,
// layered popups and modal dialogs, scroll areas and text input.
package ui

import rl "github.com/gen2brain/raylib-go/raylib"

func hex(v uint32) rl.Color {
	return rl.Color{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

// Palette. Colours carry meaning consistently across the app:
// Accent = interactive / primary, Modified = unsaved change,
// Good / Bad / Warn = status, Cal = calibration.
var (
	ColBg        = hex(0x0F1115)
	ColPanel     = hex(0x161A20)
	ColPanelHi   = hex(0x1B2028)
	ColField     = hex(0x20252E)
	ColFieldHov  = hex(0x272D37)
	ColFieldAct  = hex(0x2E3540)
	ColBorder    = hex(0x2A303A)
	ColBorderHi  = hex(0x3A4250)
	ColText      = hex(0xE8EBF1)
	ColTextDim   = hex(0xA0A8B6)
	ColTextFaint = hex(0x6C7484)
	ColAccent    = hex(0x5B9CFF)
	ColAccentDim = hex(0x2D4E80)
	ColModified  = hex(0xF2B84B)
	ColGood      = hex(0x48C78E)
	ColBad       = hex(0xF0616D)
	ColWarn      = hex(0xF59E42)
	ColCal       = hex(0xB48CFF)
	ColShadow    = rl.Color{R: 0, G: 0, B: 0, A: 110}
)

// Fade returns c with alpha multiplied by a (0..1).
func Fade(c rl.Color, a float32) rl.Color {
	if a < 0 {
		a = 0
	}
	if a > 1 {
		a = 1
	}
	c.A = uint8(float32(c.A) * a)
	return c
}

// Mix blends two colours (t = 0 -> a, 1 -> b).
func Mix(a, b rl.Color, t float32) rl.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	f := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t) }
	return rl.Color{R: f(a.R, b.R), G: f(a.G, b.G), B: f(a.B, b.B), A: f(a.A, b.A)}
}

// Text styles
type Face int

const (
	FaceBody Face = iota // Dosis SemiBold
	FaceBold             // Dosis Bold
	FaceMono             // Space Mono Bold (numbers)
)

type TextStyle struct {
	Face Face
	Size float32 // logical pixels at scale 1
}

var (
	TSmall   = TextStyle{FaceBody, 13}
	TBody    = TextStyle{FaceBody, 15}
	TBodyB   = TextStyle{FaceBold, 15}
	TLabel   = TextStyle{FaceBold, 12} // section captions (upper case)
	TTitle   = TextStyle{FaceBold, 19}
	THeading = TextStyle{FaceBold, 16}
	TBig     = TextStyle{FaceBold, 26}
	TMono    = TextStyle{FaceMono, 13}
	TMonoS   = TextStyle{FaceMono, 11}
)

// Metrics (logical pixels at scale 1).
const (
	RowH      = 30 // standard control height
	Pad       = 12
	Gap       = 8
	RadiusCtl = 6
	RadiusBox = 10
)
