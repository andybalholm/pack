package flate

import (
	"io"

	"github.com/andybalholm/pack"
	"github.com/andybalholm/pack/brotli"
)

// NewWriter returns a new pack.Writer that compresses data at the given level,
// in flate encoding. Levels 1–9 are available; levels outside this range will
// be replaced with the closest level available.
func NewWriter(w io.Writer, level int) *pack.Writer {
	return newWriter(w, level, NewEncoder())
}

// NewGZIPWriter returns a new pack.Writer that compresses data at the given
// level, in gzip encoding. Levels 1–9 are available; levels outside this range
// will be replaced by the closest level available.
func NewGZIPWriter(w io.Writer, level int) *pack.Writer {
	return newWriter(w, level, NewGZIPEncoder())
}

func newWriter(w io.Writer, level int, e pack.Encoder) *pack.Writer {
	if level < 1 {
		level = 1
	}
	if level > 9 {
		level = 9
	}

	if level == 1 {
		return &pack.Writer{
			Dest: w,
			MatchFinder: brotli.M0{
				MaxDistance: 32768,
				MaxLength:   258,
			},
			Encoder:   e,
			BlockSize: 1 << 16,
		}
	}

	var h brotli.Hasher
	switch level {
	case 1:
		return &pack.Writer{
			Dest: w,
			MatchFinder: brotli.M0{
				MaxDistance: 32768,
				MaxLength:   258,
			},
			Encoder:   e,
			BlockSize: 1 << 16,
		}
	case 2:
		return &pack.Writer{
			Dest:        w,
			MatchFinder: &DualHash{},
			Encoder:     e,
			BlockSize:   1 << 16,
		}
	case 3:
		return &pack.Writer{
			Dest:        w,
			MatchFinder: &DualHash{Lazy: true},
			Encoder:     e,
			BlockSize:   1 << 16,
		}
	case 4:
		h = &brotli.H3{}
	case 5:
		h = &brotli.H4{}
	case 6:
		h = &brotli.H6{BlockBits: 3, BucketBits: 15, HashLen: 5}
	case 7:
		h = &brotli.H6{BlockBits: 4, BucketBits: 15, HashLen: 5}
	case 8:
		h = &brotli.CompositeHasher{
			A: &brotli.H5{BlockBits: 2, BucketBits: 15},
			B: &brotli.H6{BlockBits: 3, BucketBits: 15, HashLen: 8},
		}
	case 9:
		h = &brotli.CompositeHasher{
			A: &brotli.H5{BlockBits: 3, BucketBits: 15},
			B: &brotli.H6{BlockBits: 5, BucketBits: 15, HashLen: 8},
		}
	}

	return &pack.Writer{
		Dest: w,
		MatchFinder: &brotli.MatchFinder{
			Hasher:      h,
			MaxHistory:  1 << 17,
			MinHistory:  1 << 15,
			MaxDistance: 32768,
			MaxLength:   258,
		},
		Encoder:   e,
		BlockSize: 1 << 16,
	}
}
