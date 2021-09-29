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
	return &pack.Writer{
		Dest:        w,
		MatchFinder: NewMatchFinder(level),
		Encoder:     e,
		BlockSize:   1 << 16,
	}
}

func NewMatchFinder(level int) pack.MatchFinder {
	if level < 1 {
		level = 1
	}
	if level > 9 {
		level = 9
	}

	var h brotli.Hasher
	switch level {
	case 1:
		return brotli.M0{
			MaxDistance: 32768,
			MaxLength:   258,
		}
	case 2:
		return &DualHash{}
	case 3:
		return &DualHash{Lazy: true}
	case 4, 5, 6, 7, 8, 9:
		c := new(compressor)
		c.init(level)
		return c
	}

	return &brotli.MatchFinder{
		Hasher:      h,
		MaxHistory:  1 << 17,
		MinHistory:  1 << 15,
		MaxDistance: 32768,
		MaxLength:   258,
	}
}
