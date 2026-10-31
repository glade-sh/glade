package schema

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestSnapshotExplicitCompleteness(t *testing.T) {
	cases := []struct {
		name         string
		marker       any
		legacyMarker bool
		partial      bool
		want         bool
	}{
		{name: "legacy"}, {name: "old-inferred-marker", legacyMarker: true}, {name: "false", marker: false}, {name: "true", marker: true, want: true}, {name: "partial-overrides-true", marker: true, partial: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			object := map[string]any{"name": "Account"}
			if tc.marker != nil {
				object["fieldsComplete"] = tc.marker
			}
			if tc.legacyMarker {
				object["completeSnapshot"] = true
			}
			if tc.partial {
				object["partial"] = true
			}
			raw, err := json.Marshal(map[string]any{"objects": []any{object}})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "snapshot.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(raw)
			got, err := LoadProject(project.Project{SchemaSnapshot: path, SchemaSnapshotSHA256: fmt.Sprintf("%x", sum)})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Objects) != 1 || got.Objects[0].CompleteSnapshot != tc.want {
				t.Fatalf("want complete=%t, got %#v", tc.want, got.Objects)
			}
		})
	}
}
