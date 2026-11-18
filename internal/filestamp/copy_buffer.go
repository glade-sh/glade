package filestamp

import (
	"io"
	"sync"
)

// copyBufferSize matches the buffer io.Copy allocates per call.
const copyBufferSize = 32 * 1024

var copyBuffers = sync.Pool{New: func() any {
	buf := make([]byte, copyBufferSize)
	return &buf
}}

// plainReader hides every method of the wrapped reader except Read. An
// *os.File implements io.WriterTo, and io.CopyBuffer then ignores the buffer it
// was given and allocates its own.
type plainReader struct{ io.Reader }

// CopyPooled copies src to dst with a buffer borrowed from a shared pool, so
// hashing many files does not allocate one 32 KiB buffer per file. dst must
// not retain the slices it is given, as a hash.Hash does not. The result is
// the same bytes written as io.Copy(dst, src).
func CopyPooled(dst io.Writer, src io.Reader) (int64, error) {
	bufp := copyBuffers.Get().(*[]byte)
	defer copyBuffers.Put(bufp)
	return io.CopyBuffer(writerOnly{dst}, plainReader{src}, *bufp)
}

// writerOnly hides a destination's io.ReaderFrom for the same reason.
type writerOnly struct{ io.Writer }
