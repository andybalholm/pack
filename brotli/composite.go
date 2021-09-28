package brotli

// A CompositeHasher wraps two Hashers and combines their output.
type CompositeHasher struct {
	A, B Hasher
}

func (h CompositeHasher) Init() {
	h.A.Init()
	h.B.Init()
}

func (h CompositeHasher) Store(data []byte, index int) {
	h.A.Store(data, index)
	h.B.Store(data, index)
}

func (h CompositeHasher) Candidates(dst []int, data []byte, index int) []int {
	dst = h.A.Candidates(dst, data, index)
	dst = h.B.Candidates(dst, data, index)
	return dst
}
