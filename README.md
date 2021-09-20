# Pack

Interfaces for LZ77-based data compression.

## Introduction

Many compression libraries have two main parts:

- Something that looks for repeated sequences of bytes
- An encoder for the compressed data format (often an entropy coder)

Although these are logically two separate steps, the implementations are
usually closely tied together. You can't use flate's matcher with snappy's
encoder, for example. Pack defines interfaces and an intermediate
representation to allow mixing and matching compression components.

## Interfaces

Pack defines two interfaces, `MatchFinder` and `Encoder`, 
and a `Writer` type to tie them together and implement `io.Writer`.
Both of the interfaces use an append-like API,
where a destination slice is be passed as the first parameter,
and the output is appended to it. 
If you pass `nil` as the destination, you will get a newly-allocated slice.

```go
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
	// Encode appends the encoded format of src to dst, using the match
	// information from matches.
	Encode(dst []byte, src []byte, matches []Match, lastBlock bool) []byte

	// Reset clears any internal state, preparing the Encoder to be used with
	// a new stream.
	Reset()
}

// A Writer uses MatchFinder and Encoder to write compressed data to Dest.
type Writer struct {
	Dest        io.Writer
	MatchFinder MatchFinder
	Encoder     Encoder

	// BlockSize is the number of bytes to compress at a time. If it is zero,
	// each Write operation will be treated as one block.
	BlockSize int
}

```

## Example

Here is an example program that finds repititions in the Go Proverbs,
and prints them with human-readable backreferences.

```go
package main

import (
	"fmt"

	"github.com/andybalholm/pack"
	"github.com/andybalholm/pack/flate"
)

const proverbs = `Don't communicate by sharing memory, share memory by communicating.
Concurrency is not parallelism.
Channels orchestrate; mutexes serialize.
The bigger the interface, the weaker the abstraction.
Make the zero value useful.
interface{} says nothing.
Gofmt's style is no one's favorite, yet gofmt is everyone's favorite.
A little copying is better than a little dependency.
Syscall must always be guarded with build tags.
Cgo must always be guarded with build tags.
Cgo is not Go.
With the unsafe package there are no guarantees.
Clear is better than clever.
Reflection is never clear.
Errors are values.
Don't just check errors, handle them gracefully.
Design the architecture, name the components, document the details.
Documentation is for users.
Don't panic.`

func main() {
	var mf flate.DualHash
	var e pack.TextEncoder

	b := []byte(proverbs)
	matches := mf.FindMatches(nil, b)
	out := e.Encode(nil, b, matches, true)

	fmt.Printf("%s\n", out)
}
```

## Implementations

The `brotli`, `flate`, and `snappy` directories contain implementations of
`Encoder` and `MatchFinder` based on `github.com/andybalholm/brotli`,
`compress/flate`, and `github.com/golang/snappy`.

The flate implementation, with `BestSpeed`, is faster than `compress/flate`.
This demonstrates that although there is some overhead due to using the Pack interfaces, 
it isn’t so much that you can’t get reasonable compression performance.
