// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

type blockType uint8

//go:generate stringer -type=blockType,literalsBlockType,seqCompMode,tableIndex

const (
	blockTypeRaw blockType = iota
	blockTypeRLE
	blockTypeCompressed
	blockTypeReserved
)

type literalsBlockType uint8

const (
	literalsBlockRaw literalsBlockType = iota
	literalsBlockRLE
	literalsBlockCompressed
	literalsBlockTreeless
)

const (
	// maxCompressedBlockSize is the biggest allowed compressed block size (128KB)
	maxCompressedBlockSize = 128 << 10

	// Maximum possible block size (all Raw+Uncompressed).
	maxBlockSize = (1 << 21) - 1

	// https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md#literals_section_header
	maxCompressedLiteralSize = 1 << 18
	maxRLELiteralSize        = 1 << 20
	maxMatchLen              = 131074
	maxSequences             = 0x7f00 + 0xffff

	// We support slightly less than the reference decoder to be able to
	// use ints on 32 bit archs.
	maxOffsetBits = 30
)
