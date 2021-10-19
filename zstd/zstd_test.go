package zstd

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/andybalholm/pack"
	"github.com/andybalholm/pack/brotli"
	"github.com/klauspost/compress/zstd"
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
	sr, err := zstd.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	decompressed, err := ioutil.ReadAll(sr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decompressed, data) {
		t.Fatal("decompressed output doesn't match")
	}
}

func TestEncodeM0(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", brotli.M0{}, 1<<16)
}

func TestEncodeH4(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &brotli.MatchFinder{Hasher: &brotli.H4{}, MaxHistory: 1 << 18, MinHistory: 1 << 16}, 1<<16)
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

func BenchmarkEncodeM0(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", brotli.M0{}, 1<<16)
}

func BenchmarkEncodeH4(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &brotli.MatchFinder{Hasher: &brotli.H4{}}, 1<<20)
}
