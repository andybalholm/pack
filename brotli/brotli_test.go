package brotli

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/andybalholm/pack"
	"github.com/andybalholm/pack/flate"
)

func test(t *testing.T, filename string, m pack.MatchFinder, blockSize int) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest:        b,
		MatchFinder: m,
		Encoder:     &Encoder{},
		BlockSize:   blockSize,
	}
	w.Write(data)
	w.Close()
	compressed := b.Bytes()
	sr := brotli.NewReader(bytes.NewReader(compressed))
	decompressed, err := ioutil.ReadAll(sr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decompressed, data) {
		t.Fatal("decompressed output doesn't match")
	}
}

func TestEncode(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &flate.BestSpeed{}, 1<<16)
}

func TestEncodeH2(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H2{}}, 1<<16)
}

func TestEncodeH3(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H3{}}, 1<<16)
}

func TestEncodeH4(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H4{}}, 1<<16)
}

func TestEncodeH5(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H5{BlockBits: 4, BucketBits: 14}, MaxHistory: 1 << 18, MinHistory: 1 << 16}, 1<<16)
}

func TestEncodeH6(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H6{BlockBits: 4, BucketBits: 14, HashLen: 5}, MaxHistory: 1 << 18, MinHistory: 1 << 16}, 1<<16)
}

func TestReset(t *testing.T) {
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}
	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest:        ioutil.Discard,
		MatchFinder: &flate.BestSpeed{},
		Encoder:     &Encoder{},
		BlockSize:   1 << 16,
	}
	w.Write(opticks)
	w.Close()
	w.Reset(b)
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
		Dest:        b,
		MatchFinder: &flate.BestSpeed{},
		Encoder:     &Encoder{},
		BlockSize:   1 << 16,
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

func benchmark(b *testing.B, filename string, m pack.MatchFinder, blockSize int) {
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
		Encoder:     &Encoder{},
		BlockSize:   blockSize,
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

func BenchmarkEncode(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &flate.BestSpeed{}, 1<<20)
}

func BenchmarkEncodeDualHashLazy(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &flate.DualHash{Lazy: true}, 1<<20)
}

func BenchmarkEncodeH2(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H2{}}, 1<<20)
}

func BenchmarkEncodeH3(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H3{}}, 1<<20)
}

func BenchmarkEncodeH4(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H4{}}, 1<<20)
}

func BenchmarkEncodeH5(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H5{BlockBits: 4, BucketBits: 14}, MaxHistory: 1 << 18, MinHistory: 1 << 16}, 1<<16)
}

func BenchmarkEncodeH6_5(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H6{BlockBits: 4, BucketBits: 14, HashLen: 5}, MaxHistory: 1 << 18, MinHistory: 1 << 16}, 1<<16)
}

func BenchmarkEncodeH6_6(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H6{BlockBits: 5, BucketBits: 14, HashLen: 5}, MaxHistory: 1 << 18, MinHistory: 1 << 16}, 1<<16)
}

func BenchmarkEncodeH6_7(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H6{BlockBits: 6, BucketBits: 15, HashLen: 5}, MaxHistory: 1 << 18, MinHistory: 1 << 16}, 1<<16)
}

func BenchmarkEncodeH6_8(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H6{BlockBits: 7, BucketBits: 15, HashLen: 5}, MaxHistory: 1 << 18, MinHistory: 1 << 16}, 1<<16)
}

func BenchmarkEncodeH6_9(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &MatchFinder{Hasher: &H6{BlockBits: 8, BucketBits: 15, HashLen: 5}, MaxHistory: 1 << 18, MinHistory: 1 << 16}, 1<<16)
}
