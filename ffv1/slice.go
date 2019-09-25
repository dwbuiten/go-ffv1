package ffv1

import (
	"fmt"

	"github.com/dwbuiten/go-ffv1/ffv1/rangecoder"
)

type internalFrame struct {
	keyframe   bool
	slice_info []sliceInfo
	slices     []slice
}

type sliceInfo struct {
	pos  int
	size uint32
}

type slice struct {
	header sliceHeader
}

type sliceHeader struct {
	slice_width_minus1    uint32
	slice_height_minus1   uint32
	slice_x               uint32
	slice_y               uint32
	quant_table_set_index []uint8
	picture_structure     uint8
	sar_num               uint32
	sar_den               uint32
}

func countSlices(buf []byte, header *internalFrame, ec bool) error {
	footerSize := 3
	if ec {
		footerSize += 5
	}

	endPos := len(buf)
	for endPos > 0 {
		var info sliceInfo

		size := uint32(buf[endPos-footerSize]) << 16
		size |= uint32(buf[endPos-footerSize+1]) << 8
		size |= uint32(buf[endPos-footerSize+2])

		info.size = size
		info.pos = endPos - int(size) - footerSize
		header.slice_info = append([]sliceInfo{info}, header.slice_info...) //prepend
		endPos = info.pos
	}

	if endPos < 0 {
		return fmt.Errorf("invalid slice footer")
	}

	return nil
}

func (d *Decoder) parseFooters(buf []byte, header *internalFrame) error {
	err := countSlices(buf, header, d.record.ec != 0)
	if err != nil {
		return fmt.Errorf("couldn't count slices: %s", err.Error())
	}

	header.slices = make([]slice, len(header.slice_info))

	return nil
}

func (d *Decoder) parseSliceHeader(c *rangecoder.Coder, s *slice) {
	slice_state := make([]uint8, 32)
	for i := 0; i < 32; i++ {
		slice_state[i] = 128
	}

	s.header.slice_x = c.UR(slice_state)
	s.header.slice_y = c.UR(slice_state)
	s.header.slice_width_minus1 = c.UR(slice_state)
	s.header.slice_height_minus1 = c.UR(slice_state)

	quant_table_set_index_count := 1
	if d.record.chroma_planes {
		quant_table_set_index_count++
	}
	if d.record.extra_plane {
		quant_table_set_index_count++
	}

	s.header.quant_table_set_index = make([]uint8, quant_table_set_index_count)

	for i := 0; i < quant_table_set_index_count; i++ {
		s.header.quant_table_set_index[i] = uint8(c.UR(slice_state))
	}

	s.header.picture_structure = uint8(c.UR(slice_state))
	s.header.sar_num = c.UR(slice_state)
	s.header.sar_den = c.UR(slice_state)
}

func (d *Decoder) decodeSlice(buf []byte, header *internalFrame, slicenum int) error {
	c := rangecoder.NewCoder(buf[header.slice_info[slicenum].pos:])

	state := make([]uint8, 32)
	for i := 0; i < 32; i++ {
		state[i] = 128
	}

	if slicenum == 0 {
		header.keyframe = c.BR(state)
		fmt.Println("keyframe = ", header.keyframe)
	}

	if d.record.coder_type == 2 { // Custom state transition table
		c.SetTable(d.state_transition)
	}

	d.parseSliceHeader(c, &header.slices[slicenum])

	fmt.Println(header.slices[slicenum].header)

	return nil
}
