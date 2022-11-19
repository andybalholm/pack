package pack

import "encoding/binary"

// O3 is an implementation of the MatchFinder interface
// that is comparable to Brotli level 3, but uses the overlap
// parsing strategy.
type O3 struct {
	// MaxDistance is the maximum distance (in bytes) to look back for
	// a match. The default is 65535.
	MaxDistance int

	table [1<<o3Bits + o3Sweep]uint32

	history []byte

	matchCache []AbsoluteMatch
}

const (
	o3Bits  = 16
	o3Sweep = 2
)

func (q *O3) Reset() {
	q.table = [1<<o3Bits + o3Sweep]uint32{}
	q.history = q.history[:0]
}

func (q *O3) store(index int) {
	hash := q.hash(binary.LittleEndian.Uint64(q.history[index:]))
	offset := index >> 3 % o3Sweep
	q.table[int(hash)+offset] = uint32(index)
}

// FindMatches looks for matches in src, appends them to dst, and returns dst.
func (q *O3) FindMatches(dst []Match, src []byte) []Match {
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

func (q *O3) Parse(dst []Match, start, end int) []Match {
	s := start
	nextEmit := start
	matchList := q.matchCache[:0]

	for s < end {
		matchList = matchList[:0]

		m := q.search(s, nextEmit, end)
		if m.End-m.Start < 4 {
			s++
			continue
		}
		matchList = append(matchList, m)

		for {
			for i := m.Start; i < m.End-2 && i < len(q.history)-8; i++ {
				q.store(i)
			}

			// Look for a new match overlapping the end of m.
			newMatch := q.search(m.End-2, m.Start, end)
			if newMatch.End-newMatch.Start <= m.End-m.Start {
				// It's no better than the previous match, so ignore it.
				// But store the hash for the last byte of m before
				// going on.
				i := m.End - 1
				if i <= len(q.history)-8 {
					q.store(i)
				}
				break
			}
			m = newMatch
			matchList = append(matchList, m)
		}

		// We now have a series of overlapping matches,
		// each one longer than the previous one.
		// Now we need to resolve the overlaps.
		for i := len(matchList) - 2; i >= 0; i-- {
			if matchList[i].End-matchList[i].Start > matchList[i+1].End-matchList[i+1].Start {
				// This match is actually longer than the following one, probably because
				// the following one has already been trimmed.
				// So we'll trim the following one to remove the overlap with this match.
				if matchList[i].End > matchList[i+1].Start {
					matchList[i+1].Match += matchList[i].End - matchList[i+1].Start
					matchList[i+1].Start = matchList[i].End
				}
				if matchList[i+1].End-matchList[i+1].Start < 4 {
					// The following match is too short now, so we'll just drop it.
					matchList = append(matchList[:i+1], matchList[i+2:]...)
					if i < len(matchList)-1 {
						// Run through the loop with the same index again,
						// to catch overlaps between this match and its new neighbor.
						i++
					}
				}
			} else {
				// The following match is longer than this one, so we'll trim this one
				// to remove the overlap.
				if matchList[i].End > matchList[i+1].Start {
					matchList[i].End = matchList[i+1].Start
				}
				if matchList[i].End-matchList[i].Start < 4 {
					// This match is too short now, so we'll just drop it.
					matchList = append(matchList[:i], matchList[i+1:]...)
				}
			}
		}

		for _, m := range matchList {
			dst = append(dst, Match{
				Unmatched: m.Start - nextEmit,
				Length:    m.End - m.Start,
				Distance:  m.Start - m.Match,
			})
			nextEmit = m.End
		}
		s = nextEmit
	}

	if nextEmit < end {
		dst = append(dst, Match{
			Unmatched: end - nextEmit,
		})
	}
	q.matchCache = matchList[:0]
	return dst
}

func (q *O3) checkMatch(pos, candidate, min, max int) AbsoluteMatch {
	src := q.history

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

func (q *O3) search(pos, min, max int) AbsoluteMatch {
	if pos+8 > len(q.history) {
		return AbsoluteMatch{}
	}
	src := q.history

	h := q.hash(binary.LittleEndian.Uint64(src[pos:]))

	m := AbsoluteMatch{}
	for _, c := range q.table[h : h+o3Sweep] {
		newMatch := q.checkMatch(pos, int(c), min, max)
		if newMatch.End-newMatch.Start > m.End-m.Start {
			m = newMatch
		}
	}

	offset := pos >> 3 % o3Sweep
	q.table[int(h)+offset] = uint32(pos)

	return m
}

func (*O3) hash(u uint64) uint32 {
	hash := (u << 24) * hashMul64
	return uint32(hash >> (64 - o3Bits))
}
