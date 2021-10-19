package zstd

import (
	"bytes"
	"log"
)

// enable debug printing
const debug = false

// enable encoding debug printing
const debugEncoder = debug

// enable decoding debug printing
const debugDecoder = debug

// Enable extra assertions.
const debugAsserts = debug || false

// print sequence details
const debugSequences = false

// force encoder to use predefined tables.
const forcePreDef = false

// zstdMinMatch is the minimum zstd match length.
const zstdMinMatch = 3

func println(a ...interface{}) {
	if debug || debugDecoder || debugEncoder {
		log.Println(a...)
	}
}

func printf(format string, a ...interface{}) {
	if debug || debugDecoder || debugEncoder {
		log.Printf(format, a...)
	}
}

type byter interface {
	Bytes() []byte
	Len() int
}

var _ byter = &bytes.Buffer{}
