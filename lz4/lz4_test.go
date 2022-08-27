package lz4

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/andybalholm/pack/flate"
	"github.com/pierrec/lz4/v4"
)

func TestBlockEncode(t *testing.T) {
	data, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}

	var mf flate.BestSpeed
	matches := mf.FindMatches(nil, data)
	var be BlockEncoder
	compressed := be.Encode(nil, data, matches, true)

	decompressed := make([]byte, len(data))
	n, err := lz4.UncompressBlock(compressed, decompressed)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(data) {
		t.Fatalf("Got %d bytes, wanted %d", n, len(data))
	}

	if !bytes.Equal(decompressed, data) {
		t.Fatal("Decompressed output does not match")
	}
}

func TestFrameEncode(t *testing.T) {
	data, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}

	var mf flate.BestSpeed
	matches := mf.FindMatches(nil, data)
	var fe FrameEncoder
	compressed := fe.Encode(nil, data, matches, true)

	decompressed, err := io.ReadAll(lz4.NewReader(bytes.NewReader(compressed)))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(decompressed, data) {
		t.Fatal("Decompressed output does not match")
	}
}
