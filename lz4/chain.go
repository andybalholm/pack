package lz4

import (
	"encoding/binary"

	"github.com/andybalholm/pack"
)

// This file is based on code from github.com/golang/snappy.

//Copyright (c) 2011 The Snappy-Go Authors. All rights reserved.
//
//Redistribution and use in source and binary forms, with or without
//modification, are permitted provided that the following conditions are
//met:
//
//   * Redistributions of source code must retain the above copyright
//notice, this list of conditions and the following disclaimer.
//   * Redistributions in binary form must reproduce the above
//copyright notice, this list of conditions and the following disclaimer
//in the documentation and/or other materials provided with the
//distribution.
//   * Neither the name of Google Inc. nor the names of its
//contributors may be used to endorse or promote products derived from
//this software without specific prior written permission.
//
//THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
//"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
//LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
//A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
//OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
//SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
//LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
//DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
//THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
//(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
//OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// HashChain is an implementation of the MatchFinder interface that
// uses hash chaining to find longer matches.
type HashChain struct {
	SearchLen int

	table [maxTableSize]uint32

	history []byte
	chain   []uint16
}

const (
	minHistory = 1 << 16
	maxHistory = 1 << 18
)

func (q *HashChain) Reset() {
	q.table = [maxTableSize]uint32{}
	q.history = q.history[:0]
	q.chain = q.chain[:0]
}

// FindMatches looks for matches in src, appends them to dst, and returns dst.
func (q *HashChain) FindMatches(dst []pack.Match, src []byte) []pack.Match {
	var s, nextEmit int

	if len(q.history) > maxHistory {
		// Trim down the history buffer.
		delta := len(q.history) - minHistory
		copy(q.history, q.history[delta:])
		q.history = q.history[:minHistory]
		copy(q.chain, q.chain[delta:])
		q.chain = q.chain[:len(q.chain)-delta]

		for i, v := range q.table {
			newV := int(v) - delta
			if newV < 0 {
				newV = 0
			}
			q.table[i] = uint32(newV)
		}
	}

	// Append src to the history buffer.
	s = len(q.history)
	nextEmit = len(q.history)
	q.history = append(q.history, src...)
	src = q.history

	chain := q.chain
	// Pre-calculate hashes and chains.
	for i := len(chain); i+3 < len(src); i++ {
		h := hash4(binary.LittleEndian.Uint32(src[i:]))
		candidate := int(q.table[h&tableMask])
		q.table[h&tableMask] = uint32(i)
		if candidate == 0 || i-candidate > 65535 {
			chain = append(chain, 0)
		} else {
			chain = append(chain, uint16(i-candidate))
		}
	}
	q.chain = chain

	// sLimit is when to stop looking for offset/length copies.
	sLimit := len(src) - 12

	// The parsing algorithm in the following loop is based on
	// https://fastcompression.blogspot.com/2011/12/advanced-parsing-strategies.html
	// and its implementation in LZ4HC.

	var (
		ml0, ml, ml2, ml3             int // match lengths
		start0, start, start2, start3 int // starting points
		match0, match, match2, match3 int // indexes to the earlier, matching sequences
	)

	for s < sLimit {
		match, ml = q.findMatch(s)
		if ml < 4 {
			s++
			continue
		}
		start = s

		// Extend the match backward if possible.
		for start > nextEmit && match > 0 && src[match-1] == src[start-1] {
			match--
			start--
			ml++
		}

		// Save the original match, in case we would skip too much.
		start0, ml0, match0 = start, ml, match

	search2:
		if start+ml < sLimit {
			start2, ml2, match2 = q.findOverlappingMatch(start, ml)
		} else {
			start2, ml2, match2 = 0, 0, 0
		}

		if ml2 <= ml {
			// The new match is no better than the first one,
			// so go ahead and encode match 1.
			dst = append(dst, pack.Match{
				Unmatched: start - nextEmit,
				Length:    ml,
				Distance:  start - match,
			})
			s = start + ml
			nextEmit = s
			continue
		}

		// If the original first match was skipped, and the current first would be
		// squeezed by the second match to be shorter than the original, restore the original first match.
		if start0 < start && start2 < start+ml0 {
			start, ml, match = start0, ml0, match0
		}

		if start2-start < 3 {
			// Skip the original match, and replace it with the second match.
			start, ml, match = start2, ml2, match2
			goto search2
		}

	search3:
		// Before searching for a third match, adjust the overlap between the first two matches
		// to make the first one as long as possible, up to 18 bytes (which is the optimal short match
		// in LZ4).
		if start2-start < 18 {
			newML := ml
			if newML > 18 {
				newML = 18
			}
			if start+newML > start2+ml2-4 {
				newML = start2 - start + ml2 - 4
			}
			correction := newML - (start2 - start)
			if correction > 0 {
				start2 += correction
				match2 += correction
				ml2 -= correction
			}
		}

		if start2+ml2 < sLimit {
			start3, ml3, match3 = q.findOverlappingMatch(start2, ml2)
		} else {
			start3, ml3, match3 = 0, 0, 0
		}

		if ml3 <= ml2 {
			// No better match was found; encode matches 1 and 2.
			if start2 < start+ml {
				ml = start2 - start
			}
			dst = append(dst, pack.Match{
				Unmatched: start - nextEmit,
				Length:    ml,
				Distance:  start - match,
			}, pack.Match{
				Unmatched: start2 - (start + ml),
				Length:    ml2,
				Distance:  start2 - match2,
			})
			s = start2 + ml2
			nextEmit = s
			continue
		}

		if start3 < start+ml+3 {
			// There isn't enough space for match 2; remove it.
			if start3 < start+ml {
				// Matches 1 and 3 overlap, so call match 3 match 2 and look for a new match 3.
				start2, ml2, match2 = start3, ml3, match3
				goto search3
			}

			// Matches 1 and 3 don't overlap, so we can write match 1 immediately
			// and use match 3 as our new match 1.
			dst = append(dst, pack.Match{
				Unmatched: start - nextEmit,
				Length:    ml,
				Distance:  start - match,
			})
			nextEmit = start + ml

			// Save what's left of match 2, to be restored if we skip too much.
			start0, ml0, match0 = start2, ml2, match2
			if start0 < nextEmit {
				correction := nextEmit - start0
				start0 = nextEmit
				ml0 -= correction
				match0 += correction
				if ml0 < 4 {
					start0, ml0, match0 = start3, ml3, match3
				}
			}

			start, ml, match = start3, ml3, match3
			goto search2
		}

		// We have three matches; let's adjust the length of match 1, and then write it.
		if start2 < start+ml {
			if start2-start < 18 {
				if ml > 18 {
					ml = 18
				}
				if start+ml > start2+ml2-4 {
					ml = start2 - start + ml2 - 4
				}
				correction := ml - (start2 - start)
				if correction > 0 {
					start2 += correction
					match2 += correction
					ml2 -= correction
				}
			} else {
				ml = start2 - start
			}
		}
		dst = append(dst, pack.Match{
			Unmatched: start - nextEmit,
			Length:    ml,
			Distance:  start - match,
		})
		nextEmit = start + ml

		// Match 2 becomes match 1.
		start, ml, match = start2, ml2, match2

		// Match 3 becomes match 2.
		start2, ml2, match2 = start3, ml3, match3

		// Look for a new match 3.
		goto search3
	}

	if nextEmit < len(src) {
		dst = append(dst, pack.Match{
			Unmatched: len(src) - nextEmit,
		})
	}
	return dst
}

func (q *HashChain) findMatch(pos int) (match, matchLen int) {
	src := q.history
	candidate := pos
	for i := 0; i < q.SearchLen+1; i++ {
		d := q.chain[candidate]
		if d == 0 {
			return 0, 0
		}
		candidate -= int(d)
		if candidate < 0 || pos-candidate > maxDistance {
			return 0, 0
		}
		if binary.LittleEndian.Uint32(src[pos:]) == binary.LittleEndian.Uint32(src[candidate:]) {
			goto found
		}
	}
	return 0, 0

found:
	match = candidate
	matchLen = extendMatch(src, candidate+4, pos+4) - pos

	// Follow the chain to see if we can find a longer match.
	chainPos := 0
	for i := 0; i < q.SearchLen; i++ {
		newCandidate := candidate - int(q.chain[candidate+chainPos])
		if newCandidate == candidate || newCandidate < 0 || pos-newCandidate > maxDistance {
			break
		}

		newLen := extendMatch(src, newCandidate, pos) - pos
		if newLen > matchLen {
			match, matchLen = newCandidate, newLen
			if i+1 < q.SearchLen {
				// Instead of always following the hash chain for the start of the match,
				// try to find and follow the rarest chain so that we don't have to check as many locations.
				var maxDist uint16
				for p := 0; p < matchLen-3; p++ {
					i := match + p
					dist := q.chain[i]
					if dist > maxDist {
						maxDist = dist
						chainPos = p
					}
				}
			}
		}
		candidate = newCandidate
	}

	return match, matchLen
}

// findOverlappingMatch searches for a match that overlaps the end of an
// existing match.
func (q *HashChain) findOverlappingMatch(oldStart, oldLength int) (start, length, match int) {
	src := q.history
	searchPos := oldStart + oldLength - 2
	searchSeq := binary.LittleEndian.Uint32(src[searchPos:])
	if searchPos+4 >= len(src) {
		return 0, 0, 0
	}

	candidate := searchPos
	for i := 0; i < q.SearchLen; i++ {
		d := q.chain[candidate]
		if d == 0 {
			break
		}
		candidate -= int(d)
		if candidate < 0 || searchPos-candidate > maxDistance {
			break
		}
		if binary.LittleEndian.Uint32(src[candidate:]) != searchSeq {
			continue
		}

		newEnd := extendMatch(src, candidate+4, searchPos+4)

		// Extend the match backward as far as possible.
		newStart := searchPos
		newMatch := candidate
		for newStart >= oldStart+4 && newMatch >= 4 && binary.LittleEndian.Uint32(src[newStart-4:]) == binary.LittleEndian.Uint32(src[newMatch-4:]) {
			newStart -= 4
			newMatch -= 4
		}
		for newStart > oldStart && newMatch > 0 && src[newStart-1] == src[newMatch-1] {
			newStart--
			newMatch--
		}

		if newEnd-newStart > length {
			start = newStart
			length = newEnd - newStart
			match = newMatch
		}
	}

	return start, length, match
}
