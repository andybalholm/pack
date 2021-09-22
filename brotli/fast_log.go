package brotli

import "math/bits"

/* Copyright 2013 Google Inc. All Rights Reserved.

   Distributed under MIT license.
   See file LICENSE for detail or copy at https://opensource.org/licenses/MIT
*/

/* Utilities for fast computation of logarithms. */

func log2FloorNonZero(n uint) uint32 {
	return uint32(bits.Len(n)) - 1
}
