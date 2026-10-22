// Package proto mirrors firmware/src/core/gp_protocol.h: the binary
// structures exchanged with the device. Field order and sizes must match
// the C definitions exactly; layout_test.go checks them against
// protocol/layout.json, which is generated from the C headers.
package proto

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// CStr is a fixed-size, NUL-terminated ASCII string as used on the wire.
type CStr [StrLen]byte

func (s CStr) String() string {
	if i := bytes.IndexByte(s[:], 0); i >= 0 {
		return string(s[:i])
	}
	return string(s[:])
}

// SetString stores v (truncated to StrLen-1 bytes) with zero padding, so that
// equal strings always produce equal bytes.
func (s *CStr) SetString(v string) {
	*s = CStr{}
	if len(v) > StrLen-1 {
		v = v[:StrLen-1]
	}
	copy(s[:], v)
}

func (s CStr) MarshalJSON() ([]byte, error) { return json.Marshal(s.String()) }
func (s *CStr) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if len(v) > StrLen-1 {
		return fmt.Errorf("string %q longer than %d characters", v, StrLen-1)
	}
	s.SetString(v)
	return nil
}

type Color struct {
	R        uint8 `json:"r"`
	G        uint8 `json:"g"`
	B        uint8 `json:"b"`
	Reserved uint8 `json:"-"`
}

// LedMask selects LEDs by their number across all LED pins (bit n = LED n+1).
// It is stored as two little-endian words; JSON uses one unsigned number.
type LedMask [2]uint32

func (m LedMask) Has(i int) bool { return i >= 0 && i < MaxLeds && m[i/32]>>(uint(i)%32)&1 != 0 }
func (m *LedMask) Set(i int, on bool) {
	if i < 0 || i >= MaxLeds {
		return
	}
	if on {
		m[i/32] |= 1 << (uint(i) % 32)
	} else {
		m[i/32] &^= 1 << (uint(i) % 32)
	}
}
func (m LedMask) Uint64() uint64               { return uint64(m[0]) | uint64(m[1])<<32 }
func LedMaskOf(v uint64) LedMask               { return LedMask{uint32(v), uint32(v >> 32)} }
func (m LedMask) Empty() bool                  { return m[0] == 0 && m[1] == 0 }
func (m LedMask) MarshalJSON() ([]byte, error) { return json.Marshal(m.Uint64()) }
func (m *LedMask) UnmarshalJSON(b []byte) error {
	var v uint64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*m = LedMaskOf(v)
	return nil
}

type AxisCfg struct {
	Pin       uint8   `json:"pin"`
	HidUsage  uint8   `json:"hid_usage"`
	Flags     uint8   `json:"flags"`
	LedEffect uint8   `json:"led_effect"`
	CenterDz  uint16  `json:"center_dz"`
	OuterDz   uint16  `json:"outer_dz"`
	Color     Color   `json:"color"`
	LedMask   LedMask `json:"led_mask"`
}

type ButtonCfg struct {
	Pin      uint8   `json:"pin"`
	Flags    uint8   `json:"-"`
	Reserved uint16  `json:"-"`
	Color    Color   `json:"color"`
	LedMask  LedMask `json:"led_mask"`
}

type GroupCfg struct {
	Enabled   uint8  `json:"enabled"`
	AxisH     uint8  `json:"axis_h"`
	AxisV     uint8  `json:"axis_v"`
	Shape     uint8  `json:"shape"`
	Flags     uint8  `json:"flags"`
	Reserved  uint8  `json:"-"`
	CenterDz  uint16 `json:"center_dz"`
	OuterDz   uint16 `json:"outer_dz"`
	Reserved2 uint16 `json:"-"`
}

type Config struct {
	Magic   uint32 `json:"-"`
	Version uint16 `json:"-"`
	Size    uint16 `json:"-"`

	UsbVid          uint16 `json:"usb_vid"`
	UsbPid          uint16 `json:"usb_pid"`
	UsbManufacturer CStr   `json:"usb_manufacturer"`
	UsbProduct      CStr   `json:"usb_product"`
	UsbSerial       CStr   `json:"usb_serial"`
	HidRateHz       uint16 `json:"hid_rate_hz"`
	HidReportID     uint8  `json:"hid_report_id"`
	HidDeviceType   uint8  `json:"hid_device_type"`

	PollRateHz       uint16 `json:"poll_rate_hz"`
	SmoothingSamples uint8  `json:"smoothing_samples"`
	SmoothingMode    uint8  `json:"smoothing_mode"`
	DebounceMs       uint8  `json:"debounce_ms"`
	NumAxes          uint8  `json:"num_axes"`
	NumButtons       uint8  `json:"num_buttons"`
	NumLedPins       uint8  `json:"num_led_pins"`
	CenterDz         uint16 `json:"center_dz"`
	OuterDz          uint16 `json:"outer_dz"`

	CalPin      uint8  `json:"cal_pin"`
	Reserved0   uint8  `json:"-"`
	CalHoldMs   uint16 `json:"cal_hold_ms"`
	ResetHoldMs uint16 `json:"reset_hold_ms"`

	Brightness       uint16                `json:"brightness"`
	StatusBrightness uint16                `json:"status_brightness"`
	LedFlags         uint8                 `json:"led_flags"`
	Reserved1        uint8                 `json:"-"`
	LedPins          [MaxLedPins]uint8     `json:"led_pins"`
	LedCounts        [MaxLedPins]uint8     `json:"led_counts"`
	StatusLedMask    LedMask               `json:"status_led_mask"`
	Reserved2        uint16                `json:"-"`
	Reserved3        uint16                `json:"-"`
	Groups           [MaxGroups]GroupCfg   `json:"groups"`
	Axes             [MaxAxes]AxisCfg      `json:"axes"`
	Buttons          [MaxButtons]ButtonCfg `json:"buttons"`
}

type CalChannel struct {
	Calibrated uint8  `json:"calibrated"`
	Reserved   uint8  `json:"-"`
	Min        uint16 `json:"min"`
	Center     uint16 `json:"center"`
	Max        uint16 `json:"max"`
}

type CalGroup struct {
	Calibrated uint8              `json:"calibrated"`
	PinA       uint8              `json:"pin_a"`
	PinB       uint8              `json:"pin_b"`
	Orient     uint8              `json:"orient"`
	CenterA    uint16             `json:"center_a"`
	CenterB    uint16             `json:"center_b"`
	Radial     [RadialBins]uint16 `json:"radial"`
}

type Calibration struct {
	Magic    uint32              `json:"-"`
	Version  uint16              `json:"-"`
	Size     uint16              `json:"-"`
	Channels [MaxAxes]CalChannel `json:"channels"`
	Groups   [MaxGroups]CalGroup `json:"groups"`
}

type Hello struct {
	ProtoVersion  uint16
	ConfigVersion uint16
	ConfigSize    uint16
	CalVersion    uint16
	CalSize       uint16
	TelemetrySize uint16
	LimitsSize    uint16
	Flags         uint16
	StorageSeq    uint32
	ChipID        [8]uint8
	ResetReason   uint8
	CrashCount    uint8
	Reserved      uint16
	FwVersion     CStr
	Board         CStr
}

type Range struct{ Min, Max uint16 }

type Limits struct {
	MaxAxes         uint8
	MaxButtons      uint8
	MaxLeds         uint8
	MaxGroups       uint8
	PlatformAxes    uint8
	PlatformLedPins uint8
	GpioCount       uint8
	AdcFirstPin     uint8
	RadialBins      uint8
	StrLen          uint8
	NumHidUsages    uint8
	MaxLedPins      uint8
	DzSumMax        uint16
	BuiltinLedPin   uint8
	Reserved1       uint8
	ButtonPins      [2]uint32
	LedPins         [2]uint32
	AdcPins         [2]uint32
	CalPins         [2]uint32
	HidUsages       [12]uint8
	Ranges          [FCount]Range
}

type ErrorEntry struct {
	Field    uint8
	Index    uint8
	Reserved [2]uint8
	Message  [ErrorMsgLen]byte
}

type Telemetry struct {
	UptimeMs         uint32
	Buttons          [2]uint32
	Axes             [MaxAxes]int16
	Raw              [MaxAxes]uint16
	RawNow           [MaxAxes]uint16
	CalMin           [MaxAxes]uint16
	CalCenter        [MaxAxes]uint16
	CalMax           [MaxAxes]uint16
	LedRgb           [MaxLeds][4]uint8
	CalGroupSweep    [MaxGroups]uint16
	CalChannelState  [MaxAxes]uint8
	CalGroupState    [MaxGroups]uint8
	CalGroupCoverage [MaxGroups]uint8
	State            uint8
	Reserved         uint8
	LoopMaxUs        uint16
	HidReportsPerS   uint16
	Reserved2        uint16
}

type CalResult struct {
	Channel [MaxAxes]uint8
	Group   [MaxGroups]uint8
}

type SimInput struct {
	Raw        [MaxAxes]uint16
	Buttons    [2]uint32
	CalButton  uint8
	AutoMotion uint8
	UsbState   uint8
	Reserved   uint8
}

// --- helpers ---

// Encode serialises a wire structure (little endian, packed).
func Encode(v any) []byte {
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
		panic(err) // only fixed-size types are used: cannot fail
	}
	return b.Bytes()
}

// Decode parses exactly binary.Size(v) bytes into v.
func Decode(data []byte, v any) error {
	want := binary.Size(v)
	if len(data) != want {
		return fmt.Errorf("payload is %d bytes, expected %d", len(data), want)
	}
	return binary.Read(bytes.NewReader(data), binary.LittleEndian, v)
}

// LedTotal returns the number of LEDs on all enabled LED pins.
func (c *Config) LedTotal() int {
	n := 0
	for i := 0; i < int(c.NumLedPins) && i < MaxLedPins; i++ {
		n += int(c.LedCounts[i])
	}
	if n > MaxLeds {
		n = MaxLeds
	}
	return n
}

// LedLocation tells which LED pin (0-based) and which position in its chain
// (0-based) LED number i (0-based) is. ok is false past the last LED.
func (c *Config) LedLocation(i int) (pin, pos int, ok bool) {
	for p := 0; p < int(c.NumLedPins) && p < MaxLedPins; p++ {
		n := int(c.LedCounts[p])
		if i < n {
			return p, i, true
		}
		i -= n
	}
	return 0, 0, false
}

// Button state helpers
func (t *Telemetry) Button(i int) bool {
	return i >= 0 && i < 64 && t.Buttons[i/32]>>(uint(i)%32)&1 != 0
}

func (l *Limits) maskHas(m [2]uint32, pin int) bool {
	return pin >= 0 && pin < 64 && m[pin/32]>>(uint(pin)%32)&1 != 0
}
func (l *Limits) ButtonPinOK(pin int) bool { return l.maskHas(l.ButtonPins, pin) }
func (l *Limits) LedPinOK(pin int) bool    { return l.maskHas(l.LedPins, pin) }
func (l *Limits) AdcPinOK(pin int) bool    { return l.maskHas(l.AdcPins, pin) }
func (l *Limits) CalPinOK(pin int) bool    { return l.maskHas(l.CalPins, pin) }

// Usages returns the HID usages the firmware accepts for axes.
func (l *Limits) Usages() []uint8 { return append([]uint8(nil), l.HidUsages[:l.NumHidUsages]...) }

func (e ErrorEntry) Text() string {
	if i := bytes.IndexByte(e.Message[:], 0); i >= 0 {
		return string(e.Message[:i])
	}
	return string(e.Message[:])
}

// ErrorList is the decoded response of VALIDATE/WRITE commands.
type ErrorList struct {
	Status    uint8
	Truncated bool
	Errors    []ErrorEntry
}

// DecodeErrorList parses a status-only or full error-list response.
func DecodeErrorList(p []byte) (ErrorList, error) {
	if len(p) < 1 {
		return ErrorList{}, fmt.Errorf("empty response")
	}
	el := ErrorList{Status: p[0]}
	if len(p) == 1 {
		return el, nil
	}
	if len(p) < 4 {
		return el, fmt.Errorf("short error list")
	}
	n := int(p[1])
	el.Truncated = p[2] != 0
	es := binary.Size(ErrorEntry{})
	if n > MaxErrors || len(p) != 4+n*es {
		return el, fmt.Errorf("malformed error list (%d entries, %d bytes)", n, len(p))
	}
	el.Errors = make([]ErrorEntry, n)
	for i := 0; i < n; i++ {
		if err := Decode(p[4+i*es:4+(i+1)*es], &el.Errors[i]); err != nil {
			return el, err
		}
	}
	return el, nil
}
