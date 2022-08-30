package lz4

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

// BestSpeed is an implementation of the MatchFinder interface that is
// comparable to level 1 (BestSpeed) in compress/flate.
type BestSpeed struct {
	table     [maxTableSize]uint32
	prevBlock []byte
}

func (q *BestSpeed) Reset() {
	q.table = [maxTableSize]uint32{}
	q.prevBlock = q.prevBlock[:0]
}

const (
	maxTableSize = 1 << 14
	shift        = 32 - 14
	// tableMask is redundant, but helps the compiler eliminate bounds
	// checks.
	tableMask = maxTableSize - 1

	maxDistance = 65535
)

// FindMatches looks for matches in src, appends them to dst, and returns dst.
func (q *BestSpeed) FindMatches(dst []pack.Match, src []byte) []pack.Match {
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
			nextHash = hash4(binary.LittleEndian.Uint32(src[nextS:]))
			if candidate == 0 {
				continue
			} else if candidate < s {
				if s-candidate <= maxDistance && binary.LittleEndian.Uint32(src[s:]) == binary.LittleEndian.Uint32(src[candidate:]) {
					break
				}
			} else if candidate < len(q.prevBlock)-3 {
				if s+len(q.prevBlock)-candidate <= maxDistance && binary.LittleEndian.Uint32(src[s:]) == binary.LittleEndian.Uint32(q.prevBlock[candidate:]) {
					break
				}
			}
		}

		// A 4-byte match has been found. We'll later see if more than 4 bytes
		// match. But, prior to the match, src[nextEmit:s] are unmatched.
		base := s

		if candidate < s {
			s = extendMatch(src, candidate+4, s+4)
		} else {
			s = extendMatch2(q.prevBlock, candidate+4, src, s+4)
			candidate -= len(q.prevBlock)
		}

		dst = append(dst, pack.Match{
			Unmatched: base - nextEmit,
			Length:    s - base,
			Distance:  base - candidate,
		})
		nextEmit = s
		if s >= sLimit {
			goto emitRemainder
		}

		// We could immediately start working at s now, but to improve
		// compression we first update the hash table at s-1.
		x := binary.LittleEndian.Uint32(src[s-1:])
		prevHash := hash4(x)
		q.table[prevHash&tableMask] = uint32(s - 1)
	}

emitRemainder:
	if nextEmit < len(src) {
		dst = append(dst, pack.Match{
			Unmatched: len(src) - nextEmit,
		})
	}
	q.prevBlock = append(q.prevBlock[:0], src...)
	return dst
}

const hashMul32 = 0x1e35a7bd

func hash4(u uint32) uint32 {
	return (u * hashMul32) >> shift
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
