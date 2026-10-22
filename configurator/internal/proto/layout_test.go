package proto

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode"
)

// layout.json is generated from the C header by firmware/host/layout_dump.c
// and checked in CI to be up to date; these tests make sure the Go mirror
// structures and constants match it byte for byte.

type cField struct {
	Name   string `json:"name"`
	Offset int    `json:"offset"`
	Size   int    `json:"size"`
}

type cStruct struct {
	Size   int      `json:"size"`
	Fields []cField `json:"fields"`
}

type cLayout struct {
	Structs   map[string]cStruct `json:"structs"`
	Constants map[string]int64   `json:"constants"`
}

func loadLayout(t *testing.T) cLayout {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "protocol", "layout.json"))
	if err != nil {
		t.Fatalf("reading layout.json: %v", err)
	}
	var l cLayout
	if err := json.Unmarshal(data, &l); err != nil {
		t.Fatal(err)
	}
	return l
}

// snake converts a Go field name to the C naming convention.
func snake(s string) string {
	special := map[string]string{"HidReportID": "hid_report_id", "ChipID": "chip_id", "LedRgb": "led_rgb",
		"HidReportsPerS": "hid_reports_per_s", "R": "r", "G": "g", "B": "b"}
	if v, ok := special[s]; ok {
		return v
	}
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		if unicode.IsUpper(r) {
			if i > 0 && !unicode.IsUpper(rs[i-1]) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestStructLayouts(t *testing.T) {
	l := loadLayout(t)
	types := map[string]any{
		"gp_color_t":       Color{},
		"gp_axis_cfg_t":    AxisCfg{},
		"gp_button_cfg_t":  ButtonCfg{},
		"gp_group_cfg_t":   GroupCfg{},
		"gp_config_t":      Config{},
		"gp_cal_channel_t": CalChannel{},
		"gp_cal_group_t":   CalGroup{},
		"gp_calibration_t": Calibration{},
		"gp_hello_t":       Hello{},
		"gp_range_t":       Range{},
		"gp_limits_t":      Limits{},
		"gp_error_t":       ErrorEntry{},
		"gp_telemetry_t":   Telemetry{},
		"gp_cal_result_t":  CalResult{},
		"gp_sim_input_t":   SimInput{},
	}
	if len(types) != len(l.Structs) {
		t.Errorf("layout.json has %d structs, Go mirrors %d", len(l.Structs), len(types))
	}
	for name, v := range types {
		cs, ok := l.Structs[name]
		if !ok {
			t.Errorf("%s missing from layout.json", name)
			continue
		}
		if got := binary.Size(v); got != cs.Size {
			t.Errorf("%s: Go size %d, C size %d", name, got, cs.Size)
		}
		rt := reflect.TypeOf(v)
		if rt.NumField() != len(cs.Fields) {
			t.Errorf("%s: Go has %d fields, C has %d", name, rt.NumField(), len(cs.Fields))
			continue
		}
		off := 0
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			size := binary.Size(reflect.Zero(f.Type).Interface())
			c := cs.Fields[i]
			if snake(f.Name) != c.Name || off != c.Offset || size != c.Size {
				t.Errorf("%s field %d: Go %s@%d(%d) vs C %s@%d(%d)", name, i, snake(f.Name), off, size, c.Name, c.Offset, c.Size)
			}
			off += size
		}
	}
}

func TestConstants(t *testing.T) {
	l := loadLayout(t)
	for name, v := range l.Constants {
		got, ok := cConstants[name]
		if !ok {
			t.Errorf("constant %s (= %d) has no Go mirror", name, v)
			continue
		}
		if got != v {
			t.Errorf("constant %s: Go %d, C %d", name, got, v)
		}
	}
	for name := range cConstants {
		if _, ok := l.Constants[name]; !ok {
			t.Errorf("Go constant %s does not exist in C", name)
		}
	}
}
