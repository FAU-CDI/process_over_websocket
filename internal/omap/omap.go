package omap

import (
	"encoding/json"
)

// OrderedMap is like map[string]json.RawMessage, but maintains order
type OrderedMap []omEntry

type omEntry struct {
	Key   string
	Value json.RawMessage
}

// Set sets the OrderedMap entry to the given value
func (om *OrderedMap) Set(Key string, Value json.RawMessage) {
	for index, entry := range *om {
		if entry.Key == Key {
			(*om)[index].Value = Value
			return
		}
	}
	*om = append(*om, omEntry{Key: Key, Value: Value})
}

// Get gets the entry with the given value
func (om OrderedMap) Get(Key string) (value json.RawMessage, ok bool) {
	for _, entry := range om {
		if entry.Key == Key {
			return entry.Value, true
		}
	}
	return nil, false
}
