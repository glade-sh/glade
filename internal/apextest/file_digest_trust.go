package apextest

import (
	"crypto/sha256"
	"io"
	"os"
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

// SetFileDigestReadHookForTesting observes reads that compute a source digest.
// Byte comparisons that confirm an existing digest do not compute one.
// The returned function restores the previous hook.
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
	digest, err, stamp, stamped, _ := readFileDigest(path, 0)
	return digest, err, stamp, stamped
}

func readFileDigest(path string, remaining int64) ([sha256.Size]byte, error, filestamp.Stamp, bool, []byte) {
	if trusted, stamp, ok := trustedCurrentFileDigestStamped(path); ok {
		return trusted, nil, stamp, true, nil
	}
	file, err := os.Open(path) // #nosec G304 -- indexed source or its fixed metadata companion.
	if err != nil {
		return [sha256.Size]byte{}, err, filestamp.Stamp{}, false, nil
	}
	defer file.Close()
	if hook := fileDigestReadHook.Load(); hook != nil {
		(*hook)(path)
	}
	hashedAt := fileDigestNow()
	before, beforeOK := filestamp.Fstat(file)
	old := beforeOK && before.OlderThan(hashedAt, fileDigestTrustAge)
	var digest [sha256.Size]byte
	var content []byte
	if remaining > 0 && !old && beforeOK && before.Size >= 0 && before.Size <= min(remaining, recentVisualforceFileLimit) {
		content = make([]byte, int(before.Size))
		if _, err := io.ReadFull(file, content); err != nil {
			return digest, err, filestamp.Stamp{}, false, nil
		}
		digest = sha256.Sum256(content)
	} else {
		hash := sha256.New()
		if _, err := filestamp.CopyPooled(hash, file); err != nil {
			return digest, err, filestamp.Stamp{}, false, nil
		}
		hash.Sum(digest[:0])
	}
	after, afterOK := filestamp.Fstat(file)
	if !beforeOK || !afterOK || before != after {
		return digest, nil, filestamp.Stamp{}, false, nil
	}
	if old {
		fileDigestTrustMu.Lock()
		if len(fileDigestTrust) >= fileDigestTrustMaxEntries {
			clear(fileDigestTrust)
		}
		fileDigestTrust[path] = trustedFileDigest{stamp: after, digest: digest}
		fileDigestTrustMu.Unlock()
	}
	return digest, nil, after, true, content
}

// requestFileDigests validates old inputs by stamp and recent inputs by byte
// equality. Apex bodies can reuse the bytes already captured for compilation.
// Extra copies for other inputs are bounded by the snapshot budget.
type requestFileDigests struct {
	mu          sync.Mutex
	entries     map[string]requestFileDigestEntry
	sources     map[string]runtimeSourceInput
	recentBytes int64
}

type requestFileDigestEntry struct {
	stamp          filestamp.Stamp
	digest         [sha256.Size]byte
	recent         []byte
	capturedSource bool
}

// digest returns the SHA-256 of path's current content.
func (r *requestFileDigests) digest(path string) ([sha256.Size]byte, error) {
	if r == nil {
		return currentFileDigest(path)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, hit := r.entries[path]
	source, captured := r.sources[path]
	if hit {
		switch {
		case entry.capturedSource:
			if captured && source.digest == entry.digest && matchesRecentFile(path, source.raw) {
				return entry.digest, nil
			}
		case entry.recent != nil:
			if matchesRecentFile(path, entry.recent) {
				return entry.digest, nil
			}
		default:
			if stamp, ok := requestDigestStat(path); ok && entry.stamp == stamp {
				return entry.digest, nil
			}
		}
	}
	generation := fileDigestNow()
	remaining := recentVisualforceBudget - r.recentBytes + int64(len(entry.recent))
	if captured {
		remaining = 0
	}
	digest, err, stamp, stamped, content := readFileDigest(path, remaining)
	if stamped && err == nil {
		old := stamp.OlderThan(generation, fileDigestTrustAge)
		useSource := !old && captured && source.digest == digest
		if old || useSource || content != nil {
			if r.entries == nil {
				r.entries = make(map[string]requestFileDigestEntry)
			}
			r.entries[path] = requestFileDigestEntry{
				stamp: stamp, digest: digest, recent: content, capturedSource: useSource,
			}
			r.recentBytes += int64(len(content) - len(entry.recent))
		}
	}
	return digest, err
}
