package brotli

import "encoding/binary"

// H4 is a Hasher similar to what the reference implementation of brotli
// uses for compression level 4.
type H4 struct {
	table []uint32
}

const (
	h4TableBits = 17
	h4Sweep     = 4
	h4HashLen   = 5
)

func (h *H4) Init() {
	tableLen := 1<<h4TableBits + h4Sweep
	if len(h.table) < tableLen {
		h.table = make([]uint32, tableLen)
	} else {
		for i := range h.table {
			h.table[i] = 0
		}
	}
}

const kHashMul64 = 0x1E35A7BD1E35A7BD

func (h *H4) hash(data []byte) uint64 {
	hash := (binary.LittleEndian.Uint64(data) << (64 - 8*h4HashLen)) * kHashMul64
	return hash >> (64 - h4TableBits)
}

func (h *H4) Store(data []byte, index int) {
	hash := h.hash(data[index:])
	offset := index >> 3 % h4Sweep
	h.table[int(hash)+offset] = uint32(index)
}

func (h *H4) Candidates(dst []int, data []byte, index int) []int {
	hash := h.hash(data[index:])
	for _, c := range h.table[hash : hash+h4Sweep] {
		if c != 0 {
			dst = append(dst, int(c))
		}
	}

	offset := index >> 3 % h4Sweep
	h.table[int(hash)+offset] = uint32(index)

	return dst
}
