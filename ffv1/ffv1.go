package ffv1

import (
//    "fmt"
)

type Decoder struct {
	record configRecord
}

func NewDecoder(record []byte) (*Decoder, error) {
	ret := new(Decoder)

	/*parsedRecord, err := parseConfigRecord(record)
	  if err != nil {
	      return nil, fmt.Errorf("invalid v3 configuration record: %s", err.Error())
	  }
	  ret.record = parsedRecord*/

	return ret, nil
}
