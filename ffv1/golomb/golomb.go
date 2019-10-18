// Package golomb implements a Golomb-Rice coder as per Section 3.8.2. Golomb Rice Mode
// of draft-ietf-cellar-ffv1.
package golomb

// Coder is an instance of a Golomb-Rice coder as described in  3.8.2. Golomb Rice Mode.
type Coder struct {
	r         *bitReader
	run_mode  int
	run_count int
	run_index int
	x         uint32
	w         uint32
}

// State contains a single set of states for the a Golomb-Rice coder as define in
// 3.8.2.4. Initial Values for the VLC context state.
type State struct {
	drift     int32
	error_sum int32
	bias      int32
	count     int32
}

// NewState creates a Golomb-Rice state with the initial values defined in
// 3.8.2.4. Initial Values for the VLC context state.
func NewState() State {
	return State{
		drift:     0,
		error_sum: 4,
		bias:      0,
		count:     1,
	}
}

// NewCoder creates a new Golomb-Rice coder.
func NewCoder(buf []byte) *Coder {
	ret := new(Coder)
	ret.r = newBitReader(buf)
	return ret
}

// NewPlane should be called on a given Coder as each new Plane is
// processed. It resets the run index and sets the slice width.
//
// See: 3.8.2.2.1. Run Length Coding
func (c *Coder) NewPlane(width uint32) {
	c.w = width
	c.run_index = 0
}

// Starts a new run.
func (c *Coder) newRun() {
	c.run_mode = 0
	c.run_count = 0
}

// NewLine resets the x position and starts a new run,
// since runs can only be per-line.
func (c *Coder) NewLine() {
	c.newRun()
	c.x = 0
}

// SG gets the next Golomb-Rice coded signed scalar symbol.
//
// See: * 3.8.2. Golomb Rice Mode
//      * 4. Bitstream
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
		// No more repeats; the run is over. Read a new symbol.
		if c.run_count < 0 {
			c.newRun()
			diff := c.get_vlc_symbol(state)
			// 3.8.2.2.2. Level Coding
			if diff >= 0 {
				diff++
			}
			c.x++
			return diff
		} else {
			// The run is still going; return a difference of zero.
			c.x++
			return 0
		}
	} else {
		// We aren't in run mode; get a new symbol.
		c.x++
		return c.get_vlc_symbol(state)
	}
}

// Simple sign extension. Not *actually* needd in Go.
func sign_extend(n int32) int32 {
	ret := int8(n)
	return int32(ret)
}

// Gets the next Golomb-Rice coded symbol.
//
// See: 3.8.2.3. Scalar Mode
func (c *Coder) get_vlc_symbol(state *State) int32 {
	i := state.count
	k := uint32(0)

	for i < state.error_sum {
		k++
		i += i
	}

	v := c.get_sr_golomb(k)

	if 2*state.drift < -state.count {
		v = -1 - v
	}

	ret := sign_extend(v + state.bias)

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

// Gets the next signed Golomb-Rice code
//
// See: 3.8.2.1. Signed Golomb Rice Codes
func (c *Coder) get_sr_golomb(k uint32) int32 {
	v := c.get_ur_golomb(k)
	if v&1 == 1 {
		return -(v >> 1) - 1
	} else {
		return v >> 1
	}
}

// Gets the next unsigned Golomb-Rice code
//
// See: 3.8.2.1. Signed Golomb Rice Codes
func (c *Coder) get_ur_golomb(k uint32) int32 {
	for prefix := 0; prefix < 12; prefix++ {
		if c.r.u(1) == 1 {
			return int32(c.r.u(k)) + int32((prefix << k))
		}
	}
	return int32(c.r.u(8)) + 11
}
