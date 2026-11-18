package inputview

import (
	"io/fs"
	"path/filepath"
)

func (v *View) WalkDir(root string, fn fs.WalkDirFunc) error { return v.walkDir(root, fn, "dir") }

// WalkDirNonHidden omits hidden entries and pins only visible membership.
func (v *View) WalkDirNonHidden(root string, fn fs.WalkDirFunc) error {
	return v.walkDir(root, fn, "dir-visible")
}

// WalkDirProjectRoot excludes the startup cache from the root listing; the
// callback already skips that directory. Other hidden folders remain visible.
func (v *View) WalkDirProjectRoot(root string, fn fs.WalkDirFunc) error {
	if v == nil || v.isClosed() {
		return filepath.WalkDir(root, fn)
	}
	return v.walkDir(root, fn, "dir-root")
}

func (v *View) walkDir(root string, fn fs.WalkDirFunc, kind string) error {
	if (v == nil || v.isClosed()) && kind == "dir" {
		return filepath.WalkDir(root, fn)
	}
	info, err := v.Lstat(root)
	if err != nil {
		err = fn(root, nil, err)
	} else {
		err = v.walk(root, fs.FileInfoToDirEntry(info), fn, kind)
	}
	if err == fs.SkipDir || err == fs.SkipAll {
		return nil
	}
	return err
}
func (v *View) walk(path string, d fs.DirEntry, fn fs.WalkDirFunc, kind string) error {
	if err := fn(path, d, nil); err != nil || !d.IsDir() {
		if err == fs.SkipDir && d.IsDir() {
			return nil
		}
		return err
	}
	entries, err := v.readDir(path, kind)
	if err != nil {
		if e := fn(path, d, err); e != nil {
			if e == fs.SkipDir {
				return nil
			}
			return e
		}
	}
	childKind := kind
	if kind == "dir-root" {
		childKind = "dir"
	}
	for _, entry := range entries {
		err = v.walk(filepath.Join(path, entry.Name()), entry, fn, childKind)
		if err != nil {
			if err == fs.SkipDir {
				break
			}
			return err
		}
	}
	return nil
}
