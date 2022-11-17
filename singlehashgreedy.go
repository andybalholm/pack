package pack

import "encoding/binary"

// SingleHashGreedy is an implementation of the MatchFinder interface
// that is like SingleHash with an GreedyParser inlined.
type SingleHashGreedy struct {
	// MaxDistance is the maximum distance (in bytes) to look back for
	// a match. The default is 65535.
	MaxDistance int

	table [maxTableSize]uint32

	history []byte
}

func (q *SingleHashGreedy) Reset() {
	q.table = [maxTableSize]uint32{}
	q.history = q.history[:0]
}

// FindMatches looks for matches in src, appends them to dst, and returns dst.
func (q *SingleHashGreedy) FindMatches(dst []Match, src []byte) []Match {
	if q.MaxDistance == 0 {
		q.MaxDistance = 65535
	}
	var nextEmit int

	if len(q.history) > maxHistory {
		// Trim down the history buffer.
		delta := len(q.history) - minHistory
		copy(q.history, q.history[delta:])
		q.history = q.history[:minHistory]

		for i, v := range q.table {
			newV := int(v) - delta
			if newV < 0 {
				newV = 0
			}
			q.table[i] = uint32(newV)
		}
	}

	// Append src to the history buffer.
	nextEmit = len(q.history)
	q.history = append(q.history, src...)
	src = q.history

	return q.Parse(dst, nextEmit, len(src))
}

func (q *SingleHashGreedy) Parse(dst []Match, start, end int) []Match {
	s := start
	nextEmit := start

	for s < end {
		m := q.search(s, nextEmit, end)
		if m.End-m.Start < 4 {
			s++
			continue
		}

		dst = append(dst, Match{
			Unmatched: m.Start - nextEmit,
			Length:    m.End - m.Start,
			Distance:  m.Start - m.Match,
		})
		nextEmit = m.End
		s = nextEmit
	}

	if nextEmit < end {
		dst = append(dst, Match{
			Unmatched: end - nextEmit,
		})
	}
	return dst
}

func (q *SingleHashGreedy) search(pos, min, max int) AbsoluteMatch {
	if pos+4 > len(q.history) {
		return AbsoluteMatch{}
	}
	src := q.history

	h := hash4(binary.LittleEndian.Uint32(src[pos:]))
	candidate := int(q.table[h&tableMask])
	q.table[h&tableMask] = uint32(pos)

	if candidate == 0 || pos-candidate > q.MaxDistance {
		return AbsoluteMatch{}
	}

	if binary.LittleEndian.Uint32(src[pos:]) != binary.LittleEndian.Uint32(src[candidate:]) {
		return AbsoluteMatch{}
	}

	// We have a 4-byte match now.

	start := pos
	match := candidate
	end := extendMatch(src[:max], match+4, start+4)
	for start > min && match > 0 && src[start-1] == src[match-1] {
		start--
		match--
	}

	return AbsoluteMatch{
		Start: start,
		End:   end,
		Match: match,
	}
}
