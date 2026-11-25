package apextest

import (
	"crypto/sha256"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glade-sh/glade/internal/filestamp"
)

// fileDigestTrustAge is how long both mtime and ctime must be unchanged
// before a matching stamp can stand in for a fresh hash across requests.
// Filesystem timestamps can be coarser than the gap between two writes.
// Older files are trusted only while dev, inode, size, mtime and ctime match
// the stamp taken when their content was hashed.
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
	fileDigestClock       atomic.Pointer[func() time.Time]
	fileDigestReadHook    atomic.Pointer[func(string)]
	requestDigestStatHook atomic.Pointer[func(string) (filestamp.Stamp, bool)]
)

func fileDigestNow() time.Time {
	if now := fileDigestClock.Load(); now != nil {
		return (*now)()
	}
	return time.Now()
}

func requestDigestStat(path string) (filestamp.Stamp, bool) {
	if stat := requestDigestStatHook.Load(); stat != nil {
		return (*stat)(path)
	}
	return filestamp.Stat(path)
}

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
// across the read. stamped is false when the read failed or the file could
// not be proven unchanged; such a digest must never be memoized.
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

// requestFileDigests is one request's view of the files behind its captured
// generation. Every validation boundary of the request asks it for a file's
// current digest, and it reads each file at most once.
//
// A file whose stamp is safely older than the read that hashed it is proven
// by that stamp: any later write moves the stamp. The read happens at the
// first boundary and later boundaries compare the stamp.
//
// A recent file is racy: a same-size write inside one timestamp tick can keep
// the full stamp, so no stamp comparison proves its bytes. A request that
// seals accepts such a file on the digest its generation captured, without
// reading, at every boundary, and seal reads it once after the last boundary.
// A write that lands between any two boundaries is therefore still in front
// of the one read that decides the request, whether or not it moved the
// stamp. A request that does not seal reads a recent file at every boundary.
type requestFileDigests struct {
	mu      sync.Mutex
	entries map[string]requestFileDigestEntry
	// seals is set before the first boundary by a request that calls seal
	// before it reports a result.
	seals  bool
	sealed bool
}

type requestFileDigestEntry struct {
	stamp  filestamp.Stamp
	digest [sha256.Size]byte
	state  requestFileDigestState
}

type requestFileDigestState uint8

const (
	// requestDigestProven: digest was read under stamp, and stamp was old
	// enough at that read for a later match to prove the same bytes.
	requestDigestProven requestFileDigestState = iota
	// requestDigestPending: digest is the captured digest of a recent file,
	// accepted without a read. seal reads the file.
	requestDigestPending
	// requestDigestSealed: seal read digest under stamp. The request is
	// decided; a later match serves only what the request publishes.
	requestDigestSealed
)

// deferRecentReads starts a validation phase that ends in seal. Call it
// before the phase's first boundary. Digests an earlier phase sealed are
// dropped: they decided that phase, not this one.
func (r *requestFileDigests) deferRecentReads() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.seals, r.sealed = true, false
	for path, entry := range r.entries {
		if entry.state == requestDigestSealed {
			delete(r.entries, path)
		}
	}
	r.mu.Unlock()
}

// digest returns the SHA-256 of path's current content.
func (r *requestFileDigests) digest(path string) ([sha256.Size]byte, error) {
	return r.resolve(path, nil)
}

// current returns the SHA-256 of path's current content for a caller that
// will compare it with captured, the digest its generation holds for path.
// In a request that seals, a recent file returns captured unread and is
// queued for seal.
func (r *requestFileDigests) current(path string, captured [sha256.Size]byte) ([sha256.Size]byte, error) {
	return r.resolve(path, &captured)
}

func (r *requestFileDigests) resolve(path string, captured *[sha256.Size]byte) ([sha256.Size]byte, error) {
	if r == nil {
		return currentFileDigest(path)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := fileDigestNow()
	stamp, statOK := requestDigestStat(path)
	entry, hit := r.entries[path]
	if hit && statOK && entry.stamp == stamp {
		if entry.state != requestDigestPending || (captured != nil && *captured == entry.digest) {
			return entry.digest, nil
		}
	}
	// A stamp this request has not seen for path is a possible write: read
	// now, so a changed file fails at this boundary. Only a file first seen
	// recent, with a captured digest to stand in, waits for seal.
	if !hit && statOK && captured != nil && r.seals && !r.sealed && !stamp.OlderThan(now, fileDigestTrustAge) {
		r.store(path, requestFileDigestEntry{stamp: stamp, digest: *captured, state: requestDigestPending})
		return *captured, nil
	}
	digest, err, readStamp, stamped := currentFileDigestStamped(path)
	if err != nil || !stamped {
		return digest, err
	}
	switch {
	case readStamp.OlderThan(now, fileDigestTrustAge):
		r.store(path, requestFileDigestEntry{stamp: readStamp, digest: digest})
	case captured != nil && digest == *captured && r.seals && !r.sealed:
		// Rewritten with the captured bytes. The new stamp is recent, so
		// this read proves nothing about later boundaries.
		r.store(path, requestFileDigestEntry{stamp: readStamp, digest: digest, state: requestDigestPending})
	}
	return digest, nil
}

func (r *requestFileDigests) store(path string, entry requestFileDigestEntry) {
	if r.entries == nil {
		r.entries = make(map[string]requestFileDigestEntry)
	}
	r.entries[path] = entry
}

// seal reads every file the request accepted on its captured digest and
// reports the first whose bytes differ. Call it after the request's last
// validation boundary and before the request reports a result. Boundaries
// that run after seal belong to work the request publishes later; they reuse
// a sealed digest while its stamp still matches.
func (r *requestFileDigests) seal() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := make([]string, 0, len(r.entries))
	for path, entry := range r.entries {
		if entry.state == requestDigestPending {
			pending = append(pending, path)
		}
	}
	sort.Strings(pending)
	for _, path := range pending {
		captured := r.entries[path].digest
		digest, err, stamp, stamped := currentFileDigestStamped(path)
		if err != nil {
			return sourceSnapshotMismatch(path, captured, nil, err)
		}
		if digest != captured {
			return sourceSnapshotMismatch(path, captured, &digest, nil)
		}
		if stamped {
			r.entries[path] = requestFileDigestEntry{stamp: stamp, digest: digest, state: requestDigestSealed}
		} else {
			delete(r.entries, path)
		}
	}
	r.sealed = true
	return nil
}
