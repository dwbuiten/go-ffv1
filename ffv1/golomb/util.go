package golomb

func min32(a int32, b int32) int32 {
	if a > b {
		return b
	}
	return a
}

func max32(a int32, b int32) int32 {
	if a < b {
		return b
	}
	return a
}

func abs32(n int32) int32 {
	if n >= 0 {
		return n
	}
	return -1 * n
}
