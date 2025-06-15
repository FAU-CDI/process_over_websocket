// Package finbuf implements [Buffer]
//
//spellchecker:words finbuf
package finbuf

//spellchecker:words sync github pkglib ringbuffer status
import (
	"fmt"
	"sync"

	"github.com/tkw1536/pkglib/ringbuffer"
	"github.com/tkw1536/pkglib/status"
)

// FiniteBuffer is an [io.Writer] that contains a maximal number of lines.
// Do not copy a non-zero LineBuffer.
//
// LineBuffer is safe for concurrent read and write access.
type FiniteBuffer struct {
	m   sync.RWMutex
	buf status.LineBuffer

	doInit sync.Once

	MaxLines int
	lines    *ringbuffer.RingBuffer[string]
}

// init ensures that the FiniteBuffer is initialized.
func (fb *FiniteBuffer) init() {
	fb.doInit.Do(func() {
		fb.buf.Line = fb.line

		// prepare the line array
		fb.lines = ringbuffer.MakeRingBuffer[string](fb.MaxLines)
	})
}

func (fb *FiniteBuffer) line(line string) {
	// NOTE: This function can only be called as fb.buf
	// That is set up by fb.init().
	// Hence we don't need to call fb.init() again.

	fb.m.Lock()
	defer fb.m.Unlock()

	// todo: set max line length
	fb.lines.Add(line)
}

func (fb *FiniteBuffer) Write(data []byte) (int, error) {
	fb.init()

	n, err := fb.buf.Write(data)
	if err != nil {
		return n, fmt.Errorf("failed to read from buffer: %w", err)
	}
	return n, nil
}

// String returns a copy of the lines contained in the buffer.
func (fb *FiniteBuffer) String() string {
	fb.init()

	fb.m.RLock()
	defer fb.m.RUnlock()

	return ringbuffer.Join(fb.lines, "\n")
}
