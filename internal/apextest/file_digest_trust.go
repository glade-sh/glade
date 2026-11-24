package apextest

import (
	"crypto/sha256"
	"os"
	"sync"
	"sync/atomic"
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
	fileDigestReadHook atomic.Pointer[func(string)]
)

// SetFileDigestReadHookForTesting observes every physical read a source
// digest validation performs. The returned function restores the previous
// hook. Cross-package tests use it to prove per-request read counts.
func SetFileDigestReadHookForTesting(hook func(path string)) func() {
	var next *func(string)
	if hook != nil {
		next = &hook
	}
	previous := fileDigestReadHook.Swap(next)
	return func() { fileDigestReadHook.Store(previous) }
}

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
	digest, _, ok := trustedCurrentFileDigestStamped(path)
	return digest, ok
}

// trustedCurrentFileDigestStamped also reports the trusted identity the
// digest was reused under.
func trustedCurrentFileDigestStamped(path string) (digest [sha256.Size]byte, stamp filestamp.Stamp, ok bool) {
	fileDigestTrustMu.Lock()
	entry, hit := fileDigestTrust[path]
	fileDigestTrustMu.Unlock()
	if !hit {
		return [sha256.Size]byte{}, filestamp.Stamp{}, false
	}
	if current, stamped := filestamp.Stat(path); stamped && entry.stamp == current {
		return entry.digest, current, true
	}
	return [sha256.Size]byte{}, filestamp.Stamp{}, false
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
	digest, err, _, _ := currentFileDigestStamped(path)
	return digest, err
}

// currentFileDigestStamped also reports the stat stamp proven unchanged
// across the read, so a request memo can reuse the digest while the same
// stamp is still on disk. stamped is false when the read failed or the file
// could not be proven unchanged; such a digest must never be memoized.
func currentFileDigestStamped(path string) ([sha256.Size]byte, error, filestamp.Stamp, bool) {
	if trusted, stamp, ok := trustedCurrentFileDigestStamped(path); ok {
		return trusted, nil, stamp, true
	}
	file, err := os.Open(path) // #nosec G304 -- indexed source or its fixed metadata companion.
	if err != nil {
		return [sha256.Size]byte{}, err, filestamp.Stamp{}, false
	}
	defer file.Close()
	if hook := fileDigestReadHook.Load(); hook != nil {
		(*hook)(path)
	}
	hashedAt := fileDigestNow()
	before, beforeOK := filestamp.Fstat(file)
	hash := sha256.New()
	if _, err := filestamp.CopyPooled(hash, file); err != nil {
		return [sha256.Size]byte{}, err, filestamp.Stamp{}, false
	}
	var digest [sha256.Size]byte
	hash.Sum(digest[:0])
	after, afterOK := filestamp.Fstat(file)
	if !beforeOK || !afterOK || before != after {
		return digest, nil, filestamp.Stamp{}, false
	}
	if after.OlderThan(hashedAt, fileDigestTrustAge) {
		fileDigestTrustMu.Lock()
		if len(fileDigestTrust) >= fileDigestTrustMaxEntries {
			clear(fileDigestTrust)
		}
		fileDigestTrust[path] = trustedFileDigest{stamp: after, digest: digest}
		fileDigestTrustMu.Unlock()
	}
	return digest, nil, after, true
}

// requestFileDigests remembers, for one request, the stat stamp and digest of
// every source file already validated. Repeated validation boundaries inside
// one request reuse the recorded digest while the stamp still matches, so a
// racily clean file is read and hashed once per request instead of once per
// boundary. A save between boundaries changes the stamp and forces a fresh
// read, so mid-request edits are detected exactly as before, and entries
// never outlive the request that recorded them.
type requestFileDigests struct {
	mu      sync.Mutex
	entries map[string]requestFileDigestEntry
}

type requestFileDigestEntry struct {
	stamp  filestamp.Stamp
	digest [sha256.Size]byte
}

// digest returns the SHA-256 of path's current content, reusing this
// request's earlier read while the file reports the same unchanged stamp.
func (r *requestFileDigests) digest(path string) ([sha256.Size]byte, error) {
	if r == nil {
		return currentFileDigest(path)
	}
	r.mu.Lock()
	entry, hit := r.entries[path]
	r.mu.Unlock()
	if hit {
		if stamp, ok := filestamp.Stat(path); ok && entry.stamp == stamp {
			return entry.digest, nil
		}
	}
	digest, err, stamp, stamped := currentFileDigestStamped(path)
	if stamped && err == nil {
		r.mu.Lock()
		if r.entries == nil {
			r.entries = make(map[string]requestFileDigestEntry)
		}
		r.entries[path] = requestFileDigestEntry{stamp: stamp, digest: digest}
		r.mu.Unlock()
	}
	return digest, err
}
