package lz4

import (
	"encoding/binary"

	"github.com/andybalholm/pack"
)

// A BlockEncoder implements the pack.Encoder interface, writing in the LZ4
// block format.
type BlockEncoder struct{}

func (BlockEncoder) Reset() {}

func (BlockEncoder) Encode(dst []byte, src []byte, matches []pack.Match, lastBlock bool) []byte {
	// Ensure that the block ends with at least 5 literal bytes,
	// and the last match is at least 12 bytes before the end of the block.
	trailingLiterals := 0
	for len(matches) > 0 && (trailingLiterals < 5 || trailingLiterals+matches[len(matches)-1].Length < 12) {
		lastMatch := matches[len(matches)-1]
		matches = matches[:len(matches)-1]
		trailingLiterals += lastMatch.Unmatched + lastMatch.Length
	}

	pos := 0
	for _, m := range matches {
		token := byte(0)
		if m.Unmatched > 14 {
			token |= 0xf0
		} else {
			token |= byte(m.Unmatched << 4)
		}
		if m.Length > 18 {
			token |= 0x0f
		} else {
			token |= byte(m.Length - 4)
		}
		dst = append(dst, token)

		if m.Unmatched > 14 {
			dst = appendInt(dst, m.Unmatched-15)
		}
		dst = append(dst, src[pos:pos+m.Unmatched]...)

		dst = binary.LittleEndian.AppendUint16(dst, uint16(m.Distance))
		if m.Length > 18 {
			dst = appendInt(dst, m.Length-19)
		}

		pos += m.Unmatched + m.Length
	}

	// Write the final, literals-only sequence.
	token := byte(0)
	if trailingLiterals > 14 {
		token |= 0xf0
	} else {
		token |= byte(trailingLiterals << 4)
	}
	dst = append(dst, token)
	if trailingLiterals > 14 {
		dst = appendInt(dst, trailingLiterals-15)
	}
	dst = append(dst, src[pos:]...)

	return dst
}

// appendInt appends n to dst in LZ4's variable-length integer format.
func appendInt(dst []byte, n int) []byte {
	for n >= 255 {
		dst = append(dst, 255)
		n -= 255
	}
	dst = append(dst, byte(n))
	return dst
}
