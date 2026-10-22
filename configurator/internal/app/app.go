// Package app is the configurator's user interface.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/device"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/logbuf"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/model"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/settings"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/ui"
)

// Version is set at build time (-ldflags "-X ...app.Version=v1.2.3").
var Version = "dev"

const (
	minW, minH = 1024, 640 // minimum window size at 100 % scale
)

type dialogKind int

const (
	dlgNone dialogKind = iota
	dlgSettings
	dlgConfirm
	dlgImport
	dlgAbout
)

type toast struct {
	text  string
	col   rl.Color
	until float64
}

type confirmState struct {
	title, body, action string
	danger              bool
	onYes               func()
}

type App struct {
	ui  *ui.Ctx
	mgr *device.Manager
	set *settings.Settings

	ed       model.Editor
	snap     device.Snapshot
	chipID   [8]uint8
	editedAt float64 // last time the draft changed (validation debounce)
	lastHash uint64
	sentHash uint64

	// latest device verdict on the draft
	errs     []proto.ErrorEntry
	errsHash uint64
	errsDone bool

	discardOnNextConfig bool // factory reset: drop edits once defaults arrive

	// UI state
	dialog     dialogKind
	confirm    confirmState
	toasts     []toast
	logOpen    bool
	logH       float32 // logical units
	logDrag    bool
	logFilter  int
	logSelA    logPos // selection anchor in the log view
	logSelB    logPos // selection end
	logFollow  bool
	logSeen    uint64
	navTarget  string
	sectionY   map[string]float32
	formH      map[string]float32
	contentH   float32
	contentOff float32
	demoSteps  []string
	demoAt     float64
	demoWait   float64
	liveH      float32
	lastCal    *proto.CalResult
	lastCalT   float64
	sticks     [proto.MaxGroups]stickTrail
	importList []string
	scaleSel   int
	newID      string
	newIDErr   string
	backupDir  string
}

func New(mgr *device.Manager, set *settings.Settings, ctx *ui.Ctx) *App {
	a := &App{ui: ctx, mgr: mgr, set: set, logH: 220, logFollow: true, sectionY: map[string]float32{}, formH: map[string]float32{}}
	a.backupDir = set.BackupDir
	if a.backupDir == "" {
		a.backupDir = model.DefaultBackupDir()
	}
	return a
}

func (a *App) S(v float32) float32 { return a.ui.S(v) }

// --- per-frame state -----------------------------------------------------

func (a *App) syncState() {
	// operation results
	for {
		select {
		case r := <-a.mgr.Results():
			a.onResult(r)
			continue
		default:
		}
		break
	}
	a.snap = a.mgr.Snapshot()
	if a.snap.State == device.Ready {
		if a.snap.Hello.ChipID != a.chipID && a.ed.Loaded() {
			// a different physical device: its configuration is unrelated
			if a.ed.Dirty() {
				logbuf.Warnf("A different device was connected; unsaved changes were discarded.")
			}
			a.ed.Reset()
		}
		a.chipID = a.snap.Hello.ChipID
		gen := a.snap.ConfigGen
		a.ed.Sync(a.snap.Config, gen)
		if a.discardOnNextConfig && a.snap.Busy == "" {
			a.ed.Discard()
			a.discardOnNextConfig = false
		}
	}

	h := a.ed.Hash()
	if h != a.lastHash {
		a.lastHash = h
		a.editedAt = a.ui.Time
	}
	// device verdict
	v := a.snap.Validation
	if v.Done && v.DraftHash == h {
		a.errs, a.errsHash, a.errsDone = v.Errors, v.DraftHash, true
	}
	if !a.ed.Dirty() {
		a.errs, a.errsHash, a.errsDone = nil, h, true
	}
	// ask the device to validate the draft (debounced)
	if a.snap.State == device.Ready && a.ed.Dirty() && h != a.sentHash && a.ui.Time-a.editedAt > 0.12 {
		a.mgr.Validate(a.ed.Draft, h)
		a.sentHash = h
	}
	// live LED preview of unsaved LED changes
	// The device numbers LEDs by its saved layout, so the draft's masks are
	// translated to it.
	pv := a.ed.Draft
	model.RemapMasks(&pv, model.LayoutOf(&a.ed.Draft), model.LayoutOf(&a.ed.Base))
	if a.snap.State == device.Ready && a.set.LivePreview && !a.calibrating() &&
		model.Look(&pv) != model.Look(&a.ed.Base) {
		a.mgr.SetPreview(&pv)
	} else {
		a.mgr.SetPreview(nil)
	}
}

func (a *App) onResult(r device.Result) {
	switch {
	case r.OK && r.Kind == device.OpCalFinish:
		a.lastCal, a.lastCalT = r.CalResult, a.ui.Time
		a.toast("Calibration saved", ui.ColGood)
	case r.OK && r.Kind == device.OpWriteConfig:
		a.toast("Configuration saved and verified", ui.ColGood)
	case r.OK && r.Kind == device.OpFactoryReset:
		a.toast("Factory reset done", ui.ColGood)
		a.discardOnNextConfig = true
	case r.OK && (r.Kind == device.OpCalCancel || r.Kind == device.OpCalStart):
	case r.OK:
		a.toast(strings.ToUpper(r.Message[:1])+r.Message[1:], ui.ColGood)
	default:
		msg := r.Kind.String() + " failed: " + r.Message
		if len(r.Errors) > 0 {
			msg += " (" + r.Errors[0].Text() + ")"
		}
		a.toast(msg, ui.ColBad)
		if r.Kind == device.OpWriteConfig && len(r.Errors) > 0 {
			a.errs, a.errsHash, a.errsDone = r.Errors, a.ed.Hash(), true
		}
	}
}

func (a *App) toast(text string, col rl.Color) {
	a.toasts = append(a.toasts, toast{text: text, col: col, until: a.ui.Time + 4.5})
	if len(a.toasts) > 4 {
		a.toasts = a.toasts[len(a.toasts)-4:]
	}
}

func (a *App) ready() bool { return a.snap.State == device.Ready }
func (a *App) calibrating() bool {
	return a.ready() && a.snap.Telemetry.State&proto.StateCalibrating != 0
}
func (a *App) busy() bool { return a.snap.Busy != "" }

// errFor returns the device's message for a field (index 0xFF = global).
func (a *App) errFor(field uint8, index int) string {
	for _, e := range a.errs {
		if e.Field == field && (int(e.Index) == index || e.Index == 0xFF && index < 0) {
			return e.Text()
		}
	}
	return ""
}

func (a *App) canSave() bool {
	return a.ready() && a.ed.Dirty() && len(a.errs) == 0 && !a.busy() && !a.calibrating()
}

func (a *App) save() {
	if !a.canSave() {
		return
	}
	d := a.ed.Draft
	// Make sure the configurator will find the device again if its USB ID changes.
	id := settings.DeviceID{VID: d.UsbVid, PID: d.UsbPid}
	if a.set.AddDeviceID(id) {
		logbuf.Infof("Added %s to the USB IDs the configurator looks for.", id)
		a.saveSettings()
		a.mgr.SetDeviceIDs(a.set.DeviceIDs)
	}
	logbuf.Infof("Saving %s…", plural(a.ed.Changes(), "change"))
	a.mgr.Do(device.Op{Kind: device.OpWriteConfig, Config: d})
}

func (a *App) saveSettings() {
	if err := a.set.Save(); err != nil {
		logbuf.Warnf("Could not save settings: %v", err)
	}
}

// --- main loop -----------------------------------------------------------

// Frame renders one frame. Returns false when the app should quit.
func (a *App) Frame() bool {
	c := a.ui
	a.applyScale()
	c.SetModal(a.dialog != dlgNone)
	c.Begin()
	a.syncState()
	a.runDemo()
	a.handleDrops()

	rl.BeginDrawing()
	rl.ClearBackground(ui.ColBg)

	topH := a.S(56)
	statusH := a.S(30)
	logH := float32(0)
	if a.logOpen {
		logH = clampf(a.S(a.logH), a.S(120), c.H*0.6)
	}
	main := ui.R(0, topH, c.W, c.H-topH-statusH-logH)

	a.topBar(ui.R(0, 0, c.W, topH))
	if a.ready() || (a.snap.State == device.Rebooting && a.ed.Loaded()) {
		a.mainView(main)
	} else {
		a.connectView(main)
	}
	// the status bar sits on top of the log drawer and moves up with it
	a.statusBar(ui.R(0, c.H-statusH-logH, c.W, statusH))
	a.logDrawer(ui.R(0, c.H-logH, c.W, logH))
	a.dialogs()
	a.drawToasts(topH)

	// keyboard shortcuts
	if c.Ctrl && c.Key(rl.KeyS) && !c.Editing() {
		a.save()
	}

	c.End()
	rl.EndDrawing()
	return !rl.WindowShouldClose()
}

func (a *App) applyScale() {
	s := a.set.UIScale
	if s == 0 {
		dpi := rl.GetWindowScaleDPI()
		s = dpi.X
		if s < 1 {
			s = 1
		}
	}
	if s != a.ui.Scale {
		a.ui.SetScale(s)
		rl.SetWindowMinSize(int(minW*s), int(minH*s))
	}
}

func (a *App) handleDrops() {
	if !rl.IsFileDropped() {
		return
	}
	files := rl.LoadDroppedFiles()
	rl.UnloadDroppedFiles()
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".json") {
			a.importBackup(f)
			return
		}
	}
	a.toast("Drop a .json backup file to import it", ui.ColWarn)
}

// --- backups ---------------------------------------------------------------

func (a *App) exportBackup() {
	if !a.ready() {
		return
	}
	cal := a.snap.Calibration
	b := model.NewBackup(a.ed.Draft, &cal, a.snap.Hello)
	data, err := b.Marshal()
	if err == nil {
		err = os.MkdirAll(a.backupDir, 0o755)
	}
	name := fmt.Sprintf("%s-%s.json", model.SafeName(a.ed.Draft.UsbProduct.String()), time.Now().Format("2006-01-02-150405"))
	path := filepath.Join(a.backupDir, name)
	if err == nil {
		err = os.WriteFile(path, data, 0o644)
	}
	if err != nil {
		logbuf.Errorf("Export failed: %v", err)
		a.toast("Export failed: "+err.Error(), ui.ColBad)
		return
	}
	note := ""
	if a.ed.Dirty() {
		note = " (including unsaved changes)"
	}
	logbuf.Infof("Exported configuration and calibration to %s%s", path, note)
	a.toast("Exported to "+path, ui.ColGood)
}

func (a *App) importBackup(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		a.toast("Cannot read "+filepath.Base(path)+": "+err.Error(), ui.ColBad)
		return
	}
	b, err := model.ParseBackup(data)
	if err != nil {
		logbuf.Errorf("Import of %s failed: %v", path, err)
		a.toast("Import failed: "+err.Error(), ui.ColBad)
		return
	}
	if !a.ready() {
		a.toast("Connect the device before importing", ui.ColWarn)
		return
	}
	a.ed.Draft = b.Config
	a.dialog = dlgNone
	logbuf.Infof("Imported %s (exported %s from firmware %s). Review the changes and press Save.", filepath.Base(path),
		b.Created.Local().Format("2006-01-02 15:04"), b.Firmware)
	a.toast("Backup loaded as unsaved changes - review and Save", ui.ColAccent)
	if b.Calibration != nil && *b.Calibration != a.snap.Calibration {
		cal := *b.Calibration
		a.ask("Restore calibration too?",
			"The backup also contains calibration data. Restoring it replaces the device's current calibration immediately. "+
				"Only do this if the backup was made with the same hardware.",
			"Restore calibration", false, func() {
				a.mgr.Do(device.Op{Kind: device.OpWriteCalibration, Calibration: cal})
			})
	}
}

func (a *App) ask(title, body, action string, danger bool, onYes func()) {
	a.confirm = confirmState{title, body, action, danger, onYes}
	a.dialog = dlgConfirm
}

func clampf(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// baseView is the saved configuration as the editor compares it with the
// draft: LED masks renumbered for the draft's LED layout (model.BaseView).
func (a *App) baseView() *proto.Config {
	v := a.ed.BaseView()
	return &v
}
