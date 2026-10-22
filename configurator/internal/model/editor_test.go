package model

import (
	"testing"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
)

func baseConfig() proto.Config {
	var c proto.Config
	c.Magic, c.Version = proto.ConfigMagic, proto.ConfigVersion
	c.NumAxes, c.NumButtons = 4, 7
	c.Brightness = 500
	c.UsbProduct.SetString("gamepad")
	c.Axes[0].Pin = 26
	return c
}

func TestRebaseKeepsUserEditsAndFollowsDevice(t *testing.T) {
	var e Editor
	dev := baseConfig()
	e.Sync(dev, 1)
	if e.Dirty() {
		t.Fatal("fresh editor must be clean")
	}
	// user edits two settings
	e.Draft.Brightness = 800
	e.Draft.Axes[0].Color = proto.Color{R: 1, G: 2, B: 3}
	if e.Changes() != 2 {
		t.Fatalf("changes = %d, want 2 (colour counts once)", e.Changes())
	}
	// device configuration changes underneath (e.g. re-read after reconnect)
	dev2 := dev
	dev2.NumButtons = 12
	dev2.UsbProduct.SetString("other")
	e.Sync(dev2, 2)
	if e.Draft.NumButtons != 12 || e.Draft.UsbProduct.String() != "other" {
		t.Fatal("unchanged settings must follow the device")
	}
	if e.Draft.Brightness != 800 || e.Draft.Axes[0].Color.R != 1 {
		t.Fatal("user edits must survive a re-read")
	}
	if e.Changes() != 2 {
		t.Fatalf("changes after rebase = %d", e.Changes())
	}
	// same generation: nothing happens
	e.Draft.DebounceMs = 9
	e.Sync(dev2, 2)
	if e.Draft.DebounceMs != 9 {
		t.Fatal("Sync with the same generation must not touch the draft")
	}
	// saving makes device == draft: clean again
	e.Sync(e.Draft, 3)
	if e.Dirty() {
		t.Fatal("draft equal to device must be clean")
	}
}

func TestBackupRoundTrip(t *testing.T) {
	c := baseConfig()
	c.Size = uint16(len(proto.Encode(c)))
	cal := proto.Calibration{}
	cal.Channels[1] = proto.CalChannel{Calibrated: 1, Min: 10, Center: 2000, Max: 4000}
	b := NewBackup(c, &cal, proto.Hello{})
	data, err := b.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseBackup(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config != c {
		t.Fatal("config differs after JSON round trip")
	}
	if got.Calibration == nil || got.Calibration.Channels[1] != cal.Channels[1] {
		t.Fatal("calibration lost")
	}
	if _, err := ParseBackup([]byte(`{"format":"something-else"}`)); err == nil {
		t.Fatal("foreign files must be rejected")
	}
}
