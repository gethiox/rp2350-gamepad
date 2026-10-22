// Package logbuf keeps an in-memory log for the status bar / log drawer and
// mirrors it to a small log file for troubleshooting.
package logbuf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

func (l Level) String() string {
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR"}[l]
}

type Entry struct {
	ID    uint64 // unique, increasing; survives trimming and Clear
	Time  time.Time
	Level Level
	Msg   string
}

// Line formats an entry the way the log view and the clipboard show it.
func (e Entry) Line() string {
	return fmt.Sprintf("%s  %-5s  %s", e.Time.Format("15:04:05.000"), e.Level, e.Msg)
}

// LineMsgCol is the column where the message starts in Line().
const LineMsgCol = 21

const maxEntries = 2000
const maxFileSize = 1 << 20

type Log struct {
	mu      sync.Mutex
	entries []Entry
	seq     uint64 // increments on every entry
	nextID  uint64
	file    *os.File
	path    string
	last    map[string]time.Time // rate limiting for repeated messages
}

var Default = &Log{last: map[string]time.Time{}}

// OpenFile starts mirroring to <cache dir>/rp2350-gamepad/configurator.log.
// The previous file is kept as configurator.log.1.
func (l *Log) OpenFile() {
	dir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	dir = filepath.Join(dir, "rp2350-gamepad")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, "configurator.log")
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	l.mu.Lock()
	l.file, l.path = f, path
	l.mu.Unlock()
}

// Path of the log file ("" if none).
func (l *Log) Path() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.path
}

func (l *Log) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

func (l *Log) add(level Level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.nextID++
	e := Entry{ID: l.nextID, Time: time.Now(), Level: level, Msg: msg}
	l.entries = append(l.entries, e)
	if len(l.entries) > maxEntries {
		l.entries = append(l.entries[:0:0], l.entries[len(l.entries)-maxEntries:]...)
	}
	l.seq++
	if l.file != nil {
		if st, err := l.file.Stat(); err == nil && st.Size() < maxFileSize {
			fmt.Fprintf(l.file, "%s %-5s %s\n", e.Time.Format("2006-01-02 15:04:05.000"), level, msg)
		}
	}
}

func (l *Log) Logf(level Level, format string, args ...any) {
	l.add(level, fmt.Sprintf(format, args...))
}

// LimitedLogf logs at most once per `every` for the given key (for
// repetitive conditions such as timeouts).
func (l *Log) LimitedLogf(key string, every time.Duration, level Level, format string, args ...any) {
	l.mu.Lock()
	t, ok := l.last[key]
	now := time.Now()
	if ok && now.Sub(t) < every {
		l.mu.Unlock()
		return
	}
	l.last[key] = now
	l.mu.Unlock()
	l.Logf(level, format, args...)
}

// Snapshot returns a copy of the entries and the current sequence number.
func (l *Log) Snapshot() ([]Entry, uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Entry(nil), l.entries...), l.seq
}

// Seq changes whenever an entry is added.
func (l *Log) Seq() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.seq
}

// Last returns the most recent entry at or above `min`.
func (l *Log) Last(min Level) (Entry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.entries) - 1; i >= 0; i-- {
		if l.entries[i].Level >= min {
			return l.entries[i], true
		}
	}
	return Entry{}, false
}

// Text renders all entries (for copying to the clipboard).
func (l *Log) Text() string {
	es, _ := l.Snapshot()
	var b strings.Builder
	for _, e := range es {
		b.WriteString(e.Line())
		b.WriteByte('\n')
	}
	return b.String()
}

func (l *Log) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
	l.seq++
}

func Debugf(f string, a ...any) { Default.Logf(Debug, f, a...) }
func Infof(f string, a ...any)  { Default.Logf(Info, f, a...) }
func Warnf(f string, a ...any)  { Default.Logf(Warn, f, a...) }
func Errorf(f string, a ...any) { Default.Logf(Error, f, a...) }

// LimitedLogf logs on the default log at most once per `every` for `key`.
func LimitedLogf(key string, every time.Duration, level Level, f string, a ...any) {
	Default.LimitedLogf(key, every, level, f, a...)
}
