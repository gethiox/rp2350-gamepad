package app

import (
	"fmt"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/device"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/ui"
)

// --- top bar -----------------------------------------------------------------

func (a *App) topBar(r rl.Rectangle) {
	c := a.ui
	rl.DrawRectangleRec(r, ui.ColPanel)
	c.Divider(0, r.Y+r.Height-1, r.Width)

	x := r.X + a.S(18)
	cy := r.Y + r.Height/2
	c.DrawIcon(ui.IconStick, x+a.S(10), cy, a.S(22), ui.ColAccent)
	x += a.S(30)
	c.TextIn(ui.TTitle, "rp2350-gamepad", ui.R(x, r.Y, 300, r.Height), 0, ui.ColText)
	tw, _ := c.Measure(ui.TTitle, "rp2350-gamepad")
	x += tw + a.S(18)

	// device chip
	dot, label := a.connSummary()
	chipH := a.S(30)
	chip := ui.R(x, cy-chipH/2, 0, chipH)
	lw, _ := c.Measure(ui.TBody, label)
	chip.Width = lw + a.S(34)
	if len(a.snap.Candidates) > 1 {
		chip.Width += a.S(18)
	}
	c.RoundRect(chip, chipH/2, ui.ColField)
	c.Circle(chip.X+a.S(14), cy, a.S(4.5), dot)
	c.TextIn(ui.TBody, label, ui.R(chip.X+a.S(26), chip.Y, lw, chipH), 0, ui.ColText)
	if len(a.snap.Candidates) > 1 {
		// several devices: pick one
		opts := make([]ui.Option, len(a.snap.Candidates))
		sel := -1
		for i, cd := range a.snap.Candidates {
			opts[i] = ui.Option{Label: cd.Label}
			if cd.Port == a.snap.Port {
				sel = i
			}
		}
		dr := ui.R(chip.X+chip.Width-a.S(30), chip.Y+a.S(3), a.S(26), chipH-a.S(6))
		c.Dropdown(c.ID("devpick"), dr, opts, sel, ui.FieldState{}, func(i int) {
			a.mgr.SelectPort(a.snap.Candidates[i].Port)
		})
	}
	if a.snap.Simulator {
		c.Badge(chip.X+chip.Width+a.S(10), cy-a.S(11), "SIMULATOR", ui.ColCal)
	}

	// right side: menu, save, revert
	rx := r.X + r.Width - a.S(14)
	mb := ui.R(rx-a.S(36), cy-a.S(18), a.S(36), a.S(36))
	a.mainMenu(mb)
	rx = mb.X - a.S(10)

	n := a.ed.Changes()
	saveLabel := "Save"
	if n > 0 {
		saveLabel = "Save " + plural(n, "change")
	}
	if a.busy() {
		saveLabel = a.snap.Busy
	}
	sw, _ := c.Measure(ui.TBodyB, saveLabel)
	sb := ui.R(rx-sw-a.S(48), cy-a.S(17), sw+a.S(48), a.S(34))
	if c.IconButton(c.ID("save"), sb, ui.IconSave, saveLabel, ui.BtnPrimary, !a.canSave()) {
		a.save()
	}
	sid := c.ID("save")
	if c.MouseIn(sb) {
		switch {
		case a.calibrating():
			c.Tip(sid, "Finish or cancel calibration before saving.")
		case len(a.errs) > 0:
			c.Tip(sid, "The device reports problems with these settings - see the red fields.")
		case a.ed.Dirty():
			c.Tip(sid, "Write the changes to the device (Ctrl+S). The device restarts to apply them.")
		default:
			c.Tip(sid, "No unsaved changes.")
		}
	}
	rx = sb.X - a.S(8)
	if a.ed.Dirty() && !a.busy() {
		rb := ui.R(rx-a.S(92), cy-a.S(17), a.S(92), a.S(34))
		if c.IconButton(c.ID("revert"), rb, ui.IconUndo, "Revert", ui.BtnSecondary, false) {
			a.ask("Discard unsaved changes?", "Your "+plural(n, "unsaved change")+" will be lost. The device is not affected.",
				"Discard changes", true, func() { a.ed.Discard() })
		}
		rx = rb.X - a.S(12)
		if len(a.errs) > 0 {
			label := plural(len(a.errs), "problem")
			c.Badge(rx-c.BadgeWidth(label), cy-a.S(11), label, ui.ColBad)
		}
	}
}

func (a *App) connSummary() (rl.Color, string) {
	s := a.snap
	switch s.State {
	case device.Ready:
		name := a.snap.Config.UsbProduct.String()
		return ui.ColGood, fmt.Sprintf("%s  ·  firmware %s", name, s.Hello.FwVersion)
	case device.Connecting:
		return ui.ColWarn, "Connecting to " + s.Port + "…"
	case device.Rebooting:
		return ui.ColWarn, "Device restarting…"
	case device.Incompatible:
		return ui.ColBad, "Incompatible firmware"
	}
	return ui.ColTextFaint, "No device"
}

func (a *App) mainMenu(r rl.Rectangle) {
	c := a.ui
	ready := a.ready() && !a.busy()
	items := []ui.MenuItem{
		{Label: "Export backup…", Icon: ui.IconSave, Disabled: !ready},
		{Label: "Import backup…", Icon: ui.IconFolder, Disabled: !ready},
		{Label: "Reset calibration…", Icon: ui.IconUndo, Disabled: !ready || a.calibrating(), Divider: true},
		{Label: "Factory reset…", Icon: ui.IconWarn, Danger: true, Disabled: !ready},
		{Label: "Settings…", Icon: ui.IconGear, Divider: true},
		{Label: "About", Icon: ui.IconInfo},
	}
	c.Menu(c.ID("mainmenu"), r, ui.IconMenu, "", items, func(i int) {
		switch i {
		case 0:
			a.exportBackup()
		case 1:
			a.dialog = dlgImport
			a.importList = nil
		case 2:
			a.ask("Reset calibration?", "All axes and sticks go back to uncalibrated defaults. Configuration is kept. You can calibrate again at any time.",
				"Reset calibration", true, func() { a.mgr.Do(device.Op{Kind: device.OpResetCalibration}) })
		case 3:
			a.ask("Factory reset?", "Erases the saved configuration AND calibration on the device and restarts it with defaults. "+
				"Unsaved changes in this window are discarded too. Consider exporting a backup first.",
				"Erase everything", true, func() { a.mgr.Do(device.Op{Kind: device.OpFactoryReset}) })
		case 4:
			a.dialog = dlgSettings
		case 5:
			a.dialog = dlgAbout
		}
	})
}

// --- main view ---------------------------------------------------------------

type section struct {
	key, title string
	draw       func(x, y, w float32) float32
	count      func() (mod int, errs int)
}

func (a *App) sections() []section {
	return []section{
		{"axes", "Axes", a.sectionAxes, a.countAxes},
		{"sticks", "Sticks", a.sectionSticks, a.countSticks},
		{"buttons", "Buttons", a.sectionButtons, a.countButtons},
		{"leds", "LEDs", a.sectionLeds, a.countLeds},
		{"input", "Input processing", a.sectionProcessing, a.countProcessing},
		{"usb", "Device & USB", a.sectionDevice, a.countDevice},
	}
}

func (a *App) mainView(r rl.Rectangle) {
	c := a.ui
	liveW := clampf(r.Width*0.31, a.S(330), a.S(460))
	navW := float32(0)
	if r.Width-liveW > a.S(860) {
		navW = a.S(176)
	}
	m := a.S(14)
	content := ui.R(r.X+navW+m, r.Y, r.Width-navW-liveW-m*2, r.Height)
	live := ui.R(r.X+r.Width-liveW, r.Y, liveW, r.Height)

	secs := a.sections()
	if navW > 0 {
		a.nav(ui.R(r.X, r.Y, navW, r.Height), secs)
	}

	// content scroll area
	sid := c.ID("content")
	sa := c.BeginScroll(sid, content, a.contentH)
	a.contentOff = sa.Offset
	if a.navTarget != "" {
		if y, ok := a.sectionY[a.navTarget]; ok {
			sa.Offset = clampf(y-a.S(12), 0, max(0, a.contentH-content.Height))
		}
		a.navTarget = ""
	}
	y := content.Y + a.S(14) - sa.Offset
	start := y
	w := content.Width - a.S(12)
	if a.snap.Hello.Flags&proto.HelloSafeMode != 0 {
		y += a.banner(content.X, y, w, ui.ColBad, "Safe mode",
			"The device restarted repeatedly and is running with default settings to stay usable. "+
				"Save a configuration (or factory reset) to leave safe mode.") + a.S(12)
	}
	if a.calibrating() {
		y += a.banner(content.X, y, w, ui.ColCal, "Calibration in progress",
			"Move every axis through its full range. For each stick: push it UP, then rotate it CLOCKWISE "+
				"along the edge for at least one full turn. Then press Finish in the panel on the right.") + a.S(12)
	}
	for _, s := range secs {
		a.sectionY[s.key] = y - start
		y += s.draw(content.X, y, w) + a.S(18)
	}
	a.contentH = y - start + a.S(40)
	c.EndScroll(sa)

	a.livePanel(live)
}

func (a *App) nav(r rl.Rectangle, secs []section) {
	c := a.ui
	y := r.Y + a.S(16)
	active := secs[0].key
	for _, s := range secs {
		if sy, ok := a.sectionY[s.key]; ok && sy <= a.contentOff+a.S(80) {
			active = s.key
		}
	}
	for _, s := range secs {
		ir := ui.R(r.X+a.S(10), y, r.Width-a.S(16), a.S(34))
		hover, clicked := c.Interact(c.ID("nav", s.key), ir)
		col := ui.ColTextDim
		if s.key == active {
			c.RoundRect(ir, a.S(6), ui.ColField)
			rl.DrawRectangleRec(ui.R(ir.X, ir.Y+a.S(8), a.S(3), ir.Height-a.S(16)), ui.ColAccent)
			col = ui.ColText
		} else if hover {
			c.RoundRect(ir, a.S(6), ui.ColFieldHov)
		}
		if clicked {
			a.navTarget = s.key
		}
		c.TextIn(ui.TBody, s.title, ui.R(ir.X+a.S(14), ir.Y, ir.Width, ir.Height), 0, col)
		mod, errs := s.count()
		bx := ir.X + ir.Width - a.S(10)
		if errs > 0 {
			c.DrawIcon(ui.IconWarn, bx-a.S(4), ir.Y+ir.Height/2, a.S(13), ui.ColBad)
			bx -= a.S(18)
		}
		if mod > 0 {
			label := fmt.Sprint(mod)
			tw, _ := c.Measure(ui.TSmall, label)
			pr := ui.R(bx-tw-a.S(12), ir.Y+a.S(8), tw+a.S(12), ir.Height-a.S(16))
			c.RoundRect(pr, pr.Height/2, ui.Fade(ui.ColModified, 0.2))
			c.TextIn(ui.TSmall, label, pr, 0.5, ui.ColModified)
		}
		y += a.S(38)
	}
}

// banner draws a coloured notice and returns its height.
func (a *App) banner(x, y, w float32, col rl.Color, title, body string) float32 {
	c := a.ui
	tw := w - a.S(56)
	h := a.S(16) + a.S(22) + c.ParagraphHeight(ui.TBody, body, tw) + a.S(12)
	r := ui.R(x, y, w, h)
	c.RoundRect(r, a.S(ui.RadiusBox), ui.Fade(col, 0.1))
	c.RoundRectLine(r, a.S(ui.RadiusBox), 1, ui.Fade(col, 0.45))
	c.DrawIcon(ui.IconInfo, x+a.S(24), y+a.S(26), a.S(18), col)
	c.Text(ui.TBodyB, title, x+a.S(44), y+a.S(16), col)
	c.Paragraph(ui.TBody, body, x+a.S(44), y+a.S(40), tw, ui.ColText)
	return h
}

// --- not connected -------------------------------------------------------------

func (a *App) connectView(r rl.Rectangle) {
	c := a.ui
	w := min(a.S(560), r.Width-a.S(40))
	x := r.X + (r.Width-w)/2
	y := r.Y + r.Height*0.22
	s := a.snap
	var title, body string
	col := ui.ColTextDim
	switch s.State {
	case device.Incompatible:
		title, body, col = "This firmware is not compatible", s.Incompatible, ui.ColBad
	case device.Rebooting:
		title, body = "Restarting the device…", "Waiting for it to reconnect."
	case device.Connecting:
		title, body = "Connecting…", "Talking to "+s.Port+"."
	default:
		ids := make([]string, len(a.set.DeviceIDs))
		for i, id := range a.set.DeviceIDs {
			ids[i] = id.String()
		}
		title = "Looking for your gamepad…"
		body = "Connect it with a USB cable - it is picked up automatically. " +
			"The configurator looks for USB IDs " + strings.Join(ids, ", ") + " (change them in Settings)."
		if s.LastError != "" {
			body += "\n\n" + s.LastError
		}
		if s.Permission {
			body += "\n\nLinux: your user may not access serial devices yet. Install the udev rule shipped with the release " +
				"(see docs/linux.md) and plug the device in again."
		}
	}
	if s.State == device.Rebooting || s.State == device.Connecting || s.State == device.Disconnected {
		c.Spinner(x+w/2, y-a.S(36), a.S(16), ui.ColAccent)
	} else {
		c.DrawIcon(ui.IconWarn, x+w/2, y-a.S(36), a.S(30), col)
	}
	tw, _ := c.Measure(ui.TBig, title)
	c.Text(ui.TBig, title, x+(w-tw)/2, y, ui.ColText)
	y += a.S(46)
	y += c.Paragraph(ui.TBody, body, x, y, w, ui.ColTextDim) + a.S(18)
	if s.State == device.Incompatible {
		if c.Button(c.ID("retry"), ui.R(x+w/2-a.S(70), y, a.S(140), a.S(34)), "Try again", ui.BtnSecondary, false) {
			a.mgr.Retry()
		}
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
