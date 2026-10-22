package model

import (
	"testing"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
)

func TestRemapLedMasks(t *testing.T) {
	var c proto.Config
	c.NumLedPins = 3
	c.LedCounts = [proto.MaxLedPins]uint8{1, 1, 1}
	c.Buttons[0].LedMask = proto.LedMaskOf(0b101) // pin 1 and pin 3
	c.StatusLedMask = proto.LedMaskOf(0b010)      // pin 2
	old := LayoutOf(&c)

	// pin 2 becomes a chain of 4: pin 3's LED moves from number 3 to 6
	c.LedCounts[1] = 4
	RemapLedMasks(&c, old)
	if got := c.Buttons[0].LedMask.Uint64(); got != 0b100001 {
		t.Fatalf("button mask %b", got)
	}
	if got := c.StatusLedMask.Uint64(); got != 0b000010 {
		t.Fatalf("status mask %b", got)
	}

	// dropping pin 3 removes its LED from the masks
	old = LayoutOf(&c)
	c.NumLedPins = 2
	RemapLedMasks(&c, old)
	if got := c.Buttons[0].LedMask.Uint64(); got != 0b1 {
		t.Fatalf("button mask after removing a pin %b", got)
	}
}

func TestBackupWithoutLedChains(t *testing.T) {
	data := []byte(`{"format":"rp2350-gamepad-backup","format_version":1,"config_version":1,
		"config":{"num_leds":4,"led_pins":[22,23,24,25,255,255,255,255],"status_led_mask":3,
		"buttons":[{"pin":14,"led_mask":4,"color":{"r":0,"g":0,"b":255}}]}}`)
	b, err := ParseBackup(data)
	if err != nil {
		t.Fatal(err)
	}
	c := b.Config
	if c.NumLedPins != 4 || c.LedCounts[0] != 1 || c.LedCounts[3] != 1 || c.LedTotal() != 4 {
		t.Fatalf("LED layout not upgraded: %d pins, counts %v", c.NumLedPins, c.LedCounts)
	}
	if c.StatusLedMask.Uint64() != 3 || !c.Buttons[0].LedMask.Has(2) {
		t.Fatal("masks not kept")
	}
}
