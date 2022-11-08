package ffv1

// This file implements 8, 16 and 32 bit depth median prediction.

// Calculates all the neighbouring pixel values given:
//
// +---+---+---+---+
// |   |   | T |   |
// +---+---+---+---+
// |   |tl | t |tr |
// +---+---+---+---+
// | L | l | X |   |
// +---+---+---+---+
//
// where 'X' is the pixel at our current position, and borders are:
//
// +---+---+---+---+---+---+---+---+
// | 0 | 0 |   | 0 | 0 | 0 |   | 0 |
// +---+---+---+---+---+---+---+---+
// | 0 | 0 |   | 0 | 0 | 0 |   | 0 |
// +---+---+---+---+---+---+---+---+
// |   |   |   |   |   |   |   |   |
// +---+---+---+---+---+---+---+---+
// | 0 | 0 |   | a | b | c |   | c |
// +---+---+---+---+---+---+---+---+
// | 0 | a |   | d | e | f |   | f |
// +---+---+---+---+---+---+---+---+
// | 0 | d |   | g | h | i |   | i |
// +---+---+---+---+---+---+---+---+
//
// where 'a' through 'i' are pixel values in a plane.
//
// See: * 3.1. Border
//      * 3.2. Samples
func deriveBorders[U interface{ uint8 | uint16 | uint32 }](plane []U, x int, y int, width int, height int, stride int) (int, int, int, int, int, int) {
	var T int
	var L int
	var t int
	var l int
	var tr int
	var tl int

	pos := y*stride + x

	// This is really slow and stupid but matches the spec exactly. Each of the
	// neighbouring values has been left entirely separate, and none skipped,
	// even if they could be.
	//
	// Please never implement an actual decoder this way.

	// T
	if y > 1 {
		T = int(plane[pos-(2*stride)])
	}

	// L
	if y > 0 && x == 1 {
		L = int(plane[pos-stride-1])
	} else if x > 1 {
		L = int(plane[pos-2])
	}

	// t
	if y > 0 {
		t = int(plane[pos-stride])
	}

	// l
	if x > 0 {
		l = int(plane[pos-1])
	} else if y > 0 {
		l = int(plane[pos-stride])
	}

	// tl
	if y > 1 && x == 0 {
		tl = int(plane[pos-(2*stride)])
	} else if y > 0 && x > 0 {
		tl = int(plane[pos-stride-1])
	}

	// tr
	if y > 0 {
		tr = int(plane[pos-stride+min(1, width-1-x)])
	}

	return T, L, t, l, tr, tl
}

// Given the neighbouring pixel values, calculate the context.
//
// See: * 3.4. Context
//      * 3.5. Quantization Table Sets
func getContext(quant_tables [5][256]int16, T int, L int, t int, l int, tr int, tl int) int32 {
	return int32(quant_tables[0][(l-tl)&255]) +
		int32(quant_tables[1][(tl-t)&255]) +
		int32(quant_tables[2][(t-tr)&255]) +
		int32(quant_tables[3][(L-l)&255]) +
		int32(quant_tables[4][(T-t)&255])
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

// Calculate the median value of 3 numbers
//
// See: 2.2.5. Mathematical Functions
func getMedian(a int, b int, c int) int {
	return a + b + c - min(a, min(b, c)) - max(a, max(b, c))
}
