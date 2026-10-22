package device

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/logbuf"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/proto"
	"github.com/gethiox/rp2350-gamepad/configurator/internal/settings"
)

type ConnState int

const (
	Disconnected ConnState = iota
	Connecting
	Ready
	Rebooting    // waiting for the device to come back after a save / reset
	Incompatible // firmware speaks a different protocol version
)

func (s ConnState) String() string {
	return [...]string{"Disconnected", "Connecting", "Ready", "Rebooting", "Incompatible"}[s]
}

// Validation is the device's verdict on a particular draft.
type Validation struct {
	DraftHash uint64
	Done      bool
	Errors    []proto.ErrorEntry
	Truncated bool
	Err       error // transport problem (verdict unknown)
}

// Snapshot is a consistent copy of everything the UI shows about the device.
type Snapshot struct {
	State        ConnState
	Port         string
	Simulator    bool
	Candidates   []Candidate
	Incompatible string
	Busy         string // long running operation in progress ("" = idle)
	LastError    string // last connection problem (shown while disconnected)
	Permission   bool   // LastError is an OS permission problem

	Hello       proto.Hello
	Limits      proto.Limits
	Config      proto.Config // as stored on / used by the device
	Calibration proto.Calibration
	ConfigGen   uint64 // increments whenever Config is (re)read
	CalGen      uint64

	Telemetry     proto.Telemetry
	TelemetryGen  uint64
	TelemetryTime time.Time

	Validation Validation

	Timeouts  int
	CRCErrors int
}

// OpKind identifies a user-triggered device operation.
type OpKind int

const (
	OpWriteConfig OpKind = iota
	OpWriteCalibration
	OpCalStart
	OpCalFinish
	OpCalCancel
	OpFactoryReset
	OpResetCalibration
	OpReboot
)

var opNames = [...]string{"Save configuration", "Restore calibration", "Start calibration", "Finish calibration",
	"Cancel calibration", "Factory reset", "Reset calibration", "Reboot"}

func (k OpKind) String() string { return opNames[k] }

type Op struct {
	Kind        OpKind
	Config      proto.Config
	Calibration proto.Calibration
}

// Result reports the outcome of an Op.
type Result struct {
	Kind      OpKind
	OK        bool
	Message   string
	Errors    []proto.ErrorEntry
	CalResult *proto.CalResult
}

// Manager owns the connection. All methods are safe for concurrent use.
type Manager struct {
	finder Finder

	mu         sync.Mutex
	snap       Snapshot
	ids        []settings.DeviceID
	preferPort string
	ops        []Op
	validate   *proto.Config
	validateH  uint64
	preview    *proto.Config // desired preview (nil = none)
	previewSet bool          // preview changed since last sent
	retry      bool          // user asked to retry an incompatible device

	results chan Result
	wake    chan struct{}
	stop    chan struct{}
	done    chan struct{}

	// worker-only state
	t        Transport
	rx       chan proto.Frame
	rxErr    chan error
	rxQuit   chan struct{}
	badPorts map[string]bool // incompatible devices, skipped until unplugged
	failures map[string]int  // consecutive failed connection attempts per port

	TelemetryInterval time.Duration
}

func NewManager(f Finder, ids []settings.DeviceID) *Manager {
	return &Manager{
		finder:            f,
		ids:               append([]settings.DeviceID(nil), ids...),
		results:           make(chan Result, 32),
		wake:              make(chan struct{}, 1),
		stop:              make(chan struct{}),
		done:              make(chan struct{}),
		badPorts:          map[string]bool{},
		failures:          map[string]int{},
		TelemetryInterval: 16 * time.Millisecond,
	}
}

func (m *Manager) Start() { go m.run() }

func (m *Manager) Stop() {
	close(m.stop)
	<-m.done
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.snap
	s.Candidates = append([]Candidate(nil), m.snap.Candidates...)
	s.Validation.Errors = append([]proto.ErrorEntry(nil), m.snap.Validation.Errors...)
	return s
}

// Results delivers operation outcomes; the UI drains it every frame.
func (m *Manager) Results() <-chan Result { return m.results }

func (m *Manager) poke() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) SetDeviceIDs(ids []settings.DeviceID) {
	m.mu.Lock()
	m.ids = append([]settings.DeviceID(nil), ids...)
	m.mu.Unlock()
}

// SelectPort switches to another detected device.
func (m *Manager) SelectPort(port string) {
	m.mu.Lock()
	m.preferPort = port
	m.mu.Unlock()
	m.poke()
}

// Retry clears the "incompatible" verdict and reconnects.
func (m *Manager) Retry() {
	m.mu.Lock()
	m.retry = true
	m.mu.Unlock()
	m.poke()
}

// Validate asks the device to check a draft; only the newest request is kept.
func (m *Manager) Validate(cfg proto.Config, hash uint64) {
	m.mu.Lock()
	c := cfg
	m.validate, m.validateH = &c, hash
	m.mu.Unlock()
	m.poke()
}

// SetPreview shows the draft's LED look on the device (nil stops it).
func (m *Manager) SetPreview(cfg *proto.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if (cfg == nil) == (m.preview == nil) && (cfg == nil || *cfg == *m.preview) {
		return
	}
	if cfg != nil {
		c := *cfg
		m.preview = &c
	} else {
		m.preview = nil
	}
	m.previewSet = true
	m.poke()
}

// Do queues an operation; its Result arrives on Results().
func (m *Manager) Do(op Op) {
	m.mu.Lock()
	m.ops = append(m.ops, op)
	m.mu.Unlock()
	m.poke()
}

func (m *Manager) update(f func(s *Snapshot)) {
	m.mu.Lock()
	f(&m.snap)
	m.mu.Unlock()
}

func (m *Manager) report(r Result) {
	if r.OK {
		logbuf.Infof("%s: %s", r.Kind, r.Message)
	} else {
		logbuf.Errorf("%s failed: %s", r.Kind, r.Message)
		for _, e := range r.Errors {
			logbuf.Warnf("  %s", e.Text())
		}
	}
	select {
	case m.results <- r:
	default:
	}
}

func (m *Manager) stopped() bool {
	select {
	case <-m.stop:
		return true
	default:
		return false
	}
}

func (m *Manager) sleep(d time.Duration) {
	select {
	case <-m.stop:
	case <-m.wake:
	case <-time.After(d):
	}
}

// --- worker ---------------------------------------------------------------

func (m *Manager) run() {
	defer close(m.done)
	defer m.closeTransport()
	var rebootDeadline time.Time
	var expect *proto.Config // config we expect to read back after a save
	var lastScanLog string

	for !m.stopped() {
		m.mu.Lock()
		state := m.snap.State
		ids := append([]settings.DeviceID(nil), m.ids...)
		prefer := m.preferPort
		if m.retry {
			m.badPorts = map[string]bool{}
			m.failures = map[string]int{}
			m.retry = false
			if state == Incompatible {
				state = Disconnected
				m.snap.State = Disconnected
			}
		}
		m.mu.Unlock()

		if state == Rebooting && time.Now().After(rebootDeadline) {
			logbuf.Errorf("The device did not come back after rebooting. Check the USB connection.")
			if expect != nil {
				m.report(Result{Kind: OpWriteConfig, OK: false, Message: "the device did not reconnect after saving; reconnect it and check the settings"})
				expect = nil
			}
			m.update(func(s *Snapshot) { s.State = Disconnected; s.Busy = "" })
			continue
		}

		cands, err := m.finder.Find(ids)
		if err != nil {
			logbuf.LimitedLogf("enum", 30*time.Second, logbuf.Warn, "Listing serial ports failed: %v", err)
		}
		sort.Slice(cands, func(i, j int) bool { return cands[i].Port < cands[j].Port })
		present := map[string]bool{}
		for _, c := range cands {
			present[c.Port] = true
		}
		for p := range m.badPorts { // unplugged: forget the verdict
			if !present[p] {
				delete(m.badPorts, p)
			}
		}
		for p := range m.failures {
			if !present[p] {
				delete(m.failures, p)
			}
		}
		m.update(func(s *Snapshot) { s.Candidates = cands })

		var pick *Candidate
		for i := range cands {
			if m.badPorts[cands[i].Port] {
				continue
			}
			if pick == nil || cands[i].Port == prefer {
				pick = &cands[i]
			}
		}
		if pick == nil {
			if state != Rebooting && state != Incompatible {
				m.update(func(s *Snapshot) { s.State = Disconnected; s.Port = "" })
			}
			msg := fmt.Sprintf("Searching for a device (%s)…", idList(ids))
			if msg != lastScanLog && state != Incompatible {
				logbuf.Infof("%s", msg)
				lastScanLog = msg
			}
			m.sleep(500 * time.Millisecond)
			continue
		}
		lastScanLog = ""

		// A port that keeps failing is retried quietly (with a growing pause)
		// instead of flipping the screen to "Connecting" every second.
		quiet := m.failures[pick.Port] > 0
		retryPause := time.Duration(min(1+m.failures[pick.Port], 5)) * time.Second
		if state != Rebooting && !quiet {
			m.update(func(s *Snapshot) { s.State = Connecting; s.Port = pick.Port })
		}
		if err := m.connect(*pick); err != nil {
			m.failures[pick.Port]++
			perm := errors.Is(err, ErrPermission)
			if perm {
				logbuf.LimitedLogf("perm"+pick.Port, time.Minute, logbuf.Error,
					"No permission to open %s. On Linux install the udev rule from the release (see docs/linux.md) and re-plug the device.", pick.Port)
			} else {
				logbuf.LimitedLogf("open"+pick.Port, 10*time.Second, logbuf.Warn, "Could not open %s: %v", pick.Port, err)
			}
			m.update(func(s *Snapshot) {
				s.LastError = fmt.Sprintf("Could not open %s: %v", pick.Port, err)
				s.Permission = perm
			})
			if state != Rebooting {
				m.update(func(s *Snapshot) { s.State = Disconnected })
			}
			m.sleep(retryPause)
			continue
		}

		reason, err := m.handshake(*pick)
		if err != nil {
			m.failures[pick.Port]++
			logbuf.LimitedLogf("conn"+pick.Port, 30*time.Second, logbuf.Warn, "Connecting to %s failed: %v", pick.Port, err)
			m.update(func(s *Snapshot) {
				s.LastError = fmt.Sprintf("%s did not answer: %v", pick.Port, err)
				s.Permission = false
			})
			m.closeTransport()
			if state != Rebooting {
				m.update(func(s *Snapshot) { s.State = Disconnected })
			}
			m.sleep(retryPause)
			continue
		}
		delete(m.failures, pick.Port)
		if reason != "" {
			logbuf.Errorf("%s", reason)
			m.badPorts[pick.Port] = true
			m.closeTransport()
			m.update(func(s *Snapshot) { s.State = Incompatible; s.Incompatible = reason; s.Busy = "" })
			continue
		}

		if expect != nil {
			snap := m.Snapshot()
			if snap.Config == *expect {
				m.report(Result{Kind: OpWriteConfig, OK: true, Message: "saved and verified after reboot"})
			} else {
				m.report(Result{Kind: OpWriteConfig, OK: false, Message: "the device restarted with a different configuration than the one saved"})
			}
			expect = nil
		}
		m.update(func(s *Snapshot) { s.State = Ready; s.Busy = "" })

		next, exp := m.serve()
		m.closeTransport()
		if m.stopped() {
			return
		}
		if next == Rebooting {
			rebootDeadline = time.Now().Add(12 * time.Second)
			expect = exp
			m.update(func(s *Snapshot) { s.State = Rebooting })
			m.sleep(400 * time.Millisecond) // let the old port disappear
		} else {
			m.update(func(s *Snapshot) { s.State = Disconnected; s.Busy = "" })
			m.failPendingOps("device disconnected")
		}
	}
}

func idList(ids []settings.DeviceID) string {
	s := ""
	for i, id := range ids {
		if i > 0 {
			s += ", "
		}
		s += id.String()
	}
	return s
}

func (m *Manager) failPendingOps(msg string) {
	m.mu.Lock()
	ops := m.ops
	m.ops = nil
	m.mu.Unlock()
	for _, op := range ops {
		m.report(Result{Kind: op.Kind, OK: false, Message: msg})
	}
}

func (m *Manager) connect(c Candidate) error {
	t, err := m.finder.Open(c)
	if err != nil {
		return err
	}
	m.t = t
	m.rx = make(chan proto.Frame, 64)
	m.rxErr = make(chan error, 1)
	m.rxQuit = make(chan struct{})
	go m.reader(t, m.rx, m.rxErr, m.rxQuit)
	logbuf.Infof("Opened %s", c.Label)
	return nil
}

func (m *Manager) reader(t Transport, rx chan<- proto.Frame, rxErr chan<- error, quit <-chan struct{}) {
	var p proto.Parser
	buf := make([]byte, 4096)
	crc := 0
	for {
		n, err := t.Read(buf)
		if n > 0 {
			for _, f := range p.Feed(buf[:n]) {
				select {
				case rx <- f:
				case <-quit:
					return
				}
			}
			if p.CRCErrors != crc {
				crc = p.CRCErrors
				m.update(func(s *Snapshot) { s.CRCErrors = crc })
			}
		}
		if err != nil {
			rxErr <- err
			return
		}
	}
}

func (m *Manager) closeTransport() {
	if m.t != nil {
		close(m.rxQuit)
		_ = m.t.Close()
		m.t = nil
	}
}

var errTimeout = errors.New("no response from device")
var errDisconnected = errors.New("device disconnected")

// transact sends one request and waits for the response with the same
// command code. Returns the payload (status byte first).
func (m *Manager) transact(cmd byte, payload []byte, timeout time.Duration) ([]byte, error) {
	if m.t == nil {
		return nil, errDisconnected
	}
	if _, err := m.t.Write(proto.EncodeFrame(cmd, payload)); err != nil {
		return nil, fmt.Errorf("%w: %v", errDisconnected, err)
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case f := <-m.rx:
			if f.Cmd == cmd {
				if len(f.Payload) == 0 {
					return nil, fmt.Errorf("empty response")
				}
				return f.Payload, nil
			}
			// stale response to an earlier (timed out) request: ignore
		case err := <-m.rxErr:
			if errors.Is(err, io.EOF) {
				return nil, errDisconnected
			}
			return nil, fmt.Errorf("%w: %v", errDisconnected, err)
		case <-deadline.C:
			m.update(func(s *Snapshot) { s.Timeouts++ })
			return nil, errTimeout
		case <-m.stop:
			return nil, errDisconnected
		}
	}
}

// read issues a command and decodes a status + struct response, retrying
// a couple of times (a response may be lost if the USB buffer was full).
func (m *Manager) read(cmd byte, v any) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		p, err := m.transact(cmd, nil, 500*time.Millisecond)
		if err != nil {
			lastErr = err
			if errors.Is(err, errDisconnected) {
				return err
			}
			continue
		}
		if p[0] != proto.StatusOk {
			return fmt.Errorf("device returned %s", statusText(p[0]))
		}
		return proto.Decode(p[1:], v)
	}
	return lastErr
}

func statusText(s byte) string {
	switch s {
	case proto.StatusOk:
		return "OK"
	case proto.StatusInvalid:
		return "invalid data"
	case proto.StatusBadLength:
		return "unexpected data size (version mismatch?)"
	case proto.StatusBadState:
		return "not possible right now"
	case proto.StatusStorage:
		return "flash write failed"
	case proto.StatusUnknownCmd:
		return "unknown command (firmware too old?)"
	}
	return fmt.Sprintf("status %d", s)
}

// handshake reads everything the UI needs. A non-empty reason means the
// device is reachable but incompatible.
func (m *Manager) handshake(c Candidate) (string, error) {
	var h proto.Hello
	// The first frame may get lost while the port settles; retry HELLO.
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		var p []byte
		p, err = m.transact(proto.CmdHello, nil, 400*time.Millisecond)
		if err == nil {
			if p[0] != proto.StatusOk {
				// HELLO never fails on this firmware: the device answers,
				// but speaks something else (usually older firmware).
				return fmt.Sprintf("The device on %s answered, but not in this configurator's protocol (status %d) - "+
					"it probably runs older or different firmware. Flash the firmware from the same release as this configurator.",
					c.Port, p[0]), nil
			}
			if len(p)-1 < 2 {
				return "", fmt.Errorf("hello response too short")
			}
			ver := uint16(p[1]) | uint16(p[2])<<8
			if ver != proto.ProtoVersion {
				return fmt.Sprintf("This device uses protocol version %d, but this configurator supports version %d. "+
					"Use the configurator from the same release as the firmware.", ver, proto.ProtoVersion), nil
			}
			err = proto.Decode(p[1:], &h)
			break
		}
		if errors.Is(err, errDisconnected) {
			return "", err
		}
	}
	if err != nil {
		return "", err
	}
	var wantCfg proto.Config
	var wantCal proto.Calibration
	var wantTel proto.Telemetry
	var wantLim proto.Limits
	if int(h.ConfigSize) != len(proto.Encode(wantCfg)) || int(h.CalSize) != len(proto.Encode(wantCal)) ||
		int(h.TelemetrySize) != len(proto.Encode(wantTel)) || int(h.LimitsSize) != len(proto.Encode(wantLim)) ||
		h.ConfigVersion != proto.ConfigVersion || h.CalVersion != proto.CalVersion {
		return "The firmware's data format does not match this configurator. Use the configurator from the same release as the firmware.", nil
	}

	var lim proto.Limits
	var cfg proto.Config
	var cal proto.Calibration
	if err := m.read(proto.CmdGetLimits, &lim); err != nil {
		return "", fmt.Errorf("reading limits: %w", err)
	}
	if err := m.read(proto.CmdReadConfig, &cfg); err != nil {
		return "", fmt.Errorf("reading configuration: %w", err)
	}
	if err := m.read(proto.CmdReadCalibration, &cal); err != nil {
		return "", fmt.Errorf("reading calibration: %w", err)
	}

	logbuf.Infof("Connected: firmware %s on %s (%s), protocol %d, %d axes / %d GPIOs available",
		h.FwVersion, h.Board, c.Port, h.ProtoVersion, lim.PlatformAxes, lim.GpioCount)
	if h.Flags&proto.HelloSafeMode != 0 {
		logbuf.Errorf("The device restarted %d times in a row and is running in SAFE MODE with default settings. Saving a configuration or a factory reset fixes this.", h.CrashCount)
	} else if h.ResetReason == proto.ResetWatchdog {
		logbuf.Warnf("The device recovered from a hang or crash (watchdog reset).")
	}
	if h.Flags&proto.HelloStorageEmpty != 0 {
		logbuf.Infof("No saved settings on the device yet - using defaults.")
	}
	if h.Flags&proto.HelloConfigDefaulted != 0 {
		logbuf.Warnf("The stored configuration could not be used (older format or invalid); defaults are active.")
	}
	if h.Flags&proto.HelloCalDefaulted != 0 {
		logbuf.Warnf("The stored calibration could not be used; please calibrate again.")
	}
	if h.Flags&proto.HelloStorageRecovered != 0 {
		logbuf.Warnf("A damaged settings copy was found (interrupted save?). The previous good copy is in use.")
	}

	m.update(func(s *Snapshot) {
		s.Hello, s.Limits, s.Config, s.Calibration = h, lim, cfg, cal
		s.ConfigGen++
		s.CalGen++
		s.Port = c.Port
		s.Simulator = h.Flags&proto.HelloSimulator != 0
		s.Incompatible = ""
		s.LastError, s.Permission = "", false
		s.Validation = Validation{}
	})
	return "", nil
}

// serve runs the request loop while connected. Returns Rebooting (with the
// config expected after reboot, if any) when the device is about to restart.
func (m *Manager) serve() (ConnState, *proto.Config) {
	lastTel := time.Time{}
	lastPreview := time.Time{}
	previewActive := false
	telFails := 0
	wasCalibrating := false

	for !m.stopped() {
		// 1. user operations
		m.mu.Lock()
		var op *Op
		if len(m.ops) > 0 {
			o := m.ops[0]
			m.ops = m.ops[1:]
			op = &o
		}
		m.mu.Unlock()
		if op != nil {
			next, exp, err := m.runOp(*op)
			if err != nil && errors.Is(err, errDisconnected) {
				return Disconnected, nil
			}
			if next == Rebooting {
				return Rebooting, exp
			}
			continue
		}

		// 2. validation of the newest draft
		m.mu.Lock()
		v, vh := m.validate, m.validateH
		m.validate = nil
		m.mu.Unlock()
		if v != nil {
			p, err := m.transact(proto.CmdValidateConfig, proto.Encode(*v), 800*time.Millisecond)
			res := Validation{DraftHash: vh, Done: true}
			if err != nil {
				if errors.Is(err, errDisconnected) {
					return Disconnected, nil
				}
				res.Err = err
			} else if el, err := proto.DecodeErrorList(p); err != nil {
				res.Err = err
			} else {
				res.Errors, res.Truncated = el.Errors, el.Truncated
				if el.Status != proto.StatusOk && len(el.Errors) == 0 {
					res.Err = fmt.Errorf("device returned %s", statusText(el.Status))
				}
			}
			m.update(func(s *Snapshot) { s.Validation = res })
			continue
		}

		// 3. LED preview (changes immediately, keep-alive every 500 ms)
		m.mu.Lock()
		pv, changed := m.preview, m.previewSet
		m.previewSet = false
		m.mu.Unlock()
		if pv != nil && (changed || time.Since(lastPreview) > 500*time.Millisecond) {
			lastPreview = time.Now()
			if _, err := m.transact(proto.CmdLedPreview, proto.Encode(*pv), 400*time.Millisecond); errors.Is(err, errDisconnected) {
				return Disconnected, nil
			}
			previewActive = true
			continue
		}
		if pv == nil && previewActive {
			previewActive = false
			if _, err := m.transact(proto.CmdLedPreviewStop, nil, 400*time.Millisecond); errors.Is(err, errDisconnected) {
				return Disconnected, nil
			}
			continue
		}

		// 4. telemetry
		if wait := m.TelemetryInterval - time.Since(lastTel); wait > 0 {
			m.sleep(wait)
			continue
		}
		lastTel = time.Now()
		var t proto.Telemetry
		p, err := m.transact(proto.CmdTelemetry, nil, 300*time.Millisecond)
		if err == nil && p[0] == proto.StatusOk {
			err = proto.Decode(p[1:], &t)
		} else if err == nil {
			err = fmt.Errorf("telemetry: %s", statusText(p[0]))
		}
		if err != nil {
			if errors.Is(err, errDisconnected) {
				logbuf.Warnf("Device disconnected.")
				return Disconnected, nil
			}
			telFails++
			logbuf.LimitedLogf("tel", 10*time.Second, logbuf.Warn, "Telemetry: %v", err)
			if telFails >= 8 {
				logbuf.Errorf("The device stopped responding; reconnecting.")
				return Disconnected, nil
			}
			continue
		}
		telFails = 0
		m.update(func(s *Snapshot) {
			s.Telemetry = t
			s.TelemetryGen++
			s.TelemetryTime = time.Now()
		})
		// A calibration may also be finished with the button on the device:
		// re-read the calibration whenever a session ends.
		calibrating := t.State&proto.StateCalibrating != 0
		if calibrating && !wasCalibrating {
			logbuf.Infof("Calibration is running on the device.")
		}
		if wasCalibrating && !calibrating {
			m.reloadCalibration()
		}
		wasCalibrating = calibrating
	}
	return Disconnected, nil
}

func (m *Manager) setBusy(b string) { m.update(func(s *Snapshot) { s.Busy = b }) }

// runOp executes one operation. Errors are reported through Results; the
// returned error is only used to detect disconnects.
func (m *Manager) runOp(op Op) (ConnState, *proto.Config, error) {
	fail := func(msg string, errs []proto.ErrorEntry) {
		m.report(Result{Kind: op.Kind, OK: false, Message: msg, Errors: errs})
	}
	simple := func(cmd byte, timeout time.Duration, okMsg string) ([]byte, error) {
		p, err := m.transact(cmd, nil, timeout)
		if err != nil {
			fail(err.Error(), nil)
			return nil, err
		}
		if p[0] != proto.StatusOk {
			fail(statusText(p[0]), nil)
			return p, nil
		}
		if okMsg != "" {
			m.report(Result{Kind: op.Kind, OK: true, Message: okMsg})
		}
		return p, nil
	}

	switch op.Kind {
	case OpWriteConfig:
		m.setBusy("Saving…")
		defer m.setBusy("")
		p, err := m.transact(proto.CmdWriteConfig, proto.Encode(op.Config), 4*time.Second)
		if err != nil {
			fail(err.Error()+" (the device may or may not have saved; it will be re-read on reconnect)", nil)
			return Ready, nil, err
		}
		el, derr := proto.DecodeErrorList(p)
		if derr != nil {
			fail(derr.Error(), nil)
			return Ready, nil, nil
		}
		if el.Status != proto.StatusOk {
			msg := statusText(el.Status)
			if el.Status == proto.StatusInvalid {
				msg = "the device rejected the configuration"
			} else if len(el.Errors) == 1 {
				msg = el.Errors[0].Text()
			}
			fail(msg, el.Errors)
			return Ready, nil, nil
		}
		logbuf.Infof("Configuration saved on the device; it is restarting to apply it…")
		m.setBusy("Restarting…")
		cfg := op.Config
		return Rebooting, &cfg, nil

	case OpWriteCalibration:
		p, err := m.transact(proto.CmdWriteCalibration, proto.Encode(op.Calibration), 4*time.Second)
		if err != nil {
			fail(err.Error(), nil)
			return Ready, nil, err
		}
		el, derr := proto.DecodeErrorList(p)
		if derr != nil || el.Status != proto.StatusOk {
			msg := statusText(el.Status)
			if derr != nil {
				msg = derr.Error()
			}
			fail(msg, el.Errors)
			return Ready, nil, nil
		}
		m.reloadCalibration()
		m.report(Result{Kind: op.Kind, OK: true, Message: "calibration restored and saved"})

	case OpResetCalibration:
		if p, err := simple(proto.CmdResetCalibration, 4*time.Second, "calibration cleared"); err != nil || p[0] != proto.StatusOk {
			return Ready, nil, err
		}
		m.reloadCalibration()

	case OpCalStart:
		_, err := simple(proto.CmdCalStart, time.Second, "")
		if err != nil {
			return Ready, nil, err
		}
		logbuf.Infof("Calibration started.")

	case OpCalCancel:
		_, err := simple(proto.CmdCalCancel, time.Second, "calibration cancelled, nothing changed")
		if err != nil {
			return Ready, nil, err
		}

	case OpCalFinish:
		p, err := m.transact(proto.CmdCalFinish, nil, 4*time.Second)
		if err != nil {
			fail(err.Error(), nil)
			return Ready, nil, err
		}
		var res proto.CalResult
		if len(p) == 1+len(proto.Encode(res)) {
			_ = proto.Decode(p[1:], &res)
		}
		if p[0] != proto.StatusOk {
			fail(statusText(p[0]), nil)
		} else {
			m.report(Result{Kind: op.Kind, OK: true, Message: "calibration saved", CalResult: &res})
		}
		m.reloadCalibration()

	case OpFactoryReset, OpReboot:
		cmd := byte(proto.CmdFactoryReset)
		if op.Kind == OpReboot {
			cmd = proto.CmdReboot
		}
		p, err := simple(cmd, time.Second, "")
		if err != nil || p[0] != proto.StatusOk {
			return Ready, nil, err
		}
		if op.Kind == OpFactoryReset {
			m.report(Result{Kind: op.Kind, OK: true, Message: "settings erased; the device is restarting with defaults"})
		}
		m.setBusy("Restarting…")
		return Rebooting, nil, nil
	}
	return Ready, nil, nil
}

func (m *Manager) reloadCalibration() {
	var cal proto.Calibration
	if err := m.read(proto.CmdReadCalibration, &cal); err != nil {
		logbuf.Warnf("Reading calibration failed: %v", err)
		return
	}
	m.update(func(s *Snapshot) {
		s.Calibration = cal
		s.CalGen++
	})
}
