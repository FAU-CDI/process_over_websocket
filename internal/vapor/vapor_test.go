//spellchecker:words vapor
package vapor_test

//spellchecker:words strconv sync atomic testing time github process over websocket internal vapor
import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FAU-CDI/process_over_websocket/internal/vapor"
)

func TestVapor_expire(t *testing.T) {
	closed := make(chan struct{})

	// create a vapor that just records it was closed
	vap := vapor.Vapor[CloseFunc]{
		Initialize: func(f *CloseFunc) {
			*f = func() error {
				close(closed)
				return nil
			}
		},
		Finalize: func(fr vapor.FinalizeReason, cf *CloseFunc) { cf.Close() },
		NewID: func() string {
			return "single-id"
		},
	}

	// timeout for the new element
	timeout := 100 * time.Millisecond

	// create a new element
	newElem, err := vap.New(timeout)
	if err != nil {
		t.Fatal(err)
	}

	// get the element
	if _, err := vap.Get(newElem); err != nil {
		t.Fatal(err)
	}

	select {
	case <-closed:
		/* close() called */
	case <-time.After(2 * timeout):
		t.Fatal("Close() not called after large timeout")
	}
}

func TestVapor_KeepAlive(t *testing.T) {
	// create a vapor that fails the test if close is called
	vap := vapor.Vapor[CloseFunc]{
		Initialize: func(cf *CloseFunc) {
			*cf = func() error {
				t.Fatal("Close() called unexpectedly")
				return nil
			}
		},
		Finalize: func(fr vapor.FinalizeReason, cf *CloseFunc) { cf.Close() },
		NewID: func() string {
			return "single-id"
		},
	}

	timeout := 100 * time.Millisecond

	elem, err := vap.New(timeout)
	if err != nil {
		panic(err)
	}

	// Keep getting the element every timeout / 2
	// so it doesn't expire
	for range 10 {
		time.Sleep(timeout / 2)
		vap.Get(elem)
	}
}

func TestVapor_Close(t *testing.T) {
	closes := make(chan struct{})

	var ids atomic.Int64 // holds the current numeric id
	vap := vapor.Vapor[CloseFunc]{
		Initialize: func(cf *CloseFunc) {
			*cf = func() error {
				closes <- struct{}{}
				return nil
			}
		},
		Finalize: func(fr vapor.FinalizeReason, cf *CloseFunc) { cf.Close() },
		NewID: func() string {
			// automatically generate different ids
			return strconv.FormatInt(ids.Add(1), 10)
		},
	}

	N := 1000

	// create N new elements which expire after a long time
	for range N {
		_, err := vap.New(time.Hour)
		if err != nil {
			t.Fatalf("failed to create: %v", err)
		}
	}

	// create a channel that closes once all the closes have been called
	done := make(chan struct{})
	go func() {
		defer close(done)

		for range N {
			<-closes
		}
	}()

	// cause all the closes to be called
	vap.Close()

	select {
	case <-time.After(time.Second):
		t.Fatal("not all closes called")
	case <-done:
		/* everything has closed */
	}

}

// CloseFunc has a Close Function
type CloseFunc func() error

func (cf CloseFunc) Close() {
	cf()
}
