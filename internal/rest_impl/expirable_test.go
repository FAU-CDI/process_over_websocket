package rest_impl_test

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FAU-CDI/process_over_websocket/internal/rest_impl"
)

func TestExpireAble_expire(t *testing.T) {
	closed := make(chan struct{})

	// create an expirable that just records it was closed
	exp := rest_impl.ExpireAble[CloseFunc]{
		Make: func() CloseFunc {
			return func() error {
				close(closed)
				return nil
			}
		},
		MakeID: func() string {
			return "single-id"
		},
	}

	// timeout for the new element
	timeout := 100 * time.Millisecond

	// create a new element
	newElem, err := exp.New(timeout)
	if err != nil {
		t.Fatal(err)
	}

	// get the element
	if _, err := exp.Get(newElem); err != nil {
		t.Fatal(err)
	}

	select {
	case <-closed:
		/* close() called */
	case <-time.After(2 * timeout):
		t.Fatal("Close() not called after large timeout")
	}
}

func TestExpireAble_KeepAlive(t *testing.T) {
	// create an expirable that fails the test if close is called
	exp := rest_impl.ExpireAble[CloseFunc]{
		Make: func() CloseFunc {
			return func() error {
				t.Fatal("Close() called unexpectedly")
				return nil
			}
		},
		MakeID: func() string {
			return "single-id"
		},
	}

	timeout := 100 * time.Millisecond

	elem, err := exp.New(timeout)
	if err != nil {
		panic(err)
	}

	// Keep getting the element every timeout / 2
	// so it doesn't expire
	for range 10 {
		time.Sleep(timeout / 2)
		exp.Get(elem)
	}
}

func TestExpireAble_Close(t *testing.T) {
	closes := make(chan struct{})
	var ids atomic.Int64 // holds the current numeric id
	exp := rest_impl.ExpireAble[CloseFunc]{
		Make: func() CloseFunc {
			return func() error {
				closes <- struct{}{}
				return nil
			}
		},
		MakeID: func() string {
			// automatically generate different ids
			return strconv.FormatInt(ids.Add(1), 10)
		},
	}

	N := 1000

	// create N new elements which expire after a long time
	for range N {
		_, err := exp.New(time.Hour)
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
	exp.Close()

	select {
	case <-time.After(time.Second):
		t.Fatal("not all closes called")
	case <-done:
		/* everything has closed */
	}

}

// CloseFunc implements io.Closer
type CloseFunc func() error

func (cf CloseFunc) Close() error {
	return cf()
}
