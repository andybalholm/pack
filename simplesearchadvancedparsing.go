package pack

import "encoding/binary"

const (
	ssapBits = 17
	ssapMask = (1 << ssapBits) - 1
)

// SimpleSearchAdvancedParsing is an implementation of the MatchFinder
// interface that uses a simple hash table to find matches,
// but the advanced parsing technique from
// https://fastcompression.blogspot.com/2011/12/advanced-parsing-strategies.html
type SimpleSearchAdvancedParsing struct {
	// MaxDistance is the maximum distance (in bytes) to look back for
	// a match. The default is 65535.
	MaxDistance int

	// MinLength is the length of the shortest match to return.
	// The default is 4.
	MinLength int

	// HashLen is the number of bytes to use to calculate the hashes.
	// The maximum is 8 and the default is 6.
	HashLen int

	table [1 << ssapBits]uint32

	history []byte
}

func (q *SimpleSearchAdvancedParsing) Reset() {
	q.table = [1 << ssapBits]uint32{}
	q.history = q.history[:0]
}

func (q *SimpleSearchAdvancedParsing) FindMatches(dst []Match, src []byte) []Match {
	if q.MaxDistance == 0 {
		q.MaxDistance = 65535
	}
	if q.MinLength == 0 {
		q.MinLength = 4
	}
	if q.HashLen == 0 {
		q.HashLen = 6
	}
	var nextEmit int

	if len(q.history) > q.MaxDistance*2 {
		// Trim down the history buffer.
		delta := len(q.history) - q.MaxDistance
		copy(q.history, q.history[delta:])
		q.history = q.history[:q.MaxDistance]

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

	// matches stores the matches that have been found but not emitted,
	// in reverse order. (matches[0] is the most recent one.)
	var matches [3]AbsoluteMatch
	for i := nextEmit; i < len(src)-7; i++ {
		if matches[0] != (AbsoluteMatch{}) && i >= matches[0].End {
			// We have found some matches, and we're far enough along that we probably
			// won't find overlapping matches, so we might as well emit them.
			if matches[1] != (AbsoluteMatch{}) {
				if matches[1].End > matches[0].Start {
					matches[1].End = matches[0].Start
				}
				if matches[1].End-matches[1].Start >= q.MinLength {
					dst = append(dst, Match{
						Unmatched: matches[1].Start - nextEmit,
						Length:    matches[1].End - matches[1].Start,
						Distance:  matches[1].Start - matches[1].Match,
					})
					nextEmit = matches[1].End
				}
			}
			dst = append(dst, Match{
				Unmatched: matches[0].Start - nextEmit,
				Length:    matches[0].End - matches[0].Start,
				Distance:  matches[0].Start - matches[0].Match,
			})
			nextEmit = matches[0].End
			matches = [3]AbsoluteMatch{}
		}

		// Now look for a match.
		h := ((binary.LittleEndian.Uint64(src[i:]) & (1<<(8*q.HashLen) - 1)) * hashMul64) >> (64 - ssapBits)
		candidate := int(q.table[h&ssapMask])
		q.table[h&ssapMask] = uint32(i)

		if candidate == 0 || i-candidate > q.MaxDistance || i-candidate == matches[0].Start-matches[0].Match {
			continue
		}
		if binary.LittleEndian.Uint32(src[candidate:]) != binary.LittleEndian.Uint32(src[i:]) {
			continue
		}

		// We have a 4-byte match now.

		start := i
		match := candidate
		end := extendMatch(src, match+4, start+4)
		for start > nextEmit && match > 0 && src[start-1] == src[match-1] {
			start--
			match--
		}
		if end-start <= matches[0].End-matches[0].Start {
			continue
		}

		matches = [3]AbsoluteMatch{
			AbsoluteMatch{
				Start: start,
				End:   end,
				Match: match,
			},
			matches[0],
			matches[1],
		}

		if matches[2] == (AbsoluteMatch{}) {
			continue
		}

		// We have three matches, so it's time to emit one and/or eliminate one.
		switch {
		case matches[0].Start < matches[2].End:
			// The first and third matches overlap; discard the one in between.
			matches = [3]AbsoluteMatch{
				matches[0],
				matches[2],
				AbsoluteMatch{},
			}

		case matches[0].Start < matches[2].End+q.MinLength:
			// The first and third matches don't overlap, but there's no room for
			// another match between them. Emit the first match and discard the second.
			dst = append(dst, Match{
				Unmatched: matches[2].Start - nextEmit,
				Length:    matches[2].End - matches[2].Start,
				Distance:  matches[2].Start - matches[2].Match,
			})
			nextEmit = matches[2].End
			matches = [3]AbsoluteMatch{
				matches[0],
				AbsoluteMatch{},
				AbsoluteMatch{},
			}

		default:
			// Emit the first match, shortening it if necessary to avoid overlap with the second.
			if matches[2].End > matches[1].Start {
				matches[2].End = matches[1].Start
			}
			if matches[2].End-matches[2].Start >= q.MinLength {
				dst = append(dst, Match{
					Unmatched: matches[2].Start - nextEmit,
					Length:    matches[2].End - matches[2].Start,
					Distance:  matches[2].Start - matches[2].Match,
				})
				nextEmit = matches[2].End
			}
			matches[2] = AbsoluteMatch{}
		}
	}

	// We've found all the matches now; emit the remaining ones.
	if matches[1] != (AbsoluteMatch{}) {
		if matches[1].End > matches[0].Start {
			matches[1].End = matches[0].Start
		}
		if matches[1].End-matches[1].Start >= q.MinLength {
			dst = append(dst, Match{
				Unmatched: matches[1].Start - nextEmit,
				Length:    matches[1].End - matches[1].Start,
				Distance:  matches[1].Start - matches[1].Match,
			})
			nextEmit = matches[1].End
		}
	}
	if matches[0] != (AbsoluteMatch{}) {
		dst = append(dst, Match{
			Unmatched: matches[0].Start - nextEmit,
			Length:    matches[0].End - matches[0].Start,
			Distance:  matches[0].Start - matches[0].Match,
		})
		nextEmit = matches[0].End
	}

	if nextEmit < len(src) {
		dst = append(dst, Match{
			Unmatched: len(src) - nextEmit,
		})
	}

	return dst
}
