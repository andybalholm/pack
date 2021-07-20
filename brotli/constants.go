package brotli

/* Copyright 2016 Google Inc. All Rights Reserved.

   Distributed under MIT license.
   See file LICENSE for detail or copy at https://opensource.org/licenses/MIT
*/

/* Specification: 7.3. Encoding of the context map */
const contextMapMaxRle = 16

/* Specification: 2. Compressed representation overview */
const maxNumberOfBlockTypes = 256

/* Specification: 3.3. Alphabet sizes: insert-and-copy length */
const numLiteralSymbols = 256

const numCommandSymbols = 704

const numBlockLenSymbols = 26

const maxContextMapSymbols = (maxNumberOfBlockTypes + contextMapMaxRle)

const maxBlockTypeSymbols = (maxNumberOfBlockTypes + 2)

/* Specification: 3.5. Complex prefix codes */
const repeatPreviousCodeLength = 16

const repeatZeroCodeLength = 17

const codeLengthCodes = (repeatZeroCodeLength + 1)

/* "code length of 8 is repeated" */
const initialRepeatedCodeLength = 8

/* "Large Window Brotli" */
const largeMaxDistanceBits = 62

const largeMinWbits = 10

const largeMaxWbits = 30

/* Specification: 4. Encoding of distances */
const numDistanceShortCodes = 16

const maxNpostfix = 3

const maxNdirect = 120

const maxDistanceBits = 24

func distanceAlphabetSize(NPOSTFIX uint, NDIRECT uint, MAXNBITS uint) uint {
	return numDistanceShortCodes + NDIRECT + uint(MAXNBITS<<(NPOSTFIX+1))
}

/* numDistanceSymbols == 1128 */
const numDistanceSymbols = 1128

const maxDistance = 0x3FFFFFC

const maxAllowedDistance = 0x7FFFFFFC

/* 7.1. Context modes and context ID lookup for literals */
/* "context IDs for literals are in the range of 0..63" */
const literalContextBits = 6

/* 7.2. Context ID for distances */
const distanceContextBits = 2

/* 9.1. Format of the Stream Header */
/* Number of slack bytes for window size. Don't confuse
   with BROTLI_NUM_DISTANCE_SHORT_CODES. */
const windowGap = 16

func maxBackwardLimit(W uint) uint {
	return (uint(1) << W) - windowGap
}

/** Minimal value for ::BROTLI_PARAM_LGWIN parameter. */
const minWindowBits = 10

/**
 * Maximal value for ::BROTLI_PARAM_LGWIN parameter.
 *
 * @note equal to @c BROTLI_MAX_DISTANCE_BITS constant.
 */
const maxWindowBits = 24

/**
 * Maximal value for ::BROTLI_PARAM_LGWIN parameter
 * in "Large Window Brotli" (32-bit).
 */
const largeMaxWindowBits = 30

/** Minimal value for ::BROTLI_PARAM_LGBLOCK parameter. */
const minInputBlockBits = 16

/** Maximal value for ::BROTLI_PARAM_LGBLOCK parameter. */
const maxInputBlockBits = 24

/** Minimal value for ::BROTLI_PARAM_QUALITY parameter. */
const minQuality = 0

/** Maximal value for ::BROTLI_PARAM_QUALITY parameter. */
const maxQuality = 11

/** Options for ::BROTLI_PARAM_MODE parameter. */
const (
	modeGeneric = 0
	modeText    = 1
	modeFont    = 2
)

/** Default value for ::BROTLI_PARAM_QUALITY parameter. */
const defaultQuality = 11

/** Default value for ::BROTLI_PARAM_LGWIN parameter. */
const defaultWindow = 22

/** Default value for ::BROTLI_PARAM_MODE parameter. */
const defaultMode = modeGeneric

/** Operations that can be performed by streaming encoder. */
const (
	operationProcess      = 0
	operationFlush        = 1
	operationFinish       = 2
	operationEmitMetadata = 3
)

const (
	streamProcessing     = 0
	streamFlushRequested = 1
	streamFinished       = 2
	streamMetadataHead   = 3
	streamMetadataBody   = 4
)
