package pack

import "encoding/binary"

const (
	table8Bits  = 17
	table8Size  = 1 << table8Bits
	table8Mask  = table8Size - 1
	table8Shift = 64 - table8Bits
)

// DualHash is an implementation of the MatchFinder interface
// that uses two hash tables (4-byte and 8-byte).
type DualHash struct {
	// MaxDistance is the maximum distance (in bytes) to look back for
	// a match. The default is 65535.
	MaxDistance int

	Parser Parser

	table4 [maxTableSize]uint32
	table8 [table8Size]uint32

	history []byte

	lastSearch int
}

func (q *DualHash) Reset() {
	q.table4 = [maxTableSize]uint32{}
	q.table8 = [table8Size]uint32{}
	q.history = q.history[:0]
}

// FindMatches looks for matches in src, appends them to dst, and returns dst.
func (q *DualHash) FindMatches(dst []Match, src []byte) []Match {
	if q.MaxDistance == 0 {
		q.MaxDistance = 65535
	}
	var nextEmit int

	if len(q.history) > maxHistory {
		// Trim down the history buffer.
		delta := len(q.history) - minHistory
		copy(q.history, q.history[delta:])
		q.history = q.history[:minHistory]

		for i, v := range q.table4 {
			newV := int(v) - delta
			if newV < 0 {
				newV = 0
			}
			q.table4[i] = uint32(newV)
		}

		for i, v := range q.table8 {
			newV := int(v) - delta
			if newV < 0 {
				newV = 0
			}
			q.table8[i] = uint32(newV)
		}
	}

	// Append src to the history buffer.
	nextEmit = len(q.history)
	q.history = append(q.history, src...)
	src = q.history

	return q.Parser.Parse(dst, q, nextEmit, len(src))
}

func (q *DualHash) Search(dst []AbsoluteMatch, pos, min, max int) []AbsoluteMatch {
	if pos+4 > len(q.history) {
		return dst
	}
	src := q.history

	h4 := hash4(binary.LittleEndian.Uint32(src[pos:]))
	candidate4 := int(q.table4[h4&tableMask])
	q.table4[h4&tableMask] = uint32(pos)

	if candidate4 != 0 && pos-candidate4 <= q.MaxDistance && binary.LittleEndian.Uint32(src[pos:]) == binary.LittleEndian.Uint32(src[candidate4:]) {
		// We have a 4-byte match now.
		start := pos
		match := candidate4
		end := extendMatch(src[:max], match+4, start+4)
		for start > min && match > 0 && src[start-1] == src[match-1] {
			start--
			match--
		}

		dst = append(dst, AbsoluteMatch{
			Start: start,
			End:   end,
			Match: match,
		})
	}

	if pos+8 > len(src) {
		return dst
	}

	h8 := hash8(binary.LittleEndian.Uint64(src[pos:]))
	candidate8 := int(q.table8[h8&table8Mask])
	q.table8[h8&table8Mask] = uint32(pos)

	if candidate8 != 0 && candidate8 != candidate4 && pos-candidate8 <= q.MaxDistance && binary.LittleEndian.Uint64(src[pos:]) == binary.LittleEndian.Uint64(src[candidate8:]) {
		// We have a 8-byte match now.
		start := pos
		match := candidate8
		end := extendMatch(src[:max], match+8, start+8)
		for start > min && match > 0 && src[start-1] == src[match-1] {
			start--
			match--
		}

		dst = append(dst, AbsoluteMatch{
			Start: start,
			End:   end,
			Match: match,
		})
	}

	q.lastSearch = pos
	return dst
}

func hash8(u uint64) uint32 {
	return uint32((u * 0x1FE35A7BD3579BD3) >> table8Shift)
}
