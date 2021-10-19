// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"encoding/binary"
	"math/bits"
)

type frameHeader struct {
	ContentSize   uint64
	WindowSize    uint32
	SingleSegment bool
	Checksum      bool
	DictID        uint32
}

var frameMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}

func (f frameHeader) appendTo(dst []byte) ([]byte, error) {
	dst = append(dst, frameMagic...)
	var fhd uint8
	if f.Checksum {
		fhd |= 1 << 2
	}
	if f.SingleSegment {
		fhd |= 1 << 5
	}

	var dictIDContent []byte
	if f.DictID > 0 {
		var tmp [4]byte
		if f.DictID < 256 {
			fhd |= 1
			tmp[0] = uint8(f.DictID)
			dictIDContent = tmp[:1]
		} else if f.DictID < 1<<16 {
			fhd |= 2
			binary.LittleEndian.PutUint16(tmp[:2], uint16(f.DictID))
			dictIDContent = tmp[:2]
		} else {
			fhd |= 3
			binary.LittleEndian.PutUint32(tmp[:4], f.DictID)
			dictIDContent = tmp[:4]
		}
	}
	var fcs uint8
	if f.ContentSize >= 256 {
		fcs++
	}
	if f.ContentSize >= 65536+256 {
		fcs++
	}
	if f.ContentSize >= 0xffffffff {
		fcs++
	}

	fhd |= fcs << 6

	dst = append(dst, fhd)
	if !f.SingleSegment {
		const winLogMin = 10
		windowLog := (bits.Len32(f.WindowSize-1) - winLogMin) << 3
		dst = append(dst, uint8(windowLog))
	}
	if f.DictID > 0 {
		dst = append(dst, dictIDContent...)
	}
	switch fcs {
	case 0:
		if f.SingleSegment {
			dst = append(dst, uint8(f.ContentSize))
		}
		// Unless SingleSegment is set, framessizes < 256 are nto stored.
	case 1:
		f.ContentSize -= 256
		dst = append(dst, uint8(f.ContentSize), uint8(f.ContentSize>>8))
	case 2:
		dst = append(dst, uint8(f.ContentSize), uint8(f.ContentSize>>8), uint8(f.ContentSize>>16), uint8(f.ContentSize>>24))
	case 3:
		dst = append(dst, uint8(f.ContentSize), uint8(f.ContentSize>>8), uint8(f.ContentSize>>16), uint8(f.ContentSize>>24),
			uint8(f.ContentSize>>32), uint8(f.ContentSize>>40), uint8(f.ContentSize>>48), uint8(f.ContentSize>>56))
	default:
		panic("invalid fcs")
	}
	return dst, nil
}
