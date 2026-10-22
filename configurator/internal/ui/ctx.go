package ui

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type ID uint64

// Layers: popups and dialogs are drawn after (and receive input before)
// the base layer.
const (
	LayerBase   = 0
	LayerPopup  = 10
	LayerModal  = 20
	LayerModalP = 30 // popups opened from inside a modal
	LayerTop    = 40 // tooltips / toasts
)

type occluder struct {
	r     rl.Rectangle
	layer int
}

type deferred struct {
	layer int
	seq   int
	fn    func()
}

// Ctx carries per-frame input and the interaction state between frames.
type Ctx struct {
	fonts       *Fonts
	Scale       float32 // UI scale chosen by the user (x DPI)
	renderScale float32 // framebuffer pixels per screen unit (HiDPI)
	W, H        float32
	Time        float64
	DT          float32

	Mouse                        rl.Vector2
	MouseDown, Pressed, Released bool
	RightPressed                 bool
	DoubleClick                  bool
	Wheel                        float32
	wheelUsed                    bool
	Chars                        []rune
	keys                         map[int32]bool
	Ctrl, Shift                  bool

	Hot, Active, Focus ID
	hotNext            ID
	layer              int
	modalDepth         int

	occl, occlPrev []occluder
	defers         []deferred
	deferSeq       int
	clips          []rl.Rectangle

	cursor      rl.MouseCursor
	tooltip     string
	tooltipID   ID
	tipSince    float64
	lastClickT  float64
	lastClickID ID

	// text editing
	edit editState

	// open popup (dropdown / colour picker): only one at a time
	popup     ID
	popupNext ID
}

func NewCtx(fonts *Fonts) *Ctx {
	return &Ctx{fonts: fonts, Scale: 1, renderScale: 1, keys: map[int32]bool{}}
}

// S converts logical units to screen units.
func (c *Ctx) S(v float32) float32 { return v * c.Scale }

func (c *Ctx) snap(v float32) float32 {
	return float32(math.Round(float64(v*c.renderScale))) / c.renderScale
}

// SetScale changes the UI scale (fonts are re-rasterised lazily).
func (c *Ctx) SetScale(s float32) {
	if s != c.Scale {
		c.Scale = s
		c.fonts.Purge()
	}
}

var trackedKeys = []int32{rl.KeyBackspace, rl.KeyDelete, rl.KeyLeft, rl.KeyRight, rl.KeyHome, rl.KeyEnd,
	rl.KeyEnter, rl.KeyKpEnter, rl.KeyEscape, rl.KeyTab, rl.KeyUp, rl.KeyDown, rl.KeyA, rl.KeyC, rl.KeyV, rl.KeyX,
	rl.KeyS, rl.KeyZ, rl.KeyPageUp, rl.KeyPageDown}

// Begin collects input for a new frame.
func (c *Ctx) Begin() {
	c.W, c.H = float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())
	rs := float32(1)
	if c.W > 0 {
		rs = float32(rl.GetRenderWidth()) / c.W
	}
	if rs <= 0 {
		rs = 1
	}
	if rs != c.renderScale {
		c.renderScale = rs
		c.fonts.Purge()
	}
	c.Time = rl.GetTime()
	c.DT = rl.GetFrameTime()
	c.Mouse = rl.GetMousePosition()
	c.MouseDown = rl.IsMouseButtonDown(rl.MouseButtonLeft)
	c.Pressed = rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	c.Released = rl.IsMouseButtonReleased(rl.MouseButtonLeft)
	c.RightPressed = rl.IsMouseButtonPressed(rl.MouseButtonRight)
	c.Wheel = rl.GetMouseWheelMove()
	c.wheelUsed = false
	c.Chars = c.Chars[:0]
	for r := rl.GetCharPressed(); r != 0; r = rl.GetCharPressed() {
		c.Chars = append(c.Chars, r)
	}
	for _, k := range trackedKeys {
		c.keys[k] = rl.IsKeyPressed(k) || rl.IsKeyPressedRepeat(k)
	}
	c.Ctrl = rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) ||
		rl.IsKeyDown(rl.KeyLeftSuper) || rl.IsKeyDown(rl.KeyRightSuper)
	c.Shift = rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)

	c.hotNext = 0
	c.occlPrev, c.occl = c.occl, c.occlPrev[:0]
	c.layer = LayerBase
	c.cursor = rl.MouseCursorDefault
	c.tooltip = ""
	c.popupNext = 0
	c.DoubleClick = false
	if c.Pressed {
		if c.Time-c.lastClickT < 0.35 {
			c.DoubleClick = true
		}
		c.lastClickT = c.Time
	}
}

// End runs deferred layers, draws the tooltip and finalises interaction state.
func (c *Ctx) End() {
	// Deferred drawing may itself defer (popup inside modal); loop until done.
	for len(c.defers) > 0 {
		ds := c.defers
		c.defers = nil
		sort.SliceStable(ds, func(i, j int) bool {
			if ds[i].layer != ds[j].layer {
				return ds[i].layer < ds[j].layer
			}
			return ds[i].seq < ds[j].seq
		})
		for _, d := range ds {
			prev := c.layer
			c.layer = d.layer
			d.fn()
			c.layer = prev
		}
	}
	c.drawTooltip()

	c.Hot = c.hotNext
	if !c.MouseDown {
		c.Active = 0
	}
	// A press outside any open popup closes it.
	c.popup = c.popupNext
	if c.Pressed && c.Focus != 0 && c.Focus != c.Active {
		c.commitEdit()
	}
	rl.SetMouseCursor(c.cursor)
}

// Defer runs fn after the base layer, at the given layer.
func (c *Ctx) Defer(layer int, fn func()) {
	c.deferSeq++
	c.defers = append(c.defers, deferred{layer, c.deferSeq, fn})
}

// Occlude registers r as covering lower layers (takes effect next frame).
func (c *Ctx) Occlude(r rl.Rectangle) { c.occl = append(c.occl, occluder{r, c.layer}) }

// Layer returns the current input/drawing layer.
func (c *Ctx) Layer() int { return c.layer }

// ModalOpen is set by dialogs; lower layers ignore input while it is set.
func (c *Ctx) SetModal(open bool) {
	if open {
		c.modalDepth = 1
	} else {
		c.modalDepth = 0
	}
}

func (c *Ctx) blocked() bool {
	if c.modalDepth > 0 && c.layer < LayerModal {
		return true
	}
	for _, o := range c.occlPrev {
		if o.layer > c.layer && rl.CheckCollisionPointRec(c.Mouse, o.r) {
			return true
		}
	}
	return false
}

// MouseIn: the pointer is over r, inside the current clip and not covered
// by a higher layer.
func (c *Ctx) MouseIn(r rl.Rectangle) bool {
	if !rl.CheckCollisionPointRec(c.Mouse, r) {
		return false
	}
	if n := len(c.clips); n > 0 && !rl.CheckCollisionPointRec(c.Mouse, c.clips[n-1]) {
		return false
	}
	return !c.blocked()
}

// ID hashes the given parts into a widget id.
func (c *Ctx) ID(parts ...any) ID {
	h := fnv.New64a()
	for _, p := range parts {
		fmt.Fprint(h, p, "\x1f")
	}
	return ID(h.Sum64())
}

// Interact implements the standard press/release behaviour.
// Returns hovered and clicked.
func (c *Ctx) Interact(id ID, r rl.Rectangle) (hover, clicked bool) {
	hover = c.MouseIn(r)
	if hover {
		c.hotNext = id
		c.cursor = rl.MouseCursorPointingHand
		if c.Pressed {
			c.Active = id
		}
	}
	if c.Released && c.Active == id && hover {
		clicked = true
	}
	return
}

// Pressing reports whether id is being held down.
func (c *Ctx) Pressing(id ID) bool { return c.Active == id && c.MouseDown }

func (c *Ctx) Key(k int32) bool { return c.keys[k] }

// UseWheel returns the wheel delta once per frame for the innermost
// hovered scroll target.
func (c *Ctx) UseWheel(r rl.Rectangle) float32 {
	if c.wheelUsed || c.Wheel == 0 || !c.MouseIn(r) {
		return 0
	}
	c.wheelUsed = true
	return c.Wheel
}

// --- clipping ---

func intersect(a, b rl.Rectangle) rl.Rectangle {
	x0, y0 := max32(a.X, b.X), max32(a.Y, b.Y)
	x1, y1 := min32(a.X+a.Width, b.X+b.Width), min32(a.Y+a.Height, b.Y+b.Height)
	if x1 < x0 {
		x1 = x0
	}
	if y1 < y0 {
		y1 = y0
	}
	return rl.Rectangle{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

func (c *Ctx) applyClip() {
	rl.EndScissorMode()
	if n := len(c.clips); n > 0 {
		r := c.clips[n-1]
		rs := c.renderScale
		rl.BeginScissorMode(int32(r.X*rs), int32(r.Y*rs), int32(r.Width*rs), int32(r.Height*rs))
	}
}

func (c *Ctx) PushClip(r rl.Rectangle) {
	if n := len(c.clips); n > 0 {
		r = intersect(r, c.clips[n-1])
	}
	c.clips = append(c.clips, r)
	c.applyClip()
}

func (c *Ctx) PopClip() {
	c.clips = c.clips[:len(c.clips)-1]
	c.applyClip()
}

// ClipRect returns the active clip (whole window if none).
func (c *Ctx) ClipRect() rl.Rectangle {
	if n := len(c.clips); n > 0 {
		return c.clips[n-1]
	}
	return rl.Rectangle{Width: c.W, Height: c.H}
}

// withoutClip runs fn with clipping disabled (popups escape scroll areas).
func (c *Ctx) withoutClip(fn func()) {
	saved := c.clips
	c.clips = nil
	c.applyClip()
	fn()
	c.clips = saved
	c.applyClip()
}

// --- tooltip ---

// Tip shows text near the pointer after a short hover delay.
func (c *Ctx) Tip(id ID, text string) {
	if text == "" {
		return
	}
	if c.tooltipID != id {
		c.tooltipID = id
		c.tipSince = c.Time
	}
	c.tooltip = text
}

func (c *Ctx) drawTooltip() {
	if c.tooltip == "" {
		c.tooltipID = 0
		return
	}
	if c.Time-c.tipSince < 0.45 {
		return
	}
	maxW := c.S(320)
	pad := c.S(8)
	lines := c.Wrap(TSmall, c.tooltip, maxW)
	_, lh := c.Measure(TSmall, "Ag")
	w := float32(0)
	for _, l := range lines {
		if lw, _ := c.Measure(TSmall, l); lw > w {
			w = lw
		}
	}
	h := float32(len(lines))*lh*1.3 + pad*2 - lh*0.3
	x, y := c.Mouse.X+c.S(14), c.Mouse.Y+c.S(18)
	if x+w+pad*2 > c.W {
		x = c.W - w - pad*2 - c.S(4)
	}
	if y+h > c.H {
		y = c.Mouse.Y - h - c.S(8)
	}
	r := rl.Rectangle{X: x, Y: y, Width: w + pad*2, Height: h}
	c.withoutClip(func() {
		c.Shadow(r, c.S(6))
		c.RoundRect(r, c.S(6), hex(0x2B313B))
		c.RoundRectLine(r, c.S(6), 1, ColBorderHi)
		for i, l := range lines {
			c.Text(TSmall, l, x+pad, y+pad+float32(i)*lh*1.3, ColText)
		}
	})
}

// Cursor requests a mouse cursor for this frame.
func (c *Ctx) Cursor(cur rl.MouseCursor) { c.cursor = cur }

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
