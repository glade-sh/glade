package compile

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestNamespacedSchemaImportNativeDeploy(t *testing.T) {
	const fixture = "testdata/pr163_namespaced_schema"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	csvReader := csv.NewReader(f)
	csvReader.Comma = '\t'
	rows, err := csvReader.ReadAll()
	if err != nil || len(rows) != 21 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	object, source, html := read("object.xml"), read("probe.js"), read("probe.html")
	repo, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"_"+row[1], func(t *testing.T) {
			if len(row) != 6 || row[2] != "check-only deploy" {
				t.Fatalf("invalid native row: %v", row)
			}
			root := t.TempDir()
			writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"fixtureown","sourceApiVersion":"`+row[1]+`"}`)
			base := filepath.Join(root, "force-app", "main", "default")
			writeCompileFixtureFile(t, filepath.Join(base, "objects", "CaptureNsK471__c", "CaptureNsK471__c.object-meta.xml"), object)
			bundle := filepath.Join(base, "lwc", "nscaptureprobe")
			expr := strings.ReplaceAll(row[4], "{namespace}", "fixtureown")
			writeCompileFixtureFile(t, filepath.Join(bundle, "nscaptureprobe.js"), strings.ReplaceAll(source, "{expression}", expr))
			writeCompileFixtureFile(t, filepath.Join(bundle, "nscaptureprobe.html"), html)
			writeCompileFixtureFile(t, filepath.Join(bundle, "nscaptureprobe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Compile(p, Options{OutDir: filepath.Join(root, "dist"), RepoRoot: repo})
			switch row[3] {
			case "accepted":
				if err != nil {
					t.Fatalf("native accepted, Glade rejected: %v", err)
				}
			case "rejected":
				if err == nil || !strings.Contains(err.Error(), row[5]) {
					t.Fatalf("native rejected with %s; got %v", row[5], err)
				}
			default:
				t.Fatalf("invalid native result: %v", row)
			}
		})
	}
}
