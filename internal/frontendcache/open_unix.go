//go:build darwin || linux

package frontendcache

import (
	"os"

	"golang.org/x/sys/unix"
)

// Pin each private component without following links. Opening relative to the
// parent descriptor makes a separate pathname stat/inode comparison unnecessary.
func openSnapshot(root string) (*os.File, error) {
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range []string{".glade", "semantic"} {
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if err != nil {
			return nil, err
		}
		fd = next
	}
	// Nonblocking also permits the descriptor type check to reject a FIFO
	// without waiting for a writer. Regular files retain ordinary read behavior.
	next, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	unix.Close(fd)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(next), name), nil
}
