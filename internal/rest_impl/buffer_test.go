package rest_impl_test

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/FAU-CDI/process_over_websocket/internal/rest_impl"
)

func ExampleFiniteBuffer() {
	N := 1000

	var buffer rest_impl.FiniteBuffer
	buffer.MaxLines = 2

	// do a lot of concurrent reads and writes
	// because FiniteBuffer is concurrency-safe
	// these should not produce any race conditions
	// or errors
	{
		var wg sync.WaitGroup
		wg.Add(N)
		for range N {
			go func() {
				defer wg.Done()
				_ = buffer.String()
			}()
		}
		wg.Add(N)
		for range N {
			go func() {
				defer wg.Done()
				buffer.Write([]byte("another line\n"))
			}()
		}
		wg.Wait()
	}

	// now write lines in order
	// because the buffer
	for i := range N {
		buffer.Write([]byte(strconv.Itoa(N-i) + "\n"))
	}

	// the last two lines should be left
	fmt.Println(buffer.String())

	// Output: 2
	// 1
}