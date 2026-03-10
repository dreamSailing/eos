package serve

import (
	"bytes"
	"encoding/json"
)

func decodeJSONLine(line []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	return dec.Decode(dst)
}

func decodeParams(raw json.RawMessage, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(dst)
}

