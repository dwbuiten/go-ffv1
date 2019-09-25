package ffv1

import (
	"fmt"
)

type Decoder struct {
	width            uint32
	height           uint32
	record           configRecord
	state_transition [256]uint8
	initial_states   [][][]uint8
	current_frame    internalFrame
}

type Frame struct {
	Buf    [][]byte
	Width  uint32
	Height uint32
}

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

func (d *Decoder) DecodeFrame(frame []byte) (*Frame, error) {
	err := d.parseFooters(frame, &d.current_frame)
	if err != nil {
		return nil, fmt.Errorf("invalid frame footer: %s", err.Error())
	}

	err = d.decodeSlice(frame, &d.current_frame, 0)
	if err != nil {
		return nil, fmt.Errorf("could not decode slice 0: %s", err.Error())
	}

	return &Frame{nil, d.width, d.height}, nil
}
