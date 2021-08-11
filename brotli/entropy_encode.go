package brotli

/* Copyright 2010 Google Inc. All Rights Reserved.

   Distributed under MIT license.
   See file LICENSE for detail or copy at https://opensource.org/licenses/MIT
*/

/* Entropy encoding (Huffman) utilities. */

/* A node of a Huffman tree. */
type huffmanTree struct {
	total_count_          uint32
	index_left_           int16
	index_right_or_value_ int16
}

func initHuffmanTree(self *huffmanTree, count uint32, left int16, right int16) {
	self.total_count_ = count
	self.index_left_ = left
	self.index_right_or_value_ = right
}

/* Input size optimized Shell sort. */
type huffmanTreeComparator func(huffmanTree, huffmanTree) bool

var sortHuffmanTreeItems_gaps = []uint{132, 57, 23, 10, 4, 1}

func sortHuffmanTreeItems(items []huffmanTree, n uint, comparator huffmanTreeComparator) {
	if n < 13 {
		/* Insertion sort. */
		var i uint
		for i = 1; i < n; i++ {
			var tmp huffmanTree = items[i]
			var k uint = i
			var j uint = i - 1
			for comparator(tmp, items[j]) {
				items[k] = items[j]
				k = j
				if j == 0 {
					break
				}
				j--
			}

			items[k] = tmp
		}

		return
	} else {
		var g int
		if n < 57 {
			g = 2
		} else {
			g = 0
		}
		for ; g < 6; g++ {
			var gap uint = sortHuffmanTreeItems_gaps[g]
			var i uint
			for i = gap; i < n; i++ {
				var j uint = i
				var tmp huffmanTree = items[i]
				for ; j >= gap && comparator(tmp, items[j-gap]); j -= gap {
					items[j] = items[j-gap]
				}

				items[j] = tmp
			}
		}
	}
}

/* Returns 1 if assignment of depths succeeded, otherwise 0. */
func setDepth(p0 int, pool []huffmanTree, depth []byte, max_depth int) bool {
	var stack [16]int
	var level int = 0
	var p int = p0
	assert(max_depth <= 15)
	stack[0] = -1
	for {
		if pool[p].index_left_ >= 0 {
			level++
			if level > max_depth {
				return false
			}
			stack[level] = int(pool[p].index_right_or_value_)
			p = int(pool[p].index_left_)
			continue
		} else {
			depth[pool[p].index_right_or_value_] = byte(level)
		}

		for level >= 0 && stack[level] == -1 {
			level--
		}
		if level < 0 {
			return true
		}
		p = stack[level]
		stack[level] = -1
	}
}

var reverseBits_kLut = [16]uint{
	0x00,
	0x08,
	0x04,
	0x0C,
	0x02,
	0x0A,
	0x06,
	0x0E,
	0x01,
	0x09,
	0x05,
	0x0D,
	0x03,
	0x0B,
	0x07,
	0x0F,
}

func reverseBits(num_bits uint, bits uint16) uint16 {
	var retval uint = reverseBits_kLut[bits&0x0F]
	var i uint
	for i = 4; i < num_bits; i += 4 {
		retval <<= 4
		bits = uint16(bits >> 4)
		retval |= reverseBits_kLut[bits&0x0F]
	}

	retval >>= ((0 - num_bits) & 0x03)
	return uint16(retval)
}

/* 0..15 are values for bits */
const maxHuffmanBits = 16

/* Get the actual bit values for a tree of bit depths. */
func convertBitDepthsToSymbols(depth []byte, len uint, bits []uint16) {
	var bl_count = [maxHuffmanBits]uint16{0}
	var next_code [maxHuffmanBits]uint16
	var i uint
	/* In Brotli, all bit depths are [1..15]
	   0 bit depth means that the symbol does not exist. */

	var code int = 0
	for i = 0; i < len; i++ {
		bl_count[depth[i]]++
	}

	bl_count[0] = 0
	next_code[0] = 0
	for i = 1; i < maxHuffmanBits; i++ {
		code = (code + int(bl_count[i-1])) << 1
		next_code[i] = uint16(code)
	}

	for i = 0; i < len; i++ {
		if depth[i] != 0 {
			bits[i] = reverseBits(uint(depth[i]), next_code[depth[i]])
			next_code[depth[i]]++
		}
	}
}
