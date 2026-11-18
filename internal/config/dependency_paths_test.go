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

func TestLoadFileSiblingDependencyScan(t *testing.T) {
	for _, tc := range []struct {
		body    string
		want    bool
		wantErr bool
	}{
		{"project:\n  siblingDependencyScan: true\n", true, false},
		{"project:\n  siblingDependencyScan: false\n", false, false},
		{"project:\n  dependencyPaths: []\n", false, false},
		{"project:\n  siblingDependencyScan: maybe\n", false, true},
	} {
		path := filepath.Join(t.TempDir(), "glade.yml")
		if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadFile(path)
		if (err != nil) != tc.wantErr {
			t.Fatalf("%q: err=%v, wantErr=%v", tc.body, err, tc.wantErr)
		}
		if err == nil && cfg.Project.SiblingDependencyScan != tc.want {
			t.Fatalf("%q: SiblingDependencyScan=%v, want %v", tc.body, cfg.Project.SiblingDependencyScan, tc.want)
		}
	}
}
