package brotli

import (
	"math"
	"sync"
)

/* Represents the range of values belonging to a prefix code:
   [offset, offset + 2^nbits) */
type prefixCodeRange struct {
	offset uint32
	nbits  uint32
}

var kBlockLengthPrefixCode = [numBlockLenSymbols]prefixCodeRange{
	prefixCodeRange{1, 2},
	prefixCodeRange{5, 2},
	prefixCodeRange{9, 2},
	prefixCodeRange{13, 2},
	prefixCodeRange{17, 3},
	prefixCodeRange{25, 3},
	prefixCodeRange{33, 3},
	prefixCodeRange{41, 3},
	prefixCodeRange{49, 4},
	prefixCodeRange{65, 4},
	prefixCodeRange{81, 4},
	prefixCodeRange{97, 4},
	prefixCodeRange{113, 5},
	prefixCodeRange{145, 5},
	prefixCodeRange{177, 5},
	prefixCodeRange{209, 5},
	prefixCodeRange{241, 6},
	prefixCodeRange{305, 6},
	prefixCodeRange{369, 7},
	prefixCodeRange{497, 8},
	prefixCodeRange{753, 9},
	prefixCodeRange{1265, 10},
	prefixCodeRange{2289, 11},
	prefixCodeRange{4337, 12},
	prefixCodeRange{8433, 13},
	prefixCodeRange{16625, 24},
}

func sortHuffmanTree1(v0 huffmanTree, v1 huffmanTree) bool {
	return v0.total_count_ < v1.total_count_
}

var huffmanTreePool sync.Pool

func buildAndStoreHuffmanTreeFast(histogram []uint32, histogram_total uint, max_bits uint, depth []byte, bits []uint16, bw *bitWriter) {
	var count uint = 0
	var symbols = [4]uint{0}
	var length uint = 0
	var total uint = histogram_total
	for total != 0 {
		if histogram[length] != 0 {
			if count < 4 {
				symbols[count] = length
			}

			count++
			total -= uint(histogram[length])
		}

		length++
	}

	if count <= 1 {
		bw.writeBits(4, 1)
		bw.writeBits(max_bits, uint64(symbols[0]))
		depth[symbols[0]] = 0
		bits[symbols[0]] = 0
		return
	}

	for i := 0; i < int(length); i++ {
		depth[i] = 0
	}
	{
		var max_tree_size uint = 2*length + 1
		tree, _ := huffmanTreePool.Get().(*[]huffmanTree)
		if tree == nil || cap(*tree) < int(max_tree_size) {
			tmp := make([]huffmanTree, max_tree_size)
			tree = &tmp
		} else {
			*tree = (*tree)[:max_tree_size]
		}
		var count_limit uint32
		for count_limit = 1; ; count_limit *= 2 {
			var node int = 0
			var l uint
			for l = length; l != 0; {
				l--
				if histogram[l] != 0 {
					if histogram[l] >= count_limit {
						initHuffmanTree(&(*tree)[node:][0], histogram[l], -1, int16(l))
					} else {
						initHuffmanTree(&(*tree)[node:][0], count_limit, -1, int16(l))
					}

					node++
				}
			}
			{
				var n int = node
				/* Points to the next leaf node. */ /* Points to the next non-leaf node. */
				var sentinel huffmanTree
				var i int = 0
				var j int = n + 1
				var k int

				sortHuffmanTreeItems(*tree, uint(n), huffmanTreeComparator(sortHuffmanTree1))

				/* The nodes are:
				   [0, n): the sorted leaf nodes that we start with.
				   [n]: we add a sentinel here.
				   [n + 1, 2n): new parent nodes are added here, starting from
				                (n+1). These are naturally in ascending order.
				   [2n]: we add a sentinel at the end as well.
				   There will be (2n+1) elements at the end. */
				initHuffmanTree(&sentinel, math.MaxUint32, -1, -1)

				(*tree)[node] = sentinel
				node++
				(*tree)[node] = sentinel
				node++

				for k = n - 1; k > 0; k-- {
					var left int
					var right int
					if (*tree)[i].total_count_ <= (*tree)[j].total_count_ {
						left = i
						i++
					} else {
						left = j
						j++
					}

					if (*tree)[i].total_count_ <= (*tree)[j].total_count_ {
						right = i
						i++
					} else {
						right = j
						j++
					}

					/* The sentinel node becomes the parent node. */
					(*tree)[node-1].total_count_ = (*tree)[left].total_count_ + (*tree)[right].total_count_

					(*tree)[node-1].index_left_ = int16(left)
					(*tree)[node-1].index_right_or_value_ = int16(right)

					/* Add back the last sentinel node. */
					(*tree)[node] = sentinel
					node++
				}

				if setDepth(2*n-1, *tree, depth, 14) {
					/* We need to pack the Huffman tree in 14 bits. If this was not
					   successful, add fake entities to the lowest values and retry. */
					break
				}
			}
		}

		huffmanTreePool.Put(tree)
	}

	convertBitDepthsToSymbols(depth, length, bits)
	if count <= 4 {
		var i uint

		/* value of 1 indicates a simple Huffman code */
		bw.writeBits(2, 1)

		bw.writeBits(2, uint64(count)-1) /* NSYM - 1 */

		/* Sort */
		for i = 0; i < count; i++ {
			var j uint
			for j = i + 1; j < count; j++ {
				if depth[symbols[j]] < depth[symbols[i]] {
					var tmp uint = symbols[j]
					symbols[j] = symbols[i]
					symbols[i] = tmp
				}
			}
		}

		if count == 2 {
			bw.writeBits(max_bits, uint64(symbols[0]))
			bw.writeBits(max_bits, uint64(symbols[1]))
		} else if count == 3 {
			bw.writeBits(max_bits, uint64(symbols[0]))
			bw.writeBits(max_bits, uint64(symbols[1]))
			bw.writeBits(max_bits, uint64(symbols[2]))
		} else {
			bw.writeBits(max_bits, uint64(symbols[0]))
			bw.writeBits(max_bits, uint64(symbols[1]))
			bw.writeBits(max_bits, uint64(symbols[2]))
			bw.writeBits(max_bits, uint64(symbols[3]))

			/* tree-select */
			bw.writeSingleBit(depth[symbols[0]] == 1)
		}
	} else {
		var previous_value byte = 8
		var i uint

		/* Complex Huffman Tree */
		storeStaticCodeLengthCode(bw)

		/* Actual RLE coding. */
		for i = 0; i < length; {
			var value byte = depth[i]
			var reps uint = 1
			var k uint
			for k = i + 1; k < length && depth[k] == value; k++ {
				reps++
			}

			i += reps
			if value == 0 {
				bw.writeBits(uint(kZeroRepsDepth[reps]), kZeroRepsBits[reps])
			} else {
				if previous_value != value {
					bw.writeBits(uint(kCodeLengthDepth[value]), uint64(kCodeLengthBits[value]))
					reps--
				}

				if reps < 3 {
					for reps != 0 {
						reps--
						bw.writeBits(uint(kCodeLengthDepth[value]), uint64(kCodeLengthBits[value]))
					}
				} else {
					reps -= 3
					bw.writeBits(uint(kNonZeroRepsDepth[reps]), kNonZeroRepsBits[reps])
				}

				previous_value = value
			}
		}
	}
}
