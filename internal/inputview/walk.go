package inputview

import (
	"io/fs"
	"path/filepath"
)

func (v *View) WalkDir(root string, fn fs.WalkDirFunc) error {
	if v == nil || v.isClosed() {
		return filepath.WalkDir(root, fn)
	}
	info, err := v.Lstat(root)
	if err != nil {
		err = fn(root, nil, err)
	} else {
		err = v.walk(root, fs.FileInfoToDirEntry(info), fn)
	}
	if err == fs.SkipDir || err == fs.SkipAll {
		return nil
	}
	return err
}
func (v *View) walk(path string, d fs.DirEntry, fn fs.WalkDirFunc) error {
	if err := fn(path, d, nil); err != nil || !d.IsDir() {
		if err == fs.SkipDir && d.IsDir() {
			return nil
		}
		return err
	}
	entries, err := v.ReadDir(path)
	if err != nil {
		if e := fn(path, d, err); e != nil {
			if e == fs.SkipDir {
				return nil
			}
			return e
		}
	}
	for _, entry := range entries {
		err = v.walk(filepath.Join(path, entry.Name()), entry, fn)
		if err != nil {
			if err == fs.SkipDir {
				break
			}
			return err
		}
	}
	return nil
}
