package snappy

import (
	"hash/crc32"
	"io"

	"github.com/andybalholm/pack"
)

type Encoder struct {
	wroteHeader bool
}

var magicChunk = []byte("\xff\x06\x00\x00sNaPpY")

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// crc implements the checksum specified in section 3 of
// https://github.com/google/snappy/blob/master/framing_format.txt
func crc(b []byte) uint32 {
	c := crc32.Update(0, crcTable, b)
	return uint32(c>>15|c<<17) + 0xa282ead8
}

func (e *Encoder) Reset() {
	e.wroteHeader = false
}

func (e *Encoder) Encode(dst []byte, src []byte, matches []pack.Match, lastBlock bool) []byte {
	if len(src) > 65536 {
		panic("block too large")
	}

	if !e.wroteHeader {
		dst = append(dst, magicChunk...)
		e.wroteHeader = true
	}

	start := len(dst)
	checksum := crc(src)

	dst = append(dst,
		0,       // chunk type: compressed data
		0, 0, 0, // placeholder for compressed length
		byte(checksum), byte(checksum>>8), byte(checksum>>16), byte(checksum>>24),
	)
	dataStart := len(dst)

	dst = appendUvarint(dst, uint64(len(src)))

	pos := 0
	for _, m := range matches {
		if m.Unmatched > 0 {
			dst = appendLiteral(dst, src[pos:pos+m.Unmatched])
			pos += m.Unmatched
		}
		if m.Length > 0 {
			dst = appendCopy(dst, m.Length, m.Distance)
			pos += m.Length
		}
	}
	if pos < len(src) {
		dst = appendLiteral(dst, src[pos:])
	}

	dataLen := len(dst) - dataStart
	if dataLen >= len(src)-len(src)/8 {
		// The compression isn't saving even 12.5%.
		// Just do an uncompressed chunk.
		dst = append(dst[:dataStart], src...)
		dst[start] = 1 // chunk type: uncompressed data
		dataLen = len(src)
	}

	chunkLen := dataLen + 4
	dst[start+1] = byte(chunkLen)
	dst[start+2] = byte(chunkLen >> 8)
	dst[start+3] = byte(chunkLen >> 16)

	return dst
}

const (
	tagLiteral = 0x00
	tagCopy1   = 0x01
	tagCopy2   = 0x02
	tagCopy4   = 0x03
)

func appendLiteral(dst, lit []byte) []byte {
	n := len(lit) - 1
	switch {
	case n < 60:
		dst = append(dst, byte(n)<<2|tagLiteral)
	case n < 1<<8:
		dst = append(dst, 60<<2|tagLiteral, byte(n))
	default:
		dst = append(dst, 61<<2|tagLiteral, byte(n), byte(n>>8))
	}
	return append(dst, lit...)
}

func appendCopy(dst []byte, length, offset int) []byte {
	// The maximum length for a single tagCopy1 or tagCopy2 op is 64 bytes. The
	// threshold for this loop is a little higher (at 68 = 64 + 4), and the
	// length emitted down below is is a little lower (at 60 = 64 - 4), because
	// it's shorter to encode a length 67 copy as a length 60 tagCopy2 followed
	// by a length 7 tagCopy1 (which encodes as 3+2 bytes) than to encode it as
	// a length 64 tagCopy2 followed by a length 3 tagCopy2 (which encodes as
	// 3+3 bytes). The magic 4 in the 64±4 is because the minimum length for a
	// tagCopy1 op is 4 bytes, which is why a length 3 copy has to be an
	// encodes-as-3-bytes tagCopy2 instead of an encodes-as-2-bytes tagCopy1.
	for length >= 68 {
		// Emit a length 64 copy, encoded as 3 bytes.
		dst = append(dst,
			63<<2|tagCopy2,
			byte(offset),
			byte(offset>>8),
		)
		length -= 64
	}
	if length > 64 {
		// Emit a length 60 copy, encoded as 3 bytes.
		dst = append(dst,
			59<<2|tagCopy2,
			byte(offset),
			byte(offset>>8),
		)
		length -= 60
	}
	if length >= 12 || offset >= 2048 {
		// Emit the remaining copy, encoded as 3 bytes.
		return append(dst,
			byte(length-1)<<2|tagCopy2,
			byte(offset),
			byte(offset>>8),
		)
	}
	// Emit the remaining copy, encoded as 2 bytes.
	return append(dst,
		byte(offset>>8)<<5|byte(length-4)<<2|tagCopy1,
		byte(offset),
	)
}

// appendUvarint appends x to dst in varint format.
func appendUvarint(dst []byte, x uint64) []byte {
	for x >= 0x80 {
		dst = append(dst, byte(x)|0x80)
		x >>= 7
	}
	return append(dst, byte(x))
}

func NewWriter(dst io.Writer) *pack.Writer {
	return &pack.Writer{
		Dest:        dst,
		MatchFinder: MatchFinder{},
		Encoder:     &Encoder{},
		BlockSize:   65536,
	}
}
