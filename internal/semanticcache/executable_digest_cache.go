package semanticcache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/glade-sh/glade/internal/filestamp"
)

// The build fingerprint is the SHA-256 of the running executable. Hashing a
// large binary costs hundreds of milliseconds, and every glade process pays
// it. The digest is therefore remembered on disk under the executable's stamp
// (device, inode, size, mtime, ctime), which changes whenever the file is
// rewritten, replaced or retargeted. A file modified within
// executableDigestTrustAge of being hashed is never remembered, so a stamp
// that a coarse filesystem clock could not tell apart from a later write never
// stands in for content.
const (
	executableDigestTrustAge  = 2 * time.Second
	executableDigestEntryTTL  = 30 * 24 * time.Hour
	executableDigestPruneScan = 512
)

// executableHashHook is a test seam that fires once per full hash.
var executableHashHook func()

func executableDigestCacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, "glade", "executable-digest")
}

// executableDigest returns the lowercase hex SHA-256 of the open file's
// content. cacheDir may be empty or unusable; the result is then the plain
// digest. A cache entry only ever substitutes for hashing the exact stamped
// file it was written for.
func executableDigest(file *os.File, cacheDir string, now func() time.Time) (string, error) {
	before, stamped := filestamp.Fstat(file)
	var entry string
	if stamped && cacheDir != "" {
		entry = filepath.Join(cacheDir, executableStampName(before))
		if digest, ok := readExecutableDigestEntry(cacheDir, entry); ok {
			return digest, nil
		}
	}
	if executableHashHook != nil {
		executableHashHook()
	}
	hashedAt := now()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if entry != "" {
		if after, ok := filestamp.Fstat(file); ok && after == before && after.OlderThan(hashedAt, executableDigestTrustAge) {
			writeExecutableDigestEntry(cacheDir, entry, digest)
		}
	}
	return digest, nil
}

func executableStampName(s filestamp.Stamp) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("v1\x00%d\x00%d\x00%d\x00%d\x00%d", s.Dev, s.Ino, s.Size, s.Mtime, s.Ctime)))
	return hex.EncodeToString(sum[:16])
}

func readExecutableDigestEntry(dir, path string) (string, bool) {
	if !privateDigestDir(dir) {
		return "", false
	}
	data, err := os.ReadFile(path) // #nosec G304 -- fixed cache directory, stamp-derived name.
	if err != nil || !validDigestHex(data) {
		return "", false
	}
	return string(data), true
}

func validDigestHex(data []byte) bool {
	if len(data) != sha256.Size*2 {
		return false
	}
	for _, c := range data {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// writeExecutableDigestEntry is best effort: a failure only costs a re-hash.
func writeExecutableDigestEntry(dir, entry, digest string) {
	if err := os.MkdirAll(dir, 0o700); err != nil || !privateDigestDir(dir) {
		return
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return
	}
	_, werr := tmp.WriteString(digest)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), entry) != nil {
		_ = os.Remove(tmp.Name())
		return
	}
	pruneExecutableDigestEntries(dir)
}

func pruneExecutableDigestEntries(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-executableDigestEntryTTL)
	for i, e := range entries {
		if i >= executableDigestPruneScan {
			return
		}
		if info, err := e.Info(); err == nil && info.Mode().IsRegular() && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
