package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
)

const BackupFormat = "rp2350-gamepad-backup"

// Backup is the JSON file written by "Export". It is human readable; the
// device validates an imported configuration before it can be saved.
type Backup struct {
	Format        string             `json:"format"`
	FormatVersion int                `json:"format_version"`
	Created       time.Time          `json:"created"`
	Firmware      string             `json:"firmware"`
	Board         string             `json:"board"`
	ConfigVersion int                `json:"config_version"`
	CalVersion    int                `json:"calibration_version"`
	Config        proto.Config       `json:"config"`
	Calibration   *proto.Calibration `json:"calibration,omitempty"`
}

func NewBackup(cfg proto.Config, cal *proto.Calibration, hello proto.Hello) Backup {
	return Backup{
		Format: BackupFormat, FormatVersion: 1, Created: time.Now().UTC().Truncate(time.Second),
		Firmware: hello.FwVersion.String(), Board: hello.Board.String(),
		ConfigVersion: proto.ConfigVersion, CalVersion: proto.CalVersion,
		Config: cfg, Calibration: cal,
	}
}

func (b Backup) Marshal() ([]byte, error) { return json.MarshalIndent(b, "", "  ") }

// ParseBackup decodes and normalises a backup file.
func ParseBackup(data []byte) (Backup, error) {
	var b Backup
	if err := json.Unmarshal(data, &b); err != nil {
		return b, fmt.Errorf("not a valid backup file: %v", err)
	}
	if b.Format != BackupFormat {
		return b, fmt.Errorf("not an rp2350-gamepad backup file")
	}
	if b.ConfigVersion != proto.ConfigVersion {
		return b, fmt.Errorf("the backup uses configuration format %d; this version supports %d", b.ConfigVersion, proto.ConfigVersion)
	}
	upgradeLedFields(data, &b.Config)
	b.Config.Magic, b.Config.Version, b.Config.Size = proto.ConfigMagic, proto.ConfigVersion, uint16(len(proto.Encode(proto.Config{})))
	if b.Calibration != nil {
		if b.CalVersion != proto.CalVersion {
			b.Calibration = nil // config is still usable
		} else {
			b.Calibration.Magic, b.Calibration.Version = proto.CalMagic, proto.CalVersion
			b.Calibration.Size = uint16(len(proto.Encode(proto.Calibration{})))
		}
	}
	return b, nil
}

// upgradeLedFields fills in the LED chain fields for exports made before
// LED chains existed (one LED per pin, "num_leds").
func upgradeLedFields(data []byte, c *proto.Config) {
	var raw struct {
		Config map[string]json.RawMessage `json:"config"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return
	}
	if _, ok := raw.Config["num_led_pins"]; !ok {
		var n uint8
		if v, ok := raw.Config["num_leds"]; ok && json.Unmarshal(v, &n) == nil {
			c.NumLedPins = n
		}
	}
	if _, ok := raw.Config["led_counts"]; !ok {
		for i := range c.LedCounts {
			c.LedCounts[i] = 1
		}
	}
}

// SafeName turns a device name into something usable in a file name.
func SafeName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
	s = strings.Trim(s, "-")
	if s == "" {
		return "device"
	}
	return s
}

// DefaultBackupDir is ~/Documents/rp2350-gamepad (or the home directory).
func DefaultBackupDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	docs := filepath.Join(home, "Documents")
	if st, err := os.Stat(docs); err == nil && st.IsDir() {
		return filepath.Join(docs, "rp2350-gamepad")
	}
	return filepath.Join(home, "rp2350-gamepad")
}

// ListBackups returns *.json files in dir, newest first.
func ListBackups(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type f struct {
		path string
		t    time.Time
	}
	var fs []f
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		fs = append(fs, f{filepath.Join(dir, e.Name()), info.ModTime()})
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].t.After(fs[j].t) })
	out := make([]string, len(fs))
	for i := range fs {
		out[i] = fs[i].path
	}
	return out
}
