// Package inputview records a load's filesystem observations and detects drift.
package inputview

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type ChangedError struct{ Path, What string }

func (e *ChangedError) Error() string { return fmt.Sprintf("%s (%s)", e.Path, e.What) }

type Fingerprint struct {
	Size    int64
	ModTime time.Time
	SHA256  [32]byte
}
type observation struct {
	kind        string
	exists      bool
	mode        fs.FileMode
	fingerprint Fingerprint
	listing     [32]byte
	resolved    string
	errClass    string
}
type View struct {
	mu             sync.Mutex
	pins           map[string]map[string]observation
	err            *ChangedError
	sealed, closed bool
}

func New() *View { return &View{pins: make(map[string]map[string]observation)} }
func key(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(p)
}
func errorClass(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, fs.ErrNotExist) {
		return "not-exist"
	}
	if errors.Is(err, fs.ErrPermission) {
		return "permission"
	}
	return err.Error()
}
func changedWhat(a, b observation) string {
	if !a.exists && b.exists {
		return "file added"
	}
	if a.exists && !b.exists {
		return "file removed"
	}
	if a.kind == "dir" {
		return "directory listing"
	}
	return "file content"
}
func (v *View) record(path string, o observation) error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return nil
	}
	path = key(path)
	pins := v.pins[path]
	if pins == nil {
		if v.sealed {
			return v.change(path, "file added")
		}
		pins = make(map[string]observation)
		v.pins[path] = pins
	}
	if old, ok := pins[o.kind]; ok {
		if old != o {
			return v.change(path, changedWhat(old, o))
		}
		return nil
	}
	if v.sealed {
		return v.change(path, "file added")
	}
	// Followed observations share existence and type. Lstat may describe the link itself.
	if o.kind != "lstat" && o.kind != "symlink" {
		for k, old := range pins {
			if k != "lstat" && k != "symlink" && (old.exists != o.exists || (old.exists && old.mode != o.mode)) {
				return v.change(path, changedWhat(old, o))
			}
		}
	}
	pins[o.kind] = o
	return nil
}
func (v *View) change(path, what string) error {
	if v.err == nil {
		v.err = &ChangedError{Path: path, What: what}
	}
	return v.err
}
func (v *View) Err() error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.err == nil {
		return nil
	}
	return v.err
}
func (v *View) Close() {
	if v == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.closed = true
}
func fileObservation(path string) ([]byte, observation, error) {
	o := observation{kind: "file"}
	f, err := os.Open(path)
	if err != nil {
		o.errClass = errorClass(err)
		return nil, o, err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		o.errClass = errorClass(err)
		return nil, o, err
	}
	o.exists = true
	o.mode = before.Mode().Type()
	data, err := io.ReadAll(f)
	if err != nil {
		o.errClass = errorClass(err)
		return nil, o, err
	}
	after, err := f.Stat()
	if err != nil {
		o.errClass = errorClass(err)
		return nil, o, err
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, o, &ChangedError{Path: key(path), What: "file content"}
	}
	o.fingerprint = Fingerprint{Size: after.Size(), ModTime: after.ModTime(), SHA256: sha256.Sum256(data)}
	return data, o, nil
}
func (v *View) ReadFile(path string) ([]byte, error) {
	if v == nil || v.isClosed() {
		return os.ReadFile(path)
	}
	data, o, err := fileObservation(path)
	var c *ChangedError
	if errors.As(err, &c) {
		v.mu.Lock()
		e := v.change(c.Path, c.What)
		v.mu.Unlock()
		return nil, e
	}
	if e := v.record(path, o); e != nil {
		return nil, e
	}
	return data, err
}
func (v *View) isClosed() bool { v.mu.Lock(); defer v.mu.Unlock(); return v.closed }
func (v *View) Fingerprint(path string) (Fingerprint, error) {
	if v == nil || v.isClosed() {
		_, o, e := fileObservation(path)
		return o.fingerprint, e
	}
	_, o, e := fileObservation(path)
	var c *ChangedError
	if errors.As(e, &c) {
		v.mu.Lock()
		err := v.change(c.Path, c.What)
		v.mu.Unlock()
		return Fingerprint{}, err
	}
	if err := v.record(path, o); err != nil {
		return Fingerprint{}, err
	}
	return o.fingerprint, e
}
func statObservation(path string, link bool) (os.FileInfo, observation, error) {
	kind := "stat"
	var info os.FileInfo
	var err error
	if link {
		kind = "lstat"
		info, err = os.Lstat(path)
	} else {
		info, err = os.Stat(path)
	}
	o := observation{kind: kind, errClass: errorClass(err)}
	if err == nil {
		o.exists = true
		o.mode = info.Mode().Type()
	}
	return info, o, err
}
func (v *View) Stat(path string) (os.FileInfo, error) {
	if v == nil || v.isClosed() {
		return os.Stat(path)
	}
	i, o, e := statObservation(path, false)
	if err := v.record(path, o); err != nil {
		return nil, err
	}
	return i, e
}
func (v *View) Lstat(path string) (os.FileInfo, error) {
	if v == nil || v.isClosed() {
		return os.Lstat(path)
	}
	i, o, e := statObservation(path, true)
	if err := v.record(path, o); err != nil {
		return nil, err
	}
	return i, e
}

type dirEntry struct {
	os.DirEntry
	v    *View
	path string
}

func (d dirEntry) Info() (os.FileInfo, error) { return d.v.Lstat(d.path) }
func dirObservation(path string) ([]os.DirEntry, observation, error) {
	entries, err := os.ReadDir(path)
	o := observation{kind: "dir", errClass: errorClass(err)}
	if err != nil {
		return entries, o, err
	}
	o.exists = true
	o.mode = fs.ModeDir
	h := sha256.New()
	for _, e := range entries {
		fmt.Fprintf(h, "%s\x00%d\x00", e.Name(), e.Type())
	}
	copy(o.listing[:], h.Sum(nil))
	return entries, o, nil
}
func (v *View) ReadDir(path string) ([]os.DirEntry, error) {
	if v == nil || v.isClosed() {
		return os.ReadDir(path)
	}
	entries, o, e := dirObservation(path)
	if err := v.record(path, o); err != nil {
		return nil, err
	}
	for i, d := range entries {
		entries[i] = dirEntry{d, v, filepath.Join(path, d.Name())}
	}
	return entries, e
}
func symlinkObservation(path string) (string, observation, error) {
	p, e := filepath.EvalSymlinks(path)
	return p, observation{kind: "symlink", resolved: p, errClass: errorClass(e)}, e
}
func (v *View) EvalSymlinks(path string) (string, error) {
	if v == nil || v.isClosed() {
		return filepath.EvalSymlinks(path)
	}
	p, o, e := symlinkObservation(path)
	if err := v.record(path, o); err != nil {
		return "", err
	}
	return p, e
}
func (v *View) Recheck() error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return nil
	}
	v.sealed = true
	pins := make(map[string]map[string]observation, len(v.pins))
	for p, obs := range v.pins {
		pins[p] = make(map[string]observation, len(obs))
		for k, o := range obs {
			pins[p][k] = o
		}
	}
	v.mu.Unlock()
	paths := make([]string, 0, len(pins))
	for p := range pins {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		kinds := make([]string, 0, len(pins[p]))
		for k := range pins[p] {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		for _, k := range kinds {
			var o observation
			var e error
			switch k {
			case "file":
				_, o, e = fileObservation(p)
			case "dir":
				_, o, e = dirObservation(p)
			case "stat":
				_, o, e = statObservation(p, false)
			case "lstat":
				_, o, e = statObservation(p, true)
			case "symlink":
				_, o, e = symlinkObservation(p)
			}
			old := pins[p][k]
			if (e != nil && !errors.Is(e, fs.ErrNotExist)) || o != old {
				v.mu.Lock()
				err := v.change(p, changedWhat(old, o))
				v.mu.Unlock()
				return err
			}
		}
	}
	return v.Err()
}
