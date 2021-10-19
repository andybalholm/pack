// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"errors"
	"fmt"
	"math/bits"
)

const (
	tablelogAbsoluteMax = 9
)

const (
	/*!MEMORY_USAGE :
	 *  Memory usage formula : N->2^N Bytes (examples : 10 -> 1KB; 12 -> 4KB ; 16 -> 64KB; 20 -> 1MB; etc.)
	 *  Increasing memory usage improves compression ratio
	 *  Reduced memory usage can improve speed, due to cache effect
	 *  Recommended max value is 14, for 16KB, which nicely fits into Intel x86 L1 cache */
	maxMemoryUsage = tablelogAbsoluteMax + 2

	maxTableLog    = maxMemoryUsage - 2
	maxTablesize   = 1 << maxTableLog
	maxTableMask   = (1 << maxTableLog) - 1
	minTablelog    = 5
	maxSymbolValue = 255
)

// fseDecoder provides temporary storage for compression and decompression.
type fseDecoder struct {
	dt             [maxTablesize]decSymbol // Decompression table.
	symbolLen      uint16                  // Length of active part of the symbol table.
	actualTableLog uint8                   // Selected tablelog.
	maxBits        uint8                   // Maximum number of additional bits

	// used for table creation to avoid allocations.
	stateTable [256]uint16
	norm       [maxSymbolValue + 1]int16
	preDefined bool
}

// tableStep returns the next table index.
func tableStep(tableSize uint32) uint32 {
	return (tableSize >> 1) + (tableSize >> 3) + 3
}

// decSymbol contains information about a state entry,
// Including the state offset base, the output symbol and
// the number of bits to read for the low part of the destination state.
// Using a composite uint64 is faster than a struct with separate members.
type decSymbol uint64

func (d decSymbol) addBits() uint8 {
	return uint8(d >> 8)
}

func (d *decSymbol) setNBits(nBits uint8) {
	const mask = 0xffffffffffffff00
	*d = (*d & mask) | decSymbol(nBits)
}

func (d *decSymbol) setAddBits(addBits uint8) {
	const mask = 0xffffffffffff00ff
	*d = (*d & mask) | (decSymbol(addBits) << 8)
}

func (d *decSymbol) setNewState(state uint16) {
	const mask = 0xffffffff0000ffff
	*d = (*d & mask) | decSymbol(state)<<16
}

func (d *decSymbol) setExt(addBits uint8, baseline uint32) {
	const mask = 0xffff00ff
	*d = (*d & mask) | (decSymbol(addBits) << 8) | (decSymbol(baseline) << 32)
}

func highBits(val uint32) (n uint32) {
	return uint32(bits.Len32(val) - 1)
}

// buildDtable will build the decoding table.
func (s *fseDecoder) buildDtable() error {
	tableSize := uint32(1 << s.actualTableLog)
	highThreshold := tableSize - 1
	symbolNext := s.stateTable[:256]

	// Init, lay down lowprob symbols
	{
		for i, v := range s.norm[:s.symbolLen] {
			if v == -1 {
				s.dt[highThreshold].setAddBits(uint8(i))
				highThreshold--
				symbolNext[i] = 1
			} else {
				symbolNext[i] = uint16(v)
			}
		}
	}
	// Spread symbols
	{
		tableMask := tableSize - 1
		step := tableStep(tableSize)
		position := uint32(0)
		for ss, v := range s.norm[:s.symbolLen] {
			for i := 0; i < int(v); i++ {
				s.dt[position].setAddBits(uint8(ss))
				position = (position + step) & tableMask
				for position > highThreshold {
					// lowprob area
					position = (position + step) & tableMask
				}
			}
		}
		if position != 0 {
			// position must reach all cells once, otherwise normalizedCounter is incorrect
			return errors.New("corrupted input (position != 0)")
		}
	}

	// Build Decoding table
	{
		tableSize := uint16(1 << s.actualTableLog)
		for u, v := range s.dt[:tableSize] {
			symbol := v.addBits()
			nextState := symbolNext[symbol]
			symbolNext[symbol] = nextState + 1
			nBits := s.actualTableLog - byte(highBits(uint32(nextState)))
			s.dt[u&maxTableMask].setNBits(nBits)
			newState := (nextState << nBits) - tableSize
			if newState > tableSize {
				return fmt.Errorf("newState (%d) outside table size (%d)", newState, tableSize)
			}
			if newState == uint16(u) && nBits == 0 {
				// Seems weird that this is possible with nbits > 0.
				return fmt.Errorf("newState (%d) == oldState (%d) and no bits", newState, u)
			}
			s.dt[u&maxTableMask].setNewState(newState)
		}
	}
	return nil
}

// transform will transform the decoder table into a table usable for
// decoding without having to apply the transformation while decoding.
// The state will contain the base value and the number of bits to read.
func (s *fseDecoder) transform(t []baseOffset) error {
	tableSize := uint16(1 << s.actualTableLog)
	s.maxBits = 0
	for i, v := range s.dt[:tableSize] {
		add := v.addBits()
		if int(add) >= len(t) {
			return fmt.Errorf("invalid decoding table entry %d, symbol %d >= max (%d)", i, v.addBits(), len(t))
		}
		lu := t[add]
		if lu.addBits > s.maxBits {
			s.maxBits = lu.addBits
		}
		v.setExt(lu.addBits, lu.baseLine)
		s.dt[i] = v
	}
	return nil
}
