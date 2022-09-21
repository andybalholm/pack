package pack

import (
	"encoding/binary"
	"math/bits"
	"runtime"
)

// HashChain is an implementation of the MatchFinder interface that
// uses hash chaining to find longer matches.
type HashChain struct {
	// SearchLen is how many entries to examine on the hash chain.
	// The default is 1.
	SearchLen int

	// MaxDistance is the maximum distance (in bytes) to look back for
	// a match. The default is 65535.
	MaxDistance int

	Parser Parser

	table [maxTableSize]uint32

	history []byte
	chain   []uint16
}

const (
	minHistory = 1 << 16
	maxHistory = 1 << 18

	maxTableSize = 1 << 14
	shift        = 32 - 14
	// tableMask is redundant, but helps the compiler eliminate bounds
	// checks.
	tableMask = maxTableSize - 1
)

func (q *HashChain) Reset() {
	q.table = [maxTableSize]uint32{}
	q.history = q.history[:0]
	q.chain = q.chain[:0]
}

// FindMatches looks for matches in src, appends them to dst, and returns dst.
func (q *HashChain) FindMatches(dst []Match, src []byte) []Match {
	if q.MaxDistance == 0 {
		q.MaxDistance = 65535
	}
	if q.SearchLen == 0 {
		q.SearchLen = 1
	}
	var nextEmit int

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

	return q.Parser.Parse(dst, q, nextEmit, len(src))
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

func (q *HashChain) Search(dst []AbsoluteMatch, pos, min, max int) []AbsoluteMatch {
	if pos >= len(q.chain) || pos+4 > len(q.history) {
		return dst
	}
	src := q.history
	searchSeq := binary.LittleEndian.Uint32(src[pos:])

	var length int

	candidate := pos
	for i := 0; i < q.SearchLen; i++ {
		d := q.chain[candidate]
		if d == 0 {
			break
		}
		candidate -= int(d)
		if candidate < 0 || pos-candidate > q.MaxDistance {
			break
		}
		if binary.LittleEndian.Uint32(src[candidate:]) != searchSeq {
			continue
		}

		newEnd := extendMatch(src[:max], candidate+4, pos+4)

		// Extend the match backward as far as possible.
		newStart := pos
		newMatch := candidate
		for newStart > min && newMatch > 0 && src[newStart-1] == src[newMatch-1] {
			newStart--
			newMatch--
		}

		if newEnd-newStart > length {
			dst = append(dst, AbsoluteMatch{
				Start: newStart,
				End:   newEnd,
				Match: newMatch,
			})
			length = newEnd - newStart
		}
	}

	return dst
}
