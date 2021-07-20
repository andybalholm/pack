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

	commands := make([]uint32, 4*len(matches))
	commandsFree := commands
	var literals []byte

	pos := 0
	lastDistance := -1
	for _, m := range matches {
		if m.Unmatched > 0 {
			literals = append(literals, src[pos:pos+m.Unmatched]...)
			emitInsertLen(uint32(m.Unmatched), &commandsFree)
			if m.Length == 0 {
				break
			}
			if m.Distance == lastDistance {
				commandsFree[0] = 64
				commandsFree = commandsFree[1:]
			} else {
				emitDistance(uint32(m.Distance), &commandsFree)
				lastDistance = m.Distance
			}
			emitCopyLenLastDistance(uint(m.Length), &commandsFree)
		} else {
			emitCopyLen(uint(m.Length), &commandsFree)
			emitDistance(uint32(m.Distance), &commandsFree)
			lastDistance = m.Distance
		}
		pos += m.Unmatched + m.Length
	}
	commands = commands[:len(commands)-len(commandsFree)]

	storeMetaBlockHeader(uint(len(src)), false, &e.bw)
	e.bw.writeBits(13, 0)
	storeCommands(literals, uint(len(literals)), commands, uint(len(commands)), &e.bw)

	if lastBlock {
		e.bw.writeBits(2, 3) // islast + isempty
		e.bw.jumpToByteBoundary()
	}
	return e.bw.dst
}
