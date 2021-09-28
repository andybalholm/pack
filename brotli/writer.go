package brotli

import (
	"io"

	"github.com/andybalholm/pack"
)

// NewWriter returns a new pack.Writer that compresses data at the given level.
// Levels 0–9 are currently implemented. Levels outside this range will be
// replaced with the closest level available.
func NewWriter(w io.Writer, level int) *pack.Writer {
	if level < 0 {
		level = 0
	}
	if level > 9 {
		level = 9
	}

	if level < 2 {
		return &pack.Writer{
			Dest:        w,
			MatchFinder: M0{Lazy: level == 1},
			Encoder:     &Encoder{},
			BlockSize:   1 << 16,
		}
	}

	var h Hasher
	switch level {
	case 2:
		h = &H2{}
	case 3:
		h = &H3{}
	case 4:
		h = &H4{}
	case 5:
		h = &H6{BlockBits: 3, BucketBits: 15, HashLen: 5}
	case 6:
		h = &CompositeHasher{
			A: &H4{},
			B: &H6{BlockBits: 2, BucketBits: 15, HashLen: 8},
		}
	case 7:
		h = &CompositeHasher{
			A: &H5{BlockBits: 3, BucketBits: 15},
			B: &H6{BlockBits: 3, BucketBits: 15, HashLen: 8},
		}
	case 8:
		h = &CompositeHasher{
			A: &H5{BlockBits: 3, BucketBits: 15},
			B: &H6{BlockBits: 5, BucketBits: 15, HashLen: 8},
		}
	case 9:
		h = &CompositeHasher{
			A: &H5{BlockBits: 4, BucketBits: 15},
			B: &H6{BlockBits: 6, BucketBits: 15, HashLen: 8},
		}
	}

	return &pack.Writer{
		Dest: w,
		MatchFinder: &MatchFinder{
			Hasher:     h,
			MaxHistory: 1 << 20,
			MinHistory: 1 << 16,
		},
		Encoder:   &Encoder{},
		BlockSize: 1 << 16,
	}
}
