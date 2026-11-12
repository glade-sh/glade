package testdaemon

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const serveDir = ".glade/test"

// macOS limits unix socket paths to about 104 bytes.
const maxUnixSocketPathLen = 100

func ServeSocketPath(projectRoot string) string {
	socket, _ := servePaths(projectRoot)
	return socket
}

func ServePIDPath(projectRoot string) string {
	_, pid := servePaths(projectRoot)
	return pid
}

// absoluteIfClimbing resolves a relative root that climbs out of the working
// directory. Cleaning leaves only leading parent segments, which cannot cross a
// symlink, so the absolute form is unambiguous and passes the traversal check.
func absoluteIfClimbing(projectRoot string) string {
	if projectRoot == "" || filepath.IsAbs(projectRoot) {
		return projectRoot
	}
	cleaned := filepath.Clean(projectRoot)
	if cleaned != ".." && !strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return projectRoot
	}
	if abs, err := filepath.Abs(cleaned); err == nil {
		return abs
	}
	return projectRoot
}

func servePaths(projectRoot string) (socket string, pid string) {
	base, relative := privateServeDir(projectRoot)
	runtimeDir := filepath.Join(base, relative)
	return filepath.Join(runtimeDir, "serve.sock"), filepath.Join(runtimeDir, "serve.pid")
}

// privateServeDir picks the daemon discovery directory. The project-local
// directory is used when its path is short enough for a unix socket and every
// existing component is owned by the user (or root) and not group or world
// writable. Otherwise the daemon uses a hashed directory under the per-user
// cache directory (0700, checked the same way), and the temp root only when
// the cache directory cannot be determined or fails its check. A rejected
// location prints one warning per process.
func privateServeDir(projectRoot string) (base, relative string) {
	projectRoot = absoluteIfClimbing(projectRoot)
	localDir := filepath.Join(projectRoot, serveDir)
	localSocket := filepath.Join(localDir, "serve.sock")
	if len(localSocket) <= maxUnixSocketPathLen {
		if discoveryLocationTrusted(localDir) {
			return projectRoot, filepath.FromSlash(serveDir)
		}
		warnUntrustedDiscovery(localDir)
	}
	sum := sha256.Sum256([]byte(filepath.Clean(projectRoot)))
	name := "glade-test-serve-" + hex.EncodeToString(sum[:8])
	if cache, err := os.UserCacheDir(); err == nil && cache != "" {
		if abs, err := filepath.Abs(cache); err == nil && ensureTrustedCacheBase(abs) {
			if discoveryLocationTrusted(filepath.Join(abs, "glade", name)) {
				return abs, filepath.Join("glade", name)
			}
			warnUntrustedDiscovery(filepath.Join(abs, "glade", name))
		}
	}
	tmp := os.TempDir()
	if !discoveryLocationTrusted(filepath.Join(tmp, name)) {
		// All three locations failed. Return the temp path anyway: it is
		// untrusted, so ValidateDiscoverySocket and NewServer refuse it with
		// ErrUntrustedDiscovery before any socket or PID file is read,
		// created or removed, and callers fall back to in-process runs.
		warnUntrustedDiscovery(filepath.Join(tmp, name))
	}
	return tmp, name
}

var (
	discoveryWarnOnce   sync.Once
	discoveryWarnWriter io.Writer = os.Stderr
)

func warnUntrustedDiscovery(dir string) {
	discoveryWarnOnce.Do(func() {
		fmt.Fprintf(discoveryWarnWriter, "glade: refusing daemon directory %s (not owned by you or writable by others); using the per-user cache directory\n", dir)
	})
}

// discoveryLocationTrusted reports whether dir, or the nearest existing
// ancestor when dir is absent, passes the owner and mode checks. Each call
// re-checks the filesystem.
func discoveryLocationTrusted(dir string) bool {
	if !discoveryOwnerChecks {
		return true
	}
	for candidate := dir; ; candidate = filepath.Dir(candidate) {
		if _, err := os.Lstat(candidate); err == nil {
			if candidate == dir {
				return secureDiscoveryPath(candidate)
			}
			return secureDiscoveryParentPath(candidate)
		}
		if filepath.Dir(candidate) == candidate {
			return false
		}
	}
}

// ensureTrustedCacheBase checks the existing ancestors of dir before creating
// anything, creates dir with mode 0700, then checks it again.
func ensureTrustedCacheBase(dir string) bool {
	if !discoveryLocationTrusted(dir) {
		return false
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	return discoveryLocationTrusted(dir)
}
