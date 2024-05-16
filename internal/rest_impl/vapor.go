//spellchecker:words rest impl
package rest_impl

//spellchecker:words context errors sync time github jellydator ttlcache pkglib lazy
import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

// Vapor holds elements of type *T.
//
// Each element is automatically finalized after a specific duration.
// This duration can be extended by repeatedly accessing the element.
type Vapor[T any] struct {
	// NewID is used to create elements within this vapor.
	//
	// IDs should be non-empty, unique strings.
	// Internal code within the vapor ensures that at any one time no two elements
	// receive the same ID, even if this function accidentally returns the same id.
	//
	// If NewID fails to return a new non-empty, unique id after being called an
	// unspecified amount of times, an error is returned instead of generating an
	// id itself.
	//
	// NewID might be called concurrently by multiple threads.
	NewID func() string

	// Initialize is called to initialize a new element upon being created.
	// The parameter is pointing to pointing to a new zero value of T.
	//
	// If Initialize is nil, this function will not be called and new elements will
	// simply remain the zero value.
	Initialize func(*T)

	// Finalize is called to finalize an element upon being removed from this vapor.
	// The parameter is pointing to the value, and is guaranteed to have passed through
	// the initialize function.
	//
	// If Finalize is nil, it is not called.
	// In this case Initialize may not be called on items that are evicted prior to being used.
	Finalize func(FinalizeReason, *T)

	init sync.Once

	startStopM sync.Mutex // protects starting and stopping
	started    bool       // is the background task started?

	cache *ttlcache.Cache[string, *entry[T]] // holds the actual items
}

// FinalizeReason indicates a reason why Finalize was called
type FinalizeReason uint64

const (
	FinalizeReasonDeleted FinalizeReason = iota
	FinalizeReasonExpired
)

// entry holds information about a single item
type entry[T any] struct {
	init  sync.Once
	value T
}

// maximum number of times NewID is called before producing an error.
const maxNewIDCalls = 1000

var (
	errNoID     = fmt.Errorf("Vapor: MakeID did not produce a new unique id after being called %d times", maxNewIDCalls)
	errNewIDNil = errors.New("Vapor: MakeID is nil")
)

// newEntry creates a new entry within the underlying cache and returns it
func (vap *Vapor[T]) newEntry(d time.Duration) (string, *ttlcache.Item[string, *entry[T]], error) {
	vap.start()

	if vap.NewID == nil {
		return "", nil, errNewIDNil
	}

	for range maxNewIDCalls {
		// create a new id
		id := vap.NewID()
		if id == "" {
			// that must not be empty
			continue
		}

		// check if we have the element already
		entry, found := vap.cache.GetOrSet(id, &entry[T]{}, ttlcache.WithTTL[string, *entry[T]](d), ttlcache.WithDisableTouchOnHit[string, *entry[T]]())
		if found {
			continue
		}
		return id, entry, nil
	}
	return "", nil, errNoID
}

// initItem initializes the given item, returning the actual value
func (vap *Vapor[T]) initItem(item *ttlcache.Item[string, *entry[T]]) *T {
	entry := item.Value()
	entry.init.Do(func() {
		if vap.Initialize != nil {
			vap.Initialize(&entry.value)
		}
	})
	return &entry.value
}

// vap ensures that the vapor is in started state
func (vap *Vapor[T]) start() {
	vap.init.Do(func() {
		// create a new cache which closes items on eviction
		vap.cache = ttlcache.New[string, *entry[T]]()
		vap.cache.OnEviction(func(ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, *entry[T]]) {
			if vap.Finalize == nil {
				return
			}

			// determine the reason for eveiction
			var fr FinalizeReason
			if er == ttlcache.EvictionReasonDeleted {
				fr = FinalizeReasonDeleted
			} else if er == ttlcache.EvictionReasonExpired {
				fr = FinalizeReasonExpired
			} else {
				panic("unknown eviction reason")
			}
			vap.Finalize(fr, vap.initItem(i))
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

// New reserves space for a new element within this vapor, and returns the id of the new element.
// The element is automatically removed from this vapor after time d.
//
// NOTE: The new element is not fully initialized until the Get method is called.
func (vap *Vapor[T]) New(d time.Duration) (string, error) {
	id, _, err := vap.newEntry(d)
	return id, err
}

// GetNew is like a call to New, followed by a call to Get with the returned id.
func (exp *Vapor[T]) GetNew(d time.Duration) (*T, error) {
	_, entry, err := exp.newEntry(d)
	if err != nil {
		return nil, err
	}

	return exp.initItem(entry), nil
}

var errNotFound = errors.New("Get: ID not found (is it expired?)")

// Get returns the element with the given id from the vapor.
// It extends the duration until element expires by it's respective duration.
func (exp *Vapor[T]) Get(id string) (*T, error) {
	exp.start()

	item := exp.cache.Get(id)
	if item == nil {
		return nil, errNotFound
	}

	return exp.initItem(item), nil
}

// Evict removes the item with the given id.
// If the item does not exist, this method has no effect.
func (vap *Vapor[T]) Evict(id string) {
	vap.start()

	vap.cache.Delete(id)
}

// Close expires all items.
func (vap *Vapor[T]) Close() {
	vap.stop()
}
