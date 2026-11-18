package server

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// The native rows establish both object-dependent alias admission and
// rejection controls for the Metadata API deployment route.
func TestListViewColumnNativeAdmission(t *testing.T) {
	fixture := filepath.Join("testdata", "pr163_list_shapes")
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 25 {
		t.Fatalf("native rows = %d, error %v", len(rows), err)
	}
	viewSource, err := os.ReadFile(filepath.Join(fixture, "probe.listView-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	customSource, err := os.ReadFile(filepath.Join(fixture, "ProbeItem__c.object-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		if len(row) != 6 || row[3] != "deploy" || (row[4] != "accepted" && row[4] != "rejected") {
			t.Fatalf("invalid native row: %v", row)
		}
		for _, withDefinition := range []bool{true, false} {
			// A custom object absent from a source project is not established
			// by these rows as a known object in the consuming project.
			if !withDefinition && strings.HasSuffix(row[0], "__c") {
				continue
			}
			shape := "standard-source-omitted"
			if withDefinition {
				shape = "source-present"
			}
			t.Run(row[0]+"/"+row[1]+"/"+row[2]+"/"+shape, func(t *testing.T) {
				root := t.TempDir()
				objectDir := filepath.Join(root, "force-app", "main", "default", "objects", row[0])
				viewPath := filepath.Join(objectDir, "listViews", "Probe.listView-meta.xml")
				view := strings.Replace(string(viewSource), "<columns>CREATED_DATE</columns>", "<columns>"+row[1]+"</columns>", 1)
				if view == string(viewSource) && row[1] != "CREATED_DATE" {
					t.Fatal("native fixture column placeholder not found")
				}
				writeLightningFixtureFile(t, viewPath, view)
				p := project.Project{Root: root, SourceAPIVersion: row[2], ListViewFiles: []string{viewPath}}
				if withDefinition {
					objectPath := filepath.Join(objectDir, row[0]+".object-meta.xml")
					object := `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"/>`
					if strings.HasSuffix(row[0], "__c") {
						object = string(customSource)
					}
					writeLightningFixtureFile(t, objectPath, object)
					p.ObjectFiles = []string{objectPath}
				}
				_, gotErr := NewSourceMetadataFromProject(p)
				if row[4] == "accepted" && gotErr != nil {
					t.Fatalf("native accepted, Glade rejected: %v", gotErr)
				}
				if row[4] == "rejected" && gotErr == nil {
					t.Fatalf("native rejected (%s), Glade accepted", row[5])
				}
			})
		}
	}
}
