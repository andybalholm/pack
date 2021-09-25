package brotli

import (
	"encoding/binary"

	"github.com/andybalholm/pack"
)

// M0 is an implementation of the pack.MatchFinder interface based
// on the algorithm used by snappy, but modified to be more like the algorithm
// used by compression level 0 of the brotli reference implementation.
type M0 struct{}

func (M0) Reset() {}

const (
	m0HashLen = 5

	m0TableBits = 14
	m0TableSize = 1 << m0TableBits
	m0Shift     = 32 - m0TableBits
	// m0TableMask is redundant, but helps the compiler eliminate bounds
	// checks.
	m0TableMask = m0TableSize - 1
)

func (m M0) hash(data uint64) uint64 {
	hash := (data << (64 - 8*m0HashLen)) * kHashMul64
	return hash >> (64 - m0TableBits)
}

// FindMatches looks for matches in src, appends them to dst, and returns dst.
// src must not be longer than 65536 bytes.
func (m M0) FindMatches(dst []pack.Match, src []byte) []pack.Match {
	const inputMargin = 16 - 1
	const minNonLiteralBlockSize = 1 + 1 + inputMargin

	if len(src) < minNonLiteralBlockSize {
		dst = append(dst, pack.Match{
			Unmatched: len(src),
		})
		return dst
	}
	if len(src) > 65536 {
		panic("block too long")
	}

	var table [m0TableSize]uint16

	// sLimit is when to stop looking for offset/length copies. The inputMargin
	// lets us use a fast path for emitLiteral in the main loop, while we are
	// looking for copies.
	sLimit := len(src) - inputMargin

	// nextEmit is where in src the next emitLiteral should start from.
	nextEmit := 0

	// The encoded form must start with a literal, as there are no previous
	// bytes to copy, so we start looking for hash matches at s == 1.
	s := 1
	nextHash := m.hash(binary.LittleEndian.Uint64(src[s:]))

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
			candidate = int(table[nextHash&m0TableMask])
			table[nextHash&m0TableMask] = uint16(s)
			nextHash = m.hash(binary.LittleEndian.Uint64(src[nextS:]))
			if binary.LittleEndian.Uint32(src[s:]) == binary.LittleEndian.Uint32(src[candidate:]) {
				break
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

			s = extendMatch(src, candidate+4, s+4)

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
			// compression we first update the hash table at s-1 and at s. If
			// another emitCopy is not our next move, also calculate nextHash
			// at s+1. At least on GOARCH=amd64, these three hash calculations
			// are faster as one load64 call (with some shifts) instead of
			// three load32 calls.
			x := binary.LittleEndian.Uint64(src[s-1:])
			prevHash := m.hash(x >> 0)
			table[prevHash&m0TableMask] = uint16(s - 1)
			currHash := m.hash(x >> 8)
			candidate = int(table[currHash&m0TableMask])
			table[currHash&m0TableMask] = uint16(s)
			if uint32(x>>8) != binary.LittleEndian.Uint32(src[candidate:]) {
				nextHash = m.hash(x >> 16)
				s++
				break
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