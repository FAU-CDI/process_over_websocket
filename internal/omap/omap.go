//spellchecker:words omap
package omap

//spellchecker:words encoding json
import (
	"encoding/json"
)

// OrderedMap is like map[string]json.RawMessage, but maintains order.
type OrderedMap []omEntry

type omEntry struct {
	Key   string
	Value json.RawMessage
}

// Set sets the OrderedMap entry to the given value.
func (om *OrderedMap) Set(k string, value json.RawMessage) {
	for index, entry := range *om {
		if entry.Key == k {
			(*om)[index].Value = value
			return
		}
	}
	*om = append(*om, omEntry{Key: k, Value: value})
}

// Get gets the entry with the given value.
func (om *OrderedMap) Get(key string) (value json.RawMessage, ok bool) {
	for _, entry := range *om {
		if entry.Key == key {
			return entry.Value, true
		}
	}
	return nil, false
}
