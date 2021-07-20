package brotli

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/andybalholm/pack"
)

func TestEncode(t *testing.T) {
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}
	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest: b,
		MatchFinder: &pack.QuickMatchFinder{
			MaxDistance: 32768,
			MaxLength:   258,
			ChainBlocks: true,
		},
		Encoder:   &Encoder{},
		BlockSize: 1 << 16,
	}
	w.Write(opticks)
	w.Close()
	compressed := b.Bytes()
	sr := brotli.NewReader(bytes.NewReader(compressed))
	decompressed, err := ioutil.ReadAll(sr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decompressed, opticks) {
		t.Fatal("decompressed output doesn't match")
	}
}

func TestEncodeHelloHello(t *testing.T) {
	hello := []byte("HelloHelloHelloHelloHelloHelloHelloHelloHelloHello, world")
	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest: b,
		MatchFinder: &pack.QuickMatchFinder{
			MaxDistance: 32768,
			MaxLength:   258,
			ChainBlocks: true,
		},
		Encoder:   &Encoder{},
		BlockSize: 1 << 16,
	}
	w.Write(hello)
	w.Close()
	compressed := b.Bytes()
	sr := brotli.NewReader(bytes.NewReader(compressed))
	decompressed, err := ioutil.ReadAll(sr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decompressed, hello) {
		t.Fatalf("decompressed output doesn't match: got %q, want %q", decompressed, hello)
	}
}

func BenchmarkEncode(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(opticks)))
	buf := new(bytes.Buffer)
	w := &pack.Writer{
		Dest: buf,
		MatchFinder: &pack.QuickMatchFinder{
			MaxDistance: 32768,
			MaxLength:   258,
			ChainBlocks: true,
		},
		Encoder:   &Encoder{},
		BlockSize: 1 << 20,
	}
	w.Write(opticks)
	w.Close()
	b.ReportMetric(float64(len(opticks))/float64(buf.Len()), "ratio")
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		w.Reset(ioutil.Discard)
		w.Write(opticks)
		w.Close()
	}
}
