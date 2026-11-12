package semanticcache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A persisted gate is an opaque payload that a caller computed from a content
// key (a digest of every input the computation reads). The entry is addressed
// by that key and the build fingerprint, so a different binary never reads it.
// The file carries a checksum over key, fingerprint and payload; an entry that
// is truncated, rewritten or planted fails the checksum and reads as a miss.
// The directory follows the executable digest cache rules: a real directory
// owned by this user that group and others cannot write. Writes go through a
// temporary file and an atomic rename, so concurrent processes never see a
// partial entry and the last complete writer wins with identical content.
const (
	persistedGateMagic    = "glade-gate-v1\n"
	persistedGateMaxBytes = 64 << 20
)

var persistedGateDir = defaultPersistedGateDir

func defaultPersistedGateDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, "glade", "controller-gate")
}

// SetPersistedGateDirForTesting redirects the persisted gate directory until
// the returned function is called. An empty dir disables persistence.
func SetPersistedGateDirForTesting(dir string) func() {
	previous := persistedGateDir
	persistedGateDir = func() string { return dir }
	return func() { persistedGateDir = previous }
}

func persistedGateFingerprint() (string, bool) {
	fingerprint := buildFingerprint()
	if fingerprint == "" || strings.HasPrefix(fingerprint, "process:") {
		return "", false
	}
	return fingerprint, true
}

func persistedGateSum(key, fingerprint string, payload []byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte("sum\x00" + key + "\x00" + fingerprint + "\x00"))
	_, _ = h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func persistedGatePath(dir, key, fingerprint string) string {
	name := sha256.Sum256([]byte("name\x00" + key + "\x00" + fingerprint))
	return filepath.Join(dir, hex.EncodeToString(name[:16]))
}

// LoadPersistedGate returns the payload stored for key by this build, if any
// valid entry exists. Every failure is a miss.
func LoadPersistedGate(key string) ([]byte, bool) {
	fingerprint, ok := persistedGateFingerprint()
	dir := persistedGateDir()
	if !ok || dir == "" || key == "" || !privateDigestDir(dir) {
		return nil, false
	}
	path := persistedGatePath(dir, key, fingerprint)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > persistedGateMaxBytes {
		return nil, false
	}
	data, err := os.ReadFile(path) // #nosec G304 -- fixed cache directory, key-derived name.
	if err != nil {
		return nil, false
	}
	rest, found := bytes.CutPrefix(data, []byte(persistedGateMagic))
	if !found || len(rest) < sha256.Size*2+1 || rest[sha256.Size*2] != '\n' {
		return nil, false
	}
	sum, payload := string(rest[:sha256.Size*2]), rest[sha256.Size*2+1:]
	if sum != persistedGateSum(key, fingerprint, payload) {
		return nil, false
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	return payload, true
}

// StorePersistedGate records payload for key. It is best effort: a failure
// only costs the next process a recompute.
func StorePersistedGate(key string, payload []byte) {
	fingerprint, ok := persistedGateFingerprint()
	dir := persistedGateDir()
	if !ok || dir == "" || key == "" || len(payload) > persistedGateMaxBytes {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil || !privateDigestDir(dir) {
		return
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return
	}
	var buf bytes.Buffer
	buf.WriteString(persistedGateMagic)
	buf.WriteString(persistedGateSum(key, fingerprint, payload))
	buf.WriteByte('\n')
	buf.Write(payload)
	_, werr := tmp.Write(buf.Bytes())
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), persistedGatePath(dir, key, fingerprint)) != nil {
		_ = os.Remove(tmp.Name())
		return
	}
	pruneExecutableDigestEntries(dir)
}
