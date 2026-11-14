package inputview

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryListingDetectsHiddenMembership(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "added", true: "removed"}[remove], func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".hidden")
			if remove {
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			v := New()
			if _, err := v.ReadDir(dir); err != nil {
				t.Fatal(err)
			}
			var err error
			if remove {
				err = os.Remove(path)
			} else {
				err = os.WriteFile(path, nil, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := v.Recheck(); err == nil {
				t.Fatal("hidden membership change was not detected")
			}
		})
	}
}
