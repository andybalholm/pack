package brotli

/* Copyright 2015 Google Inc. All Rights Reserved.

   Distributed under MIT license.
   See file LICENSE for detail or copy at https://opensource.org/licenses/MIT
*/

/* Function for fast encoding of an input fragment, independently from the input
   history. This function uses two-pass processing: in the first pass we save
   the found backward matches and literal bytes into a buffer, and in the
   second pass we emit them into the bit stream using prefix codes built based
   on the actual command and literal byte histograms. */

/* REQUIRES: len <= 1 << 24. */
func storeMetaBlockHeader(len uint, is_uncompressed bool, bw *bitWriter) {
	var nibbles uint = 6

	/* ISLAST */
	bw.writeBits(1, 0)

	if len <= 1<<16 {
		nibbles = 4
	} else if len <= 1<<20 {
		nibbles = 5
	}

	bw.writeBits(2, uint64(nibbles)-4)
	bw.writeBits(nibbles*4, uint64(len)-1)

	/* ISUNCOMPRESSED */
	bw.writeSingleBit(is_uncompressed)
}
