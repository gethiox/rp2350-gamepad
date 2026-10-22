package proto

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	payload := bytes.Repeat([]byte{1, 2, 3}, 100)
	f := EncodeFrame(CmdTelemetry, payload)
	var p Parser
	// junk with fake sync bytes in front, then the frame byte by byte
	got := p.Feed([]byte{'A', 'T', '\r', 0xAA, 0x55, 0x01, 0xFF})
	for _, b := range f {
		got = append(got, p.Feed([]byte{b})...)
	}
	if len(got) != 1 || got[0].Cmd != CmdTelemetry || !bytes.Equal(got[0].Payload, payload) {
		t.Fatalf("unexpected frames: %+v", got)
	}
}

func TestFrameCorruptionRecovery(t *testing.T) {
	good := EncodeFrame(CmdHello, []byte{9, 9})
	bad := append([]byte(nil), good...)
	bad[6] ^= 0xFF
	var p Parser
	got := p.Feed(append(bad, good...))
	if len(got) != 1 || p.CRCErrors == 0 {
		t.Fatalf("got %d frames, %d crc errors", len(got), p.CRCErrors)
	}
}

func TestFrameFuzz(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	var p Parser
	sent, got := 0, 0
	for i := 0; i < 5000; i++ {
		junk := make([]byte, r.Intn(40))
		r.Read(junk)
		got += len(p.Feed(junk))
		pl := make([]byte, r.Intn(300))
		r.Read(pl)
		got += len(p.Feed(EncodeFrame(byte(r.Intn(0x20)), pl)))
		sent++
	}
	if got < sent*9/10 {
		t.Fatalf("received %d of %d frames", got, sent)
	}
}

func TestCStr(t *testing.T) {
	var s CStr
	s.SetString("gamepad")
	if s.String() != "gamepad" {
		t.Fatal(s.String())
	}
	s.SetString("x")
	for _, b := range s[1:] {
		if b != 0 {
			t.Fatal("SetString must zero the remainder")
		}
	}
}
