package ffv1

import (
	"github.com/dwbuiten/go-ffv1/ffv1/rangecoder"
)

func (d *Decoder) initializeStates() {
	for i := 1; i < 256; i++ {
		d.state_transition[i] = uint8(int16(rangecoder.DefaultStateTransition[i]) + d.record.state_transition_delta[i])
	}

	d.initial_states = make([][][]uint8, len(d.record.initial_state_delta))
	for i := 0; i < len(d.record.initial_state_delta); i++ {
		d.initial_states[i] = make([][]uint8, len(d.record.initial_state_delta[i]))
		for j := 0; j < len(d.record.initial_state_delta[i]); j++ {
			d.initial_states[i][j] = make([]uint8, len(d.record.initial_state_delta[i][j]))
			for k := 0; k < len(d.record.initial_state_delta[i][j]); k++ {
				pred := int16(128)
				if j != 0 {
					pred = int16(d.initial_states[i][j-1][k])
				}
				d.initial_states[i][j][k] = uint8((pred + d.record.initial_state_delta[i][j][k]) & 255)
			}
		}
	}
}
