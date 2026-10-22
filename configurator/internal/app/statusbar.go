package app

import (
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/logbuf"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/ui"
)

func levelColor(l logbuf.Level) rl.Color {
	switch l {
	case logbuf.Error:
		return ui.ColBad
	case logbuf.Warn:
		return ui.ColWarn
	case logbuf.Debug:
		return ui.ColTextFaint
	}
	return ui.ColTextDim
}

// statusBar: last message on the left, log toggle on the right. It sits on
// top of the log drawer; dragging it up or down opens and resizes the log.
func (a *App) statusBar(r rl.Rectangle) {
	c := a.ui
	rl.DrawRectangleRec(r, ui.ColPanel)
	c.Divider(0, r.Y, r.Width)

	// drag handle: the whole bar (except buttons) can be dragged
	hid := c.ID("statusdrag")
	toggleR := ui.R(r.X+r.Width-a.S(118), r.Y+a.S(3), a.S(110), r.Height-a.S(6))
	hover, clicked := c.Interact(hid, ui.R(r.X, r.Y-a.S(3), r.Width-a.S(124), r.Height+a.S(3)))
	if hover || c.Pressing(hid) {
		c.Cursor(rl.MouseCursorResizeNS)
	}
	if c.Pressing(hid) {
		h := (c.H - c.Mouse.Y - r.Height/2) / a.ui.Scale
		if h > 60 {
			a.logOpen = true
			a.logH = clampf(h, 120, c.H*0.6/a.ui.Scale)
			a.logDrag = true
		} else if a.logDrag {
			a.logOpen = false
		}
	} else if clicked && !a.logDrag {
		a.logOpen = !a.logOpen
	}
	if !c.MouseDown {
		a.logDrag = false
	}
	// grip
	gx := r.X + r.Width/2
	for i := -1; i <= 1; i++ {
		c.Circle(gx+float32(i)*a.S(6), r.Y+a.S(5), a.S(1.3), ui.ColTextFaint)
	}

	if e, ok := logbuf.Default.Last(logbuf.Info); ok {
		msg := e.Time.Format("15:04:05") + "  " + e.Msg
		c.DrawIcon(ui.IconDot, r.X+a.S(16), r.Y+r.Height/2, a.S(12), levelColor(e.Level))
		col := ui.ColTextDim
		if e.Level >= logbuf.Warn {
			col = levelColor(e.Level)
		}
		c.TextIn(ui.TSmall, c.Ellipsize(ui.TSmall, msg, toggleR.X-r.X-a.S(48)), ui.R(r.X+a.S(28), r.Y, r.Width, r.Height), 0, col)
	}
	icon := ui.IconChevronUp
	if a.logOpen {
		icon = ui.IconChevronDown
	}
	label := "Log"
	if n := a.unseenProblems(); n > 0 && !a.logOpen {
		label = fmt.Sprintf("Log (%d)", n)
	}
	if c.IconButton(c.ID("logtoggle"), toggleR, icon, label, ui.BtnGhost, false) {
		a.logOpen = !a.logOpen
	}
	if a.logOpen {
		_, a.logSeen = logbuf.Default.Snapshot()
	}
}

func (a *App) unseenProblems() int {
	es, seq := logbuf.Default.Snapshot()
	if seq == a.logSeen {
		return 0
	}
	n := 0
	start := len(es) - int(seq-a.logSeen)
	if start < 0 {
		start = 0
	}
	for _, e := range es[start:] {
		if e.Level >= logbuf.Warn {
			n++
		}
	}
	return n
}

// logPos is a position in the log text: entry ID and rune column in its Line().
type logPos struct {
	id  uint64
	col int
}

func (p logPos) less(q logPos) bool { return p.id < q.id || p.id == q.id && p.col < q.col }

// logRow is one visual row of the log view (long entries wrap).
type logRow struct {
	entry      int // index into the shown entries
	start, end int // rune range of the entry's Line()
	indent     int // columns of indentation (continuation rows)
}

func (a *App) logDrawer(r rl.Rectangle) {
	if r.Height <= 0 {
		return
	}
	c := a.ui
	rl.DrawRectangleRec(r, ui.Mix(ui.ColBg, ui.ColPanel, 0.6))
	hy := r.Y + a.S(8)
	c.Text(ui.THeading, "Log", r.X+a.S(16), hy+a.S(4), ui.ColText)
	filters := []string{"Info", "Warnings", "Errors"}
	if a.logFilter >= len(filters) {
		a.logFilter = 0
	}
	c.Segmented(c.ID("logfilter"), ui.R(r.X+a.S(70), hy, a.S(250), a.S(28)), filters, &a.logFilter, ui.FieldState{})
	bx := r.X + r.Width - a.S(16)
	btn := func(label string, icon ui.Icon, w float32) bool {
		bx -= w
		clicked := c.IconButton(c.ID("logbtn", label), ui.R(bx, hy, w, a.S(28)), icon, label, ui.BtnSecondary, false)
		bx -= a.S(8)
		return clicked
	}
	if btn("Clear", ui.IconCross, a.S(84)) {
		logbuf.Default.Clear()
		a.logSelA, a.logSelB = logPos{}, logPos{}
	}
	if path := logbuf.Default.Path(); path != "" && btn("Log folder", ui.IconFolder, a.S(116)) {
		openFolder(filepath.Dir(path))
	}

	es, _ := logbuf.Default.Snapshot()
	minLevel := []logbuf.Level{logbuf.Info, logbuf.Warn, logbuf.Error}[a.logFilter]
	var shown []logbuf.Entry
	for _, e := range es {
		if e.Level >= minLevel {
			shown = append(shown, e)
		}
	}
	hasSel := a.logSelA != a.logSelB
	copyLabel := "Copy all"
	if hasSel {
		copyLabel = "Copy"
	}
	if btn(copyLabel, ui.IconCopy, a.S(104)) || hasSel && c.Ctrl && c.Key(rl.KeyC) && !c.Editing() {
		text := a.logSelection(shown)
		if !hasSel {
			text = logText(shown)
		}
		rl.SetClipboardText(text)
		a.toast("Copied to the clipboard", ui.ColGood)
	}

	// layout: monospace rows, long messages wrap under the message column
	list := ui.R(r.X, hy+a.S(36), r.Width, r.Height-a.S(44))
	textX := r.X + a.S(16)
	cw, _ := c.Measure(ui.TMono, "M")
	lh := a.S(18)
	cols := max(logbuf.LineMsgCol+16, int((list.Width-a.S(32))/cw))
	lines := make([][]rune, len(shown))
	var rows []logRow
	for i, e := range shown {
		ln := []rune(e.Line())
		lines[i] = ln
		start, indent := 0, 0
		for {
			n := cols - indent
			end := min(len(ln), start+n)
			if end < len(ln) { // break at a space when possible
				if sp := lastSpace(ln[start:end]); sp > n/2 {
					end = start + sp + 1
				}
			}
			rows = append(rows, logRow{entry: i, start: start, end: end, indent: indent})
			if end >= len(ln) {
				break
			}
			start, indent = end, logbuf.LineMsgCol
		}
	}

	content := float32(len(rows))*lh + a.S(8)
	sid := c.ID("logscroll")
	if a.logFollow {
		c.ScrollTo(sid, content) // clamped by BeginScroll, before the wheel is applied
	}
	listID := c.ID("loglist")
	hover, _ := c.Interact(listID, list)
	sa := c.BeginScroll(sid, list, content)
	maxOff := max(0, content-list.Height)
	top := list.Y + a.S(4)

	// mouse -> text position
	hit := func(mx, my float32) logPos {
		if len(rows) == 0 {
			return logPos{}
		}
		ri := int((my - top + sa.Offset) / lh)
		if ri < 0 {
			return logPos{shown[rows[0].entry].ID, 0}
		}
		if ri >= len(rows) {
			last := rows[len(rows)-1]
			return logPos{shown[last.entry].ID, last.end}
		}
		row := rows[ri]
		col := int(math.Round(float64((mx-textX)/cw))) - row.indent
		col = row.start + min(max(col, 0), row.end-row.start)
		return logPos{shown[row.entry].ID, col}
	}
	if hover {
		c.Cursor(rl.MouseCursorIBeam)
	}
	if c.Pressing(listID) {
		p := hit(c.Mouse.X, c.Mouse.Y)
		if c.Pressed {
			a.logSelA, a.logSelB = p, p
			if c.DoubleClick { // whole entry
				a.logSelA.col = 0
				for i := range shown {
					if shown[i].ID == p.id {
						a.logSelB.col = len(lines[i])
					}
				}
			}
		} else if !c.DoubleClick {
			a.logSelB = p
		}
		// drag past the edges scrolls
		if c.Mouse.Y < list.Y {
			sa.Offset = max(0, sa.Offset-lh/2)
		} else if c.Mouse.Y > list.Y+list.Height {
			sa.Offset = min(maxOff, sa.Offset+lh/2)
		}
	}
	if hover && c.Ctrl && c.Key(rl.KeyA) && !c.Editing() && len(rows) > 0 {
		last := rows[len(rows)-1]
		a.logSelA, a.logSelB = logPos{shown[0].ID, 0}, logPos{shown[last.entry].ID, last.end}
	}
	selA, selB := a.logSelA, a.logSelB
	if selB.less(selA) {
		selA, selB = selB, selA
	}

	first := max(0, int((sa.Offset-a.S(4))/lh))
	for ri := first; ri < len(rows); ri++ {
		row := rows[ri]
		y := top + float32(ri)*lh - sa.Offset
		if y > list.Y+list.Height {
			break
		}
		e := shown[row.entry]
		ln := lines[row.entry]
		x0 := textX + float32(row.indent)*cw
		// selection highlight
		if selA != selB {
			s, t := -1, -1
			for col := row.start; col <= row.end; col++ {
				p := logPos{e.ID, col}
				if !p.less(selA) && p.less(selB) {
					if s < 0 {
						s = col
					}
					t = col + 1
				}
			}
			if s >= 0 {
				w := float32(min(t, row.end)-s) * cw
				if t > row.end { // selection continues past this row
					w += cw * 0.6
				}
				c.RoundRect(ui.R(x0+float32(s-row.start)*cw, y, w, lh), a.S(2), ui.Fade(ui.ColAccent, 0.35))
			}
		}
		// text: time and level coloured, message plain
		seg := func(from, to int, col rl.Color) {
			from, to = max(from, row.start), min(to, row.end)
			if from < to {
				c.Text(ui.TMono, string(ln[from:to]), x0+float32(from-row.start)*cw, y+a.S(2), col)
			}
		}
		seg(0, 12, ui.ColTextFaint)
		seg(14, 19, levelColor(e.Level))
		seg(logbuf.LineMsgCol, len(ln), ui.ColText)
	}
	c.EndScroll(sa)
	a.logFollow = sa.Offset >= maxOff-a.S(2) && !c.Pressing(listID)
}

func lastSpace(rs []rune) int {
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i] == ' ' {
			return i
		}
	}
	return -1
}

func logText(es []logbuf.Entry) string {
	var b strings.Builder
	for _, e := range es {
		b.WriteString(e.Line())
		b.WriteByte('\n')
	}
	return b.String()
}

// logSelection returns the selected text of the shown entries.
func (a *App) logSelection(shown []logbuf.Entry) string {
	selA, selB := a.logSelA, a.logSelB
	if selB.less(selA) {
		selA, selB = selB, selA
	}
	var b strings.Builder
	for _, e := range shown {
		if e.ID < selA.id || e.ID > selB.id {
			continue
		}
		ln := []rune(e.Line())
		s, t := 0, len(ln)
		if e.ID == selA.id {
			s = min(selA.col, len(ln))
		}
		if e.ID == selB.id {
			t = min(selB.col, len(ln))
		}
		if s < t {
			b.WriteString(string(ln[s:t]))
		}
		if e.ID != selB.id {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// openFolder shows a directory in the system file manager.
func openFolder(dir string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		logbuf.Warnf("Could not open %s: %v", dir, err)
		return
	}
	go func() { _ = cmd.Wait() }()
}
