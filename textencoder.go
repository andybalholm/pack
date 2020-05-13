package press

import "fmt"

// A TextEncoder is an Encoder that produces a human-readable representation of
// the LZ77 compression. Matches are replaced with <Length,Distance> symbols.
type TextEncoder struct{}

func (t TextEncoder) Header(dst []byte) []byte {
	return dst
}

func (t TextEncoder) Reset() {}

func (t TextEncoder) Encode(dst []byte, src []byte, matches []Match, lastBlock bool) []byte {
	pos := 0
	for _, m := range matches {
		if m.Unmatched > 0 {
			dst = append(dst, src[pos:pos+m.Unmatched]...)
			pos += m.Unmatched
		}
		if m.Length > 0 {
			dst = append(dst, []byte(fmt.Sprintf("<%d,%d>", m.Length, m.Distance))...)
			pos += m.Length
		}
	}
	if pos < len(src) {
		dst = append(dst, src[pos:]...)
	}
	return dst
}
