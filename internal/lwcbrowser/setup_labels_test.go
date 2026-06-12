package lwcbrowser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestVerifiableSetupBundleIncludesLabelsSibling(t *testing.T) {
	root := "/Users/matt/.sf-repo-analysis/repos/sf-cred-pkg-develop"
	if _, err := os.Stat(root); err != nil {
		t.Skip("sf-cred-pkg-develop not present")
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "cache")
	_, compiled, err := PreparePageConfig(p, cache)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := compiled.Modules["verifiable:setup"]; !ok {
		t.Fatal("missing verifiable:setup")
	}
	labelsPath := filepath.Join(cache, "lwc", "verifiable", "setup", "labels.js")
	if _, err := os.Stat(labelsPath); err != nil {
		t.Fatalf("missing labels.js: %v", err)
	}
	body, err := os.ReadFile(labelsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "connectVerifiableSync") {
		t.Fatalf("unexpected labels.js: %s", body[:min(200, len(body))])
	}
}

func TestVerifiableSetupImportMapIncludesLanding(t *testing.T) {
	root := "/Users/matt/.sf-repo-analysis/repos/sf-cred-pkg-develop"
	if _, err := os.Stat(root); err != nil {
		t.Skip("sf-cred-pkg-develop not present")
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "cache")
	cfg, _, err := PreparePageConfig(p, cache)
	if err != nil {
		t.Fatal(err)
	}
	imports := LocalLWCImportMap(cfg.Namespace, cfg.Manifest)
	if imports["c/landing"] == "" {
		t.Fatal("missing c/landing import map entry")
	}
	if imports["c/wizard"] == "" {
		t.Fatal("missing c/wizard import map entry")
	}
}
