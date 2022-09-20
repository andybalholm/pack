package lz4

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/andybalholm/pack"
	"github.com/andybalholm/pack/flate"
	"github.com/pierrec/lz4/v4"
)

func TestBlockEncode(t *testing.T) {
	data, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}

	var mf BestSpeed
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

	var mf BestSpeed
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

func TestWriter(t *testing.T) {
	data, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}

	var mf BestSpeed
	var fe FrameEncoder

	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest:        b,
		MatchFinder: &mf,
		Encoder:     &fe,
		BlockSize:   65536,
	}
	w.Write(data)
	w.Close()
	compressed := b.Bytes()

	decompressed, err := io.ReadAll(lz4.NewReader(bytes.NewReader(compressed)))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(decompressed, data) {
		t.Fatal("Decompressed output does not match")
	}
}

func TestWriterReset(t *testing.T) {
	data, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}

	var mf BestSpeed
	var fe FrameEncoder

	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest:        b,
		MatchFinder: &mf,
		Encoder:     &fe,
		BlockSize:   65536,
	}
	w.Write(data)
	w.Close()

	b = new(bytes.Buffer)
	w.Reset(b)
	w.Write(data)
	w.Close()
	compressed := b.Bytes()

	decompressed, err := io.ReadAll(lz4.NewReader(bytes.NewReader(compressed)))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(decompressed, data) {
		t.Fatal("Decompressed output does not match")
	}
}

func test(t *testing.T, filename string, m pack.MatchFinder) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest:        b,
		MatchFinder: m,
		Encoder:     &FrameEncoder{},
		BlockSize:   65536,
	}
	w.Write(data)
	w.Close()
	compressed := b.Bytes()
	sr := lz4.NewReader(bytes.NewReader(compressed))
	decompressed, err := ioutil.ReadAll(sr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decompressed, data) {
		t.Fatal("decompressed output doesn't match")
	}
}

func TestBestSpeed(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &BestSpeed{})
}

func TestHashChain(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &HashChain{SearchLen: 100})
}

func TestGreedyParser(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 100, Parser: &pack.GreedyParser{}})
}

func TestLazyParser(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 100, Parser: &pack.LazyParser{}})
}

func TestOverlapParser(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 100, Parser: &pack.OverlapParser{}})
}

func benchmark(b *testing.B, filename string, m pack.MatchFinder) {
	b.StopTimer()
	b.ReportAllocs()
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(data)))
	buf := new(bytes.Buffer)
	w := &pack.Writer{
		Dest:        buf,
		MatchFinder: m,
		Encoder:     &FrameEncoder{},
		BlockSize:   65536,
	}
	w.Write(data)
	w.Close()
	b.ReportMetric(float64(len(data))/float64(buf.Len()), "ratio")
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		w.Reset(ioutil.Discard)
		w.Write(data)
		w.Close()
	}
}

func BenchmarkEncodeFlateBestSpeed(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &flate.BestSpeed{})
}

func BenchmarkEncodeBestSpeed(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &BestSpeed{})
}

func BenchmarkEncodeHashChain1(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &HashChain{SearchLen: 1})
}

func BenchmarkEncodeHashChain2(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &HashChain{SearchLen: 2})
}

func BenchmarkEncodeHashChain10(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &HashChain{SearchLen: 10})
}

func BenchmarkEncodeHashChain100(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &HashChain{SearchLen: 100})
}

func BenchmarkEncodeHashChain1000(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &HashChain{SearchLen: 1000})
}

func BenchmarkEncodeGreedyParser1(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 1, Parser: &pack.GreedyParser{}})
}

func BenchmarkEncodeGreedyParser10(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 10, Parser: &pack.GreedyParser{}})
}

func BenchmarkEncodeGreedyParser100(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 100, Parser: &pack.GreedyParser{}})
}

func BenchmarkEncodeGreedyParser1000(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 1000, Parser: &pack.GreedyParser{}})
}

func BenchmarkEncodeLazyParser1(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 1, Parser: &pack.LazyParser{}})
}

func BenchmarkEncodeLazyParser10(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 10, Parser: &pack.LazyParser{}})
}

func BenchmarkEncodeLazyParser100(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 100, Parser: &pack.LazyParser{}})
}

func BenchmarkEncodeLazyParser1000(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 1000, Parser: &pack.LazyParser{}})
}

func BenchmarkEncodeOverlapParser1(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 1, Parser: &pack.OverlapParser{}})
}

func BenchmarkEncodeOverlapParser10(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 10, Parser: &pack.OverlapParser{}})
}

func BenchmarkEncodeOverlapParser100(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 100, Parser: &pack.OverlapParser{}})
}

func BenchmarkEncodeOverlapParser1000(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.HashChain{SearchLen: 1000, Parser: &pack.OverlapParser{}})
}
