package brotli

var kInsBase = []uint32{
	0,
	1,
	2,
	3,
	4,
	5,
	6,
	8,
	10,
	14,
	18,
	26,
	34,
	50,
	66,
	98,
	130,
	194,
	322,
	578,
	1090,
	2114,
	6210,
	22594,
}

var kInsExtra = []uint32{
	0,
	0,
	0,
	0,
	0,
	0,
	1,
	1,
	2,
	2,
	3,
	3,
	4,
	4,
	5,
	5,
	6,
	7,
	8,
	9,
	10,
	12,
	14,
	24,
}

var kCopyBase = []uint32{
	2,
	3,
	4,
	5,
	6,
	7,
	8,
	9,
	10,
	12,
	14,
	18,
	22,
	30,
	38,
	54,
	70,
	102,
	134,
	198,
	326,
	582,
	1094,
	2118,
}

var kCopyExtra = []uint32{
	0,
	0,
	0,
	0,
	0,
	0,
	0,
	0,
	1,
	1,
	2,
	2,
	3,
	3,
	4,
	4,
	5,
	5,
	6,
	7,
	8,
	9,
	10,
	24,
}

func getInsertLengthCode(insertlen uint) uint16 {
	if insertlen < 6 {
		return uint16(insertlen)
	} else if insertlen < 130 {
		var nbits uint32 = log2FloorNonZero(insertlen-2) - 1
		return uint16((nbits << 1) + uint32((insertlen-2)>>nbits) + 2)
	} else if insertlen < 2114 {
		return uint16(log2FloorNonZero(insertlen-66) + 10)
	} else if insertlen < 6210 {
		return 21
	} else if insertlen < 22594 {
		return 22
	} else {
		return 23
	}
}

func getCopyLengthCode(copylen uint) uint16 {
	if copylen < 10 {
		return uint16(copylen - 2)
	} else if copylen < 134 {
		var nbits uint32 = log2FloorNonZero(copylen-6) - 1
		return uint16((nbits << 1) + uint32((copylen-6)>>nbits) + 4)
	} else if copylen < 2118 {
		return uint16(log2FloorNonZero(copylen-70) + 12)
	} else {
		return 23
	}
}

func combineLengthCodes(inscode uint16, copycode uint16, use_last_distance bool) uint16 {
	var bits64 uint16 = uint16(copycode&0x7 | (inscode&0x7)<<3)
	if use_last_distance && inscode < 8 && copycode < 16 {
		if copycode < 8 {
			return bits64
		} else {
			return bits64 | 64
		}
	} else {
		/* Specification: 5 Encoding of ... (last table) */
		/* offset = 2 * index, where index is in range [0..8] */
		var offset uint32 = 2 * ((uint32(copycode) >> 3) + 3*(uint32(inscode)>>3))

		/* All values in specification are K * 64,
		   where   K = [2, 3, 6, 4, 5, 8, 7, 9, 10],
		       i + 1 = [1, 2, 3, 4, 5, 6, 7, 8,  9],
		   K - i - 1 = [1, 1, 3, 0, 0, 2, 0, 1,  2] = D.
		   All values in D require only 2 bits to encode.
		   Magic constant is shifted 6 bits left, to avoid final multiplication. */
		offset = (offset << 5) + 0x40 + ((0x520D40 >> offset) & 0xC0)

		return uint16(offset | uint32(bits64))
	}
}
