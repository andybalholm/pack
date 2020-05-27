package flate

import (
	"bytes"
	"compress/flate"
	"io/ioutil"
	"testing"

	"github.com/andybalholm/pack"
	kflate "github.com/klauspost/compress/flate"
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
		Encoder:   NewEncoder(),
		BlockSize: 1 << 16,
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

func TestMaxLength(t *testing.T) {
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		t.Fatal(err)
	}
	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest: b,
		MatchFinder: &pack.QuickMatchFinder{
			MaxDistance: 32768,
			MaxLength:   8,
		},
		Encoder:   NewEncoder(),
		BlockSize: 32768,
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
		Encoder:   NewEncoder(),
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
