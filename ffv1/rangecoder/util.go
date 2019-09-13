package rangecoder

func min32(a int32, b int32) int32 {
	if a > b {
		return b
	}
	return a
}
