package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/filestamp"
)

// Two identifiers name a runtime asset. The ETag comes from the file's stamp
// (device, inode, size, mtime, ctime) and costs one stat, so no request hashes
// a body. The content version in a shell asset URL is a hash of the bytes,
// remembered under the same stamp so an unchanged file is hashed once.
//
// A stamp is trusted only when the file last changed at least
// runtimeAssetTrustAge ago and the platform has a real ctime. Otherwise the
// asset has no ETag and the URL carries a fresh hash on every page. ctime
// cannot be set from user space, so restoring an mtime does not hide an edit.
const runtimeAssetMemoLimit = 256

var runtimeAssetTrustAge = 2 * time.Second

type runtimeAssetMemoEntry struct {
	stamp   filestamp.Stamp
	version string
}

var runtimeAssetMemo struct {
	sync.Mutex
	entries map[string]runtimeAssetMemoEntry
}

// runtimeAssetHashHook is a test seam that fires once per full-body hash.
var runtimeAssetHashHook func(file string)

func resetRuntimeAssetMemo() {
	runtimeAssetMemo.Lock()
	runtimeAssetMemo.entries = nil
	runtimeAssetMemo.Unlock()
}

func trustedRuntimeAssetStamp(file string) (filestamp.Stamp, bool) {
	stamp, ok := filestamp.Stat(file)
	return stamp, ok && stamp.OlderThan(time.Now(), runtimeAssetTrustAge)
}

// runtimeAssetETag returns a weak validator for the file, or "" when its stamp
// cannot be trusted.
func runtimeAssetETag(file string) string {
	stamp, ok := trustedRuntimeAssetStamp(file)
	if !ok {
		return ""
	}
	return fmt.Sprintf("W/\"%x-%x-%x-%x-%x\"", stamp.Dev, stamp.Ino, stamp.Size, stamp.Mtime, stamp.Ctime)
}

// runtimeAssetContentVersion returns the hash of the file's bytes as 16 hex
// digits, or "" when the file cannot be read.
func runtimeAssetContentVersion(file string) string {
	stamp, trusted := trustedRuntimeAssetStamp(file)
	if trusted {
		runtimeAssetMemo.Lock()
		entry, ok := runtimeAssetMemo.entries[file]
		runtimeAssetMemo.Unlock()
		if ok && entry.stamp == stamp {
			return entry.version
		}
	}
	if runtimeAssetHashHook != nil {
		runtimeAssetHashHook(file)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(content)
	version := hex.EncodeToString(sum[:8])
	if after, ok := trustedRuntimeAssetStamp(file); trusted && ok && after == stamp {
		runtimeAssetMemo.Lock()
		if runtimeAssetMemo.entries == nil || len(runtimeAssetMemo.entries) >= runtimeAssetMemoLimit {
			runtimeAssetMemo.entries = map[string]runtimeAssetMemoEntry{}
		}
		runtimeAssetMemo.entries[file] = runtimeAssetMemoEntry{stamp: stamp, version: version}
		runtimeAssetMemo.Unlock()
	}
	return version
}
