//go:build !darwin && !linux

package frontendcache

import (
	"errors"
	"os"
)

func openSnapshot(root string) (*os.File, error) {
	dir, err := directory(root, false)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	before, err := dir.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > maxSnapshotBytes {
		return nil, errors.New("invalid frontend cache file")
	}
	file, err := dir.Open(name)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		file.Close()
		return nil, errors.New("frontend cache file changed")
	}
	return file, nil
}
