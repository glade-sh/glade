package gladecli

import (
	"os"
	"path/filepath"
)

// writeReadyFileAtomically makes a completed readiness document visible with a
// single same-directory rename, so polling clients never observe a truncated
// JSON document.
func writeReadyFileAtomically(path string, data []byte, mode os.FileMode, rename func(string, string) error) (err error) {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return rename(tempPath, path)
}
