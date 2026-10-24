package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadFileDependencyPathsRelativeToConfig(t *testing.T) {
	root := t.TempDir()
	absolute := filepath.Join(t.TempDir(), "library")
	path := filepath.Join(root, "glade.yml")
	if err := os.WriteFile(path, []byte("project:\n  dependencyPaths: [\"../library\", \""+filepath.ToSlash(absolute)+"\"]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(cfg.Project)
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ DependencyPaths []string }
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(filepath.Dir(root), "library"), absolute}
	if !reflect.DeepEqual(got.DependencyPaths, want) {
		t.Fatalf("dependencyPaths=%v, want %v", got.DependencyPaths, want)
	}
}
