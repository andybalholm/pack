package zstd

import (
	"github.com/andybalholm/pack"
)

type Encoder struct {
	block       *blockEnc
	wroteHeader bool
}

func (e *Encoder) Reset() {
	if e.block == nil {
		e.block = new(blockEnc)
		e.block.init()
	} else {
		e.block.reset(nil)
	}
	e.block.initNewEncode()
	e.wroteHeader = false
}

func (e *Encoder) Encode(dst []byte, src []byte, matches []pack.Match, lastBlock bool) []byte {
	initPredefined()
	if e.block == nil {
		e.block = new(blockEnc)
		e.block.init()
	} else {
		e.block.reset(nil)
	}

	if !e.wroteHeader {
		dst, _ = frameHeader{WindowSize: 1 << 23}.appendTo(dst)
		e.block.initNewEncode()
		e.wroteHeader = true
	}

	blk := e.block

	blk.pushOffsets()
	blk.last = lastBlock
	blk.size = len(src)

	pos := 0
	for _, m := range matches {
		blk.literals = append(blk.literals, src[pos:pos+m.Unmatched]...)
		if m.Length == 0 {
			blk.extraLits = m.Unmatched
			break
		}
		blk.sequences = append(blk.sequences, seq{
			litLen:   uint32(m.Unmatched),
			offset:   uint32(m.Distance + 3),
			matchLen: uint32(m.Length - 3),
		})
		pos += m.Unmatched + m.Length
	}

	err := blk.encode(src, false, false)
	switch err {
	case errIncompressible:
		blk.popOffsets()
		blk.reset(nil)
		err = blk.encodeLits(src, false)
		if err != nil {
			panic(err)
		}
	case nil:
	default:
		panic(err)
	}

	return append(dst, e.block.output...)
}
