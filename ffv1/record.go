package ffv1

type configRecord struct {
	version                 uint8
	micro_version           uint8
	coder_type              uint8
	state_transition_delta  [255]uint8
	colorspace_type         uint8
	bits_per_raw_sample     uint8
	chroma_planes           uint8
	log2_h_chroma_subsample uint8
	log2_v_chroma_subsample uint8
	extra_plane             uint8
	num_h_slices_minus1     uint8
	num_v_slices_minus1     uint8
	quant_table_set_count   uint8
	context_counts          [MaxQuantTables]int32
	quant_tables            [MaxQuantTables][MaxContextInputs][256]int16
	states_coded            bool
	initial_state_delta     [][][]uint8 // initial_state_delta[MaxQuantTables][context_count[i]][ContextSize]
	ec                      bool
	intra                   bool
}
