package schema

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestFieldSetSnapshotAuthorityAndSourceOverlay(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		complete  bool
	}{
		{"complete", `{"objects":[{"name":"Account","fieldSetsComplete":true,"fieldSetNames":["Existing"]}]}`, true},
		{"incomplete", `{"objects":[{"name":"Account","fieldSetNames":["Existing"]}]}`, false},
		{"partial", `{"objects":[{"name":"Account","partial":true,"fieldSetsComplete":true,"fieldSetNames":["Existing"]}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			snapshot := filepath.Join(root, "snapshot.json")
			if err := os.WriteFile(snapshot, []byte(tc.raw), 0600); err != nil {
				t.Fatal(err)
			}
			setfile := filepath.Join(root, "objects/Account/fieldSets/Added.fieldSet-meta.xml")
			if err := os.MkdirAll(filepath.Dir(setfile), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(setfile, []byte(`<FieldSet/>`), 0600); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256([]byte(tc.raw))
			got, err := LoadProject(project.Project{SchemaSnapshot: snapshot, SchemaSnapshotSHA256: fmt.Sprintf("%x", sum), FieldSetFiles: []string{setfile}})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Objects) != 1 || got.Objects[0].FieldSetsComplete != tc.complete {
				t.Fatalf("wrong authority: %#v", got.Objects)
			}
			if tc.complete && (!stringSliceContainsFold(got.Objects[0].FieldSetNames, "Existing") || !stringSliceContainsFold(got.Objects[0].FieldSetNames, "Added")) {
				t.Fatalf("missing snapshot or overlay: %#v", got.Objects[0].FieldSetNames)
			}
		})
	}
}
