package ffv1

import (
	"fmt"

	"github.com/dwbuiten/go-ffv1/ffv1/rangecoder"
)

type configRecord struct {
	version                 uint8
	micro_version           uint8
	coder_type              uint8
	state_transition_delta  [256]int16
	colorspace_type         uint8
	bits_per_raw_sample     uint8
	chroma_planes           bool
	log2_h_chroma_subsample uint8
	log2_v_chroma_subsample uint8
	extra_plane             bool
	num_h_slices_minus1     uint8
	num_v_slices_minus1     uint8
	quant_table_set_count   uint8
	context_count           [maxQuantTables]int32
	quant_tables            [maxQuantTables][maxContextInputs][256]int16
	states_coded            bool
	initial_state_delta     [][][]int16
	ec                      uint8
	intra                   uint8
}

// Parses the configuration record from the codec private data.
//
// See: * 4.1. Parameters
//      * 4.2. Configuration Record
func parseConfigRecord(buf []byte, record *configRecord) error {
	// Before we do anything, CRC check.
	//
	// See: 4.2.2. configuration_record_crc_parity
	if crc32MPEG2(buf) != 0 {
		return fmt.Errorf("failed CRC check for configuration record")
	}
	c := rangecoder.NewCoder(buf)

	// 4. Bitstream
	state := make([]uint8, contextSize)
	for i := 0; i < contextSize; i++ {
		state[i] = 128
	}

	// 4.1.1. version
	record.version = uint8(c.UR(state))
	if record.version != 3 {
		return fmt.Errorf("only FFV1 version 3 is supported")
	}

	// 4.1.2. micro_version
	record.micro_version = uint8(c.UR(state))
	if record.micro_version < 1 {
		return fmt.Errorf("only FFV1 micro version >1 supported")
	}

	// 4.1.3. coder_type
	record.coder_type = uint8(c.UR(state))
	if record.coder_type > 2 {
		return fmt.Errorf("invalid coder_type: %d", record.coder_type)
	}

	// 4.1.4. state_transition_delta
	if record.coder_type > 1 {
		for i := 1; i < 256; i++ {
			record.state_transition_delta[i] = int16(c.SR(state))
		}
	}

	// 4.1.5. colorspace_type
	record.colorspace_type = uint8(c.UR(state))
	if record.colorspace_type > 1 {
		return fmt.Errorf("invalid colorspace_type: %d", record.colorspace_type)
	}

	// 4.1.7. bits_per_raw_sample
	record.bits_per_raw_sample = uint8(c.UR(state))
	if record.bits_per_raw_sample == 0 {
		record.bits_per_raw_sample = 8
	}
	if record.coder_type == 0 && record.bits_per_raw_sample != 8 {
		return fmt.Errorf("golomb-rice mode cannot have >8bit per sample")
	}

	// TODO: Add 32-bit scratch buffer after refactoring.
	if record.bits_per_raw_sample == 16 && record.colorspace_type == 1 {
		return fmt.Errorf("16-bit RGB mode is currently unimplemented")
	}

	// 4.1.6. chroma_planes
	record.chroma_planes = c.BR(state)
	if record.colorspace_type == 1 && !record.chroma_planes {
		return fmt.Errorf("RGB must contain chroma planes")
	}

	// 4.1.8. log2_h_chroma_subsample
	record.log2_h_chroma_subsample = uint8(c.UR(state))
	if record.colorspace_type == 1 && record.log2_h_chroma_subsample != 0 {
		return fmt.Errorf("RGB cannot be subsampled")
	}

	// 4.1.9. log2_v_chroma_subsample
	record.log2_v_chroma_subsample = uint8(c.UR(state))
	if record.colorspace_type == 1 && record.log2_v_chroma_subsample != 0 {
		return fmt.Errorf("RGB cannot be subsampled")
	}

	// 4.1.10. extra_plane
	record.extra_plane = c.BR(state)
	// 4.1.11. num_h_slices
	record.num_h_slices_minus1 = uint8(c.UR(state))
	// 4.1.12. num_v_slices
	record.num_v_slices_minus1 = uint8(c.UR(state))

	// 4.1.13. quant_table_set_count
	record.quant_table_set_count = uint8(c.UR(state))
	if record.quant_table_set_count == 0 {
		return fmt.Errorf("quant_table_set_count may not be zero")
	} else if record.quant_table_set_count > maxQuantTables {
		return fmt.Errorf("too many quant tables: %d > %d", record.quant_table_set_count, maxQuantTables)
	}

	for i := 0; i < int(record.quant_table_set_count); i++ {
		// 4.9.  Quantization Table Set
		scale := 1
		for j := 0; j < maxContextInputs; j++ {
			// Each table has its own state table.
			quant_state := make([]byte, contextSize)
			for qs := 0; qs < contextSize; qs++ {
				quant_state[qs] = 128
			}
			v := 0
			for k := 0; k < 128; {
				len_minus1 := c.UR(quant_state)
				for a := 0; a < int(len_minus1+1); a++ {
					record.quant_tables[i][j][k] = int16(scale * v)
					k++
				}
				v++
			}
			for k := 1; k < 128; k++ {
				record.quant_tables[i][j][256-k] = -record.quant_tables[i][j][k]
			}
			record.quant_tables[i][j][128] = -record.quant_tables[i][j][127]
			scale *= 2*v - 1
		}
		record.context_count[i] = int32((scale + 1) / 2)
	}

	// Why on earth did they choose to do a variable length buffer in the
	// *middle and start* of a 3D array?
	record.initial_state_delta = make([][][]int16, int(record.quant_table_set_count))
	for i := 0; i < int(record.quant_table_set_count); i++ {
		record.initial_state_delta[i] = make([][]int16, int(record.context_count[i]))
		for j := 0; j < int(record.context_count[i]); j++ {
			record.initial_state_delta[i][j] = make([]int16, contextSize)
		}
		states_coded := c.BR(state)
		if states_coded {
			for j := 0; j < int(record.context_count[i]); j++ {
				for k := 0; k < contextSize; k++ {
					record.initial_state_delta[i][j][k] = int16(c.SR(state))
				}
			}
		}
	}

	// 4.1.16. ec
	record.ec = uint8(c.UR(state))
	// 4.1.17. intra
	record.intra = uint8(c.UR(state))

	return nil
}

// Initializes initial state for the range coder.
//
// See: 4.1.15. initial_state_delta
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
