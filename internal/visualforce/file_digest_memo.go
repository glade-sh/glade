package visualforce

import (
	"crypto/sha256"
	"os"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/filestamp"
)

// A warm generation hit still computes the generation key, which used to read
// and hash every project file. The content digest of a file is remembered
// under its stamp (device, inode, size, mtime, ctime), so an unchanged file is
// stat-ed but not read. A digest is remembered only when the stamp did not
// change while the file was read and the file was last modified at least
// fileDigestTrustAge before the read began, so a stamp that a coarse clock
// could not tell apart from a later write never stands in for content.
const fileDigestMemoLimit = 1 << 17

// fileDigestTrustAge is a variable so tests can shrink the window; a file's
// ctime cannot be backdated.
var fileDigestTrustAge = 2 * time.Second

type memoizedDigest struct {
	stamp  filestamp.Stamp
	digest [sha256.Size]byte
}

var fileDigestMemo struct {
	sync.Mutex
	entries map[string]memoizedDigest
}

// fileDigestReadHook is a test seam that fires once per full file read.
var fileDigestReadHook func(path string)

func resetFileDigestMemo() {
	fileDigestMemo.Lock()
	fileDigestMemo.entries = nil
	fileDigestMemo.Unlock()
}

func fileContentDigest(path string) ([sha256.Size]byte, error) {
	stamp, stamped := filestamp.Stat(path)
	if stamped {
		fileDigestMemo.Lock()
		cached, ok := fileDigestMemo.entries[path]
		fileDigestMemo.Unlock()
		if ok && cached.stamp == stamp {
			return cached.digest, nil
		}
	}
	if fileDigestReadHook != nil {
		fileDigestReadHook(path)
	}
	var digest [sha256.Size]byte
	readAt := time.Now()
	file, err := os.Open(path) // #nosec G304 -- a path the loaded project names.
	if err != nil {
		return digest, err
	}
	defer file.Close()
	before, beforeOK := filestamp.Fstat(file)
	hash := sha256.New()
	if _, err := filestamp.CopyPooled(hash, file); err != nil {
		return digest, err
	}
	copy(digest[:], hash.Sum(nil))
	if stamped && beforeOK && before == stamp {
		if after, ok := filestamp.Fstat(file); ok && after == before && after.OlderThan(readAt, fileDigestTrustAge) {
			fileDigestMemo.Lock()
			if fileDigestMemo.entries == nil || len(fileDigestMemo.entries) >= fileDigestMemoLimit {
				fileDigestMemo.entries = map[string]memoizedDigest{}
			}
			fileDigestMemo.entries[path] = memoizedDigest{stamp: stamp, digest: digest}
			fileDigestMemo.Unlock()
		}
	}
	return digest, nil
}
