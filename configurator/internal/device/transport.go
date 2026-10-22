// Package device talks to the gamepad: discovery, connection handling and
// the request/response protocol, all on a background goroutine so the UI
// never blocks on I/O.
package device

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"

	"github.com/gethiox/rp2350-gamepad/configurator/internal/settings"
)

// Transport is an open connection to a device.
type Transport interface {
	io.ReadWriteCloser
}

// Candidate is a device that could be connected to.
type Candidate struct {
	Port    string // OS port name, or "simulator"
	Label   string // human readable
	VID     uint16
	PID     uint16
	Serial  string
	Product string
}

// Finder discovers and opens devices.
type Finder interface {
	Find(ids []settings.DeviceID) ([]Candidate, error)
	Open(c Candidate) (Transport, error)
}

// --- serial ports ----------------------------------------------------------

type SerialFinder struct{}

func parseHex(s string) (uint16, bool) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 16, 16)
	return uint16(v), err == nil
}

func (SerialFinder) Find(ids []settings.DeviceID) ([]Candidate, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}
	var out []Candidate
	for _, p := range ports {
		if !p.IsUSB {
			continue
		}
		vid, ok1 := parseHex(p.VID)
		pid, ok2 := parseHex(p.PID)
		if !ok1 || !ok2 {
			continue
		}
		for _, id := range ids {
			if id.VID == vid && id.PID == pid {
				label := p.Name
				if p.SerialNumber != "" {
					label = fmt.Sprintf("%s (serial %s)", p.Name, p.SerialNumber)
				}
				out = append(out, Candidate{Port: p.Name, Label: label, VID: vid, PID: pid, Serial: p.SerialNumber, Product: p.Product})
				break
			}
		}
	}
	return out, nil
}

// ErrPermission is returned when the OS refuses access to the port.
var ErrPermission = errors.New("permission denied")

func (SerialFinder) Open(c Candidate) (Transport, error) {
	port, err := serial.Open(c.Port, &serial.Mode{BaudRate: 115200})
	if err != nil {
		var pe *serial.PortError
		if errors.As(err, &pe) && pe.Code() == serial.PermissionDenied || errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf("%w: %v", ErrPermission, err)
		}
		return nil, err
	}
	// DTR tells the device a host is listening (TinyUSB only transmits
	// reliably once DTR is set).
	_ = port.SetDTR(true)
	_ = port.SetReadTimeout(20 * time.Millisecond)
	_ = port.ResetInputBuffer()
	return &serialTransport{port: port}, nil
}

type serialTransport struct {
	port serial.Port
}

func (s *serialTransport) Read(p []byte) (int, error) {
	n, err := s.port.Read(p)
	if err == nil && n == 0 {
		return 0, nil // read timeout
	}
	return n, err
}
func (s *serialTransport) Write(p []byte) (int, error) { return s.port.Write(p) }
func (s *serialTransport) Close() error                { return s.port.Close() }

// --- simulator ---------------------------------------------------------------

// SimFinder runs the firmware simulator (firmware/host) as a child process
// and talks to it over pipes. Used for development, screenshots and tests.
type SimFinder struct {
	Binary string
	Args   []string

	mu   sync.Mutex
	last chan struct{} // exit channel of the previous simulator process
}

func (f *SimFinder) Find([]settings.DeviceID) ([]Candidate, error) {
	return []Candidate{{Port: "simulator", Label: "Simulator", VID: settings.DefaultDeviceID.VID, PID: settings.DefaultDeviceID.PID}}, nil
}

func (f *SimFinder) Open(Candidate) (Transport, error) {
	// Like a real device that is still rebooting, a new simulator instance
	// must not start before the previous one has finished writing its flash.
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.last != nil {
		select {
		case <-f.last:
		case <-time.After(5 * time.Second):
		}
	}
	cmd := exec.Command(f.Binary, f.Args...)
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	t := &simTransport{cmd: cmd, in: in, out: out, exited: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(t.exited)
	}()
	f.last = t.exited
	return t, nil
}

type simTransport struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    io.ReadCloser
	once   sync.Once
	exited chan struct{}
}

func (s *simTransport) Read(p []byte) (int, error)  { return s.out.Read(p) }
func (s *simTransport) Write(p []byte) (int, error) { return s.in.Write(p) }
func (s *simTransport) Close() error {
	s.once.Do(func() {
		_ = s.in.Close()
		go func() { // the simulator exits by itself shortly after EOF on stdin
			select {
			case <-s.exited:
			case <-time.After(3 * time.Second):
				_ = s.cmd.Process.Kill()
			}
		}()
	})
	return nil
}
