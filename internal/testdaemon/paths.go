package testdaemon

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
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

func servePaths(projectRoot string) (socket string, pid string) {
	localSocket := filepath.Join(projectRoot, serveDir, "serve.sock")
	localPID := filepath.Join(projectRoot, serveDir, "serve.pid")
	if len(localSocket) <= maxUnixSocketPathLen {
		return localSocket, localPID
	}
	sum := sha256.Sum256([]byte(filepath.Clean(projectRoot)))
	base := "glade-test-serve-" + hex.EncodeToString(sum[:8])
	return filepath.Join(os.TempDir(), base+".sock"), filepath.Join(os.TempDir(), base+".pid")
}
