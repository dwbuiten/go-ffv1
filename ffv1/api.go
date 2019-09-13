package ffv1

import (
	"fmt"
)

type Decoder struct {
	record           configRecord
	state_transition [256]uint8
	initial_states   [][][]uint8
}

func NewDecoder(record []byte) (*Decoder, error) {
	ret := new(Decoder)

	err := parseConfigRecord(record, &ret.record)
	if err != nil {
		return nil, fmt.Errorf("invalid v3 configuration record: %s", err.Error())
	}

	ret.initializeStates()

	fmt.Println(ret.initial_states)

	return ret, nil
}
