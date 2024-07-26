package omap

import (
	"bytes"
	"encoding/json"
	"errors"
)

// MarshalJSON marshals the OrderedMap back into a map
func (om OrderedMap) MarshalJSON() ([]byte, error) {
	if om == nil {
		return []byte(`null`), nil
	}
	if len(om) == 0 {
		return []byte(`{}`), nil
	}

	var buffer bytes.Buffer
	buffer.WriteRune('{')

	last := len(om) - 1
	for i, entry := range om {
		key, err := json.Marshal(entry.Key)
		if err != nil {
			return nil, err
		}

		buffer.Write(key)
		buffer.WriteRune(':')
		buffer.Write(entry.Value)

		if i != last {
			buffer.WriteRune(',')
		}
	}
	buffer.WriteRune('}')

	return buffer.Bytes(), nil
}

var errWantObjectStart = errors.New("expected object start")

// UnmarshalJSON unmarshals data into a byte
func (om *OrderedMap) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))

	t, err := d.Token()
	if err != nil {
		return err
	}
	if t != json.Delim('{') {
		return errWantObjectStart
	}

	// create an empty object
	*om = make([]omEntry, 0)

	for {
		// read the next token
		tok, err := d.Token()
		if err != nil {
			return err
		}

		// end of object
		if tok == json.Delim('}') {
			return nil
		}

		var entry omEntry
		entry.Key = tok.(string)

		// read the raw value
		start := d.InputOffset()
		if err := readValue(d); err != nil {
			return err
		}
		end := d.InputOffset()
		entry.Value = parseValue(data, start, end)

		// and append it to the entries
		*om = append(*om, entry)
	}
}

// parseValue parses the actual raw value starting at start, and ending at end
func parseValue(data []byte, start int64, end int64) json.RawMessage {
	value := data[start:end]

	index := bytes.IndexRune(value, ':')
	if index < 0 {
		return nil
	}
	return bytes.Clone(value[index+1:])
}

var errObjectEnded = errors.New("object or array ended unexpectedly")

// readValue reads a value from d, and returns the start and end of it
func readValue(d *json.Decoder) (err error) {
	t, err := d.Token()
	if err != nil {
		return err
	}

	switch t {
	case json.Delim('['), json.Delim('{'):
		for {
			if err := readValue(d); err != nil {
				if err == errObjectEnded {
					break
				}
				return err
			}
		}
	case json.Delim(']'), json.Delim('}'):
		return errObjectEnded
	}

	return nil
}
