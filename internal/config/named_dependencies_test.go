package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseYAMLSubsetNamedDependencies(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "nested", "glade.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`
project:
  dependencies:
    - name: SampleAlias
      path: ../vendor/library
      package: LibrarySource
      version: "1.2.3"
    - name: SecondAlias
      path: /opt/local-library
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("load named dependencies: %v", err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Project struct {
			Dependencies []struct {
				Name    string `json:"name"`
				Path    string `json:"path"`
				Package string `json:"package"`
				Version string `json:"version"`
			} `json:"dependencies"`
		} `json:"project"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded.Project.Dependencies; len(got) != 2 {
		t.Fatalf("dependencies = %#v, want two entries", got)
	} else {
		wantPath := filepath.Clean(filepath.Join(root, "vendor", "library"))
		if got[0].Name != "SampleAlias" || got[0].Path != wantPath || got[0].Package != "LibrarySource" || got[0].Version != "1.2.3" {
			t.Fatalf("first dependency = %#v", got[0])
		}
		if got[1].Name != "SecondAlias" || got[1].Path != "/opt/local-library" || got[1].Package != "" || got[1].Version != "" {
			t.Fatalf("second dependency = %#v", got[1])
		}
	}
}

func TestParseYAMLSubsetRejectsDuplicateNamedDependency(t *testing.T) {
	_, err := parseYAMLSubset(`
project:
  dependencies:
    - name: SampleAlias
      path: ../first
    - name: samplealias
      path: ../second
`)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "duplicate") || !strings.Contains(err.Error(), "../first") || !strings.Contains(err.Error(), "../second") {
		t.Fatalf("error = %v, want duplicate dependency name diagnostic", err)
	}
}

func TestParseYAMLSubsetAcceptsEmptyNamedDependencies(t *testing.T) {
	cfg, err := parseYAMLSubset("project:\n  dependencies: []\n")
	if err != nil {
		t.Fatalf("parse empty dependencies: %v", err)
	}
	if len(cfg.Project.Dependencies) != 0 {
		t.Fatalf("dependencies = %#v, want none", cfg.Project.Dependencies)
	}
}
