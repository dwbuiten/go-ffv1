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
	context_count           [MaxQuantTables]int32
	quant_tables            [MaxQuantTables][MaxContextInputs][256]int16
	states_coded            bool
	initial_state_delta     [][][]int16
	ec                      uint8
	intra                   uint8
}

func parseConfigRecord(buf []byte, record *configRecord) error {
	c := rangecoder.NewCoder(buf)

	state := make([]uint8, 32)
	for i := 0; i < 32; i++ {
		state[i] = 128
	}

	record.version = uint8(c.UR(state))
	if record.version != 3 {
		return fmt.Errorf("only FFV1 version 3 is supported")
	}

	record.micro_version = uint8(c.UR(state))
	if record.micro_version < 1 {
		return fmt.Errorf("only FFV1 micro version >1 supported")
	}

	record.coder_type = uint8(c.UR(state))
	if record.coder_type > 2 {
		return fmt.Errorf("invalid coder_type: %d", record.coder_type)
	}
	fmt.Printf("coder type is %d\n", record.coder_type)

	if record.coder_type > 1 {
		for i := 1; i < 256; i++ {
			record.state_transition_delta[i] = int16(c.SR(state))
		}
	}

	record.colorspace_type = uint8(c.UR(state))
	if record.colorspace_type > 1 {
		return fmt.Errorf("invalid colorspace_type: %d", record.colorspace_type)
	}

	record.bits_per_raw_sample = uint8(c.UR(state))
	if record.bits_per_raw_sample == 0 {
		record.bits_per_raw_sample = 8
	}
	if record.bits_per_raw_sample != 8 {
		panic("high bit depth not implemented yet!")
	}

	record.chroma_planes = c.BR(state)
	if record.colorspace_type == 1 && !record.chroma_planes {
		return fmt.Errorf("RGB must contain chroma planes")
	}

	record.log2_h_chroma_subsample = uint8(c.UR(state))
	if record.colorspace_type == 1 && record.log2_h_chroma_subsample != 0 {
		return fmt.Errorf("RGB cannot be subsampled")
	}

	record.log2_v_chroma_subsample = uint8(c.UR(state))
	if record.colorspace_type == 1 && record.log2_v_chroma_subsample != 0 {
		return fmt.Errorf("RGB cannot be subsampled")
	}

	record.extra_plane = c.BR(state)
	record.num_h_slices_minus1 = uint8(c.UR(state))
	record.num_v_slices_minus1 = uint8(c.UR(state))

	record.quant_table_set_count = uint8(c.UR(state))
	if record.quant_table_set_count == 0 {
		return fmt.Errorf("quant_table_set_count may not be zero")
	} else if record.quant_table_set_count > MaxQuantTables {
		return fmt.Errorf("too many quant tables: %d > %d", record.quant_table_set_count, MaxQuantTables)
	}

	for i := 0; i < int(record.quant_table_set_count); i++ {
		scale := 1
		for j := 0; j < MaxContextInputs; j++ {
			// Each table has its own state table! Not mentioned in the spec.
			quant_state := make([]byte, 32)
			for qs := 0; qs < 32; qs++ {
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
			record.initial_state_delta[i][j] = make([]int16, ContextSize)
		}
		states_coded := c.BR(state)
		if states_coded {
			for j := 0; j < int(record.context_count[i]); j++ {
				for k := 0; k < ContextSize; k++ {
					record.initial_state_delta[i][j][k] = int16(c.SR(state))
				}
			}
		}
	}

	record.ec = uint8(c.UR(state))
	record.intra = uint8(c.UR(state))

	return nil
}
