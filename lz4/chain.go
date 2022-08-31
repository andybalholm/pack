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
		q.chain = q.chain[:minHistory]

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
	q.chain = append(q.chain, make([]uint16, len(src))...)
	src = q.history

	// sLimit is when to stop looking for offset/length copies.
	sLimit := len(src) - 12

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

			nextHash = hash4(binary.LittleEndian.Uint32(src[nextS:]))

			if candidate == 0 {
				continue
			}
			if s-candidate < 65536 {
				q.chain[s] = uint16(s - candidate)
			}

			if s-candidate <= maxDistance && binary.LittleEndian.Uint32(src[s:]) == binary.LittleEndian.Uint32(src[candidate:]) {
				break
			}
		}

		// A 4-byte match has been found. We'll later see if more than 4 bytes
		// match. But, prior to the match, src[nextEmit:s] are unmatched.
		base := s
		s = extendMatch(src, candidate+4, s+4)
		match := candidate

		// Follow the chain to see if we can find a longer match.
		chainPos := 0
		for i := 0; i < q.SearchLen; i++ {
			newCandidate := candidate - int(q.chain[candidate+chainPos])
			if newCandidate == candidate || newCandidate < 0 || s-newCandidate > maxDistance {
				break
			}

			newS := extendMatch(src, newCandidate, base)
			if newS > s {
				s, match = newS, newCandidate
				if i+1 < q.SearchLen {
					// Instead of always following the hash chain for the start of the match,
					// try to find and follow the rarest chain so that we don't have to check as many locations.
					var maxDist uint16
					for pos := 0; pos < s-base-3; pos++ {
						i := match + pos
						dist := q.chain[i]
						if dist > maxDist {
							maxDist = dist
							chainPos = pos
						}
					}
				}
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
	return dst
}
