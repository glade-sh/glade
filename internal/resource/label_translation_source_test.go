package resource

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Resource consumers keep the established normalized Value; exact source text
// survives separately for an explicitly scoped runtime lookup.
func TestLabelTranslationRawSourcePreservesResourceLookup(t *testing.T) {
	p, err := project.Load(filepath.Join("..", "apextest", "testdata", "conformance", "label_translation_space_values", "project"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProjectWithDependencies(p)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.MetadataRegistry
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		registry storage.MetadataRegistry
	}{{"source", loaded}, {"restored", restored}} {
		t.Run(tc.name, func(t *testing.T) {
			for _, label := range tc.registry.Labels {
				if label.Name != "ProbePaddedText" || label.Language != "fr" {
					continue
				}
				isolated := tc.registry
				isolated.Labels = []storage.LabelMetadata{label}
				value, status := ResolveLabel(isolated, "", "", "ProbePaddedText")
				if status != LabelLookupResolved || value != "text" {
					t.Fatalf("existing resource value=%q status=%s", value, status)
				}
				encoded, err := json.Marshal(label)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				var raw string
				if err := json.Unmarshal(fields["translationSourceRaw"], &raw); err != nil {
					t.Fatal(err)
				}
				if raw != " text " {
					t.Fatalf("source text=%q", raw)
				}
				return
			}
			t.Fatal("source translation not found")
		})
	}
}
