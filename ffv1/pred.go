package ffv1

func deriveBorders(plane []uint8, x int, y int, width int, height int, stride int) (int, int, int, int, int, int) {
	var T int
	var L int
	var t int
	var l int
	var tr int
	var tl int

	// This is really slow and stupid but matches the spec exactly.

	// T
	if y == 0 || y == 1 {
		T = 0
	} else {
		T = int(plane[(y*stride+x)-(2*stride)])
	}

	// L
	if y == 0 {
		if x == 0 || x == 1 {
			L = 0
		} else {
			L = int(plane[(y*stride+x)-2])
		}
	} else {
		if x == 0 {
			L = 0
		} else if x == 1 {
			L = int(plane[(y*stride+x)-(1*stride)-1])
		} else {
			L = int(plane[(y*stride+x)-2])
		}
	}

	// t
	if y == 0 {
		t = 0
	} else {
		t = int(plane[(y*stride+x)-(1*stride)])
	}

	// l
	if y == 0 {
		if x == 0 {
			l = 0
		} else {
			l = int(plane[(y*stride+x)-1])
		}
	} else {
		if x == 0 {
			l = int(plane[(y*stride+x)-(1*stride)])
		} else {
			l = int(plane[(y*stride+x)-1])
		}
	}

	// tl
	if y == 0 {
		tl = 0
	} else {
		if x == 0 {
			if y == 1 {
				tl = 0
			} else {
				tl = int(plane[(y*stride+x)-(2*stride)])
			}
		} else {
			tl = int(plane[(y*stride+x)-(1*stride)-1])
		}
	}

	// tr
	if y == 0 {
		tr = 0
	} else {
		if x == width-1 {
			tr = int(plane[(y*stride+x)-(1*stride)])
		} else {
			tr = int(plane[(y*stride+x)-(1*stride)+1])
		}
	}

	return T, L, t, l, tr, tl
}

func getContext(quant_tables [5][256]int16, T int, L int, t int, l int, tr int, tl int) int32 {
	return int32(quant_tables[0][(l-tl)&255]) +
		int32(quant_tables[1][(tl-t)&255]) +
		int32(quant_tables[2][(t-tr)&255]) +
		int32(quant_tables[3][(L-l)&255]) +
		int32(quant_tables[4][(T-t)&255])
}

func getMedian(a int, b int, c int) int {
	if a > b {
		if b > c {
			return b
		}

		if c > a {
			return a
		}

		return c
	}

	if c > b {
		return b
	}

	if c > a {
		return c
	}

	return a
}
