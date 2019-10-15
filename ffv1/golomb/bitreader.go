package golomb

import (
	"bytes"
)

type bitReader struct {
	r         *bytes.Reader
	bitBuf    uint32
	bitsInBuf uint32
}

func newBitReader(buffer []byte) (r *bitReader) {
	reader := bytes.NewReader(buffer)
	r = &bitReader{r: reader}
	return
}

func (r *bitReader) readByte() (result uint8) {
	result, err := r.r.ReadByte()
	if err != nil {
		panic("wtf read error will go away" + err.Error())
	}
	return result
}

func (r *bitReader) u(count uint32) (result uint32) {
	if count > 32 {
		panic("WTF more than 32 bits")
	}
	for count > r.bitsInBuf {
		r.bitBuf <<= 8
		r.bitBuf |= uint32(r.readByte())
		r.bitsInBuf += 8

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
