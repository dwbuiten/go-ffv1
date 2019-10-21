// Package ffv1 implements an FFV1 Version 3 decoder based off of
// draft-ietf-cellar-ffv1.
package ffv1

import (
	"fmt"
	"sync"
)

// Decoder is a FFV1 decoder instance.
type Decoder struct {
	width            uint32
	height           uint32
	record           configRecord
	state_transition [256]uint8
	initial_states   [][][]uint8
	current_frame    internalFrame
}

// Frame contains a decoded FFV1 frame and relevant
// data about the frame.
//
// If BitDepth is 8, image data is in Buf. If it is anything else,
// image data is in Buf16.
//
// Image data consists of up to four contiguous planes, as follows:
//   - If ColorSpace is YCbCr:
//     - Plane 0 is Luma (always present)
//     - If HasChroma is true, the next two planes are Cr and Cr, subsampled by
//       ChromaSubsampleV and ChromaSubsampleH.
//     - If HasAlpha is true, the next plane is alpha.
//  - If ColorSpace is RGB:
//    - Plane 0 is Red
//    - Plane 1 is Green
//    - Plane 2 is Blue
//    - If HasAlpha is true, plane 4 is alpha.
type Frame struct {
	// Image data. Valid only when BitDepth is 8.
	Buf [][]byte
	// Image data. Valid only when BitDepth is greater than 8.
	Buf16 [][]uint16
	// Width of the frame, in pixels.
	Width uint32
	// Height of the frame, in pixels.
	Height uint32
	// BitDepth of the frame (8-16).
	BitDepth uint8
	// ColorSpace of the frame. See the ColorSpace constants.
	ColorSpace int
	// Whether or not chroma planes are present.
	HasChroma bool
	// Whether or not an alpha plane is present.
	HasAlpha bool
	// The log2 vertical chroma subampling value.
	ChromaSubsampleV uint8
	// The log2 horizontal chroma subsampling value.
	ChromaSubsampleH uint8
}

// NewDecoder creates a new FFV1 decoder instance.
//
// 'record' is the codec private data provided by the container. For
// Matroska, this is what is in CodecPrivate (adjusted for e.g. VFW
// data that may be before it). For ISOBMFF, this is the 'glbl' box.
//
// 'width' and 'height' are the frame width and height provided by
// the container.
func NewDecoder(record []byte, width uint32, height uint32) (*Decoder, error) {
	ret := new(Decoder)

	if width == 0 || height == 0 {
		return nil, fmt.Errorf("invalid dimensions: %dx%d", width, height)
	}

	ret.width = width
	ret.height = height

	err := parseConfigRecord(record, &ret.record)
	if err != nil {
		return nil, fmt.Errorf("invalid v3 configuration record: %s", err.Error())
	}

	ret.initializeStates()

	return ret, nil
}

// DecodeFrame takes a packet and decodes it to a ffv1.Frame.
//
// Slice threading is used by default, with one goroutine per
// slice.
func (d *Decoder) DecodeFrame(frame []byte) (*Frame, error) {

	// Allocate and fill frame info
	ret := new(Frame)
	ret.Width = d.width
	ret.Height = d.height
	ret.BitDepth = d.record.bits_per_raw_sample
	ret.ColorSpace = int(d.record.colorspace_type)
	ret.HasChroma = d.record.chroma_planes
	ret.HasAlpha = d.record.extra_plane
	if ret.HasChroma {
		ret.ChromaSubsampleV = d.record.log2_v_chroma_subsample
		ret.ChromaSubsampleH = d.record.log2_h_chroma_subsample
	}

	numPlanes := 1
	if d.record.chroma_planes {
		numPlanes += 2
	}
	if d.record.extra_plane {
		numPlanes++
	}

	// Hideous and temporary.
	if d.record.bits_per_raw_sample == 8 {
		ret.Buf = make([][]byte, numPlanes)
		ret.Buf[0] = make([]byte, int(d.width*d.height))
		if d.record.chroma_planes {
			chromaWidth := d.width >> d.record.log2_h_chroma_subsample
			chromaHeight := d.height >> d.record.log2_v_chroma_subsample
			ret.Buf[1] = make([]byte, int(chromaWidth*chromaHeight))
			ret.Buf[2] = make([]byte, int(chromaWidth*chromaHeight))
		}
		if d.record.extra_plane {
			ret.Buf[3] = make([]byte, int(d.width*d.height))
		}
	}

	// We allocate *both* if it's 8bit RGB since I'm a terrible person and
	// I wanted to use it as a scratch space, since JPEG2000-RCT is very
	// annoyingly coded as n+1 bits, and I wanted the implementation
	// to be straightforward... RIP.
	if d.record.bits_per_raw_sample > 8 || d.record.colorspace_type == 1 {
		ret.Buf16 = make([][]uint16, numPlanes)
		ret.Buf16[0] = make([]uint16, int(d.width*d.height))
		if d.record.chroma_planes {
			chromaWidth := d.width >> d.record.log2_h_chroma_subsample
			chromaHeight := d.height >> d.record.log2_v_chroma_subsample
			ret.Buf16[1] = make([]uint16, int(chromaWidth*chromaHeight))
			ret.Buf16[2] = make([]uint16, int(chromaWidth*chromaHeight))
		}
		if d.record.extra_plane {
			ret.Buf16[3] = make([]uint16, int(d.width*d.height))
		}
	}

	// We parse the frame's keyframe info outside the slice decoding
	// loop so we know ahead of time if each slice has to refresh its
	// states or not. This allows easy slice threading.
	d.current_frame.keyframe = isKeyframe(frame)

	// We parse all the footers ahead of time too, for the same reason.
	// It allows us to know all the slice positions and sizes.
	//
	// See: 9.1.1. Multi-threading Support and Independence of Slices
	err := d.parseFooters(frame, &d.current_frame)
	if err != nil {
		return nil, fmt.Errorf("invalid frame footer: %s", err.Error())
	}

	// Slice threading lazymode
	errs := make([]error, len(d.current_frame.slices))
	wg := new(sync.WaitGroup)
	for i := 0; i < len(d.current_frame.slices); i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, errs []error, n int) {
			errs[n] = d.decodeSlice(frame, &d.current_frame, n, ret)
			wg.Done()
		}(wg, errs, i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("slice %d failed: %s", i, err.Error())
		}
	}

	// Delete the scratch buffer, if needed, as per above.
	if d.record.bits_per_raw_sample == 8 && d.record.colorspace_type == 1 {
		ret.Buf16 = nil
	}

	return ret, nil
}
