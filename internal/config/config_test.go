package config

import "testing"

func TestParseYAMLSubset(t *testing.T) {
	cfg, err := parseYAMLSubset(`
project:
  root: .
  packageDirs: ["force-app", "packages/core"]
  defaultNamespace: verifiable
`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Root != "." {
		t.Fatalf("root = %q", cfg.Project.Root)
	}
	if got := len(cfg.Project.PackageDirs); got != 2 {
		t.Fatalf("package dir count = %d", got)
	}
	if cfg.Project.DefaultNamespace != "verifiable" {
		t.Fatalf("default namespace = %q", cfg.Project.DefaultNamespace)
	}
}
