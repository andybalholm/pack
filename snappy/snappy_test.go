package snappy

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/andybalholm/pack"
	"github.com/andybalholm/pack/flate"
	"github.com/golang/snappy"
)

func test(t *testing.T, filename string, m pack.MatchFinder) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	b := new(bytes.Buffer)
	w := &pack.Writer{
		Dest:        b,
		MatchFinder: m,
		Encoder:     &Encoder{},
		BlockSize:   65536,
	}
	w.Write(data)
	w.Close()
	compressed := b.Bytes()
	sr := snappy.NewReader(bytes.NewReader(compressed))
	decompressed, err := ioutil.ReadAll(sr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decompressed, data) {
		t.Fatal("decompressed output doesn't match")
	}
}

func TestEncode(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", MatchFinder{})
}

func TestEncodeDualHash(t *testing.T) {
	test(t, "../testdata/Isaac.Newton-Opticks.txt", pack.AutoReset{&flate.DualHash{}})
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
		Encoder:     &Encoder{},
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

func BenchmarkEncode(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", MatchFinder{})
}

func BenchmarkEncodeDualHash(b *testing.B) {
	benchmark(b, "../testdata/Isaac.Newton-Opticks.txt", pack.AutoReset{&flate.DualHash{}})
}

func BenchmarkEncodeGolangSnappy(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	opticks, err := ioutil.ReadFile("../testdata/Isaac.Newton-Opticks.txt")
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(opticks)))
	buf := new(bytes.Buffer)
	w := snappy.NewBufferedWriter(buf)
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
