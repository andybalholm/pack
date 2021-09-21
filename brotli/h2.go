package brotli

import "encoding/binary"

// H2 is a Hasher similar to what the reference implementation of brotli
// uses for compression level 2.
type H2 struct {
	table []uint32
}

const (
	h2TableBits = 16
	h2HashLen   = 5
)

func (h *H2) Init() {
	tableLen := 1 << h2TableBits
	if len(h.table) < tableLen {
		h.table = make([]uint32, tableLen)
	} else {
		for i := range h.table {
			h.table[i] = 0
		}
	}
}

func (h *H2) hash(data []byte) uint64 {
	hash := (binary.LittleEndian.Uint64(data) << (64 - 8*h2HashLen)) * kHashMul64
	return hash >> (64 - h2TableBits)
}

func (h *H2) Store(data []byte, index int) {
	hash := h.hash(data[index:])
	h.table[hash] = uint32(index)
}

func (h *H2) Candidates(dst []int, data []byte, index int) []int {
	hash := h.hash(data[index:])
	c := h.table[hash]
	if c != 0 {
		dst = append(dst, int(c))
	}

	h.table[hash] = uint32(index)

	return dst
}
