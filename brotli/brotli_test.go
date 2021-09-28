package brotli

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/andybalholm/pack"
	"github.com/andybalholm/pack/snappy"
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

func TestEncodeM0(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", M0{}, 1<<16)
}

func TestEncodeM0Lazy(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", M0{Lazy: true}, 1<<16)
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

func TestWriterLevels(t *testing.T) {
	data, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		b := new(bytes.Buffer)
		w := NewWriter(b, i)
		w.Write(data)
		w.Close()
		compressed := b.Bytes()
		sr := brotli.NewReader(bytes.NewReader(compressed))
		decompressed, err := ioutil.ReadAll(sr)
		if err != nil {
			t.Fatalf("error decompressing level %d: %v", i, err)
		}
		if !bytes.Equal(decompressed, data) {
			t.Fatalf("decompressed output doesn't match on level %d", i)
		}
	}
}

func TestComposite(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt",
		&MatchFinder{
			Hasher: &CompositeHasher{
				A: &H5{BlockBits: 1, BucketBits: 15},
				B: &H6{BlockBits: 2, BucketBits: 15, HashLen: 8},
			},
			MaxHistory: 1 << 18,
			MinHistory: 1 << 16,
		},
		1<<16)
}

func TestReset(t *testing.T) {
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}
	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest: ioutil.Discard,
		MatchFinder: &MatchFinder{
			Hasher:     &H2{},
			MaxHistory: 1 << 17,
			MinHistory: 1 << 16,
		},
		Encoder:   &Encoder{},
		BlockSize: 1 << 16,
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
		MatchFinder: M0{},
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

func BenchmarkEncodeSnappy(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", snappy.MatchFinder{}, 1<<16)
}

func BenchmarkEncodeM0(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", M0{}, 1<<16)
}

func BenchmarkEncodeM0Lazy(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", M0{Lazy: true}, 1<<16)
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

func BenchmarkEncodeComposite_5(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt",
		&MatchFinder{
			Hasher: &CompositeHasher{
				A: &H4{},
				B: &H6{BlockBits: 2, BucketBits: 15, HashLen: 8},
			},
			MaxHistory: 1 << 18,
			MinHistory: 1 << 16,
		},
		1<<16)
}

func BenchmarkEncodeComposite_6(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt",
		&MatchFinder{
			Hasher: &CompositeHasher{
				A: &H4{},
				B: &H6{BlockBits: 3, BucketBits: 15, HashLen: 8},
			},
			MaxHistory: 1 << 18,
			MinHistory: 1 << 16,
		},
		1<<16)
}

func BenchmarkEncodeComposite_7(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt",
		&MatchFinder{
			Hasher: &CompositeHasher{
				A: &H5{BlockBits: 3, BucketBits: 15},
				B: &H6{BlockBits: 4, BucketBits: 15, HashLen: 8},
			},
			MaxHistory: 1 << 18,
			MinHistory: 1 << 16,
		},
		1<<16)
}

func BenchmarkEncodeComposite_8(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt",
		&MatchFinder{
			Hasher: &CompositeHasher{
				A: &H5{BlockBits: 3, BucketBits: 15},
				B: &H6{BlockBits: 5, BucketBits: 15, HashLen: 8},
			},
			MaxHistory: 1 << 18,
			MinHistory: 1 << 16,
		},
		1<<16)
}

func BenchmarkEncodeComposite_9(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt",
		&MatchFinder{
			Hasher: &CompositeHasher{
				A: &H5{BlockBits: 4, BucketBits: 15},
				B: &H6{BlockBits: 6, BucketBits: 15, HashLen: 8},
			},
			MaxHistory: 1 << 18,
			MinHistory: 1 << 16,
		},
		1<<16)
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

func BenchmarkWriterLevels(b *testing.B) {
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	for level := 0; level <= 9; level++ {
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
