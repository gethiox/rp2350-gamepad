// Package settings stores the configurator's own preferences (not the
// device configuration) in the user's config directory.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DeviceID is a USB vendor/product pair the configurator looks for.
type DeviceID struct {
	VID uint16 `json:"vid"`
	PID uint16 `json:"pid"`
}

func (d DeviceID) String() string { return fmt.Sprintf("%04X:%04X", d.VID, d.PID) }

// ParseDeviceID parses "ACDC:2137".
func ParseDeviceID(s string) (DeviceID, error) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return DeviceID{}, fmt.Errorf("expected VID:PID, e.g. ACDC:2137")
	}
	v, err1 := strconv.ParseUint(parts[0], 16, 16)
	p, err2 := strconv.ParseUint(parts[1], 16, 16)
	if err1 != nil || err2 != nil {
		return DeviceID{}, fmt.Errorf("VID and PID must be 4-digit hexadecimal numbers")
	}
	return DeviceID{uint16(v), uint16(p)}, nil
}

var DefaultDeviceID = DeviceID{0xACDC, 0x2137}

type Settings struct {
	// UIScale multiplies all sizes; 0 = follow the display's scaling.
	UIScale     float32    `json:"ui_scale"`
	DeviceIDs   []DeviceID `json:"device_ids"`
	LivePreview bool       `json:"live_led_preview"`
	// MaxFPS caps the frame rate; 0 = the display's refresh rate.
	MaxFPS    int    `json:"max_fps"`
	WindowW   int    `json:"window_w,omitempty"`
	WindowH   int    `json:"window_h,omitempty"`
	BackupDir string `json:"backup_dir,omitempty"`

	path string
}

func Default() *Settings {
	return &Settings{DeviceIDs: []DeviceID{DefaultDeviceID}, LivePreview: true, MaxFPS: 60}
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rp2350-gamepad", "settings.json"), nil
}

// Load reads the settings file; on any problem defaults are returned along
// with the error (the caller logs it).
func Load() (*Settings, error) {
	s := Default()
	p, err := configPath()
	if err != nil {
		return s, err
	}
	s.path = p
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		d := Default()
		d.path = p
		return d, fmt.Errorf("settings file %s is damaged (%v); using defaults", p, err)
	}
	s.normalize()
	return s, nil
}

func (s *Settings) normalize() {
	if s.UIScale != 0 && (s.UIScale < 0.5 || s.UIScale > 3) {
		s.UIScale = 0
	}
	if s.MaxFPS != 0 && (s.MaxFPS < 30 || s.MaxFPS > 500) {
		s.MaxFPS = 60
	}
	seen := map[DeviceID]bool{}
	var ids []DeviceID
	for _, id := range s.DeviceIDs {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		ids = []DeviceID{DefaultDeviceID}
	}
	s.DeviceIDs = ids
}

// Save writes atomically (temp file + rename).
func (s *Settings) Save() error {
	if s.path == "" {
		p, err := configPath()
		if err != nil {
			return err
		}
		s.path = p
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Settings) Path() string { return s.path }

// AddDeviceID adds id if it is not known yet; returns true if added.
func (s *Settings) AddDeviceID(id DeviceID) bool {
	for _, d := range s.DeviceIDs {
		if d == id {
			return false
		}
	}
	s.DeviceIDs = append(s.DeviceIDs, id)
	return true
}
