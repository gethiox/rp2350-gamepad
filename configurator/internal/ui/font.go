package ui

import (
	"math"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Fonts are rasterised per (face, pixel size) so text is always drawn 1:1
// from its atlas - sharp at every UI scale and on HiDPI screens.
type fontKey struct {
	face Face
	px   int
}

type Fonts struct {
	data  map[Face][]byte
	cache map[fontKey]rl.Font
	cps   []rune
}

func NewFonts(body, bold, mono []byte) *Fonts {
	f := &Fonts{data: map[Face][]byte{FaceBody: body, FaceBold: bold, FaceMono: mono}, cache: map[fontKey]rl.Font{}}
	for r := rune(32); r < 127; r++ {
		f.cps = append(f.cps, r)
	}
	for r := rune(0xA0); r <= 0xFF; r++ {
		if r != 0xAD { // soft hyphen: invisible, and missing from Space Mono
			f.cps = append(f.cps, r)
		}
	}
	f.cps = append(f.cps, '•', '…', '–', '—', '→', '←', '↑', '↓', '·')
	return f
}

func (f *Fonts) get(face Face, px int) rl.Font {
	k := fontKey{face, px}
	if font, ok := f.cache[k]; ok {
		return font
	}
	font := rl.LoadFontFromMemory(".ttf", f.data[face], int32(px), f.cps)
	rl.SetTextureFilter(font.Texture, rl.FilterBilinear)
	f.cache[k] = font
	return font
}

// Purge unloads all atlases (after a scale change).
func (f *Fonts) Purge() {
	for k, font := range f.cache {
		rl.UnloadFont(font)
		delete(f.cache, k)
	}
}

// resolve returns the font and the size to draw it at (screen units).
func (c *Ctx) resolve(st TextStyle) (rl.Font, float32) {
	size := st.Size * c.Scale
	px := int(math.Round(float64(size * c.renderScale)))
	if px < 6 {
		px = 6
	}
	return c.fonts.get(st.Face, px), float32(px) / c.renderScale
}

// Measure returns the width and height of single-line text.
func (c *Ctx) Measure(st TextStyle, s string) (float32, float32) {
	font, size := c.resolve(st)
	v := rl.MeasureTextEx(font, s, size, 0)
	return v.X, size
}

// Text draws single-line text with its top-left at (x, y).
func (c *Ctx) Text(st TextStyle, s string, x, y float32, col rl.Color) {
	font, size := c.resolve(st)
	rl.DrawTextEx(font, s, rl.Vector2{X: c.snap(x), Y: c.snap(y)}, size, 0, col)
}

// TextIn draws text vertically centred in r with the given horizontal
// alignment (0 = left, 0.5 = centre, 1 = right).
func (c *Ctx) TextIn(st TextStyle, s string, r rl.Rectangle, align float32, col rl.Color) {
	w, h := c.Measure(st, s)
	x := r.X + (r.Width-w)*align
	y := r.Y + (r.Height-h)/2
	c.Text(st, s, x, y, col)
}

// Ellipsize shortens s to fit maxW, adding an ellipsis.
func (c *Ctx) Ellipsize(st TextStyle, s string, maxW float32) string {
	if w, _ := c.Measure(st, s); w <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		t := string(r) + "…"
		if w, _ := c.Measure(st, t); w <= maxW {
			return t
		}
	}
	return ""
}

// Wrap splits text into lines no wider than maxW.
func (c *Ctx) Wrap(st TextStyle, s string, maxW float32) []string {
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		line := words[0]
		for _, w := range words[1:] {
			if tw, _ := c.Measure(st, line+" "+w); tw <= maxW {
				line += " " + w
			} else {
				lines = append(lines, line)
				line = w
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// Paragraph draws wrapped text and returns the height used.
func (c *Ctx) Paragraph(st TextStyle, s string, x, y, maxW float32, col rl.Color) float32 {
	_, h := c.Measure(st, "Ag")
	lh := h * 1.3
	lines := c.Wrap(st, s, maxW)
	for i, l := range lines {
		c.Text(st, l, x, y+float32(i)*lh, col)
	}
	return float32(len(lines)) * lh
}

// ParagraphHeight measures what Paragraph would use.
func (c *Ctx) ParagraphHeight(st TextStyle, s string, maxW float32) float32 {
	_, h := c.Measure(st, "Ag")
	return float32(len(c.Wrap(st, s, maxW))) * h * 1.3
}
