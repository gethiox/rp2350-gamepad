// Package model holds the editing state: the configuration as the device
// has it (Base) and the user's unsaved edits (Draft).
package model

import (
	"hash/fnv"
	"reflect"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
)

// Editor tracks the draft against the device configuration.
//
// Whenever the device configuration is re-read (reconnect, after a save,
// after a factory reset) Sync rebases the draft: every setting the user
// changed keeps the user's value, everything else follows the device. The
// UI therefore never shows stale device values, and never silently drops
// unsaved edits.
type Editor struct {
	Base    proto.Config
	Draft   proto.Config
	gen     uint64
	hasBase bool
}

// Sync is called every frame with the device snapshot; cheap when gen is unchanged.
func (e *Editor) Sync(device proto.Config, gen uint64) {
	if e.hasBase && gen == e.gen {
		return
	}
	if !e.hasBase {
		e.Base, e.Draft = device, device
	} else {
		merged := device
		mergeEdits(reflect.ValueOf(&merged).Elem(), reflect.ValueOf(&e.Draft).Elem(), reflect.ValueOf(&e.Base).Elem())
		e.Base, e.Draft = device, merged
	}
	e.gen, e.hasBase = gen, true
}

// Loaded reports whether a device configuration has been seen.
func (e *Editor) Loaded() bool { return e.hasBase }

// Discard throws away all edits.
func (e *Editor) Discard() { e.Draft = e.Base }

// Reset forgets everything (device disconnected for good / different device).
func (e *Editor) Reset() { *e = Editor{} }

func (e *Editor) Dirty() bool { return e.Draft != e.Base }

// Changes counts modified settings (a colour or a string counts once).
// LED selections are compared by physical LED (see BaseView).
func (e *Editor) Changes() int {
	return countDiff(reflect.ValueOf(e.Draft), reflect.ValueOf(e.BaseView()))
}

// BaseView is the saved configuration with its LED masks renumbered for the
// draft's LED layout, so that "unchanged" means "lights the same physical
// LEDs" even after a chain was made longer or shorter. Everything else,
// including the LED layout itself, is the saved value.
func (e *Editor) BaseView() proto.Config {
	v := e.Base
	RemapMasks(&v, LayoutOf(&e.Base), LayoutOf(&e.Draft))
	return v
}

// Hash identifies the draft (to match validation results).
func (e *Editor) Hash() uint64 { return Hash(e.Draft) }

func Hash(c proto.Config) uint64 {
	h := fnv.New64a()
	h.Write(proto.Encode(c))
	return h.Sum64()
}

var leafTypes = map[reflect.Type]bool{
	reflect.TypeOf(proto.CStr{}):    true,
	reflect.TypeOf(proto.Color{}):   true,
	reflect.TypeOf(proto.LedMask{}): true,
}

// mergeEdits: dst starts as the new device value; wherever draft differs
// from the old base, the user's draft value is kept.
func mergeEdits(dst, draft, base reflect.Value) {
	if leafTypes[dst.Type()] || (dst.Kind() != reflect.Struct && dst.Kind() != reflect.Array) {
		if !reflect.DeepEqual(draft.Interface(), base.Interface()) && dst.CanSet() {
			dst.Set(draft)
		}
		return
	}
	switch dst.Kind() {
	case reflect.Struct:
		for i := 0; i < dst.NumField(); i++ {
			mergeEdits(dst.Field(i), draft.Field(i), base.Field(i))
		}
	case reflect.Array:
		for i := 0; i < dst.Len(); i++ {
			mergeEdits(dst.Index(i), draft.Index(i), base.Index(i))
		}
	}
}

func countDiff(a, b reflect.Value) int {
	if leafTypes[a.Type()] || (a.Kind() != reflect.Struct && a.Kind() != reflect.Array) {
		if reflect.DeepEqual(a.Interface(), b.Interface()) {
			return 0
		}
		return 1
	}
	n := 0
	switch a.Kind() {
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			n += countDiff(a.Field(i), b.Field(i))
		}
	case reflect.Array:
		for i := 0; i < a.Len(); i++ {
			n += countDiff(a.Index(i), b.Index(i))
		}
	}
	return n
}

// LedLook extracts what the LED preview depends on, so the UI can tell
// whether the draft would look different from the device.
type LedLook struct {
	Brightness       uint16
	StatusBrightness uint16
	StatusLedMask    proto.LedMask
	LedFlags         uint8
	Axes             [proto.MaxAxes]struct {
		Mask   proto.LedMask
		Effect uint8
		C      proto.Color
	}
	Buttons [proto.MaxButtons]struct {
		Mask proto.LedMask
		C    proto.Color
	}
}

func Look(c *proto.Config) LedLook {
	l := LedLook{Brightness: c.Brightness, StatusBrightness: c.StatusBrightness, StatusLedMask: c.StatusLedMask, LedFlags: c.LedFlags}
	for i := range c.Axes {
		l.Axes[i].Mask, l.Axes[i].Effect, l.Axes[i].C = c.Axes[i].LedMask, c.Axes[i].LedEffect, c.Axes[i].Color
	}
	for i := range c.Buttons {
		l.Buttons[i].Mask, l.Buttons[i].C = c.Buttons[i].LedMask, c.Buttons[i].Color
	}
	return l
}

// LedLayout is how many LEDs each enabled LED pin drives.
type LedLayout struct {
	Pins   uint8
	Counts [proto.MaxLedPins]uint8
}

func LayoutOf(c *proto.Config) LedLayout {
	l := LedLayout{Pins: c.NumLedPins}
	for i := 0; i < int(c.NumLedPins) && i < proto.MaxLedPins; i++ {
		l.Counts[i] = c.LedCounts[i]
	}
	return l
}

// RemapLedMasks keeps every LED selection attached to the same physical LED
// (pin and chain position) after the LED layout changed from old to the
// layout of c. LEDs that no longer exist are dropped from the masks.
func RemapLedMasks(c *proto.Config, old LedLayout) { RemapMasks(c, old, LayoutOf(c)) }

// RemapMasks renumbers the LED masks of c from layout old to layout now
// (c's own layout fields are not changed).
func RemapMasks(c *proto.Config, old, now LedLayout) {
	if now == old {
		return
	}
	var oldOff [proto.MaxLedPins]int
	off := 0
	for p := 0; p < proto.MaxLedPins; p++ {
		oldOff[p] = off
		if p < int(old.Pins) {
			off += int(old.Counts[p])
		}
	}
	remap := func(m proto.LedMask) proto.LedMask {
		var out proto.LedMask
		n := 0
		for p := 0; p < int(now.Pins) && p < proto.MaxLedPins; p++ {
			for k := 0; k < int(now.Counts[p]); k++ {
				if p < int(old.Pins) && k < int(old.Counts[p]) && m.Has(oldOff[p]+k) {
					out.Set(n, true)
				}
				n++
			}
		}
		return out
	}
	c.StatusLedMask = remap(c.StatusLedMask)
	for i := range c.Axes {
		c.Axes[i].LedMask = remap(c.Axes[i].LedMask)
	}
	for i := range c.Buttons {
		c.Buttons[i].LedMask = remap(c.Buttons[i].LedMask)
	}
}
