package flate

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/andybalholm/pack"
	"github.com/andybalholm/pack/brotli"
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

func TestEncodeSSAP(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", &pack.SimpleSearchAdvancedParsing{MaxDistance: 32768, HashLen: 5}, 1<<20)
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

func TestWriterLevels(t *testing.T) {
	data, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i < 10; i++ {
		b := new(bytes.Buffer)
		w := NewWriter(b, i)
		w.Write(data)
		w.Close()
		compressed := b.Bytes()
		sr := flate.NewReader(bytes.NewReader(compressed))
		decompressed, err := ioutil.ReadAll(sr)
		if err != nil {
			t.Fatalf("error decompressing level %d: %v", i, err)
		}
		if !bytes.Equal(decompressed, data) {
			t.Fatalf("decompressed output doesn't match on level %d", i)
		}
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

func BenchmarkEncodeM0(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", brotli.M0{MaxDistance: 32768, MaxLength: 258}, 1<<16)
}

func BenchmarkEncodeSSAP(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", &pack.SimpleSearchAdvancedParsing{MaxDistance: 32768, HashLen: 5}, 1<<16)
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

func BenchmarkWriterLevels(b *testing.B) {
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	for level := 1; level <= 9; level++ {
		buf := new(bytes.Buffer)
		w := NewWriter(buf, level)
		w.Write(opticks)
		w.Close()
		b.Run(fmt.Sprintf("%d", level), func(b *testing.B) {
			b.ReportAllocs()
			b.ReportMetric(float64(len(opticks))/float64(buf.Len()), "ratio")
			b.SetBytes(int64(len(opticks)))
			for i := 0; i < b.N; i++ {
				w.Reset(ioutil.Discard)
				w.Write(opticks)
				w.Close()
			}
		})
	}
}

func BenchmarkStdlibLevels(b *testing.B) {
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	for level := 1; level <= 9; level++ {
		buf := new(bytes.Buffer)
		w, _ := flate.NewWriter(buf, level)
		w.Write(opticks)
		w.Close()
		b.Run(fmt.Sprintf("%d", level), func(b *testing.B) {
			b.ReportAllocs()
			b.ReportMetric(float64(len(opticks))/float64(buf.Len()), "ratio")
			b.SetBytes(int64(len(opticks)))
			for i := 0; i < b.N; i++ {
				w.Reset(ioutil.Discard)
				w.Write(opticks)
				w.Close()
			}
		})
	}
}

func BenchmarkKlausPostLevels(b *testing.B) {
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	for level := 1; level <= 9; level++ {
		buf := new(bytes.Buffer)
		w, _ := kflate.NewWriter(buf, level)
		w.Write(opticks)
		w.Close()
		b.Run(fmt.Sprintf("%d", level), func(b *testing.B) {
			b.ReportAllocs()
			b.ReportMetric(float64(len(opticks))/float64(buf.Len()), "ratio")
			b.SetBytes(int64(len(opticks)))
			for i := 0; i < b.N; i++ {
				w.Reset(ioutil.Discard)
				w.Write(opticks)
				w.Close()
			}
		})
	}
}
