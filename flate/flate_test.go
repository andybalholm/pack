package flate

import (
	"bytes"
	"compress/flate"
	"io/ioutil"
	"testing"

	"github.com/andybalholm/pack"
	"github.com/andybalholm/pack/snappy"
)

func TestEncode(t *testing.T) {
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}
	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest:        b,
		MatchFinder: snappy.MatchFinder{},
		Encoder:     NewEncoder(),
		BlockSize:   32768,
	}
	w.Write(opticks)
	w.Close()
	compressed := b.Bytes()
	sr := flate.NewReader(bytes.NewReader(compressed))
	decompressed, err := ioutil.ReadAll(sr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decompressed, opticks) {
		t.Fatal("decompressed output doesn't match")
	}
}
