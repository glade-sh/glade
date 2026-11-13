package inputview

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepeatReadMismatchIsSticky(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte("A"), 0600); err != nil {
		t.Fatal(err)
	}
	v := New()
	if _, err := v.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("B"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := v.ReadFile(path)
	var changed *ChangedError
	if !errors.As(err, &changed) {
		t.Fatalf("got %v, want ChangedError", err)
	}
	if err := os.WriteFile(path, []byte("A"), 0600); err != nil {
		t.Fatal(err)
	}
	if !errors.As(v.Err(), &changed) {
		t.Fatalf("sticky error = %v", v.Err())
	}
}

func TestStatAbsentThenReadFilePresent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input")
	v := New()
	if _, err := v.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stat: %v", err)
	}
	if err := os.WriteFile(path, []byte("A"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := v.ReadFile(path)
	var changed *ChangedError
	if !errors.As(err, &changed) {
		t.Fatalf("got %v, want ChangedError", err)
	}
}

func TestRecheckDetectsRevertedSwap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte("B"), 0600); err != nil {
		t.Fatal(err)
	}
	v := New()
	if _, err := v.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("A"), 0600); err != nil {
		t.Fatal(err)
	}
	var changed *ChangedError
	if err := v.Recheck(); !errors.As(err, &changed) {
		t.Fatalf("got %v, want ChangedError", err)
	}
}

func TestDirectoryListingPin(t *testing.T) {
	for _, change := range []string{"add", "remove", "type"} {
		t.Run(change, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "one")
			if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
				t.Fatal(err)
			}
			v := New()
			if _, err := v.ReadDir(dir); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "add":
				if err := os.WriteFile(filepath.Join(dir, "two"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "remove":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
			case "type":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(p, 0700); err != nil {
					t.Fatal(err)
				}
			}
			var changed *ChangedError
			if err := v.Recheck(); !errors.As(err, &changed) {
				t.Fatalf("recheck: %v", err)
			}
		})
	}
}

func TestRecheckCleanWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file")
	if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	v := New()
	if _, err := v.ReadFile(p); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ReadDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := v.Recheck(); err != nil {
		t.Fatal(err)
	}
}

func TestObservationAfterSealIsAnError(t *testing.T) {
	v := New()
	if err := v.Recheck(); err != nil {
		t.Fatal(err)
	}
	_, err := v.Stat(t.TempDir())
	var changed *ChangedError
	if !errors.As(err, &changed) {
		t.Fatalf("stat: %v", err)
	}
}

func TestClosedViewPassesThrough(t *testing.T) {
	p := filepath.Join(t.TempDir(), "file")
	v := New()
	if _, err := v.ReadFile(p); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	v.Close()
	if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ReadFile(p); err != nil {
		t.Fatal(err)
	}
	if err := v.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestViewsAreIsolated(t *testing.T) {
	p := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(p, []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	v1, v2 := New(), New()
	if _, err := v1.ReadFile(p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("b"), 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 3)
	go func() { _, err := v1.ReadFile(p); done <- err }()
	go func() { _, err := v2.ReadFile(p); done <- err }()
	go func() { _, err := os.ReadFile(p); done <- err }()
	errorsSeen := 0
	for i := 0; i < 3; i++ {
		if <-done != nil {
			errorsSeen++
		}
	}
	if errorsSeen != 1 || v2.Err() != nil {
		t.Fatalf("errors=%d view2=%v", errorsSeen, v2.Err())
	}
}

func TestAbsentThenPresent(t *testing.T) { testPresenceChange(t, false) }
func TestPresentThenAbsent(t *testing.T) { testPresenceChange(t, true) }
func testPresenceChange(t *testing.T, initiallyPresent bool) {
	t.Helper()
	for _, method := range []string{"read", "stat"} {
		t.Run(method, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "file")
			if initiallyPresent {
				if err := os.WriteFile(p, []byte("a"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			v := New()
			observe := func() error {
				if method == "read" {
					_, e := v.ReadFile(p)
					return e
				}
				_, e := v.Stat(p)
				return e
			}
			err := observe()
			if initiallyPresent && err != nil || !initiallyPresent && !os.IsNotExist(err) {
				t.Fatalf("initial: %v", err)
			}
			if initiallyPresent {
				err = os.Remove(p)
			} else {
				err = os.WriteFile(p, []byte("a"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			var changed *ChangedError
			if err = observe(); !errors.As(err, &changed) {
				t.Fatalf("second: %v", err)
			}
		})
	}
}

func TestNilViewPassesThrough(t *testing.T) {
	var v *View
	dir := t.TempDir()
	p := filepath.Join(dir, "file")
	if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if b, e := v.ReadFile(p); e != nil || string(b) != "x" {
		t.Fatalf("read: %q %v", b, e)
	}
	if _, e := v.ReadFile(p + "missing"); !os.IsNotExist(e) {
		t.Fatal(e)
	}
	if _, e := v.ReadDir(dir); e != nil {
		t.Fatal(e)
	}
	if _, e := v.Stat(p); e != nil {
		t.Fatal(e)
	}
	if _, e := v.Lstat(p); e != nil {
		t.Fatal(e)
	}
	if _, e := v.EvalSymlinks(p); e != nil {
		t.Fatal(e)
	}
	if e := v.WalkDir(dir, func(_ string, _ os.DirEntry, e error) error { return e }); e != nil {
		t.Fatal(e)
	}
	if e := v.Recheck(); e != nil {
		t.Fatal(e)
	}
	v.Close()
}

func TestWalkDirMatchesFilepathWalkDir(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"a/one", "a/two", "b/three"} {
		path := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, skip := range []string{"", "dir", "all"} {
		t.Run(skip, func(t *testing.T) {
			collect := func(walk func(string, fs.WalkDirFunc) error) []string {
				var got []string
				err := walk(root, func(path string, d os.DirEntry, err error) error {
					got = append(got, strings.TrimPrefix(path, root))
					if err != nil {
						return err
					}
					if skip == "dir" && d.Name() == "a" {
						return filepath.SkipDir
					}
					if skip == "all" && d.Name() == "one" {
						return filepath.SkipAll
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				return got
			}
			want := collect(filepath.WalkDir)
			got := collect(New().WalkDir)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v want %v", got, want)
			}
		})
	}
}
