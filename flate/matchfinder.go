// Copyright 2009 The Go Authors. All rights reserved.
// Copyright (c) 2015 Klaus Post
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flate

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"

	"github.com/andybalholm/pack"
)

const (
	logWindowSize    = 15
	windowSize       = 1 << logWindowSize
	windowMask       = windowSize - 1
	logMaxOffsetSize = 15  // Standard DEFLATE
	minMatchLength   = 4   // The smallest match that the compressor looks for
	maxMatchLength   = 258 // The longest match for the compressor
	minOffsetSize    = 1   // The shortest offset that makes any sense

	hashBits      = 17 // After 17 performance degrades
	hashSize      = 1 << hashBits
	hashMask      = (1 << hashBits) - 1
	hashShift     = (hashBits + minMatchLength - 1) / minMatchLength
	maxHashOffset = 1 << 24

	debugDeflate = false
)

type compressionLevel struct {
	good, lazy, nice, chain, level int
}

// Compression levels have been rebalanced from zlib deflate defaults
// to give a bigger spread in speed and compression.
// See https://blog.klauspost.com/rebalancing-deflate-compression-levels/
var levels = []compressionLevel{
	{}, // 0
	// Level 1-3 uses specialized algorithm - values not used
	{0, 0, 0, 0, 1},
	{0, 0, 0, 0, 2},
	{0, 0, 0, 0, 3},
	{4, 4, 8, 8, 4},
	{4, 4, 12, 12, 5},
	{4, 6, 16, 16, 6},
	{8, 8, 24, 16, 7},
	{10, 16, 24, 64, 8},
	{32, 258, 258, 4096, 9},
}

// advancedState contains state for the advanced levels, with bigger hash tables, etc.
type advancedState struct {
	// deflate state
	length         int
	offset         int
	maxInsertIndex int

	// Input hash chains
	// hashHead[hashValue] contains the largest inputIndex with the specified hash value
	// If hashHead[hashValue] is within the current window, then
	// hashPrev[hashHead[hashValue] & windowMask] contains the previous index
	// with the same hash value.
	chainHead  int
	hashHead   [hashSize]uint32
	hashPrev   [windowSize]uint32
	hashOffset int

	// input window: unprocessed data is window[index:windowEnd]
	index     int
	hashMatch [maxMatchLength + minMatchLength]uint32

	hash uint32
	ii   uint16 // position of last match, intended to overflow to reset.
}

// compressor is the compressor from github.com/klauspost/compress/flate,
// modified to implement pack.MatchFinder.
type compressor struct {
	compressionLevel

	window     []byte
	windowEnd  int
	blockStart int // window index where current tokens start

	// queued output tokens
	matches []pack.Match

	state *advancedState

	sync          bool // requesting flush
	byteAvailable bool // if true, still need to process window[index-1].
	unmatched     int  // unmatched bytes to output with the next match
}

func (d *compressor) fillDeflate(b []byte) int {
	s := d.state
	if s.index >= 2*windowSize-(minMatchLength+maxMatchLength) {
		// shift the window by windowSize
		copy(d.window[:], d.window[windowSize:2*windowSize])
		s.index -= windowSize
		d.windowEnd -= windowSize
		if d.blockStart >= windowSize {
			d.blockStart -= windowSize
		} else {
			d.blockStart = math.MaxInt32
		}
		s.hashOffset += windowSize
		if s.hashOffset > maxHashOffset {
			delta := s.hashOffset - 1
			s.hashOffset -= delta
			s.chainHead -= delta
			// Iterate over slices instead of arrays to avoid copying
			// the entire table onto the stack (Issue #18625).
			for i, v := range s.hashPrev[:] {
				if int(v) > delta {
					s.hashPrev[i] = uint32(int(v) - delta)
				} else {
					s.hashPrev[i] = 0
				}
			}
			for i, v := range s.hashHead[:] {
				if int(v) > delta {
					s.hashHead[i] = uint32(int(v) - delta)
				} else {
					s.hashHead[i] = 0
				}
			}
		}
	}
	n := copy(d.window[d.windowEnd:], b)
	d.windowEnd += n
	return n
}

// Try to find a match starting at index whose length is greater than prevSize.
// We only look at chainCount possibilities before giving up.
// pos = s.index, prevHead = s.chainHead-s.hashOffset, prevLength=minMatchLength-1, lookahead
func (d *compressor) findMatch(pos int, prevHead int, prevLength int, lookahead int) (length, offset int, ok bool) {
	minMatchLook := maxMatchLength
	if lookahead < minMatchLook {
		minMatchLook = lookahead
	}

	win := d.window[0 : pos+minMatchLook]

	// We quit when we get a match that's at least nice long
	nice := len(win) - pos
	if d.nice < nice {
		nice = d.nice
	}

	// If we've got a match that's good enough, only look in 1/4 the chain.
	tries := d.chain
	length = prevLength
	if length >= d.good {
		tries >>= 2
	}

	wEnd := win[pos+length]
	wPos := win[pos:]
	minIndex := pos - windowSize

	for i := prevHead; tries > 0; tries-- {
		if wEnd == win[i+length] {
			n := matchLen(win[i:i+minMatchLook], wPos)

			if n > length && (n > minMatchLength || pos-i <= 4096) {
				length = n
				offset = pos - i
				ok = true
				if n >= nice {
					// The match is good enough that we don't try to find a better one.
					break
				}
				wEnd = win[pos+n]
			}
		}
		if i == minIndex {
			// hashPrev[i & windowMask] has already been overwritten, so stop now.
			break
		}
		i = int(d.state.hashPrev[i&windowMask]) - d.state.hashOffset
		if i < minIndex || i < 0 {
			break
		}
	}
	return
}

// hash4 returns a hash representation of the first 4 bytes
// of the supplied slice.
// The caller must ensure that len(b) >= 4.
func hash4(b []byte) uint32 {
	b = b[:4]
	return hash4u(uint32(b[3])|uint32(b[2])<<8|uint32(b[1])<<16|uint32(b[0])<<24, hashBits)
}

const prime4bytes = 2654435761
const reg8SizeMask32 = 31

// hash4 returns the hash of u to fit in a hash table with h bits.
// Preferably h should be a constant and should always be <32.
func hash4u(u uint32, h uint8) uint32 {
	return (u * prime4bytes) >> ((32 - h) & reg8SizeMask32)
}

// bulkHash4 will compute hashes using the same
// algorithm as hash4
func bulkHash4(b []byte, dst []uint32) {
	if len(b) < 4 {
		return
	}
	hb := uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
	dst[0] = hash4u(hb, hashBits)
	end := len(b) - 4 + 1
	for i := 1; i < end; i++ {
		hb = (hb << 8) | uint32(b[i+3])
		dst[i] = hash4u(hb, hashBits)
	}
}

func (d *compressor) initDeflate() {
	d.window = make([]byte, 2*windowSize)
	d.byteAvailable = false
	if d.state == nil {
		return
	}
	s := d.state
	s.index = 0
	s.hashOffset = 1
	s.length = minMatchLength - 1
	s.offset = 0
	s.hash = 0
	s.chainHead = -1
}

// deflateLazy is the same as deflate, but with d.fastSkipHashing == skipNever,
// meaning it always has lazy matching on.
func (d *compressor) deflateLazy() {
	s := d.state
	// Sanity enables additional runtime tests.
	// It's intended to be used during development
	// to supplement the currently ad-hoc unit tests.
	const sanity = debugDeflate

	if d.windowEnd-s.index < minMatchLength+maxMatchLength && !d.sync {
		return
	}

	s.maxInsertIndex = d.windowEnd - (minMatchLength - 1)
	if s.index < s.maxInsertIndex {
		s.hash = hash4(d.window[s.index : s.index+minMatchLength])
	}

	for {
		if sanity && s.index > d.windowEnd {
			panic("index > windowEnd")
		}
		lookahead := d.windowEnd - s.index
		if lookahead < minMatchLength+maxMatchLength {
			if !d.sync {
				return
			}
			if sanity && s.index > d.windowEnd {
				panic("index > windowEnd")
			}
			if lookahead == 0 {
				// Flush current output block if any.
				if d.byteAvailable {
					// There is still one pending token that needs to be flushed
					d.unmatched++
					d.byteAvailable = false
				}
				return
			}
		}
		if s.index < s.maxInsertIndex {
			// Update the hash
			s.hash = hash4(d.window[s.index : s.index+minMatchLength])
			ch := s.hashHead[s.hash&hashMask]
			s.chainHead = int(ch)
			s.hashPrev[s.index&windowMask] = ch
			s.hashHead[s.hash&hashMask] = uint32(s.index + s.hashOffset)
		}
		prevLength := s.length
		prevOffset := s.offset
		s.length = minMatchLength - 1
		s.offset = 0
		minIndex := s.index - windowSize
		if minIndex < 0 {
			minIndex = 0
		}

		if s.chainHead-s.hashOffset >= minIndex && lookahead > prevLength && prevLength < d.lazy {
			if newLength, newOffset, ok := d.findMatch(s.index, s.chainHead-s.hashOffset, minMatchLength-1, lookahead); ok {
				s.length = newLength
				s.offset = newOffset
			}
		}
		if prevLength >= minMatchLength && s.length <= prevLength {
			// There was a match at the previous step, and the current match is
			// not better. Output the previous match.
			d.matches = append(d.matches, pack.Match{
				Unmatched: d.unmatched,
				Length:    prevLength,
				Distance:  prevOffset,
			})
			d.unmatched = 0

			// Insert in the hash table all strings up to the end of the match.
			// index and index-1 are already inserted. If there is not enough
			// lookahead, the last two strings are not inserted into the hash
			// table.
			newIndex := s.index + prevLength - 1
			// Calculate missing hashes
			end := newIndex
			if end > s.maxInsertIndex {
				end = s.maxInsertIndex
			}
			end += minMatchLength - 1
			startindex := s.index + 1
			if startindex > s.maxInsertIndex {
				startindex = s.maxInsertIndex
			}
			tocheck := d.window[startindex:end]
			dstSize := len(tocheck) - minMatchLength + 1
			if dstSize > 0 {
				dst := s.hashMatch[:dstSize]
				bulkHash4(tocheck, dst)
				var newH uint32
				for i, val := range dst {
					di := i + startindex
					newH = val & hashMask
					// Get previous value with the same hash.
					// Our chain should point to the previous value.
					s.hashPrev[di&windowMask] = s.hashHead[newH]
					// Set the head of the hash chain to us.
					s.hashHead[newH] = uint32(di + s.hashOffset)
				}
				s.hash = newH
			}

			s.index = newIndex
			d.byteAvailable = false
			s.length = minMatchLength - 1
		} else {
			// Reset, if we got a match this run.
			if s.length >= minMatchLength {
				s.ii = 0
			}
			// We have a byte waiting. Emit it.
			if d.byteAvailable {
				s.ii++
				d.unmatched++
				s.index++

				// If we have a long run of no matches, skip additional bytes
				// Resets when s.ii overflows after 64KB.
				if s.ii > 31 {
					n := int(s.ii >> 5)
					for j := 0; j < n; j++ {
						if s.index >= d.windowEnd-1 {
							break
						}

						d.unmatched++
						s.index++
					}
					// Flush last byte
					d.unmatched++
					d.byteAvailable = false
				}
			} else {
				s.index++
				d.byteAvailable = true
			}
		}
	}
}

func (d *compressor) FindMatches(dst []pack.Match, b []byte) []pack.Match {
	d.matches = dst
	for len(b) > 0 {
		d.deflateLazy()
		b = b[d.fillDeflate(b):]
	}
	d.syncFlush()
	if d.unmatched > 0 {
		d.matches = append(d.matches, pack.Match{
			Unmatched: d.unmatched,
		})
		d.unmatched = 0
	}
	return d.matches
}

func (d *compressor) syncFlush() {
	d.sync = true
	d.deflateLazy()
	d.sync = false
}

func (d *compressor) init(level int) (err error) {
	switch {
	case 4 <= level && level <= 9:
		d.state = &advancedState{}
		d.compressionLevel = levels[level]
		d.initDeflate()
	default:
		return fmt.Errorf("flate: invalid compression level %d: want value in range [7, 9]", level)
	}
	d.level = level
	return nil
}

// reset the state of the compressor.
func (d *compressor) Reset() {
	d.sync = false
	s := d.state
	s.chainHead = -1
	for i := range s.hashHead {
		s.hashHead[i] = 0
	}
	for i := range s.hashPrev {
		s.hashPrev[i] = 0
	}
	s.hashOffset = 1
	s.index, d.windowEnd = 0, 0
	d.blockStart, d.byteAvailable = 0, false
	d.matches = d.matches[:0]
	s.length = minMatchLength - 1
	s.offset = 0
	s.hash = 0
	s.ii = 0
	s.maxInsertIndex = 0
	d.unmatched = 0
}

// matchLen returns the maximum length.
// 'a' must be the shortest of the two.
func matchLen(a, b []byte) int {
	var checked int

	for len(a) >= 8 {
		if diff := binary.LittleEndian.Uint64(a) ^ binary.LittleEndian.Uint64(b); diff != 0 {
			return checked + (bits.TrailingZeros64(diff) >> 3)
		}
		checked += 8
		a = a[8:]
		b = b[8:]
	}
	b = b[:len(a)]
	for i := range a {
		if a[i] != b[i] {
			return i + checked
		}
	}
	return len(a) + checked
}
