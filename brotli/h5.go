package brotli

import "encoding/binary"

// H5 is a Hasher similar to what the reference implementation of brotli uses
// for compression levels 5–9.
type H5 struct {
	// BlockBits is the base-2 logarithm of the number of entries per hash
	// bucket. The reference implementation sets it to one less than the
	// compression level.
	BlockBits int

	// BucketBits is the base-2 logarithm of the number of hash buckets.
	// The reference implementation sets it to 14 or 15.
	BucketBits int

	blockSize   int
	bucketCount int
	hashShift   int
	blockMask   int

	num     []uint16
	buckets []uint32
}

func (h *H5) Init() {
	h.hashShift = 32 - h.BucketBits
	h.bucketCount = 1 << h.BucketBits
	h.blockSize = 1 << h.BlockBits
	h.blockMask = h.blockSize - 1

	if len(h.num) < h.bucketCount {
		h.num = make([]uint16, h.bucketCount)
	} else {
		for i := range h.num {
			h.num[i] = 0
		}
	}

	if len(h.buckets) < h.bucketCount*h.blockSize {
		h.buckets = make([]uint32, h.bucketCount*h.blockSize)
	} else {
		for i := range h.buckets {
			h.buckets[i] = 0
		}
	}
}

const kHashMul32 uint32 = 0x1E35A7BD

func (h *H5) hash(data []byte) uint32 {
	hash := binary.LittleEndian.Uint32(data) * kHashMul32
	return hash >> h.hashShift
}

func (h *H5) Store(data []byte, index int) {
	key := h.hash(data[index:])
	minorIndex := int(h.num[key]) & h.blockMask
	h.buckets[int(key)<<h.BlockBits+minorIndex] = uint32(index)
	h.num[key]++
}

func (h *H5) Candidates(dst []int, data []byte, index int) []int {
	key := h.hash(data[index:])
	bucket := h.buckets[key<<h.BlockBits:]
	down := 0
	if int(h.num[key]) > h.blockSize {
		down = int(h.num[key]) - h.blockSize
	}
	for i := int(h.num[key]); i > down; {
		i--
		dst = append(dst, int(bucket[i&h.blockMask]))
	}
	bucket[int(h.num[key])&h.blockMask] = uint32(index)
	h.num[key]++
	return dst
}
