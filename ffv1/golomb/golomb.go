package golomb

type Coder struct {
	r         *bitReader
	run_mode  int
	run_count int
	run_index int
	x         uint32
	w         uint32
}

type State struct {
	drift     int32
	error_sum int32
	bias      int32
	count     int32
}

func NewState() State {
	return State{
		drift:     0,
		error_sum: 4,
		bias:      0,
		count:     1,
	}
}

func NewCoder(buf []byte) *Coder {
	ret := new(Coder)
	ret.r = newBitReader(buf)
	return ret
}

func (c *Coder) NewPlane(width uint32) {
	c.w = width
	c.run_index = 0
}

func (c *Coder) newRun() {
	c.run_mode = 0
	c.run_count = 0
}

func (c *Coder) NewLine() {
	c.newRun()
	c.x = 0
}

func (c *Coder) SG(context int32, state *State) int32 {
	// Section 3.8.2.2. Run Mode
	if context == 0 && c.run_mode == 0 {
		c.run_mode = 1
	}

	// Section 3.8.2.2.1. Run Length Coding
	if c.run_mode != 0 {
		if c.run_count == 0 && c.run_mode == 1 {
			if c.r.u(1) == 1 {
				c.run_count = 1 << log2_run[c.run_index]
				if c.x+uint32(c.run_count) <= c.w {
					c.run_index++
				}
			} else {
				if log2_run[c.run_index] != 0 {
					c.run_count = int(c.r.u(uint32(log2_run[c.run_index])))
				} else {
					c.run_count = 0
				}
				if c.run_index != 0 {
					c.run_index--
				}
				// This is in the spec but how it works is... non-obvious.
				c.run_mode = 2
			}
		}

		c.run_count--
		if c.run_count < 0 {
			c.newRun()
			diff := c.get_vlc_symbol(state)
			if diff >= 0 {
				diff++
			}
			c.x++
			return diff
		} else {
			c.x++
			return 0
		}
	} else {
		c.x++
		return c.get_vlc_symbol(state)
	}
}

func sign_extend(n int32, bits uint8) int32 {
	if bits == 8 {
		ret := int8(n) + 0
		return int32(ret)
	} else {
		panic("no high bit depth golomb")
	}
}

func (c *Coder) get_vlc_symbol(state *State) int32 {
	i := state.count
	k := uint32(0)

	for i < state.error_sum {
		k++
		i += i
	}

	// TODO: High bit depth
	v := c.get_sr_golomb(k, 8)

	if 2*state.drift < -state.count {
		v = -1 - v
	}

	//TODO: High bit depth
	ret := sign_extend(v+state.bias, 8)

	state.error_sum += abs32(v)
	state.drift += v

	if state.count == 128 {
		state.count >>= 1
		state.drift >>= 1
		state.error_sum >>= 1
	}
	state.count++
	if state.drift <= -state.count {
		state.bias = max32(state.bias-1, -128)
		state.drift = max32(state.drift+state.count, -state.count+1)
	} else if state.drift > 0 {
		state.bias = min32(state.bias+1, 127)
		state.drift = min32(state.drift-state.count, 0)
	}

	return ret
}

func (c *Coder) get_sr_golomb(k uint32, bits uint32) int32 {
	v := c.get_ur_golomb(k, bits)
	if v&1 == 1 {
		return -(v >> 1) - 1
	} else {
		return v >> 1
	}
}

func (c *Coder) get_ur_golomb(k uint32, bits uint32) int32 {
	for prefix := 0; prefix < 12; prefix++ {
		if c.r.u(1) == 1 {
			return int32(c.r.u(k)) + int32((prefix << k))
		}
	}
	return int32(c.r.u(bits)) + 11
}
