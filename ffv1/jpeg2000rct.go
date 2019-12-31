package ffv1

// Converts one line from 9-bit JPEG2000-RCT to planar GBR.
//
// See: 3.7.2. RGB
func rct8(dst [][]byte, src [][]uint16, w int, h int, stride int, offset int) {
	Y := src[0][offset:]
	Cb := src[1][offset:]
	Cr := src[2][offset:]
	G := dst[0][offset:]
	B := dst[1][offset:]
	R := dst[2][offset:]
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			Cbtmp := int32(Cb[(y*stride)+x]) - (1 << 8) // Missing from spec
			Crtmp := int32(Cr[(y*stride)+x]) - (1 << 8) // Missing from spec
			g := int32(Y[(y*stride)+x]) - ((int32(Cbtmp) + int32(Crtmp)) >> 2)
			r := int32(Crtmp) + g
			b := int32(Cbtmp) + g
			G[(y*stride)+x] = byte(g)
			B[(y*stride)+x] = byte(b)
			R[(y*stride)+x] = byte(r)
		}
	}
	if len(src) == 4 {
		s := src[3][offset:]
		d := dst[3][offset:]
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				d[(y*stride)+x] = byte(s[(y*stride)+x])
			}
		}
	}
}

// Converts one line from 10 to 16 bit JPEG2000-RCT to planar GBR, in place.
//
// See: 3.7.2. RGB
func rctMid(src [][]uint16, w int, h int, stride int, offset int, bits uint) {
	Y := src[0][offset:]
	Cb := src[1][offset:]
	Cr := src[2][offset:]
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			Cbtmp := int32(Cb[(y*stride)+x]) - int32(1<<bits) // Missing from spec
			Crtmp := int32(Cr[(y*stride)+x]) - int32(1<<bits) // Missing from spec
			b := int32(Y[(y*stride)+x]) - ((int32(Cbtmp) + int32(Crtmp)) >> 2)
			r := int32(Crtmp) + b
			g := int32(Cbtmp) + b
			Y[(y*stride)+x] = uint16(g)
			Cb[(y*stride)+x] = uint16(b)
			Cr[(y*stride)+x] = uint16(r)
		}
	}
}

// Converts one line from 17-bit JPEG2000-RCT to planar GBR, in place.
//
// Currently unused until I refactor and allow for 17-bit buffers.
//
// See: 3.7.2. RGB
func rct16(dst [][]uint16, src [][]uint32, w int, h int, stride int, offset int) {
	Y := src[0][offset:]
	Cb := src[1][offset:]
	Cr := src[2][offset:]
	G := dst[0][offset:]
	B := dst[1][offset:]
	R := dst[2][offset:]
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			Cbtmp := int32(Cb[(y*stride)+x]) - (1 << 16) // Missing from spec
			Crtmp := int32(Cr[(y*stride)+x]) - (1 << 16) // Missing from spec
			g := int32(Y[(y*stride)+x]) - ((int32(Cbtmp) + int32(Crtmp)) >> 2)
			r := int32(Crtmp) + g
			b := int32(Cbtmp) + g
			G[(y*stride)+x] = uint16(g)
			B[(y*stride)+x] = uint16(b)
			R[(y*stride)+x] = uint16(r)
		}
	}
	if len(src) == 4 {
		s := src[3][offset:]
		d := dst[3][offset:]
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				d[(y*stride)+x] = uint16(s[(y*stride)+x])
			}
		}
	}
}
