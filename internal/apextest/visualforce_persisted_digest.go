package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/glade-sh/glade/internal/filestamp"
	"github.com/glade-sh/glade/internal/semanticcache"
	"github.com/glade-sh/glade/internal/startupcache"
)

// Persist only digests already admitted by the age-gated file stamp cache.
// The table is an optimization: an absent, damaged or unsafe file is a miss.
const visualforceDigestCacheLimit = 8 << 20

type visualforceDigestEntry struct {
	Path   string
	Stamp  filestamp.Stamp
	Digest []byte
}

type visualforceDigestEnvelope struct {
	Version int
	Payload json.RawMessage
	SHA256  [sha256.Size]byte
}

func visualforceDigestCachePath(root string) string {
	return filepath.Join(root, startupcache.SubdirTest, "visualforce-input-digests.json")
}

func loadVisualforceDigestCache(root string, paths []string) error {
	path := visualforceDigestCachePath(root)
	if !semanticcache.DigestDirectoryIsPrivate(root) || !semanticcache.DigestDirectoryIsPrivate(filepath.Dir(path)) {
		return nil
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	file, err := os.Open(path) // #nosec G304 -- fixed project-local cache path.
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, visualforceDigestCacheLimit+1))
	if err != nil {
		return err
	}
	if len(data) > visualforceDigestCacheLimit {
		return errors.New("visualforce digest cache is too large")
	}
	var envelope visualforceDigestEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if envelope.Version != 1 || sha256.Sum256(envelope.Payload) != envelope.SHA256 {
		return errors.New("visualforce digest cache checksum mismatch")
	}
	var entries []visualforceDigestEntry
	if err := json.Unmarshal(envelope.Payload, &entries); err != nil {
		return err
	}
	for _, entry := range entries {
		if len(entry.Digest) != sha256.Size {
			return errors.New("visualforce digest cache contains an invalid digest")
		}
	}
	wanted := make(map[string]bool, len(paths))
	for _, path := range paths {
		wanted[path] = true
	}
	now := fileDigestNow()
	fileDigestTrustMu.Lock()
	defer fileDigestTrustMu.Unlock()
	for _, entry := range entries {
		if wanted[entry.Path] && entry.Stamp.OlderThan(now, fileDigestTrustAge) && len(fileDigestTrust) < fileDigestTrustMaxEntries {
			var digest [sha256.Size]byte
			copy(digest[:], entry.Digest)
			fileDigestTrust[entry.Path] = trustedFileDigest{stamp: entry.Stamp, digest: digest}
		}
	}
	return nil
}

func saveVisualforceDigestCache(root string, paths []string) error {
	// The project root and cache directory must be owned by this user and not
	// writable by other accounts, as for executable digest entries.
	if !semanticcache.DigestDirectoryIsPrivate(root) {
		return nil
	}
	path := visualforceDigestCachePath(root)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if !semanticcache.DigestDirectoryIsPrivate(dir) {
		return nil
	}
	entries := make([]visualforceDigestEntry, 0, min(len(paths), 1<<16))
	pathBytes := 0
	fileDigestTrustMu.Lock()
	for _, path := range paths {
		if entry, ok := fileDigestTrust[path]; ok {
			pathBytes += len(path)
			if len(entries) >= 1<<16 || pathBytes > visualforceDigestCacheLimit/2 {
				fileDigestTrustMu.Unlock()
				return errors.New("visualforce digest cache is too large")
			}
			digest := make([]byte, sha256.Size)
			copy(digest, entry.digest[:])
			entries = append(entries, visualforceDigestEntry{Path: path, Stamp: entry.stamp, Digest: digest})
		}
	}
	fileDigestTrustMu.Unlock()
	payload, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	data, err := json.Marshal(visualforceDigestEnvelope{Version: 1, Payload: payload, SHA256: sha256.Sum256(payload)})
	if err != nil {
		return err
	}
	if len(data) > visualforceDigestCacheLimit {
		return errors.New("visualforce digest cache is too large")
	}
	tmp, err := os.CreateTemp(dir, ".visualforce-digests-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp.Name(), path)
}

// The first generation in a process can import and publish a disk table.
// Later generations use the existing process cache; stale entries are checked
// against the current stamp before use. Concurrent first callers may hash.
func firstVisualforceDigestGeneration(root string) bool {
	fileDigestTrustMu.Lock()
	defer fileDigestTrustMu.Unlock()
	if visualforceDigestRoots[root] {
		return false
	}
	if len(visualforceDigestRoots) >= 4096 {
		clear(visualforceDigestRoots)
	}
	visualforceDigestRoots[root] = true
	return true
}
