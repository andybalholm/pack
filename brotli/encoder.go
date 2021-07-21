package brotli

import "github.com/andybalholm/pack"

// An Encoder implements the pack.Encoder interface, writing in Brotli format.
type Encoder struct {
	wroteHeader bool
	bw          bitWriter
}

func (e *Encoder) Reset() {
	e.wroteHeader = false
	e.bw = bitWriter{}
}

func (e *Encoder) Encode(dst []byte, src []byte, matches []pack.Match, lastBlock bool) []byte {
	e.bw.dst = dst
	if !e.wroteHeader {
		e.bw.writeBits(4, 15)
		e.wroteHeader = true
	}

	var literalHisto [256]uint32
	var commandHisto [704]uint32
	var distanceHisto [64]uint32
	literalCount := 0
	commandCount := 0
	distanceCount := 0

	// first pass: build the histograms
	pos := 0
	lastDistance := -1
	for _, m := range matches {
		if m.Unmatched > 0 {
			for _, c := range src[pos : pos+m.Unmatched] {
				literalHisto[c]++
			}
			literalCount += m.Unmatched
		}

		insertCode := getInsertLengthCode(uint(m.Unmatched))
		copyCode := getCopyLengthCode(uint(m.Length))
		if m.Length == 0 {
			// If the stream ends with unmatched bytes, we need a dummy copy length.
			copyCode = 2
		}
		command := combineLengthCodes(insertCode, copyCode, false && lastDistance == m.Distance)
		commandHisto[command]++
		commandCount++

		if command >= 128 && m.Length != 0 {
			distCode, _, _ := getDistanceCode(m.Distance)
			distanceHisto[distCode]++
			distanceCount++
		}

		lastDistance = m.Distance
		pos += m.Unmatched + m.Length
	}

	storeMetaBlockHeader(uint(len(src)), false, &e.bw)
	e.bw.writeBits(13, 0)

	var literalDepths [256]byte
	var literalBits [256]uint16
	buildAndStoreHuffmanTreeFast(literalHisto[:], uint(literalCount), 8, literalDepths[:], literalBits[:], &e.bw)

	var commandDepths [704]byte
	var commandBits [704]uint16
	buildAndStoreHuffmanTreeFast(commandHisto[:], uint(commandCount), 10, commandDepths[:], commandBits[:], &e.bw)

	var distanceDepths [64]byte
	var distanceBits [64]uint16
	buildAndStoreHuffmanTreeFast(distanceHisto[:], uint(distanceCount), 6, distanceDepths[:], distanceBits[:], &e.bw)

	pos = 0
	lastDistance = -1
	for _, m := range matches {
		insertCode := getInsertLengthCode(uint(m.Unmatched))
		copyCode := getCopyLengthCode(uint(m.Length))
		if m.Length == 0 {
			// If the stream ends with unmatched bytes, we need a dummy copy length.
			copyCode = 2
		}
		command := combineLengthCodes(insertCode, copyCode, false && lastDistance == m.Distance)
		e.bw.writeBits(uint(commandDepths[command]), uint64(commandBits[command]))
		if kInsExtra[insertCode] > 0 {
			e.bw.writeBits(uint(kInsExtra[insertCode]), uint64(m.Unmatched)-uint64(kInsBase[insertCode]))
		}
		if kCopyExtra[copyCode] > 0 {
			e.bw.writeBits(uint(kCopyExtra[copyCode]), uint64(m.Length)-uint64(kCopyBase[copyCode]))
		}

		if m.Unmatched > 0 {
			for _, c := range src[pos : pos+m.Unmatched] {
				e.bw.writeBits(uint(literalDepths[c]), uint64(literalBits[c]))
			}
		}

		if command >= 128 && m.Length != 0 {
			distCode, nExtra, extraBits := getDistanceCode(m.Distance)
			e.bw.writeBits(uint(distanceDepths[distCode]), uint64(distanceBits[distCode]))
			if nExtra > 0 {
				e.bw.writeBits(nExtra, extraBits)
			}
		}

		lastDistance = m.Distance
		pos += m.Unmatched + m.Length
	}

	if lastBlock {
		e.bw.writeBits(2, 3) // islast + isempty
		e.bw.jumpToByteBoundary()
	}
	return e.bw.dst
}

func getDistanceCode(distance int) (code int, nExtra uint, extraBits uint64) {
	d := distance + 3
	nbits := log2FloorNonZero(uint(d)) - 1
	prefix := (d >> nbits) & 1
	offset := (2 + prefix) << nbits
	distcode := int(2*(nbits-1)) + prefix + 16
	extra := d - offset
	return distcode, uint(nbits), uint64(extra)
}
