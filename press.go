// The press package is a modular system for data compression.
//
// Many compression libraries have two main parts:
//  - Something that looks for repeated sequences of bytes
//  - An encoder for the compressed data format (often an entropy coder)
//
// Although these are logically two separate steps, the implementations are
// usually closely tied together. You can't use flate's matcher with snappy's
// encoder, for example. This packages defines interfaces and an intermediate
// representation to allow mixing and matching compression components.
package press

// A Match is the basic unit of LZ77 compression.
type Match struct {
	Unmatched int // the number of unmatched bytes since the previous match
	Length    int // the number of bytes in the matched string; it may be 0 at the end of the input
	Distance  int // how far back in the stream to copy from
}

// A MatchFinder performs the LZ77 stage of compression, looking for matches.
type MatchFinder interface {
	// FindMatches looks for matches in src, appends them to dst, and returns dst.
	FindMatches(dst []Match, src []byte) []Match

	// Reset clears any internal state, preparing the MatchFinder to be used with
	// a new stream.
	Reset()
}

// An Encoder encodes the data in its final format.
type Encoder interface {
	// Header appends the appropriate stream header to dst.
	Header(dst []byte) []byte

	// Encode appends the encoded format of src to dst, using the match
	// information from matches.
	Encode(dst []byte, src []byte, matches []Match, lastBlock bool) []byte

	// Reset clears any internal state, preparing the Encoder to be used with
	// a new stream.
	Reset()
}
