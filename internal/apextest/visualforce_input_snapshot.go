package apextest

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/glade-sh/glade/internal/filestamp"
)

// Recent inputs cannot use the age-gated digest cache. Keep a bounded,
// request-local copy and compare bytes on later validation reads instead of
// repeatedly hashing the same content. Large inputs retain the normal path.
const recentVisualforceBudget = 32 << 20
const recentVisualforceFileLimit = 1 << 20
const recentVisualforceCompareChunk = 64 << 10

type recentVisualforceInput struct {
	content []byte
	digest  [sha256.Size]byte
}

var recentVisualforceBuffers = sync.Pool{New: func() any {
	return new([recentVisualforceCompareChunk]byte)
}}

func captureRecentVisualforceInput(path string, remaining int64) (recentVisualforceInput, bool, error) {
	stamp, ok := filestamp.Stat(path)
	if !ok || stamp.OlderThan(fileDigestNow(), fileDigestTrustAge) || stamp.Size < 0 || stamp.Size > recentVisualforceFileLimit || stamp.Size > remaining {
		return recentVisualforceInput{}, false, nil
	}
	f, err := os.Open(path) // #nosec G304 -- a discovered Visualforce validation input.
	if err != nil {
		return recentVisualforceInput{}, true, err
	}
	defer f.Close()
	before, ok := filestamp.Fstat(f)
	if !ok || before != stamp {
		return recentVisualforceInput{}, true, sourceSnapshotMismatch(path, [sha256.Size]byte{}, nil, errors.New("input changed while opening"))
	}
	if fileDigestReadHook != nil {
		fileDigestReadHook(path)
	}
	data := make([]byte, int(stamp.Size))
	_, err = io.ReadFull(f, data)
	if err != nil {
		return recentVisualforceInput{}, true, err
	}
	after, afterOK := filestamp.Fstat(f)
	if !afterOK || before != after || !sameVisualforcePathStamp(path, after) || int64(len(data)) != stamp.Size {
		return recentVisualforceInput{}, true, sourceSnapshotMismatch(path, [sha256.Size]byte{}, nil, errors.New("input changed while reading"))
	}
	return recentVisualforceInput{content: data, digest: sha256.Sum256(data)}, true, nil
}

// matches reads all current bytes and verifies the open file and its path still
// name the same trusted identity. Equal timestamps alone never prove equality.
func (input recentVisualforceInput) matches(path string) bool {
	f, err := os.Open(path) // #nosec G304 -- an input captured by this generation.
	if err != nil {
		return false
	}
	defer f.Close()
	before, ok := filestamp.Fstat(f)
	if !ok || before.Size != int64(len(input.content)) {
		return false
	}
	buffer := recentVisualforceBuffers.Get().(*[recentVisualforceCompareChunk]byte)
	defer recentVisualforceBuffers.Put(buffer)
	for offset := 0; offset < len(input.content); {
		n := min(len(buffer), len(input.content)-offset)
		if _, err := io.ReadFull(f, buffer[:n]); err != nil || !bytes.Equal(buffer[:n], input.content[offset:offset+n]) {
			return false
		}
		offset += n
	}
	after, afterOK := filestamp.Fstat(f)
	return afterOK && before == after && sameVisualforcePathStamp(path, after)
}

// The final path must still identify the file descriptor after reading.
func sameVisualforcePathStamp(path string, before filestamp.Stamp) bool {
	current, ok := filestamp.Stat(path)
	return ok && current == before
}
