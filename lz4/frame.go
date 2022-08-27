package lz4

import (
	"encoding/binary"
	"hash"

	"github.com/andybalholm/pack"
	"github.com/pierrec/xxHash/xxHash32"
)

// A FrameEncoder implements the pack.Encoder interface,
// writing in the LZ4 frame format.
type FrameEncoder struct {
	hasher      hash.Hash32
	blockBuffer []byte
}

func (f *FrameEncoder) Reset() {
	f.hasher = nil
}

func (f *FrameEncoder) Encode(dst []byte, src []byte, matches []pack.Match, lastBlock bool) []byte {
	if f.hasher == nil {
		f.hasher = xxHash32.New(0)
		dst = binary.LittleEndian.AppendUint32(dst, 0x184D2204)
		// Frame header for content checksum enabled, and 4-MB blocks.
		dst = append(dst, 0x44, 0x70, 0x1d)
	}

	var be BlockEncoder
	f.blockBuffer = be.Encode(f.blockBuffer[:0], src, matches, lastBlock)
	dst = binary.LittleEndian.AppendUint32(dst, uint32(len(f.blockBuffer)))
	dst = append(dst, f.blockBuffer...)

	f.hasher.Write(src)

	if lastBlock {
		dst = append(dst, 0, 0, 0, 0)
		dst = binary.LittleEndian.AppendUint32(dst, f.hasher.Sum32())
	}

	return dst
}
