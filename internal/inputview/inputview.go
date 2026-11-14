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
	"strings"
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
	if a.kind == "dir" || a.kind == "dir-visible" || a.kind == "dir-root" {
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
	v.pins = nil
}
func fileObservation(path string) ([]byte, observation, error) {
	o := observation{kind: "file"}
	f, err := os.Open(path)
	if err != nil {
		o.errClass = errorClass(err)
		if info, statErr := os.Stat(path); statErr == nil {
			o.exists = true
			o.mode = info.Mode().Type()
		}
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
	v.mu.Lock()
	if !v.closed {
		if pinned, ok := v.pins[key(path)]["file"]; ok && pinned.errClass == "" {
			v.mu.Unlock()
			return pinned.fingerprint, nil
		}
	}
	v.mu.Unlock()
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
func dirObservation(path, kind string) ([]os.DirEntry, observation, error) {
	entries, err := os.ReadDir(path)
	o := observation{kind: kind, errClass: errorClass(err)}
	if err != nil {
		if info, statErr := os.Stat(path); statErr == nil {
			o.exists = true
			o.mode = info.Mode().Type()
		}
		return entries, o, err
	}
	o.exists = true
	o.mode = fs.ModeDir
	h := sha256.New()
	for _, e := range entries {
		if excludeDirEntry(kind, e.Name()) || ignoredChurnFile(path, e) {
			continue
		}
		fmt.Fprintf(h, "%s\x00%d\x00", e.Name(), e.Type())
	}
	copy(o.listing[:], h.Sum(nil))
	return entries, o, nil
}

// ignoredChurnFile keeps source bundle contents observable even when their
// names look like local editor artifacts. Bundle content can have any name.
func ignoredChurnFile(dir string, entry os.DirEntry) bool {
	if !entry.Type().IsRegular() {
		return false
	}
	name := entry.Name()
	churn := name == ".DS_Store" || strings.HasSuffix(name, ".swp") || strings.HasSuffix(name, "~")
	if strings.HasPrefix(name, ".#") {
		target := strings.TrimPrefix(name, ".#")
		churn = churn || !strings.Contains(target, ".") || strings.HasSuffix(target, ".cls") || strings.HasSuffix(target, ".trigger")
	}
	if !churn {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(filepath.Clean(dir)), "/") {
		switch strings.ToLower(part) {
		case "aiagentdefinitionversions", "aiauthoringbundles", "aura", "digitalexperiences", "documents", "experiences", "experiencepropertytypebundles", "genaifunctions", "genaiplannerbundles", "integartifactdefs", "lwc", "staticresources", "wavetemplates", "uibundles", "apptemplates", "lightningtypes", "contenttypes", "uiwidgets":
			return false
		}
	}
	return true
}

func (v *View) ReadDir(path string) ([]os.DirEntry, error) {
	return v.readDir(path, "dir")
}

// ReadDirNonHidden lists only non-hidden entries and pins that listing.
func (v *View) ReadDirNonHidden(path string) ([]os.DirEntry, error) {
	return v.readDir(path, "dir-visible")
}

func (v *View) readDir(path, kind string) ([]os.DirEntry, error) {
	if v == nil || v.isClosed() {
		entries, err := os.ReadDir(path)
		entries = filterDirEntries(entries, kind)
		return entries, err
	}
	entries, o, e := dirObservation(path, kind)
	entries = filterDirEntries(entries, kind)
	if err := v.record(path, o); err != nil {
		return nil, err
	}
	for i, d := range entries {
		entries[i] = dirEntry{d, v, filepath.Join(path, d.Name())}
	}
	return entries, e
}
func excludeDirEntry(kind, name string) bool {
	return kind == "dir-visible" && strings.HasPrefix(name, ".") || kind == "dir-root" && name == ".glade"
}
func filterDirEntries(entries []os.DirEntry, kind string) []os.DirEntry {
	if kind == "dir" {
		return entries
	}
	out := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if !excludeDirEntry(kind, entry.Name()) {
			out = append(out, entry)
		}
	}
	return out
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

// CheckDirectories detects membership drift without sealing the view. Later
// reads may still add pins, and Recheck remains the publication barrier.
func (v *View) CheckDirectories() error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return nil
	}
	pins := make(map[string]map[string]observation)
	for path, kinds := range v.pins {
		for kind, old := range kinds {
			if kind == "dir" || kind == "dir-visible" || kind == "dir-root" {
				if pins[path] == nil {
					pins[path] = make(map[string]observation)
				}
				pins[path][kind] = old
			}
		}
	}
	v.mu.Unlock()
	paths := make([]string, 0, len(pins))
	for path := range pins {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		kinds := make([]string, 0, len(pins[path]))
		for kind := range pins[path] {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		for _, kind := range kinds {
			_, current, _ := dirObservation(path, kind)
			old := pins[path][kind]
			if current != old {
				v.mu.Lock()
				err := v.change(path, changedWhat(old, current))
				v.mu.Unlock()
				return err
			}
		}
	}
	return v.Err()
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
			switch k {
			case "file":
				_, o, _ = fileObservation(p)
			case "dir", "dir-visible", "dir-root":
				_, o, _ = dirObservation(p, k)
			case "stat":
				_, o, _ = statObservation(p, false)
			case "lstat":
				_, o, _ = statObservation(p, true)
			case "symlink":
				_, o, _ = symlinkObservation(p)
			}
			old := pins[p][k]
			// A pinned error that repeats unchanged is not a change; any other
			// difference, including a new I/O error, is.
			if o != old {
				v.mu.Lock()
				err := v.change(p, changedWhat(old, o))
				v.mu.Unlock()
				return err
			}
		}
	}
	return v.Err()
}
