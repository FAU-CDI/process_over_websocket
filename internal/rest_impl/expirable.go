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

// ExpireAble provides concurrent access to objects that can expire
type ExpireAble[T io.Closer] struct {
	MakeID func() string
	Make   func() T

	init sync.Once

	startStopM sync.Mutex // protects starting and stopping
	started    bool       // is the background task started?

	cache *ttlcache.Cache[string, *entry[T]] // holds the actual items
}

type entry[T io.Closer] struct {
	value lazy.Lazy[T]
}

func (exp *ExpireAble[T]) getEntry(entry *ttlcache.Item[string, *entry[T]]) T {
	return entry.Value().value.Get(exp.Make)
}

// start starts the background process
func (exp *ExpireAble[T]) start() {
	exp.init.Do(func() {
		// create a new cache which closes items on eviction
		exp.cache = ttlcache.New[string, *entry[T]]()
		exp.cache.OnEviction(func(ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, *entry[T]]) {
			exp.getEntry(i).Close()
		})
	})

	exp.startStopM.Lock()
	defer exp.startStopM.Unlock()

	if exp.started {
		return
	}

	exp.started = true
	go exp.cache.Start()
}

// stop stops the background process
func (exp *ExpireAble[T]) stop() {
	exp.startStopM.Lock()
	defer exp.startStopM.Unlock()

	if !exp.started {
		return
	}

	exp.cache.DeleteAll()

	exp.cache.Stop()
	exp.started = false
}

const maxNewIDCalls = 10_000

var errNoID = errors.New("MakeID did not produce a sufficiently unique ID")

// New creates a new object, and causes it to expire after timeout d.
func (exp *ExpireAble[T]) New(d time.Duration) (string, error) {
	exp.start()

	for range maxNewIDCalls {
		id := exp.MakeID()

		_, found := exp.cache.GetOrSet(id, &entry[T]{}, ttlcache.WithTTL[string, *entry[T]](d), ttlcache.WithDisableTouchOnHit[string, *entry[T]]())
		if found {
			continue
		}
		return id, nil
	}
	return "", errNoID
}

var errNotFound = errors.New("Get: ID not found (is it expired?)")

// Get returns the existing item with the given id, and extends the timeout by the given duration.
func (exp *ExpireAble[T]) Get(id string) (T, error) {
	exp.start()

	item := exp.cache.Get(id)
	if item == nil {
		var zero T
		return zero, errNotFound
	}

	return exp.getEntry(item), nil
}

// Expire expires the given item with the provided id.
func (exp *ExpireAble[T]) Expire(id string) {
	exp.start()

	exp.cache.Delete(id)
}

// Close expires all items.
func (exp *ExpireAble[T]) Close() {
	exp.stop()
}
