package gladecli

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Native visibility oracle: no_rows_visibility/org.tsv:7,14,22,23.
func TestOwnerSharingDBConfigTransitions(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "no_rows_visibility", "project")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				writeTestFile(t, filepath.Join(root, rel), string(data))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			rules := filepath.Join(root, "force-app/main/default/sharingRules/PrivateProbe__c.sharingRules-meta.xml")
			contents, err := os.ReadFile(rules)
			if err != nil {
				t.Fatal(err)
			}
			db := filepath.Join(root, "state.sqlite")
			for _, want := range []int{2, 0, 2} {
				if want == 0 {
					if err := os.Remove(rules); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.WriteFile(rules, contents, 0600); err != nil {
						t.Fatal(err)
					}
				}
				store, org, err := openDBStore(db, root)
				if err != nil {
					t.Fatal(err)
				}
				got := len(org.Objects["PrivateProbe__c"].Definition.OwnerSharingRules)
				if err := store.Save(org); err != nil {
					t.Fatal(err)
				}
				if err := store.Close(); err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("rules=%d want=%d", got, want)
				}
			}
		})
	}
}
