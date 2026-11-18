package apextest

import (
	"crypto/sha256"
	"os"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/filestamp"
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

type trustedFileDigest struct {
	stamp  filestamp.Stamp
	digest [sha256.Size]byte
}

var (
	fileDigestTrustMu            sync.Mutex
	fileDigestTrust              = make(map[string]trustedFileDigest)
	visualforceDigestRoots       = make(map[string]bool)
	recentVisualforceShared      = make(map[string]recentVisualforceInput)
	recentVisualforceSharedBytes int64

	// Test seams.
	fileDigestNow      = time.Now
	fileDigestReadHook func(path string)
)

func resetFileDigestTrust() {
	fileDigestTrustMu.Lock()
	clear(fileDigestTrust)
	clear(visualforceDigestRoots)
	clear(recentVisualforceShared)
	recentVisualforceSharedBytes = 0
	fileDigestTrustMu.Unlock()
}

// trustedCurrentFileDigest returns a previously hashed digest only while the
// filesystem still reports the exact trusted file identity.
func trustedCurrentFileDigest(path string) ([sha256.Size]byte, bool) {
	fileDigestTrustMu.Lock()
	entry, hit := fileDigestTrust[path]
	fileDigestTrustMu.Unlock()
	if !hit {
		return [sha256.Size]byte{}, false
	}
	if stamp, ok := filestamp.Stat(path); ok {
		if entry.stamp == stamp {
			return entry.digest, true
		}
	}
	return [sha256.Size]byte{}, false
}

// oldTrustedVisualforceDigest combines the old-file check with the existing
// trusted-digest lookup. Recent files still require byte comparison or hash.
func oldTrustedVisualforceDigest(path string) ([sha256.Size]byte, bool) {
	fileDigestTrustMu.Lock()
	entry, hit := fileDigestTrust[path]
	recent, hasRecent := recentVisualforceShared[path]
	fileDigestTrustMu.Unlock()
	if !hit && !hasRecent {
		return [sha256.Size]byte{}, false
	}
	stamp, ok := filestamp.Stat(path)
	if !ok || !stamp.OlderThan(fileDigestNow(), fileDigestTrustAge) {
		return [sha256.Size]byte{}, false
	}
	if hit && stamp == entry.stamp {
		return entry.digest, true
	}
	// A snapshot taken while the file was recent becomes trustworthy only
	// after comparing every byte and rechecking the same now-old identity.
	if !hasRecent {
		return [sha256.Size]byte{}, false
	}
	return promoteRecentVisualforceInput(path, stamp, recent)
}

// promoteRecentVisualforceInput receives a stamp already shown to be old.
// Both the bytes and the same identity must still match before promotion.
func promoteRecentVisualforceInput(path string, stamp filestamp.Stamp, recent recentVisualforceInput) ([sha256.Size]byte, bool) {
	if !stamp.OlderThan(fileDigestNow(), fileDigestTrustAge) || !recent.matches(path) || !sameVisualforcePathStamp(path, stamp) {
		return [sha256.Size]byte{}, false
	}
	fileDigestTrustMu.Lock()
	if len(fileDigestTrust) >= fileDigestTrustMaxEntries {
		clear(fileDigestTrust)
	}
	fileDigestTrust[path] = trustedFileDigest{stamp: stamp, digest: recent.digest}
	fileDigestTrustMu.Unlock()
	return recent.digest, true
}

// currentFileDigest returns the SHA-256 of the file's current content. A
// matching trusted stamp avoids rereading; all other cases hash the file.
func currentFileDigest(path string) ([sha256.Size]byte, error) {
	if digest, ok := trustedCurrentFileDigest(path); ok {
		return digest, nil
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
	before, beforeOK := filestamp.Fstat(file)
	hash := sha256.New()
	if _, err := filestamp.CopyPooled(hash, file); err != nil {
		return [sha256.Size]byte{}, err
	}
	var digest [sha256.Size]byte
	hash.Sum(digest[:0])
	after, afterOK := filestamp.Fstat(file)
	if beforeOK && afterOK && before == after && after.OlderThan(hashedAt, fileDigestTrustAge) {
		fileDigestTrustMu.Lock()
		if len(fileDigestTrust) >= fileDigestTrustMaxEntries {
			clear(fileDigestTrust)
		}
		fileDigestTrust[path] = trustedFileDigest{stamp: after, digest: digest}
		fileDigestTrustMu.Unlock()
	}
	return digest, nil
}
