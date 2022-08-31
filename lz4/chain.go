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

	table     [maxTableSize]uint32
	prevBlock []byte

	chain     []uint16
	prevChain []uint16
}

func (q *HashChain) Reset() {
	q.table = [maxTableSize]uint32{}
	q.prevBlock = q.prevBlock[:0]
	q.chain = q.chain[:0]
	q.prevChain = q.prevChain[:0]
}

// FindMatches looks for matches in src, appends them to dst, and returns dst.
func (q *HashChain) FindMatches(dst []pack.Match, src []byte) []pack.Match {
	if cap(q.chain) >= len(src) {
		q.chain = q.chain[:len(src)]
		for i := range q.chain {
			q.chain[i] = 0
		}
	} else {
		q.chain = make([]uint16, len(src))
	}

	// sLimit is when to stop looking for offset/length copies.
	sLimit := len(src) - 12

	// nextEmit is where in src the next emitLiteral should start from.
	nextEmit := 0

	// The encoded form must start with a literal, as there are no previous
	// bytes to copy, so we start looking for hash matches at s == 1.
	s := 1

	if s > sLimit {
		goto emitRemainder
	}

	for {
		nextHash := hash4(binary.LittleEndian.Uint32(src[s:]))

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
		candidate := 0
		for {
			s = nextS
			bytesBetweenHashLookups := skip >> 5
			nextS = s + bytesBetweenHashLookups
			skip += bytesBetweenHashLookups
			if nextS > sLimit {
				goto emitRemainder
			}
			candidate = int(q.table[nextHash&tableMask])
			q.table[nextHash&tableMask] = uint32(s)

			// If the candidate is after the current position,
			// it is actually from the previous block.
			if candidate >= s {
				candidate -= len(q.prevBlock)
			}

			nextHash = hash4(binary.LittleEndian.Uint32(src[nextS:]))

			if candidate == 0 {
				continue
			}
			if s-candidate < 65536 {
				q.chain[s] = uint16(s - candidate)
			}

			if candidate > 0 {
				if s-candidate <= maxDistance && binary.LittleEndian.Uint32(src[s:]) == binary.LittleEndian.Uint32(src[candidate:]) {
					break
				}
			} else if candidate < -3 {
				if s-candidate <= maxDistance && binary.LittleEndian.Uint32(src[s:]) == binary.LittleEndian.Uint32(q.prevBlock[candidate+len(q.prevBlock):]) {
					break
				}
			}
		}

		// A 4-byte match has been found. We'll later see if more than 4 bytes
		// match. But, prior to the match, src[nextEmit:s] are unmatched.
		base := s

		if candidate > 0 {
			s = extendMatch(src, candidate+4, s+4)
		} else {
			s = extendMatch2(q.prevBlock, candidate+len(q.prevBlock)+4, src, s+4)
		}

		match := candidate

		// Follow the chain to see if we can find a longer match.
		for i := 0; i < q.SearchLen; i++ {
			var newCandidate int
			if candidate > 0 {
				newCandidate = candidate - int(q.chain[candidate])
			} else {
				newCandidate = candidate - int(q.prevChain[candidate+len(q.prevChain)])
			}
			if newCandidate == candidate || newCandidate < -len(q.prevBlock) || s-newCandidate > maxDistance {
				break
			}

			var newS int
			if newCandidate > 0 {
				newS = extendMatch(src, newCandidate, base)
			} else {
				newS = extendMatch2(q.prevBlock, newCandidate+len(q.prevBlock), src, base)
			}
			if newS > s {
				s, match = newS, newCandidate
			}
			candidate = newCandidate
		}

		dst = append(dst, pack.Match{
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
		for i := base + 1; i < s; i++ {
			h := hash4(binary.LittleEndian.Uint32(src[i:]))
			prev := int(q.table[h&tableMask])
			q.table[h&tableMask] = uint32(i)

			if prev == 0 {
				continue
			}
			if prev >= i {
				prev -= len(q.prevBlock)
			}
			if i-prev < 65536 {
				q.chain[i] = uint16(i - prev)
			}
		}
	}

emitRemainder:
	if nextEmit < len(src) {
		dst = append(dst, pack.Match{
			Unmatched: len(src) - nextEmit,
		})
	}
	q.prevBlock = append(q.prevBlock[:0], src...)
	q.chain, q.prevChain = q.prevChain[:0], q.chain
	return dst
}
