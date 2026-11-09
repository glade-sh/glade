package visualforce

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Native profile_access_local org.tsv (59.0 and 67.0): profile
// classAccesses and pageAccesses entries are validated like permission set
// entries, including a missing name on a disabled entry and a non-boolean
// enabled value.
func TestProfileAccessEntriesNativeConformance(t *testing.T) {
	fixture := filepath.Join("testdata", "profile_access_local")
	read := func(name string) [][]string {
		t.Helper()
		f, err := os.Open(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		r.FieldsPerRecord = -1
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows[1:]
	}
	cases := map[string][]string{}
	for _, row := range read("cases.tsv") {
		cases[row[0]] = row
	}
	rows := read("org.tsv")
	if len(rows) != 12 || len(cases) != 6 {
		t.Fatalf("native rows=%d cases=%d", len(rows), len(cases))
	}
	for _, row := range rows {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			c := cases[row[1]]
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			entries := ""
			if c[1] != "" {
				entries += fmt.Sprintf("<classAccesses><apexClass>%s</apexClass><enabled>%s</enabled></classAccesses>", c[1], c[3])
			}
			if c[2] != "" {
				entries += fmt.Sprintf("<pageAccesses><apexPage>%s</apexPage><enabled>%s</enabled></pageAccesses>", c[2], c[3])
			}
			path := filepath.Join(root, "src/profiles/Standard.profile-meta.xml")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(`<Profile xmlns="http://soap.sforce.com/2006/04/metadata">`+entries+`</Profile>`), 0o644); err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.ProfileFiles) != 1 {
				t.Fatalf("profile files = %v", p.ProfileFiles)
			}
			err = ValidateVisualforceAccessMetadata(p)
			if row[2] == "DEPLOY_OK" {
				if err != nil {
					t.Fatalf("native accepted access: %v", err)
				}
				return
			}
			var access *VisualforceAccessMetadataError
			if !errors.As(err, &access) || access.Message != row[3] || access.File != path {
				t.Fatalf("access error=%v, want %s at %s", err, row[3], path)
			}
			if _, err := LoadProject(p); err == nil {
				t.Fatal("strict loader accepted a rejected profile entry")
			}
		})
	}
}
