package server

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestListViewColumnAdditionalCustomShape(t *testing.T) {
	dir := filepath.Join("testdata", "pr163_list_custom_shapes")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 9 {
		t.Fatalf("capture rows: %d, %v", len(rows), err)
	}
	object, err := os.ReadFile(filepath.Join(dir, "SampleEntry__c.object-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	view, err := os.ReadFile(filepath.Join(dir, "probe.listView-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[1]+"/"+row[2], func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "objects", row[0])
			objPath := filepath.Join(base, row[0]+".object-meta.xml")
			viewPath := filepath.Join(base, "listViews", "Probe.listView-meta.xml")
			writeLightningFixtureFile(t, objPath, string(object))
			writeLightningFixtureFile(t, viewPath, strings.Replace(string(view), "CREATED_DATE", row[1], 1))
			_, err := NewSourceMetadataFromProject(project.Project{Root: root, ObjectFiles: []string{objPath}, ListViewFiles: []string{viewPath}, SourceAPIVersion: row[2]})
			if (err == nil) != (row[4] == "accepted") {
				t.Fatalf("native %s, Glade error: %v", row[4], err)
			}
		})
	}
}

func TestListViewColumnCatalogFallback(t *testing.T) {
	for _, tc := range []struct {
		api, column           string
		definition, wantError bool
	}{
		{"66.0", "CREATED_DATE", true, true},
		{"67.0", "UNOBSERVED_COLUMN", false, false},
		{"67.0", "ACCOUNT.UNKNOWN", true, false},
	} {
		t.Run(tc.api+"/"+tc.column, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "objects", "Account")
			view := filepath.Join(base, "listViews", "Probe.listView-meta.xml")
			writeLightningFixtureFile(t, view, `<ListView xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Probe</fullName><columns>`+tc.column+`</columns><filterScope>Everything</filterScope><label>Probe</label></ListView>`)
			p := project.Project{Root: root, ListViewFiles: []string{view}, SourceAPIVersion: tc.api}
			if tc.definition {
				obj := filepath.Join(base, "Account.object-meta.xml")
				writeLightningFixtureFile(t, obj, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"/>`)
				p.ObjectFiles = []string{obj}
			}
			_, err := NewSourceMetadataFromProject(p)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v; want error %v", err, tc.wantError)
			}
		})
	}
}

func TestListViewColumnAccountNativeAdmission(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "pr163_list_columns", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[2] != "deploy" || strings.Contains(row[0], ".") {
			continue
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "objects", "Account")
			view := filepath.Join(base, "listViews", "Probe.listView-meta.xml")
			writeLightningFixtureFile(t, view, `<ListView xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Probe</fullName><columns>`+row[0]+`</columns><filterScope>Everything</filterScope><label>Probe</label></ListView>`)
			_, err := NewSourceMetadataFromProject(project.Project{Root: root, ListViewFiles: []string{view}, SourceAPIVersion: row[1]})
			if (err == nil) != (row[3] == "accepted") {
				t.Fatalf("native %s, Glade error: %v", row[3], err)
			}
		})
	}
}

func TestListViewColumnUncapturedCustomShapeFallback(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "objects", "Different__c")
	object := filepath.Join(base, "Different__c.object-meta.xml")
	view := filepath.Join(base, "listViews", "Probe.listView-meta.xml")
	writeLightningFixtureFile(t, object, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Different</label><pluralLabel>Differents</pluralLabel><nameField><label>Number</label><type>AutoNumber</type><displayFormat>D-{0000}</displayFormat></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeLightningFixtureFile(t, view, `<ListView xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Probe</fullName><columns>CREATED_DATE</columns><filterScope>Everything</filterScope><label>Probe</label></ListView>`)
	_, err := NewSourceMetadataFromProject(project.Project{Root: root, ObjectFiles: []string{object}, ListViewFiles: []string{view}, SourceAPIVersion: "67.0"})
	if err == nil {
		t.Fatal("uncaptured auto-number shape must retain existing field lookup")
	}
}
