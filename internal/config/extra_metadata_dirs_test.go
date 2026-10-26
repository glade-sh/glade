package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtraMetadataDirsConfigLoadAndRejectRoutes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "glade.yml")
	if err := os.WriteFile(path, []byte("project:\n  extraMetadataDirs: [supplemental, ../shared]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Project struct {
			ExtraMetadataDirs []string `json:"extraMetadataDirs"`
		}
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "supplemental"), filepath.Join(root, "../shared")}
	if strings.Join(got.Project.ExtraMetadataDirs, "|") != strings.Join(want, "|") {
		t.Fatalf("resolved extra metadata dirs: %s", data)
	}
	for _, source := range []string{"project:\n  extraMetadataDirs: supplemental\n", "project:\n  extraMetadataDirs: [\"\"]\n"} {
		if _, err := parseYAMLSubset(source); err == nil {
			t.Fatalf("invalid extra metadata dirs accepted: %q", source)
		}
	}
}
