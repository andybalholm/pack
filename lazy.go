package pack

import (
	"encoding/binary"
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

const (
	lazyTableSize = 1 << 18
	lazyShift     = 32 - 18
	lazyTableMask = lazyTableSize - 1
)

// LazyMatchFinder is an implementation of the MatchFinder interface that does
// lazy matching and uses two hash lengths (4-byte and 8-byte).
type LazyMatchFinder struct {
	MaxDistance int
	MaxLength   int

	ChainBlocks bool // Should it find matches in the previous block?

	table     [lazyTableSize]uint32
	prevBlock []byte
}

func (q *LazyMatchFinder) Reset() {
	q.table = [lazyTableSize]uint32{}
	q.prevBlock = q.prevBlock[:0]
}

// FindMatches looks for matches in src, appends them to dst, and returns dst.
func (q *LazyMatchFinder) FindMatches(dst []Match, src []byte) []Match {
	// sLimit is when to stop looking for offset/length copies. The input margin
	// gives us room to use a 64-bit load for hashing.
	sLimit := len(src) - 8

	// nextEmit is where in src the next emitLiteral should start from.
	nextEmit := 0

	// The encoded form must start with a literal, as there are no previous
	// bytes to copy, so we start looking for hash matches at s == 1.
	s := 1

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
		var match, matchLen int
		for {
			s = nextS
			nextHash := lazyHash(binary.LittleEndian.Uint32(src[s:]))
			bytesBetweenHashLookups := skip >> 5
			nextS = s + bytesBetweenHashLookups
			skip += bytesBetweenHashLookups
			if nextS > sLimit {
				goto emitRemainder
			}
			candidate := int(q.table[nextHash&lazyTableMask])
			q.table[nextHash&lazyTableMask] = uint32(s)
			match, matchLen = q.checkMatch(src, s, candidate)
			if matchLen >= 4 {
				break
			}
		}

		// We have found a match of at least 4 bytes at s.
		// The location and length of the match are in match and matchLen.

		base := s

		// See if we can find a longer match using an 8-byte hash.
		h := hash8(binary.LittleEndian.Uint64(src[base:]))
		candidate8 := int(q.table[h&lazyTableMask])
		q.table[h&lazyTableMask] = uint32(base)
		match8, matchLen8 := q.checkMatch(src, base, candidate8)
		if matchLen8 > matchLen {
			match = match8
			matchLen = matchLen8
		}

		origBase := base

		// Now try lazy matching.
		if base+1 < sLimit {
			i := base + 1
			h := hash8(binary.LittleEndian.Uint64(src[i:]))
			lazyCandidate := int(q.table[h&lazyTableMask])
			q.table[h&lazyTableMask] = uint32(i)
			lazyMatch, lazyLen := q.checkMatch(src, i, lazyCandidate)
			if lazyLen > matchLen {
				base = i
				match = lazyMatch
				matchLen = lazyLen
			}
		}

		s = base + matchLen

		for s-base > q.MaxLength {
			// The match is too long; break it up into shorter matches.
			length := q.MaxLength
			if s-base < q.MaxLength+4 {
				length = s - base - 4
			}
			dst = append(dst, Match{
				Unmatched: base - nextEmit,
				Length:    length,
				Distance:  base - match,
			})
			base += length
			match += length
			nextEmit = base
		}

		dst = append(dst, Match{
			Unmatched: base - nextEmit,
			Length:    s - base,
			Distance:  base - match,
		})
		nextEmit = s
		if s >= sLimit {
			goto emitRemainder
		}

		// We could immediately start working at s now, but to improve
		// compression we first update the hash table.
		for i := origBase + 1; i < s; i++ {
			x := binary.LittleEndian.Uint64(src[i:])
			q.table[lazyHash(uint32(x))&lazyTableMask] = uint32(i)
			q.table[hash8(x)&lazyTableMask] = uint32(i)
		}
	}

emitRemainder:
	if nextEmit < len(src) {
		dst = append(dst, Match{
			Unmatched: len(src) - nextEmit,
		})
	}
	if q.ChainBlocks {
		q.prevBlock = append(q.prevBlock[:0], src...)
	}
	return dst
}

func hash8(u uint64) uint32 {
	return uint32((u * 0x1FE35A7BD3579BD3) >> (lazyShift + 32))
}

func lazyHash(u uint32) uint32 {
	return (u * 0x1e35a7bd) >> lazyShift
}

// checkMatch checks whether there is a usable match for pos at candidate.
// It returns the adjusted match location (negative if it's in the previous
// block), and the length of the match.
func (q *LazyMatchFinder) checkMatch(src []byte, pos, candidate int) (matchPos, matchLen int) {
	if candidate == 0 {
		return 0, 0
	}

	if candidate < pos {
		if pos-candidate <= q.MaxDistance && binary.LittleEndian.Uint32(src[pos:]) == binary.LittleEndian.Uint32(src[candidate:]) {
			end := extendMatch(src, candidate+4, pos+4)
			return candidate, end - pos
		}
	} else if q.ChainBlocks && candidate < len(q.prevBlock)-3 {
		if pos+len(q.prevBlock)-candidate <= q.MaxDistance && binary.LittleEndian.Uint32(src[pos:]) == binary.LittleEndian.Uint32(q.prevBlock[candidate:]) {
			end := extendMatch2(q.prevBlock, candidate, src, pos)
			return candidate - len(q.prevBlock), end - pos
		}
	}

	return 0, 0
}
