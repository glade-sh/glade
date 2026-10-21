package visualforce

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Native API 59/67 deployments accepted the asset expression in this project.
// Rendering the hosted asset remains separate from deployment and registration.
func TestAssetPageRegistrationNativeConformance(t *testing.T) {
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			fixture := filepath.Join("..", "apextest", "testdata", "conformance", "visualforce_asset_pages", "project")
			if err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := LoadProject(p); err != nil {
				t.Fatalf("native deployment accepted the asset expression: %v", err)
			}
			want := []string{"MeadowAssetPage", "MeadowPlainPage"}
			if got := PageNames(p); !slices.Equal(got, want) {
				t.Fatalf("page names = %v, want %v", got, want)
			}
		})
	}
}

func TestAssetPageRegistrationRetainsNativeRejections(t *testing.T) {
	family := filepath.Join("..", "apextest", "testdata", "conformance", "visualforce_asset_pages")
	file, err := os.Open(filepath.Join(family, "asset-controls.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("native control rows = %d, want 4 plus header", len(rows))
	}
	for _, row := range rows[1:] {
		if len(row) != 4 || row[2] != "false" {
			t.Fatalf("invalid native rejection row: %v", row)
		}
		api, name := row[0], row[1]
		t.Run(api+"/"+name, func(t *testing.T) {
			root := t.TempDir()
			for _, source := range []string{filepath.Join(family, "project"), filepath.Join(family, "negative", name)} {
				if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					relative, err := filepath.Rel(source, path)
					if err != nil {
						return err
					}
					writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "62.0", api))
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			// Native deployment validates the negative page independently. Remove
			// the positive asset expression so its local failure cannot mask this
			// control before asset support is present.
			writeFile(t, filepath.Join(root, "force-app", "main", "default", "components", "MeadowAssetPanel.component"), `<apex:component controller="AssetPageController"><apex:outputText value="{!caption}"/></apex:component>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := LoadProject(p); err == nil {
				t.Fatalf("native deployment rejected %s; local loader accepted it", name)
			}
		})
	}
}
