package pack

import (
	"encoding/binary"
	"math/bits"
	"runtime"
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

const inputMargin = 16 - 1

// QuickMatchFinder is an implementation of the MatchFinder interface based
// on the algorithm used by snappy.
type QuickMatchFinder struct {
	MaxDistance int
	MaxLength   int

	ChainBlocks bool // Should it find matches in the previous block?

	table     [maxTableSize]uint32
	prevBlock []byte
}

func (q *QuickMatchFinder) Reset() {
	q.table = [maxTableSize]uint32{}
	q.prevBlock = q.prevBlock[:0]
}

const (
	maxTableSize = 1 << 14
	shift        = 32 - 14
	// tableMask is redundant, but helps the compiler eliminate bounds
	// checks.
	tableMask = maxTableSize - 1
)

// FindMatches looks for matches in src, appends them to dst, and returns dst.
func (q *QuickMatchFinder) FindMatches(dst []Match, src []byte) []Match {
	// sLimit is when to stop looking for offset/length copies. The inputMargin
	// lets us use a fast path for emitLiteral in the main loop, while we are
	// looking for copies.
	sLimit := len(src) - inputMargin

	// nextEmit is where in src the next emitLiteral should start from.
	nextEmit := 0

	// The encoded form must start with a literal, as there are no previous
	// bytes to copy, so we start looking for hash matches at s == 1.
	s := 1
	nextHash := hash(binary.LittleEndian.Uint32(src[s:]))

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
			nextHash = hash(binary.LittleEndian.Uint32(src[nextS:]))
			if candidate == 0 {
				continue
			} else if candidate < s {
				if s-candidate <= q.MaxDistance && binary.LittleEndian.Uint32(src[s:]) == binary.LittleEndian.Uint32(src[candidate:]) {
					break
				}
			} else if q.ChainBlocks && candidate < len(q.prevBlock)-3 {
				if s+len(q.prevBlock)-candidate <= q.MaxDistance && binary.LittleEndian.Uint32(src[s:]) == binary.LittleEndian.Uint32(q.prevBlock[candidate:]) {
					break
				}
			}
		}

		// A 4-byte match has been found. We'll later see if more than 4 bytes
		// match. But, prior to the match, src[nextEmit:s] are unmatched.

		// Call emitCopy, and then see if another emitCopy could be our next
		// move. Repeat until we find no match for the input immediately after
		// what was consumed by the last emitCopy call.
		//
		// If we exit this loop normally then we need to call emitLiteral next,
		// though we don't yet know how big the literal will be. We handle that
		// by proceeding to the next iteration of the main loop. We also can
		// exit this loop via goto if we get close to exhausting the input.
		for {
			// Invariant: we have a 4-byte match at s.
			base := s

			if candidate < s {
				s = extendMatch(src, candidate+4, s+4)
			} else {
				s = extendMatch2(q.prevBlock, candidate+4, src, s+4)
				candidate -= len(q.prevBlock)
			}

			for s-base > q.MaxLength {
				// The match is too long; break it up into shorter matches.
				length := q.MaxLength
				if s-base < q.MaxLength+4 {
					length = s - base - 4
				}
				dst = append(dst, Match{
					Unmatched: base - nextEmit,
					Length:    length,
					Distance:  base - candidate,
				})
				base += length
				candidate += length
				nextEmit = base
			}

			dst = append(dst, Match{
				Unmatched: base - nextEmit,
				Length:    s - base,
				Distance:  base - candidate,
			})
			nextEmit = s
			if s >= sLimit {
				goto emitRemainder
			}

			// We could immediately start working at s now, but to improve
			// compression we first update the hash table at s-1 and at s. If
			// another emitCopy is not our next move, also calculate nextHash
			// at s+1. At least on GOARCH=amd64, these three hash calculations
			// are faster as one load64 call (with some shifts) instead of
			// three load32 calls.
			x := binary.LittleEndian.Uint64(src[s-1:])
			prevHash := hash(uint32(x >> 0))
			q.table[prevHash&tableMask] = uint32(s - 1)
			currHash := hash(uint32(x >> 8))
			candidate = int(q.table[currHash&tableMask])
			q.table[currHash&tableMask] = uint32(s)
			if candidate == 0 {
				// Do nothing.
			} else if candidate < s {
				if s-candidate <= q.MaxDistance && uint32(x>>8) == binary.LittleEndian.Uint32(src[candidate:]) {
					continue
				}
			} else if q.ChainBlocks && candidate < len(q.prevBlock)-3 {
				if s+len(q.prevBlock)-candidate <= q.MaxDistance && uint32(x>>8) == binary.LittleEndian.Uint32(q.prevBlock[candidate:]) {
					continue
				}
			}
			nextHash = hash(uint32(x >> 16))
			s++
			break
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

func hash(u uint32) uint32 {
	return (u * 0x1e35a7bd) >> shift
}

// extendMatch returns the largest k such that k <= len(src) and that
// src[i:i+k-j] and src[j:k] have the same contents.
//
// It assumes that:
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

// extendMatch2 returns the largest k such that src1[i:i+k-j] and src2[j:k]
// have the same contents (and all these indexes are valid).
func extendMatch2(src1 []byte, i int, src2 []byte, j int) int {
	switch runtime.GOARCH {
	case "amd64":
		// As long as we are 8 or more bytes before the end of src, we can load and
		// compare 8 bytes at a time. If those 8 bytes are equal, repeat.
		for i+8 < len(src1) && j+8 < len(src2) {
			iBytes := binary.LittleEndian.Uint64(src1[i:])
			jBytes := binary.LittleEndian.Uint64(src2[j:])
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
		for i+4 < len(src1) && j+4 < len(src2) {
			iBytes := binary.LittleEndian.Uint32(src1[i:])
			jBytes := binary.LittleEndian.Uint32(src2[j:])
			if iBytes != jBytes {
				return j + bits.TrailingZeros32(iBytes^jBytes)>>3
			}
			i, j = i+4, j+4
		}
	}
	for ; i < len(src1) && j < len(src2) && src1[i] == src2[j]; i, j = i+1, j+1 {
	}
	return j
}
