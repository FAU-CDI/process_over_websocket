//spellchecker:words rest impl
package rest_impl

//spellchecker:words context errors sync time github jellydator ttlcache pkglib lazy
import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/tkw1536/pkglib/lazy"
)

// Vapor provides concurrent access to objects that can expire
type Vapor[T io.Closer] struct {
	NewID func() string
	Make  func() T

	init sync.Once

	startStopM sync.Mutex // protects starting and stopping
	started    bool       // is the background task started?

	cache *ttlcache.Cache[string, *entry[T]] // holds the actual items
}

type entry[T io.Closer] struct {
	value lazy.Lazy[T]
}

// newEntry creates a new entry within the underlying cache and returns it
func (vap *Vapor[T]) newEntry(d time.Duration) (string, *ttlcache.Item[string, *entry[T]], error) {
	vap.start()

	for range maxNewIDCalls {
		id := vap.NewID()
		if id == "" { // an empty id is invalid
			continue
		}

		entry, found := vap.cache.GetOrSet(id, &entry[T]{}, ttlcache.WithTTL[string, *entry[T]](d), ttlcache.WithDisableTouchOnHit[string, *entry[T]]())
		if found {
			continue
		}
		return id, entry, nil
	}
	return "", nil, errNoID
}

// initItem initializes the given item, returning the actual value
func (vap *Vapor[T]) initItem(item *ttlcache.Item[string, *entry[T]]) T {
	return item.Value().value.Get(vap.Make)
}

// vap ensures that the vapor is in started state
func (vap *Vapor[T]) start() {
	vap.init.Do(func() {
		// create a new cache which closes items on eviction
		vap.cache = ttlcache.New[string, *entry[T]]()
		vap.cache.OnEviction(func(ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, *entry[T]]) {
			vap.initItem(i).Close()
		})
	})

	vap.startStopM.Lock()
	defer vap.startStopM.Unlock()

	if vap.started {
		return
	}

	vap.started = true
	go vap.cache.Start()
}

// vap ensures that the vapor is in stopped state
func (vap *Vapor[T]) stop() {
	vap.startStopM.Lock()
	defer vap.startStopM.Unlock()

	if !vap.started {
		return
	}

	vap.cache.DeleteAll()

	vap.cache.Stop()
	vap.started = false
}

const maxNewIDCalls = 10_000

var errNoID = errors.New("MakeID did not produce a sufficiently unique ID")

// New reserves space for a new element within this vapor, and returns the id of the new element.
// The element is automatically removed from this vapor after time d.
//
// NOTE: The new element is not fully initialized until the Get method is called.
func (vap *Vapor[T]) New(d time.Duration) (string, error) {
	id, _, err := vap.newEntry(d)
	return id, err
}

// GetNew is like a call to New, followed by a call to Get with the returned id.
func (exp *Vapor[T]) GetNew(d time.Duration) (T, error) {
	_, entry, err := exp.newEntry(d)
	if err != nil {
		var zero T
		return zero, err
	}

	return exp.initItem(entry), nil
}

var errNotFound = errors.New("Get: ID not found (is it expired?)")

// Get returns the element with the given id from the vapor.
// It extends the duration until element expires by it's respective duration.
func (exp *Vapor[T]) Get(id string) (T, error) {
	exp.start()

	item := exp.cache.Get(id)
	if item == nil {
		var zero T
		return zero, errNotFound
	}

	return exp.initItem(item), nil
}

// Expire expires the given item with the provided id.
func (vap *Vapor[T]) Expire(id string) {
	vap.start()

	vap.cache.Delete(id)
}

// Close expires all items.
func (vap *Vapor[T]) Close() {
	vap.stop()
}
