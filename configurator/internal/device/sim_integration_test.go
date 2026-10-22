package device

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/settings"
)

// These tests run the configurator's device layer against the real firmware
// core compiled for the host (firmware/host, rp2350-gamepad-sim). They prove
// that both sides agree on framing, structures, validation and the
// save -> reboot -> verify cycle. Set RP2350_GAMEPAD_SIM to the simulator
// binary to run them (CI does).

func simManager(t *testing.T) *Manager {
	t.Helper()
	bin := os.Getenv("RP2350_GAMEPAD_SIM")
	if bin == "" {
		t.Skip("RP2350_GAMEPAD_SIM not set")
	}
	flash := filepath.Join(t.TempDir(), "flash.bin")
	m := NewManager(&SimFinder{Binary: bin, Args: []string{"--flash", flash}}, []settings.DeviceID{settings.DefaultDeviceID})
	m.Start()
	t.Cleanup(m.Stop)
	waitState(t, m, Ready, 10*time.Second)
	return m
}

func waitState(t *testing.T, m *Manager, want ConnState, d time.Duration) Snapshot {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		s := m.Snapshot()
		if s.State == want && s.Busy == "" {
			return s
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("state %v not reached (now %v)", want, m.Snapshot().State)
	return Snapshot{}
}

func waitResult(t *testing.T, m *Manager, kind OpKind) Result {
	t.Helper()
	timeout := time.After(15 * time.Second)
	for {
		select {
		case r := <-m.Results():
			if r.Kind == kind {
				return r
			}
		case <-timeout:
			t.Fatalf("no result for %v", kind)
		}
	}
}

func waitTelemetry(t *testing.T, m *Manager, cond func(proto.Telemetry) bool) proto.Telemetry {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s := m.Snapshot()
		if s.TelemetryGen > 0 && cond(s.Telemetry) {
			return s.Telemetry
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("telemetry condition not met")
	return proto.Telemetry{}
}

func TestSimHandshakeAndValidation(t *testing.T) {
	m := simManager(t)
	s := m.Snapshot()
	if !s.Simulator || s.Hello.ProtoVersion != proto.ProtoVersion {
		t.Fatalf("hello: %+v", s.Hello)
	}
	if s.Config.Magic != proto.ConfigMagic || s.Config.NumAxes != 4 || s.Limits.PlatformAxes != 4 {
		t.Fatalf("unexpected config/limits: axes %d, platform %d", s.Config.NumAxes, s.Limits.PlatformAxes)
	}
	if s.Config.UsbProduct.String() != "gamepad" || s.Config.UsbManufacturer.String() != "rp2350" {
		t.Fatalf("strings: %q %q", s.Config.UsbManufacturer, s.Config.UsbProduct)
	}
	waitTelemetry(t, m, func(proto.Telemetry) bool { return true })

	// the device's own validator reports problems with field and index
	bad := s.Config
	bad.Buttons[3].Pin = bad.Buttons[0].Pin
	bad.CenterDz, bad.OuterDz = 500, 500
	m.Validate(bad, 42)
	deadline := time.Now().Add(3 * time.Second)
	var v Validation
	for time.Now().Before(deadline) {
		v = m.Snapshot().Validation
		if v.Done && v.DraftHash == 42 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !v.Done || v.Err != nil {
		t.Fatalf("validation: %+v", v)
	}
	var pin, dz bool
	for _, e := range v.Errors {
		pin = pin || (e.Field == proto.FButtonPin && e.Index == 3)
		dz = dz || e.Field == proto.FOuterDz
	}
	if !pin || !dz {
		t.Fatalf("expected pin and deadzone errors, got %+v", v.Errors)
	}

	m.Validate(s.Config, 43)
	time.Sleep(300 * time.Millisecond)
	if v := m.Snapshot().Validation; v.DraftHash != 43 || len(v.Errors) != 0 {
		t.Fatalf("defaults should be valid: %+v", v)
	}
}

func TestSimSaveRebootVerify(t *testing.T) {
	m := simManager(t)
	cfg := m.Snapshot().Config
	cfg.UsbProduct.SetString("test stick")
	cfg.Axes[2].Flags = proto.AxisLinear
	cfg.Groups[0].Enabled = 1
	cfg.HidRateHz = 500
	m.Do(Op{Kind: OpWriteConfig, Config: cfg})
	r := waitResult(t, m, OpWriteConfig)
	if !r.OK {
		t.Fatalf("save failed: %+v", r)
	}
	s := waitState(t, m, Ready, 10*time.Second)
	if s.Config != cfg {
		t.Fatal("config read back after reboot differs")
	}
	if s.Hello.ResetReason != proto.ResetPowerOn && s.Hello.StorageSeq != 1 {
		t.Fatalf("unexpected hello after save: %+v", s.Hello)
	}

	// an invalid save is rejected with reasons and nothing restarts
	bad := cfg
	bad.NumLedPins = 99
	m.Do(Op{Kind: OpWriteConfig, Config: bad})
	r = waitResult(t, m, OpWriteConfig)
	if r.OK || len(r.Errors) == 0 {
		t.Fatalf("invalid save should be rejected: %+v", r)
	}
	if m.Snapshot().State != Ready {
		t.Fatal("device should stay connected after a rejected save")
	}

	// factory reset brings defaults back
	m.Do(Op{Kind: OpFactoryReset})
	if r := waitResult(t, m, OpFactoryReset); !r.OK {
		t.Fatalf("factory reset: %+v", r)
	}
	time.Sleep(200 * time.Millisecond)
	s = waitState(t, m, Ready, 10*time.Second)
	if s.Config.UsbProduct.String() != "gamepad" || s.Hello.Flags&proto.HelloStorageEmpty == 0 {
		t.Fatalf("expected defaults after factory reset, got %q flags %x", s.Config.UsbProduct, s.Hello.Flags)
	}
}

func TestSimCalibrationAndPreview(t *testing.T) {
	m := simManager(t)
	m.Do(Op{Kind: OpCalStart})
	waitTelemetry(t, m, func(t proto.Telemetry) bool { return t.State&proto.StateCalibrating != 0 })
	m.Do(Op{Kind: OpCalCancel})
	if r := waitResult(t, m, OpCalCancel); !r.OK {
		t.Fatalf("cancel: %+v", r)
	}
	waitTelemetry(t, m, func(t proto.Telemetry) bool { return t.State&proto.StateCalibrating == 0 })

	cfg := m.Snapshot().Config
	cfg.Brightness = 123
	m.SetPreview(&cfg)
	waitTelemetry(t, m, func(t proto.Telemetry) bool { return t.State&proto.StatePreview != 0 })
	m.SetPreview(nil)
	waitTelemetry(t, m, func(t proto.Telemetry) bool { return t.State&proto.StatePreview == 0 })
}
