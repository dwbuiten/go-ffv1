package golomb

type bitReader struct {
	buf       []byte
	pos       int
	bitBuf    uint32
	bitsInBuf uint32
}

// Creates a new bitreader.
func newBitReader(buf []byte) (r *bitReader) {
	ret := new(bitReader)
	ret.buf = buf
	return ret
}

// Reads 'count' bits, up to 32.
func (r *bitReader) u(count uint32) (result uint32) {
	if count > 32 {
		panic("WTF more than 32 bits")
	}
	for count > r.bitsInBuf {
		r.bitBuf <<= 8
		r.bitBuf |= uint32(r.buf[r.pos])
		r.bitsInBuf += 8
		r.pos++

		if r.bitsInBuf > 24 {
			if count <= r.bitsInBuf {
				break
			}
			if count <= 32 {
				return r.u(16)<<16 | r.u(count-16)
			}
		}
	}
	r.bitsInBuf -= count
	return (r.bitBuf >> r.bitsInBuf) & ((uint32(1) << count) - 1)
}
