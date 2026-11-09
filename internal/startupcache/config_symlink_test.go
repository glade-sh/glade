package startupcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestCollectConfigPathsRefusesEscapingConfigFileLink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeStartupCacheTestFile(t, filepath.Join(outside, "out.json"), `{}`)
	writeStartupCacheTestFile(t, filepath.Join(root, "config", "real.json"), `{}`)
	writeStartupCacheTestFile(t, filepath.Join(root, "cumulusci.yml"), "orgs:\n  scratch:\n    a:\n      config_file: config/escape.json\n    b:\n      config_file: config/inside.json\n")
	if err := os.Symlink(filepath.Join(outside, "out.json"), filepath.Join(root, "config", "escape.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "config", "real.json"), filepath.Join(root, "config", "inside.json")); err != nil {
		t.Fatal(err)
	}
	paths, err := collectConfigPaths(project.Project{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	var escape, inside bool
	for _, p := range paths {
		switch filepath.Base(p) {
		case "escape.json":
			escape = true
		case "inside.json":
			inside = true
		}
	}
	if escape {
		t.Errorf("escaping config_file link listed: %v", paths)
	}
	if !inside {
		t.Errorf("in-project config_file link dropped: %v", paths)
	}
}
