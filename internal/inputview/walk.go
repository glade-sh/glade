package inputview

import (
	"io/fs"
	"path/filepath"
)

func (v *View) WalkDir(root string, fn fs.WalkDirFunc) error { return v.walkDir(root, fn, false) }

// WalkDirNonHidden omits hidden entries and pins only visible membership.
func (v *View) WalkDirNonHidden(root string, fn fs.WalkDirFunc) error {
	return v.walkDir(root, fn, true)
}

func (v *View) walkDir(root string, fn fs.WalkDirFunc, visibleOnly bool) error {
	if (v == nil || v.isClosed()) && !visibleOnly {
		return filepath.WalkDir(root, fn)
	}
	info, err := v.Lstat(root)
	if err != nil {
		err = fn(root, nil, err)
	} else {
		err = v.walk(root, fs.FileInfoToDirEntry(info), fn, visibleOnly)
	}
	if err == fs.SkipDir || err == fs.SkipAll {
		return nil
	}
	return err
}
func (v *View) walk(path string, d fs.DirEntry, fn fs.WalkDirFunc, visibleOnly bool) error {
	if err := fn(path, d, nil); err != nil || !d.IsDir() {
		if err == fs.SkipDir && d.IsDir() {
			return nil
		}
		return err
	}
	entries, err := v.readDir(path, visibleOnly)
	if err != nil {
		if e := fn(path, d, err); e != nil {
			if e == fs.SkipDir {
				return nil
			}
			return e
		}
	}
	for _, entry := range entries {
		err = v.walk(filepath.Join(path, entry.Name()), entry, fn, visibleOnly)
		if err != nil {
			if err == fs.SkipDir {
				break
			}
			return err
		}
	}
	return nil
}
