package rangecoder

type Coder struct {
	buf        []byte
	pos        int
	low        uint16
	rng        uint16
	left       int32
	cur_byte   int32
	zero_state [256]uint8
	one_state  [256]uint8
	overread   uint8
}

func NewCoder(buf []byte) *Coder {
	ret := new(Coder)

	ret.buf = buf
	ret.pos = 2
	ret.low = uint16(buf[0])<<8 | uint16(buf[1])
	ret.rng = 0xFF00
	ret.left = 0
	ret.cur_byte = -1
	if ret.low >= ret.rng {
		ret.low = ret.rng
		ret.pos = len(buf) - 1
	}

	ret.buildStates()

	return ret
}

// Change this to a LUT
func (c *Coder) buildStates() {
	factor := int32(214748364) // int32(float64(int64(1<<32)) * 0.05)
	max_p := int32(256 - 8)
	one := int64(1) << 32
	last_p8 := int32(0)
	p := one / 2

	var p8 int32
	for i := 0; i < 128; i++ {
		p8 = int32((256*p + one/2) >> 32)
		if p8 <= last_p8 {
			p8 = last_p8 + 1
		}
		if last_p8 != 0 && last_p8 < 256 && p8 <= max_p {
			c.one_state[int(last_p8)] = uint8(p8)
		}

		p += ((one-p)*int64(factor) + one/2) >> 32
		last_p8 = p8
	}

	for i := 256 - max_p; i <= max_p; i++ {
		if c.one_state[i] != 0 {
			continue
		}

		p = (int64(i)*one + 128) >> 8
		p += ((one-p)*int64(factor) + one/2) >> 32
		p8 = int32((256*p + one/2) >> 32)
		if p8 <= i {
			p8 = i + 1
		}
		if p8 > max_p {
			p8 = max_p
		}
		c.one_state[i] = uint8(p8)
	}
	for i := 1; i < 255; i++ {
		c.zero_state[i] = uint8(uint16(256) - uint16(c.one_state[256-i]))
	}
}

func (c *Coder) refill() {
	if c.rng < 0x100 {
		c.rng = c.rng << 8
		c.low = c.low << 8
		if c.pos < len(c.buf) {
			c.low += uint16(c.buf[c.pos])
			c.pos++
		} else {
			c.overread++
		}
	}
}

func (c *Coder) get(state *uint8) bool {
	rangeoff := uint16((uint32(c.rng) * uint32((*state))) >> 8)
	c.rng -= rangeoff
	if c.low < c.rng {
		*state = c.zero_state[int(*state)]
		c.refill()
		return false
	} else {
		c.low -= c.rng
		*state = c.one_state[int(*state)]
		c.rng = rangeoff
		c.refill()
		return true
	}
}

func (c *Coder) UR(state []uint8) int32 {
	return c.symbol(state, false)
}

func (c *Coder) SR(state []uint8) int32 {
	return c.symbol(state, true)
}

func (c *Coder) BR(state []uint8) int32 {
	if c.get(&state[0]) {
		return 1
	} else {
		return 0
	}
}

func (c *Coder) symbol(state []uint8, signed bool) int32 {
	if c.get(&state[0]) {
		return 0
	}

	e := int32(0)
	for c.get(&state[1+min32(e, 9)]) {
		e++
		if e > 31 {
			panic("WTF range coder!")
		}
	}

	a := uint32(1)
	for i := e - 1; i >= 0; i-- {
		a = a * 2
		if c.get(&state[22+min32(i, 9)]) {
			a++
		}
	}

	if signed && c.get(&state[11+min32(e, 10)]) {
		return -(int32(a))
	} else {
		return int32(a)
	}
}
