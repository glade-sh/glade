package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// PathInsideRoot reports whether full stays inside root once symlinks are
// resolved on both sides. A symlink inside the project that points outside it
// is refused; a symlink that stays inside is accepted. A target that does not
// exist is judged by its nearest existing ancestor, so a missing file under a
// real in-project directory is inside, a missing file under an escaping link
// is not, and a dangling link is refused.
func PathInsideRoot(root, full string) bool {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	realRoot = filepath.Clean(realRoot)
	probe := filepath.Clean(full)
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(probe)
		if err == nil {
			target := filepath.Join(append([]string{resolved}, tail...)...)
			return target != realRoot && strings.HasPrefix(target, realRoot+string(os.PathSeparator))
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false
		}
		if _, lerr := os.Lstat(probe); lerr == nil {
			// A dangling symlink: its target cannot be placed, so refuse it.
			return false
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return false
		}
		tail = append([]string{filepath.Base(probe)}, tail...)
		probe = parent
	}
}
