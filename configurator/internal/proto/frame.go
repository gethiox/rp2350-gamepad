package proto

import (
	"encoding/binary"
	"hash/crc32"
)

// Frame is one decoded protocol frame.
type Frame struct {
	Cmd     byte
	Payload []byte
}

// EncodeFrame builds AA 55 | cmd | len | payload | crc32.
func EncodeFrame(cmd byte, payload []byte) []byte {
	n := len(payload)
	out := make([]byte, 0, n+FrameOverhead)
	out = append(out, FrameSync0, FrameSync1, cmd, byte(n), byte(n>>8))
	out = append(out, payload...)
	crc := crc32.ChecksumIEEE(out[2:])
	return binary.LittleEndian.AppendUint32(out, crc)
}

// Parser reassembles frames from a byte stream. Junk and corrupted frames
// are skipped byte by byte (same strategy as the firmware), so a valid frame
// following garbage is never lost.
type Parser struct {
	buf       []byte
	CRCErrors int
	Dropped   int
}

// Feed appends data and returns all complete frames.
func (p *Parser) Feed(data []byte) []Frame {
	p.buf = append(p.buf, data...)
	var out []Frame
	for len(p.buf) >= FrameOverhead {
		if p.buf[0] != FrameSync0 || p.buf[1] != FrameSync1 {
			p.skip(1)
			continue
		}
		n := int(p.buf[3]) | int(p.buf[4])<<8
		if n > ProtoMaxPayload {
			p.skip(1)
			continue
		}
		total := n + FrameOverhead
		if len(p.buf) < total {
			break
		}
		want := binary.LittleEndian.Uint32(p.buf[5+n:])
		if crc32.ChecksumIEEE(p.buf[2:5+n]) != want {
			p.CRCErrors++
			p.skip(1)
			continue
		}
		payload := make([]byte, n)
		copy(payload, p.buf[5:5+n])
		out = append(out, Frame{Cmd: p.buf[2], Payload: payload})
		p.buf = p.buf[total:]
	}
	if cap(p.buf) > 1<<16 && len(p.buf) < 1024 { // don't keep a huge backing array
		p.buf = append([]byte(nil), p.buf...)
	}
	return out
}

func (p *Parser) skip(n int) {
	p.Dropped += n
	p.buf = p.buf[n:]
}

// Reset discards buffered bytes.
func (p *Parser) Reset() { p.buf = p.buf[:0] }
