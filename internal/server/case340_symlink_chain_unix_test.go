//go:build darwin || linux

package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCase340LightningRejectsUnsafeIntermediateSymlinkTarget(t *testing.T) {
	for _, relative := range []bool{false, true} {
		root := t.TempDir()
		safe := filepath.Join(root, "safe")
		unsafe := filepath.Join(root, "unsafe")
		if err := os.MkdirAll(filepath.Join(safe, "nested"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(unsafe, 0700); err != nil {
			t.Fatal(err)
		}
		target := safe
		if relative {
			target = "../safe"
		}
		if err := os.Symlink(target, filepath.Join(unsafe, "next")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(unsafe, "next"), filepath.Join(root, "entry")); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(unsafe, 0777); err != nil {
			t.Fatal(err)
		}
		if trustedLightningTempParent(filepath.Join(root, "entry", "nested")) {
			t.Errorf("relative=%v: accepted replaceable intermediate target", relative)
		}
		if err := os.Chmod(unsafe, 0700); err != nil {
			t.Fatal(err)
		}
		if !trustedLightningTempParent(filepath.Join(root, "entry", "nested")) {
			t.Errorf("relative=%v: rejected safe chain", relative)
		}
	}
}

func TestCase340LightningRejectsDotDotAfterSymlink(t *testing.T) {
	root := t.TempDir()
	safe := filepath.Join(root, "safe")
	unsafe := filepath.Join(root, "unsafe")
	if err := os.Mkdir(safe, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(unsafe, "inner"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(unsafe, "safe"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(unsafe, "inner"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unsafe, 0777); err != nil {
		t.Fatal(err)
	}
	if !trustedLightningTempParent(safe) {
		t.Fatal("safe control path is not trusted in this test environment")
	}
	for _, suffix := range []string{"/link/../safe", "/link/../../safe"} {
		path := root + suffix
		if trustedLightningTempParent(path) {
			t.Errorf("accepted unclean symlink path %q", suffix)
		}
	}
}
