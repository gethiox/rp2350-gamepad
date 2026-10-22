package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/logbuf"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/model"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/settings"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/ui"
)

var scaleChoices = []float32{0, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2}

func scaleLabel(s float32) string {
	if s == 0 {
		return "Automatic"
	}
	return fmt.Sprintf("%.0f %%", s*100)
}

func (a *App) dialogs() {
	if a.dialog == dlgNone {
		return
	}
	c := a.ui
	c.Defer(ui.LayerModal, func() {
		switch a.dialog {
		case dlgConfirm:
			a.confirmDialog()
		case dlgSettings:
			a.settingsDialog()
		case dlgImport:
			a.importDialog()
		case dlgAbout:
			a.aboutDialog()
		}
	})
	_ = c
}

func (a *App) confirmDialog() {
	c := a.ui
	cs := &a.confirm
	w := a.S(460)
	bodyH := c.ParagraphHeight(ui.TBody, cs.body, w-a.S(40))
	r, closed := c.Modal(c.ID("confirm"), cs.title, w, a.S(58)+bodyH+a.S(24)+a.S(36)+a.S(20))
	c.Paragraph(ui.TBody, cs.body, r.X, r.Y, r.Width, ui.ColTextDim)
	by := r.Y + r.Height - a.S(36)
	kind := ui.BtnPrimary
	if cs.danger {
		kind = ui.BtnDanger
	}
	aw, _ := c.Measure(ui.TBodyB, cs.action)
	ab := ui.R(r.X+r.Width-aw-a.S(36), by, aw+a.S(36), a.S(36))
	if c.Button(c.ID("confirm", "yes"), ab, cs.action, kind, false) {
		a.dialog = dlgNone
		if cs.onYes != nil {
			cs.onYes()
		}
		return
	}
	if c.Button(c.ID("confirm", "no"), ui.R(ab.X-a.S(110), by, a.S(100), a.S(36)), "Cancel", ui.BtnSecondary, false) || closed {
		a.dialog = dlgNone
	}
}

var fpsChoices = []int{30, 60, 75, 90, 120, 144, 165, 240, 0}

func (a *App) settingsDialog() {
	c := a.ui
	r, closed := c.Modal(c.ID("settings"), "Settings", a.S(560), a.S(550))
	if closed {
		a.dialog = dlgNone
		return
	}
	f := form{a: a, x: r.X - a.S(20), y: r.Y, w: r.Width + a.S(20), labelW: a.S(170)}

	// UI scale
	sel := 0
	for i, s := range scaleChoices {
		if s == a.set.UIScale {
			sel = i
		}
	}
	opts := make([]ui.Option, len(scaleChoices))
	for i, s := range scaleChoices {
		opts[i] = ui.Option{Label: scaleLabel(s)}
	}
	f.row(c.ID("set", "scale"), "Interface size", "Makes all text and controls larger or smaller. Automatic follows the display scaling.",
		false, "", nil, "", a.S(180), func(rr rl.Rectangle) {
			c.Dropdown(c.ID("set", "scale", "dd"), rr, opts, sel, ui.FieldState{}, func(i int) {
				a.set.UIScale = scaleChoices[i]
				a.saveSettings()
			})
		})

	// frame rate
	fpsSel := 0
	fpsOpts := make([]ui.Option, len(fpsChoices))
	for i, v := range fpsChoices {
		fpsOpts[i] = ui.Option{Label: fmt.Sprintf("%d FPS", v)}
		if v == 0 {
			fpsOpts[i].Label = "Display refresh rate"
		}
		if v == a.set.MaxFPS {
			fpsSel = i
		}
	}
	f.row(c.ID("set", "fps"), "Frame rate", "Upper limit for how often the window redraws. Higher is smoother on fast displays and uses a bit more CPU.",
		false, "", nil, "", a.S(220), func(rr rl.Rectangle) {
			c.Dropdown(c.ID("set", "fps", "dd"), rr, fpsOpts, fpsSel, ui.FieldState{}, func(i int) {
				a.set.MaxFPS = fpsChoices[i]
				a.saveSettings()
			})
		})

	// live preview
	lp := a.set.LivePreview
	f.row(c.ID("set", "preview"), "Live LED preview", "Shows unsaved colour and brightness changes on the device's LEDs while you edit. Inputs keep using the saved configuration.",
		false, "", nil, "", 0, func(rr rl.Rectangle) {
			if c.Switch(c.ID("set", "preview", "sw"), rr, &lp, ui.FieldState{}) {
				a.set.LivePreview = lp
				a.saveSettings()
			}
		})

	// device ids
	f.row(c.ID("set", "ids"), "Devices to look for", "USB vendor:product IDs. A new ID is added automatically when you save one in the configuration.",
		false, "", nil, a.newIDErr, 0, func(rr rl.Rectangle) {
			x := rr.X
			for i, id := range a.set.DeviceIDs {
				label := id.String()
				tw, _ := c.Measure(ui.TMono, label)
				chip := ui.R(x, rr.Y, tw+a.S(40), rr.Height)
				c.RoundRect(chip, a.S(6), ui.ColField)
				c.TextIn(ui.TMono, label, ui.R(chip.X+a.S(10), chip.Y, tw, chip.Height), 0, ui.ColText)
				if len(a.set.DeviceIDs) > 1 &&
					c.IconButton(c.ID("set", "rmid", i), ui.R(chip.X+chip.Width-a.S(26), chip.Y+a.S(5), a.S(20), chip.Height-a.S(10)), ui.IconCross, "", ui.BtnGhost, false) {
					a.set.DeviceIDs = append(a.set.DeviceIDs[:i:i], a.set.DeviceIDs[i+1:]...)
					a.saveSettings()
					a.mgr.SetDeviceIDs(a.set.DeviceIDs)
					return
				}
				x += chip.Width + a.S(6)
			}
		})
	f.y -= a.S(4)
	f.row(c.ID("set", "addid"), "", "", false, "", nil, "", a.S(260), func(rr rl.Rectangle) {
		s, done := c.TextField(c.ID("set", "newid"), ui.R(rr.X, rr.Y, a.S(150), rr.Height), a.newID,
			ui.TextOpts{MaxLen: 9, Mono: true, Placeholder: "VID:PID", Filter: func(r rune) bool { return strings.ContainsRune("0123456789abcdefABCDEF:", r) }},
			ui.FieldState{Error: a.newIDErr != ""})
		if done {
			a.newID = s
		}
		if c.IconButton(c.ID("set", "addidbtn"), ui.R(rr.X+a.S(160), rr.Y, a.S(90), rr.Height), ui.IconPlus, "Add", ui.BtnSecondary, a.newID == "") {
			id, err := settings.ParseDeviceID(a.newID)
			if err != nil {
				a.newIDErr = err.Error()
			} else {
				a.newIDErr = ""
				a.newID = ""
				a.set.AddDeviceID(id)
				a.saveSettings()
				a.mgr.SetDeviceIDs(a.set.DeviceIDs)
			}
		}
	})

	// backup folder
	f.row(c.ID("set", "backup"), "Backup folder", "Where Export saves files and Import looks for them.", false, "", nil, "", 0, func(rr rl.Rectangle) {
		s, done := c.TextField(c.ID("set", "backupdir"), ui.R(rr.X, rr.Y, rr.Width-a.S(44), rr.Height), a.backupDir, ui.TextOpts{MaxLen: 400}, ui.FieldState{})
		if done && s != "" {
			a.backupDir = s
			a.set.BackupDir = s
			a.saveSettings()
		}
		if c.IconButton(c.ID("set", "openbackup"), ui.R(rr.X+rr.Width-a.S(36), rr.Y, a.S(36), rr.Height), ui.IconFolder, "", ui.BtnSecondary, false) {
			_ = os.MkdirAll(a.backupDir, 0o755)
			openFolder(a.backupDir)
		}
	})

	if p := a.set.Path(); p != "" {
		c.Paragraph(ui.TSmall, "Settings file: "+p, r.X, r.Y+r.Height-a.S(16), r.Width, ui.ColTextFaint)
	}
}

func (a *App) importDialog() {
	c := a.ui
	r, closed := c.Modal(c.ID("import"), "Import backup", a.S(600), a.S(480))
	if closed {
		a.dialog = dlgNone
		return
	}
	if a.importList == nil {
		a.importList = model.ListBackups(a.backupDir)
		if a.importList == nil {
			a.importList = []string{}
		}
	}
	y := r.Y
	y += c.Paragraph(ui.TBody, "Pick a backup from "+a.backupDir+" or drag any backup .json file onto this window. "+
		"The configuration is loaded as unsaved changes so you can review it before saving.", r.X, y, r.Width, ui.ColTextDim) + a.S(12)
	list := ui.R(r.X, y, r.Width, r.Y+r.Height-y)
	c.RoundRect(list, a.S(8), ui.ColBg)
	if len(a.importList) == 0 {
		c.TextIn(ui.TBody, "No backups found in this folder.", list, 0.5, ui.ColTextFaint)
		return
	}
	rowH := a.S(44)
	sa := c.BeginScroll(c.ID("import", "scroll"), list, float32(len(a.importList))*rowH+a.S(8))
	yy := list.Y + a.S(4) - sa.Offset
	for i, p := range a.importList {
		ir := ui.R(list.X+a.S(4), yy, list.Width-a.S(8)-sa.BarW, rowH-a.S(4))
		yy += rowH
		hover, clicked := c.Interact(c.ID("import", "item", i), ir)
		if hover {
			c.RoundRect(ir, a.S(6), ui.ColFieldHov)
		}
		c.Text(ui.TBodyB, filepath.Base(p), ir.X+a.S(12), ir.Y+a.S(5), ui.ColText)
		if st, err := os.Stat(p); err == nil {
			c.Text(ui.TSmall, st.ModTime().Format("2006-01-02 15:04")+"  ·  "+humanAge(st.ModTime()), ir.X+a.S(12), ir.Y+a.S(23), ui.ColTextFaint)
		}
		if clicked {
			a.importBackup(p)
		}
	}
	c.EndScroll(sa)
}

func humanAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}

func (a *App) aboutDialog() {
	c := a.ui
	r, closed := c.Modal(c.ID("about"), "About", a.S(480), a.S(330))
	if closed {
		a.dialog = dlgNone
		return
	}
	y := r.Y
	c.DrawIcon(ui.IconStick, r.X+a.S(18), y+a.S(16), a.S(30), ui.ColAccent)
	c.Text(ui.TTitle, "rp2350-gamepad configurator", r.X+a.S(44), y+a.S(2), ui.ColText)
	c.Text(ui.TBody, "Version "+Version, r.X+a.S(44), y+a.S(28), ui.ColTextDim)
	y += a.S(66)
	text := "Configures RP2350 based gamepads, joysticks and HOTAS extensions running the rp2350-gamepad firmware.\n\n" +
		"MIT licensed. Fonts: Dosis and Space Mono (SIL Open Font License).\n" +
		"github.com/gethiox/rp2350-gamepad"
	y += c.Paragraph(ui.TBody, text, r.X, y, r.Width, ui.ColTextDim) + a.S(12)
	if a.ready() {
		c.Paragraph(ui.TSmall, fmt.Sprintf("Connected firmware %s · protocol %d", a.snap.Hello.FwVersion, a.snap.Hello.ProtoVersion),
			r.X, y, r.Width, ui.ColTextFaint)
	}
	if p := logbuf.Default.Path(); p != "" {
		c.Paragraph(ui.TSmall, "Log file: "+p, r.X, r.Y+r.Height-a.S(16), r.Width, ui.ColTextFaint)
	}
}

func (a *App) drawToasts(topH float32) {
	c := a.ui
	now := c.Time
	keep := a.toasts[:0]
	for _, t := range a.toasts {
		if t.until > now {
			keep = append(keep, t)
		}
	}
	a.toasts = keep
	if len(a.toasts) == 0 {
		return
	}
	c.Defer(ui.LayerTop, func() {
		y := topH + a.S(12)
		for i := len(a.toasts) - 1; i >= 0; i-- {
			t := a.toasts[i]
			w := min(a.S(460), c.W-a.S(40))
			lines := c.Wrap(ui.TBody, t.text, w-a.S(48))
			h := float32(len(lines))*a.S(20) + a.S(20)
			r := ui.R(c.W-w-a.S(16), y, w, h)
			alpha := clampf(float32(t.until-now)/0.4, 0, 1)
			c.Shadow(r, a.S(8))
			c.RoundRect(r, a.S(8), ui.Fade(ui.ColPanelHi, alpha))
			c.RoundRectLine(r, a.S(8), 1, ui.Fade(t.col, 0.6*alpha))
			c.Circle(r.X+a.S(18), r.Y+a.S(20), a.S(4), ui.Fade(t.col, alpha))
			for k, l := range lines {
				c.Text(ui.TBody, l, r.X+a.S(32), r.Y+a.S(10)+float32(k)*a.S(20), ui.Fade(ui.ColText, alpha))
			}
			if hover, clicked := c.Interact(c.ID("toast", i), r); hover && clicked {
				a.toasts[i].until = now
			}
			y += h + a.S(8)
		}
	})
}
