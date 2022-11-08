package ffv1

import (
	"fmt"
	"math"

	"github.com/dwbuiten/go-ffv1/ffv1/golomb"
	"github.com/dwbuiten/go-ffv1/ffv1/rangecoder"
)

type internalFrame struct {
	keyframe   bool
	slice_info []sliceInfo
	slices     []slice
}

type sliceInfo struct {
	pos          int
	size         uint32
	error_status uint8
}

type slice struct {
	header       sliceHeader
	start_x      uint32
	start_y      uint32
	width        uint32
	height       uint32
	state        [][][]uint8
	golomb_state [][]golomb.State
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

// Counts the number of slices in a frame, as described in
// 9.1.1. Multi-threading Support and Independence of Slices.
//
// See: 4.8. Slice Footer
func countSlices(buf []byte, header *internalFrame, ec bool) error {
	footerSize := 3
	if ec {
		footerSize += 5
	}

	// Go over the packet from the end to start, reading the footer,
	// so we can derive the slice positions within the packet, and
	// allow multithreading.
	endPos := len(buf)
	header.slice_info = nil
	for endPos > 0 {
		var info sliceInfo

		// 4.8.1. slice_size
		size := uint32(buf[endPos-footerSize]) << 16
		size |= uint32(buf[endPos-footerSize+1]) << 8
		size |= uint32(buf[endPos-footerSize+2])
		info.size = size

		// 4.8.2. error_status
		info.error_status = uint8(buf[endPos-footerSize+3])

		info.pos = endPos - int(size) - footerSize
		header.slice_info = append([]sliceInfo{info}, header.slice_info...) //prepend
		endPos = info.pos
	}

	if endPos < 0 {
		return fmt.Errorf("invalid slice footer")
	}

	return nil
}

// Parses all footers in a frame and allocates any necessary slice structures.
//
// See: * 9.1.1. Multi-threading Support and Independence of Slices
//      * 3.8.1.3. Initial Values for the Context Model
//      * 3.8.2.4. Initial Values for the VLC context state
func (d *Decoder) parseFooters(buf []byte, header *internalFrame) error {
	err := countSlices(buf, header, d.record.ec != 0)
	if err != nil {
		return fmt.Errorf("couldn't count slices: %s", err.Error())
	}

	slices := make([]slice, len(header.slice_info))
	if !header.keyframe {
		if len(slices) != len(header.slices) {
			return fmt.Errorf("inter frames must have the same number of slices as the preceding intra frame")
		}
		for i := 0; i < len(slices); i++ {
			slices[i].state = header.slices[i].state
		}
		if d.record.coder_type == 0 {
			for i := 0; i < len(slices); i++ {
				slices[i].golomb_state = header.slices[i].golomb_state
			}
		}
	}
	header.slices = slices

	return nil
}

// Parses a slice's header.
//
// See: 4.5. Slice Header
func (d *Decoder) parseSliceHeader(c *rangecoder.Coder, s *slice) {
	// 4. Bitstream
	slice_state := make([]uint8, contextSize)
	for i := 0; i < contextSize; i++ {
		slice_state[i] = 128
	}

	// 4.5.1. slice_x
	s.header.slice_x = c.UR(slice_state)
	// 4.5.2. slice_y
	s.header.slice_y = c.UR(slice_state)
	// 4.5.3 slice_width
	s.header.slice_width_minus1 = c.UR(slice_state)
	// 4.5.4 slice_height
	s.header.slice_height_minus1 = c.UR(slice_state)

	// 4.5.5. quant_table_set_index_count
	quant_table_set_index_count := 1
	if d.record.chroma_planes {
		quant_table_set_index_count++
	}
	if d.record.extra_plane {
		quant_table_set_index_count++
	}

	// 4.5.6. quant_table_set_index
	s.header.quant_table_set_index = make([]uint8, quant_table_set_index_count)
	for i := 0; i < quant_table_set_index_count; i++ {
		s.header.quant_table_set_index[i] = uint8(c.UR(slice_state))
	}

	// 4.5.7. picture_structure
	s.header.picture_structure = uint8(c.UR(slice_state))

	// It's really weird for slices within the same frame to code
	// their own SAR values...
	//
	// See: * 4.5.8. sar_num
	//      * 4.5.9. sar_den
	s.header.sar_num = c.UR(slice_state)
	s.header.sar_den = c.UR(slice_state)

	// Calculate bounaries for easy use elsewhere
	//
	// See: * 4.6.3. slice_pixel_height
	//      * 4.6.4. slice_pixel_y
	//      * 4.7.2. slice_pixel_width
	//      * 4.7.3. slice_pixel_x
	s.start_x = s.header.slice_x * d.width / (uint32(d.record.num_h_slices_minus1) + 1)
	s.start_y = s.header.slice_y * d.height / (uint32(d.record.num_v_slices_minus1) + 1)
	s.width = ((s.header.slice_x + s.header.slice_width_minus1 + 1) * d.width / (uint32(d.record.num_h_slices_minus1) + 1)) - s.start_x
	s.height = ((s.header.slice_y + s.header.slice_height_minus1 + 1) * d.height / (uint32(d.record.num_v_slices_minus1) + 1)) - s.start_y
}

// Line decoding.
//
// So, so many arguments. I would have just inlined this whole thing
// but it needs to be separate because of RGB mode where every line
// is done in its entirety instead of per plane.
//
// Many could be refactored into being in the context, but I haven't
// got to it yet, so instead, I shall repent once for each function
// argument, twice daily.
//
// See: 4.7. Line
func (d *Decoder) decodeLine(c *rangecoder.Coder, gc *golomb.Coder, s *slice, frame *Frame, w int, h int, stride int, offset int, y int, p int, qt int) {
	// Runs are horizontal and thus cannot run more than a line.
	//
	// See: 3.8.2.2.1. Run Length Coding
	if gc != nil {
		gc.NewLine()
	}

	// 4.7.4. sample_difference
	for x := 0; x < w; x++ {
		var sign bool

		var buf []byte
		var buf16 []uint16
		var buf32 []uint32
		if d.record.bits_per_raw_sample == 8 && d.record.colorspace_type != 1 {
			buf = frame.Buf[p][offset:]
		} else if d.record.bits_per_raw_sample == 16 && d.record.colorspace_type == 1 {
			buf32 = frame.buf32[p][offset:]
		} else {
			buf16 = frame.Buf16[p][offset:]
		}

		// 3.8. Coding of the Sample Difference
		shift := d.record.bits_per_raw_sample
		if d.record.colorspace_type == 1 {
			shift = d.record.bits_per_raw_sample + 1
		}

		// Derive neighbours
		//
		// See pred.go for details.
		var T, L, t, l, tr, tl int
		if d.record.bits_per_raw_sample == 8 && d.record.colorspace_type != 1 {
			T, L, t, l, tr, tl = deriveBorders(buf, x, y, w, h, stride)
		} else if d.record.bits_per_raw_sample == 16 && d.record.colorspace_type == 1 {
			T, L, t, l, tr, tl = deriveBorders(buf32, x, y, w, h, stride)
		} else {
			T, L, t, l, tr, tl = deriveBorders(buf16, x, y, w, h, stride)
		}

		// See pred.go for details.
		//
		// See also: * 3.4. Context
		//           * 3.6. Quantization Table Set Indexes
		context := getContext(d.record.quant_tables[s.header.quant_table_set_index[qt]], T, L, t, l, tr, tl)
		if context < 0 {
			context = -context
			sign = true
		} else {
			sign = false
		}

		var diff int32
		if gc != nil {
			diff = gc.SG(context, &s.golomb_state[qt][context], uint(shift))
		} else {
			diff = c.SR(s.state[qt][context])
		}

		// 3.4. Context
		if sign {
			diff = -diff
		}

		// 3.8. Coding of the Sample Difference
		val := diff
		if d.record.colorspace_type == 0 && d.record.bits_per_raw_sample == 16 && gc == nil {
			// 3.3. Median Predictor
			var left16s int
			var top16s int
			var diag16s int

			if l >= 32768 {
				left16s = l - 65536
			} else {
				left16s = l
			}
			if t >= 32768 {
				top16s = t - 65536
			} else {
				top16s = t
			}
			if tl >= 32768 {
				diag16s = tl - 65536
			} else {
				diag16s = tl
			}

			val += int32(getMedian(left16s, top16s, left16s+top16s-diag16s))
		} else {
			val += int32(getMedian(l, t, l+t-tl))
		}

		val = val & ((1 << shift) - 1)

		if d.record.bits_per_raw_sample == 8 && d.record.colorspace_type != 1 {
			buf[(y*stride)+x] = byte(val)
		} else if d.record.bits_per_raw_sample == 16 && d.record.colorspace_type == 1 {
			buf32[(y*stride)+x] = uint32(val)
		} else {
			buf16[(y*stride)+x] = uint16(val)
		}
	}
}

// Decoding happens here.
//
// See: * 4.6. Slice Content
func (d *Decoder) decodeSliceContent(c *rangecoder.Coder, gc *golomb.Coder, si *sliceInfo, s *slice, frame *Frame) {
	// 4.6.1. primary_color_count
	primary_color_count := 1
	chroma_planes := 0
	if d.record.chroma_planes {
		chroma_planes = 2
		primary_color_count += 2
	}
	if d.record.extra_plane {
		primary_color_count++
	}

	if d.record.colorspace_type != 1 {
		// YCbCr Mode
		//
		// Planes are independent.
		//
		// See: 3.7.1. YCbCr
		for p := 0; p < primary_color_count; p++ {
			var plane_pixel_height int
			var plane_pixel_width int
			var plane_pixel_stride int
			var start_x int
			var start_y int
			var quant_table int

			// See: * 4.6.2. plane_pixel_height
			//      * 4.7.1. plane_pixel_width
			if p == 0 || p == 1+chroma_planes {
				plane_pixel_height = int(s.height)
				plane_pixel_width = int(s.width)
				plane_pixel_stride = int(d.width)
				start_x = int(s.start_x)
				start_y = int(s.start_y)
				if p == 0 {
					quant_table = 0
				} else {
					quant_table = chroma_planes
				}
			} else {
				// This is, of course, silly, but I want to do it "by the spec".
				plane_pixel_height = int(math.Ceil(float64(s.height) / float64(uint32(1)<<d.record.log2_v_chroma_subsample)))
				plane_pixel_width = int(math.Ceil(float64(s.width) / float64(uint32(1)<<d.record.log2_h_chroma_subsample)))
				plane_pixel_stride = int(math.Ceil(float64(d.width) / float64(uint32(1)<<d.record.log2_h_chroma_subsample)))
				start_x = int(math.Ceil(float64(s.start_x) / float64(uint32(1)<<d.record.log2_v_chroma_subsample)))
				start_y = int(math.Ceil(float64(s.start_y) / float64(uint32(1)<<d.record.log2_h_chroma_subsample)))
				quant_table = 1
			}

			// 3.8.2.2.1. Run Length Coding
			if gc != nil {
				gc.NewPlane(uint32(plane_pixel_width))
			}

			for y := 0; y < plane_pixel_height; y++ {
				offset := start_y*plane_pixel_stride + start_x
				d.decodeLine(c, gc, s, frame, plane_pixel_width, plane_pixel_height, plane_pixel_stride, offset, y, p, quant_table)
			}
		}
	} else {
		// RGB (JPEG2000-RCT) Mode
		//
		// All planes are coded per line.
		//
		// See: 3.7.2. RGB
		if gc != nil {
			gc.NewPlane(uint32(s.width))
		}
		offset := int(s.start_y*d.width + s.start_x)
		for y := 0; y < int(s.height); y++ {
			// RGB *must* have chroma planes, so this is safe.
			d.decodeLine(c, gc, s, frame, int(s.width), int(s.height), int(d.width), offset, y, 0, 0)
			d.decodeLine(c, gc, s, frame, int(s.width), int(s.height), int(d.width), offset, y, 1, 1)
			d.decodeLine(c, gc, s, frame, int(s.width), int(s.height), int(d.width), offset, y, 2, 1)
			if d.record.extra_plane {
				d.decodeLine(c, gc, s, frame, int(s.width), int(s.height), int(d.width), offset, y, 3, 2)
			}
		}

		// Convert to RGB all at once, cache locality be damned.
		if d.record.bits_per_raw_sample == 8 {
			rct8(frame.Buf, frame.Buf16, int(s.width), int(s.height), int(d.width), offset)
		} else if d.record.bits_per_raw_sample >= 9 && d.record.bits_per_raw_sample <= 15 && !d.record.extra_plane {
			// See: 3.7.2. RGB
			rctMid(frame.Buf16, int(s.width), int(s.height), int(d.width), offset, uint(d.record.bits_per_raw_sample))
		} else {
			rct16(frame.Buf16, frame.buf32, int(s.width), int(s.height), int(d.width), offset)
		}
	}
}

// Determines whether a given frame is a keyframe.
//
// See: 4.3. Frame
func isKeyframe(buf []byte) bool {
	// 4. Bitstream
	state := make([]uint8, contextSize)
	for i := 0; i < contextSize; i++ {
		state[i] = 128
	}

	c := rangecoder.NewCoder(buf)

	return c.BR(state)
}

// Resets the range coder and Golomb-Rice coder states.
func (d *Decoder) resetSliceStates(s *slice) {
	// Range coder states
	s.state = make([][][]uint8, len(d.initial_states))
	for i := 0; i < len(d.initial_states); i++ {
		s.state[i] = make([][]uint8, len(d.initial_states[i]))
		for j := 0; j < len(d.initial_states[i]); j++ {
			s.state[i][j] = make([]uint8, len(d.initial_states[i][j]))
			copy(s.state[i][j], d.initial_states[i][j])
		}
	}

	// Golomb-Rice Code states
	if d.record.coder_type == 0 {
		s.golomb_state = make([][]golomb.State, d.record.quant_table_set_count)
		for i := 0; i < len(s.golomb_state); i++ {
			s.golomb_state[i] = make([]golomb.State, d.record.context_count[i])
			for j := 0; j < len(s.golomb_state[i]); j++ {
				s.golomb_state[i][j] = golomb.NewState()
			}
		}
	}
}

func (d *Decoder) decodeSlice(buf []byte, header *internalFrame, slicenum int, frame *Frame) error {
	// Before we do anything, let's try and check the integrity
	//
	// See: * 4.8.2. error_status
	//      * 4.8.3. slice_crc_parity
	if d.record.ec == 1 {
		if header.slice_info[slicenum].error_status != 0 {
			return fmt.Errorf("error_status is non-zero: %d", header.slice_info[slicenum].error_status)
		}

		sliceBuf := buf[header.slice_info[slicenum].pos:]
		sliceBuf = sliceBuf[:header.slice_info[slicenum].size+8] // 8 bytes for footer size
		if crc32MPEG2(sliceBuf) != 0 {
			return fmt.Errorf("CRC mismatch")
		}
	}

	// If this is a keyframe, refresh states.
	//
	// See: * 3.8.1.3. Initial Values for the Context Model
	//      * 3.8.2.4. Initial Values for the VLC context state
	if header.keyframe {
		d.resetSliceStates(&header.slices[slicenum])
	}

	c := rangecoder.NewCoder(buf[header.slice_info[slicenum].pos:])

	// 4. Bitstream
	state := make([]uint8, contextSize)
	for i := 0; i < contextSize; i++ {
		state[i] = 128
	}

	// Skip keyframe bit on slice 0
	if slicenum == 0 {
		c.BR(state)
	}

	if d.record.coder_type == 2 { // Custom state transition table
		c.SetTable(d.state_transition)
	}

	d.parseSliceHeader(c, &header.slices[slicenum])

	var gc *golomb.Coder
	if d.record.coder_type == 0 {
		// We're switching to Golomb-Rice mode now so we need the bitstream
		// position.
		//
		// See: 3.8.1.1.1. Termination
		c.SentinalEnd()
		offset := c.GetPos() - 1
		gc = golomb.NewCoder(buf[header.slice_info[slicenum].pos+offset:])
	}

	// Don't worry, I fully understand how non-idiomatic and
	// ugly passing both c and gc is.
	d.decodeSliceContent(c, gc, &header.slice_info[slicenum], &header.slices[slicenum], frame)

	return nil
}
