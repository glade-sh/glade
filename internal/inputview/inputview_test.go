package inputview

import (
	"errors"
	"os"
	"path/filepath"
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
