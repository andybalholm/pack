package brotli

import "encoding/binary"

// H3 is a Hasher similar to what the reference implementation of brotli
// uses for compression level 3.
type H3 struct {
	table []uint32
}

const (
	h3TableBits = 16
	h3Sweep     = 2
	h3HashLen   = 5
)

func (h *H3) Init() {
	tableLen := 1<<h3TableBits + h3Sweep
	if len(h.table) < tableLen {
		h.table = make([]uint32, tableLen)
	} else {
		for i := range h.table {
			h.table[i] = 0
		}
	}
}

func (h *H3) hash(data []byte) uint64 {
	hash := (binary.LittleEndian.Uint64(data) << (64 - 8*h3HashLen)) * kHashMul64
	return hash >> (64 - h3TableBits)
}

func (h *H3) Store(data []byte, index int) {
	hash := h.hash(data[index:])
	offset := index >> 3 % h3Sweep
	h.table[int(hash)+offset] = uint32(index)
}

func (h *H3) Candidates(dst []int, data []byte, index int) []int {
	hash := h.hash(data[index:])
	for _, c := range h.table[hash : hash+h3Sweep] {
		if c != 0 {
			dst = append(dst, int(c))
		}
	}

	offset := index >> 3 % h3Sweep
	h.table[int(hash)+offset] = uint32(index)

	return dst
}
