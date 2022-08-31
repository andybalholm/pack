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

	if s > sLimit {
		goto emitRemainder
	}

	for {
		nextS := s
		candidate := 0

	searchLoop:
		for {
			s = nextS
			nextS = s + 1
			if nextS > sLimit {
				goto emitRemainder
			}

			candidate = s
		chainLoop:
			for i := 0; i < q.SearchLen+1; i++ {
				d := q.chain[candidate]
				if d == 0 {
					break chainLoop
				}
				candidate -= int(d)
				if candidate < 0 || s-candidate > maxDistance {
					break chainLoop
				}
				if binary.LittleEndian.Uint32(src[s:]) == binary.LittleEndian.Uint32(src[candidate:]) {
					break searchLoop
				}
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

		// Extend the match backward if possible.
		for base > nextEmit && match > 0 && src[match-1] == src[base-1] {
			match--
			base--
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
	}

emitRemainder:
	if nextEmit < len(src) {
		dst = append(dst, pack.Match{
			Unmatched: len(src) - nextEmit,
		})
	}
	return dst
}
