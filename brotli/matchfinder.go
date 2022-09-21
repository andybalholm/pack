package brotli

import (
	"encoding/binary"
	"math/bits"
	"runtime"

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

// MatchFinder is an implementation of pack.MatchFinder that uses a
// Hasher to find matches.
type MatchFinder struct {
	Hasher Hasher

	// MaxLength is the limit on the length of matches to find;
	// 0 means unlimited.
	MaxLength int

	// MaxDistance is the limit on the distance to look back for matches;
	// 0 means unlimited.
	MaxDistance int

	// MaxHistory is the limit on how much data from previous blocks is
	// kept around to look for matches in; 0 means that no matches from previous
	// blocks will be found.
	MaxHistory int

	// MinHistory is the amount of data that is used to start a new history
	// buffer after the size exceeds MaxHistory.
	MinHistory int

	initialized bool
	history     []byte

	// candidateCache is a place to store a reference to the candidates
	// slice, and avoid an allocation.
	candidateCache []int
}

func (q *MatchFinder) Reset() {
	q.Hasher.Init()
	q.history = q.history[:0]
}

// FindMatches looks for matches in src, appends them to dst, and returns dst.
func (q *MatchFinder) FindMatches(dst []pack.Match, src []byte) []pack.Match {
	var s, nextEmit int

	switch {
	case q.MaxHistory == 0:
		// Don't use the history buffer, and start with a freshly initialized
		// Hasher.
		q.Hasher.Init()

		// The encoded form must start with a literal, as there are no previous
		// bytes to copy, so we start looking for hash matches at s == 1.
		s = 1
		nextEmit = 0

	case len(q.history) > q.MaxHistory:
		// Trim down the history buffer, and reset the Hasher.
		copy(q.history, q.history[len(q.history)-q.MinHistory:])
		q.history = q.history[:q.MinHistory]
		s = q.MinHistory
		nextEmit = q.MinHistory
		q.history = append(q.history, src...)
		src = q.history

		q.Hasher.Init()
		for i := 1; i < q.MinHistory && i+8 < len(src); i++ {
			q.Hasher.Store(src, i)
		}

	default:
		// Append src to the history buffer.
		s = len(q.history)
		nextEmit = len(q.history)
		q.history = append(q.history, src...)
		src = q.history

		if !q.initialized {
			q.Hasher.Init()
			q.initialized = true
		}
	}

	// sLimit is when to stop looking for offset/length copies. The input margin
	// gives us room to use a 64-bit load for hashing.
	sLimit := len(src) - 8

	candidates := q.candidateCache

	prevDistance := 0

	if s > sLimit {
		goto emitRemainder
	}

	for {
		// Copied from the C++ snappy implementation:
		//
		// Heuristic match skipping: If 32 bytes are scanned with no matches
		// found, start looking only at every other byte. If 32 more bytes are
		// scanned (or skipped), look at every third byte, etc.. When a match
		// is found, immediately go back to looking at every byte. This is a
		// small loss (~5% performance, ~0.1% density) for compressible data
		// due to more bookkeeping, but for non-compressible data (such as
		// JPEG) it's a huge win since the compressor quickly "realizes" the
		// data is incompressible and doesn't bother looking for matches
		// everywhere.
		//
		// The "skip" variable keeps track of how many bytes there are since
		// the last match; dividing it by 32 (ie. right-shifting by five) gives
		// the number of bytes to move ahead for each iteration.
		skip := 32

		nextS := s
		var match, matchLen, bestScore int
		for {
			s = nextS
			bytesBetweenHashLookups := skip >> 5
			nextS = s + bytesBetweenHashLookups
			skip += bytesBetweenHashLookups
			if nextS > sLimit {
				goto emitRemainder
			}
			match, matchLen, bestScore = 0, 0, 0
			if prevDistance != 0 {
				// Often there is a match at the same distance back as the previous one.
				// Check for that first.
				candidate := s - prevDistance
				m, ml := q.checkMatch(src, s, candidate)
				score := backwardReferenceScoreUsingLastDistance(ml)
				if score > bestScore {
					match, matchLen, bestScore = m, ml, score
				}
			}
			candidates = q.Hasher.Candidates(candidates[:0], src, s)
			for _, c := range candidates {
				m, ml := q.checkMatch(src, s, c)
				score := backwardReferenceScore(ml, s-m)
				if score > bestScore {
					match, matchLen, bestScore = m, ml, score
				}
			}
			if bestScore > minScore {
				break
			}
		}

		// We have found a match of at least 4 bytes at s.
		// The location and length of the match are in match and matchLen.
		base := s
		origBase := base

		found := true
		for i := origBase + 1; i < origBase+5 && i < sLimit && found; i++ {
			found = false
			lazyThreshold := bestScore + 175
			candidates := q.Hasher.Candidates(candidates[:0], src, i)
			for _, c := range candidates {
				m, ml := q.checkMatch(src, i, c)
				score := backwardReferenceScore(ml, i-m)
				if score > bestScore && score > lazyThreshold {
					base = i
					match, matchLen, bestScore = m, ml, score
					found = true
				}
			}
		}

		// Extend the match backward if possible.
		for base > nextEmit && match > 0 && src[match-1] == src[base-1] {
			match--
			base--
			matchLen++
		}

		s = base + matchLen

		for q.MaxLength != 0 && s-base > q.MaxLength {
			// The match is too long; break it up into shorter matches.
			length := q.MaxLength
			if s-base < q.MaxLength+4 {
				length = s - base - 4
			}
			dst = append(dst, pack.Match{
				Unmatched: base - nextEmit,
				Length:    length,
				Distance:  base - match,
			})
			base += length
			match += length
			nextEmit = base
		}

		dst = append(dst, pack.Match{
			Unmatched: base - nextEmit,
			Length:    s - base,
			Distance:  base - match,
		})
		nextEmit = s
		prevDistance = base - match
		if s >= sLimit {
			goto emitRemainder
		}

		// We could immediately start working at s now, but to improve
		// compression we first update the hash table.
		for i := origBase + 1; i < s; i++ {
			q.Hasher.Store(src, i)
		}
	}

emitRemainder:
	if nextEmit < len(src) {
		dst = append(dst, pack.Match{
			Unmatched: len(src) - nextEmit,
		})
	}
	q.candidateCache = candidates
	return dst
}

// checkMatch checks whether there is a usable match for pos at candidate.
// It returns the adjusted match location (negative if it's in the previous
// block), and the length of the match.
func (q *MatchFinder) checkMatch(src []byte, pos, candidate int) (matchPos, matchLen int) {
	if candidate == 0 {
		return 0, 0
	}

	if candidate < pos {
		if (q.MaxDistance == 0 || pos-candidate <= q.MaxDistance) && binary.LittleEndian.Uint32(src[pos:]) == binary.LittleEndian.Uint32(src[candidate:]) {
			end := extendMatch(src, candidate+4, pos+4)
			return candidate, end - pos
		}
	}

	return 0, 0
}

// extendMatch returns the largest k such that k <= len(src) and that
// src[i:i+k-j] and src[j:k] have the same contents.
//
// It assumes that:
//
//	0 <= i && i < j && j <= len(src)
func extendMatch(src []byte, i, j int) int {
	switch runtime.GOARCH {
	case "amd64":
		// As long as we are 8 or more bytes before the end of src, we can load and
		// compare 8 bytes at a time. If those 8 bytes are equal, repeat.
		for j+8 < len(src) {
			iBytes := binary.LittleEndian.Uint64(src[i:])
			jBytes := binary.LittleEndian.Uint64(src[j:])
			if iBytes != jBytes {
				// If those 8 bytes were not equal, XOR the two 8 byte values, and return
				// the index of the first byte that differs. The BSF instruction finds the
				// least significant 1 bit, the amd64 architecture is little-endian, and
				// the shift by 3 converts a bit index to a byte index.
				return j + bits.TrailingZeros64(iBytes^jBytes)>>3
			}
			i, j = i+8, j+8
		}
	case "386":
		// On a 32-bit CPU, we do it 4 bytes at a time.
		for j+4 < len(src) {
			iBytes := binary.LittleEndian.Uint32(src[i:])
			jBytes := binary.LittleEndian.Uint32(src[j:])
			if iBytes != jBytes {
				return j + bits.TrailingZeros32(iBytes^jBytes)>>3
			}
			i, j = i+4, j+4
		}
	}
	for ; j < len(src) && src[i] == src[j]; i, j = i+1, j+1 {
	}
	return j
}

const literalByteScore = 135

const distanceBitPenalty = 30

/* Score must be positive after applying maximal penalty. */
const scoreBase = (distanceBitPenalty * 8 * 8)

const minScore = scoreBase + 100

/*
Usually, we always choose the longest backward reference. This function

	allows for the exception of that rule.

	If we choose a backward reference that is further away, it will
	usually be coded with more bits. We approximate this by assuming
	log2(distance). If the distance can be expressed in terms of the
	last four distances, we use some heuristic constants to estimate
	the bits cost. For the first up to four literals we use the bit
	cost of the literals from the literal cost model, after that we
	use the average bit cost of the cost model.

	This function is used to sometimes discard a longer backward reference
	when it is not much longer and the bit cost for encoding it is more
	than the saved literals.

	backward_reference_offset MUST be positive.
*/
func backwardReferenceScore(copy_length int, backward_reference_offset int) int {
	return scoreBase + literalByteScore*copy_length - distanceBitPenalty*int(log2FloorNonZero(uint(backward_reference_offset)))
}

func backwardReferenceScoreUsingLastDistance(copy_length int) int {
	return literalByteScore*copy_length + scoreBase + 15
}

func Score(m pack.AbsoluteMatch) int {
	return backwardReferenceScore(m.End-m.Start, m.Start-m.Match)
}
