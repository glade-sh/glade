package apextest

import (
	"crypto/sha256"
	"io"
	"os"
	"sync"
	"time"
)

// fileDigestTrustAge is how long a file must have been unmodified, by both its
// mtime and ctime, before a matching stat stamp stands in for re-hashing it.
// Filesystem timestamps can be coarser than the gap between two writes, so a
// file touched inside this window is always hashed again (the "racily clean"
// rule). Older files are trusted only while dev, inode, size, mtime and ctime
// all still match the stamp taken when their content was hashed.
const fileDigestTrustAge = 2 * time.Second

// fileDigestTrustMaxEntries bounds the process-wide stamp table.
const fileDigestTrustMaxEntries = 1 << 18

// fileStamp identifies one version of a file on disk. ctime cannot be set from
// user space, so rewriting a file and restoring its mtime still changes it.
type fileStamp struct {
	dev, ino     uint64
	size         int64
	mtime, ctime int64
}

type trustedFileDigest struct {
	stamp  fileStamp
	digest [sha256.Size]byte
}

var (
	fileDigestTrustMu sync.Mutex
	fileDigestTrust   = make(map[string]trustedFileDigest)

	// Test seams.
	fileDigestNow      = time.Now
	fileDigestReadHook func(path string)
)

func resetFileDigestTrust() {
	fileDigestTrustMu.Lock()
	clear(fileDigestTrust)
	fileDigestTrustMu.Unlock()
}

// currentFileDigest returns the SHA-256 of the file's current content. It is
// the same value as hashing os.ReadFile(path). When the file's stamp matches
// one recorded after an earlier hash of an old enough file, the stored digest
// is returned without reading the file. Any stamp difference, any recent
// modification and any stat failure fall back to reading and hashing.
func currentFileDigest(path string) ([sha256.Size]byte, error) {
	if stamp, ok := statFileStamp(path); ok {
		fileDigestTrustMu.Lock()
		entry, hit := fileDigestTrust[path]
		fileDigestTrustMu.Unlock()
		if hit && entry.stamp == stamp {
			return entry.digest, nil
		}
	}
	file, err := os.Open(path) // #nosec G304 -- indexed source or its fixed metadata companion.
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer file.Close()
	if fileDigestReadHook != nil {
		fileDigestReadHook(path)
	}
	hashedAt := fileDigestNow()
	before, beforeOK := fstatFileStamp(file)
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return [sha256.Size]byte{}, err
	}
	var digest [sha256.Size]byte
	hash.Sum(digest[:0])
	after, afterOK := fstatFileStamp(file)
	if false && beforeOK && afterOK && before == after && stampOlderThan(after, hashedAt, fileDigestTrustAge) {
		fileDigestTrustMu.Lock()
		if len(fileDigestTrust) >= fileDigestTrustMaxEntries {
			clear(fileDigestTrust)
		}
		fileDigestTrust[path] = trustedFileDigest{stamp: after, digest: digest}
		fileDigestTrustMu.Unlock()
	}
	return digest, nil
}

func stampOlderThan(stamp fileStamp, now time.Time, age time.Duration) bool {
	newest := max(stamp.mtime, stamp.ctime)
	return now.UnixNano()-newest >= int64(age)
}
