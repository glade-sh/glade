package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEffectiveOrgShapeReadsDependencyEvidence(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/person_account_evidence/dependency_layouts.json")
	if err != nil {
		t.Fatal(err)
	}
	var layouts []struct {
		Name    string
		Enabled bool
		Files   map[string]string
	}
	if err := json.Unmarshal(raw, &layouts); err != nil {
		t.Fatal(err)
	}
	for _, layout := range layouts {
		t.Run(layout.Name, func(t *testing.T) {
			root := t.TempDir()
			for path, source := range layout.Files {
				writeFile(t, filepath.Join(root, filepath.FromSlash(path)), strings.ReplaceAll(source, "{{API}}", "67.0"))
			}
			consumer := filepath.Join(root, "consumer")
			p, err := Load(consumer)
			if err != nil {
				t.Fatal(err)
			}
			features := EffectiveOrgShapeFeatures(p)
			if slices.Contains(features, "PersonAccounts") != layout.Enabled {
				t.Fatalf("features=%v, want PersonAccounts=%t", features, layout.Enabled)
			}
			// The consumer root's own shape stays first and unchanged.
			if own := OrgShapeFeatures(p.Root); !slices.Equal(features[:len(own)], own) {
				t.Fatalf("effective features %v do not start with root features %v", features, own)
			}
			inputs := EffectiveOrgShapeInputFiles(p)
			for path := range layout.Files {
				evidence := strings.Contains(path, "/recordTypes/PersonAccount.") || strings.HasSuffix(path, "project-scratch-def.json")
				if !evidence || strings.HasPrefix(path, "other/") {
					continue
				}
				if want := filepath.Join(root, filepath.FromSlash(path)); !slices.Contains(inputs, want) {
					t.Fatalf("evidence %s missing from effective shape inputs %v", path, inputs)
				}
			}
		})
	}
}
