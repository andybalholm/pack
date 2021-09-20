package flate

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io/ioutil"
	"testing"

	"github.com/andybalholm/pack"
	kflate "github.com/klauspost/compress/flate"
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
		Encoder:     NewEncoder(),
		BlockSize:   blockSize,
	}
	w.Write(data)
	w.Close()
	compressed := b.Bytes()
	sr := flate.NewReader(bytes.NewReader(compressed))
	decompressed, err := ioutil.ReadAll(sr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decompressed, data) {
		t.Fatal("decompressed output doesn't match")
	}
}

func TestEncode(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &BestSpeed{}, 1<<16)
}

func TestEncodeDualHash(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &DualHash{}, 1<<16)
}

func TestEncodeDualHashLazy(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &DualHash{Lazy: true}, 1<<20)
}

func TestEncodeHuffmanOnly(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", pack.NoMatchFinder{}, 1<<16)
}

func TestGZIP(t *testing.T) {
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}
	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest:        b,
		MatchFinder: &BestSpeed{},
		Encoder:     NewGZIPEncoder(),
		BlockSize:   1 << 16,
	}
	w.Write(opticks)
	w.Close()
	compressed := b.Bytes()
	sr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	decompressed, err := ioutil.ReadAll(sr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decompressed, opticks) {
		t.Fatal("decompressed output doesn't match")
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
		Encoder:     NewEncoder(),
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
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &BestSpeed{}, 1<<20)
}

func BenchmarkEncodeDualHash(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &DualHash{}, 1<<20)
}

func BenchmarkEncodeDualHashLazy(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &DualHash{Lazy: true}, 1<<20)
}

func BenchmarkEncodeHuffmanOnly(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", pack.NoMatchFinder{}, 1<<20)
}

func BenchmarkGZIP(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(opticks)))
	buf := new(bytes.Buffer)
	w := &pack.Writer{
		Dest:        buf,
		MatchFinder: &BestSpeed{},
		Encoder:     NewGZIPEncoder(),
		BlockSize:   1 << 20,
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

func BenchmarkEncodeStdlib(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(opticks)))
	buf := new(bytes.Buffer)
	w, _ := flate.NewWriter(buf, flate.BestSpeed)
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

func BenchmarkEncodeKlausPost(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(opticks)))
	buf := new(bytes.Buffer)
	w, _ := kflate.NewWriter(buf, kflate.BestSpeed)
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
