package brotli

/* Copyright 2013 Google Inc. All Rights Reserved.

   Distributed under MIT license.
   See file LICENSE for detail or copy at https://opensource.org/licenses/MIT
*/

/* Utilities for fast computation of logarithms. */

func log2FloorNonZero(n uint) uint32 {
	/* TODO: generalize and move to platform.h */
	var result uint32 = 0
	for {
		n >>= 1
		if n == 0 {
			break
		}
		result++
	}
	return result
}
