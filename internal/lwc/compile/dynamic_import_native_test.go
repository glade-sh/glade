package compile

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// TestDynamicImportNativeDeploy follows the deployment outcomes recorded for
// a variable import, with and without the dynamic-component capability.
func TestDynamicImportNativeDeploy(t *testing.T) {
	const fixture = "testdata/pr163_dynamic"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows: got %d rows, err %v", len(rows), err)
	}
	js, err := os.ReadFile(filepath.Join(fixture, "probe.js"))
	if err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(filepath.Join(fixture, "probe.html"))
	if err != nil {
		t.Fatal(err)
	}
	repo, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"_"+row[1], func(t *testing.T) {
			if row[2] != "deploy" || (row[0] != "dynamic_cap" && row[0] != "dynamic_no_cap") {
				t.Fatalf("invalid native row: %v", row)
			}
			root := t.TempDir()
			bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "probe")
			writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js"), string(js))
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.html"), string(html))
			capabilities := ""
			if row[0] == "dynamic_cap" {
				capabilities = `<capabilities><capability>lightning__dynamicComponent</capability></capabilities>`
			}
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><isExposed>false</isExposed>`+capabilities+`</LightningComponentBundle>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Compile(p, Options{OutDir: filepath.Join(root, "dist"), Namespace: "c", RepoRoot: repo})
			switch row[3] {
			case "accepted":
				if err != nil {
					t.Fatalf("native accepted, Glade rejected: %v", err)
				}
			case "rejected":
				if err == nil || !strings.Contains(err.Error(), "LWC1503") {
					t.Fatalf("native rejected with %s, Glade error: %v", row[4], err)
				}
			default:
				t.Fatalf("invalid native result: %v", row)
			}
		})
	}
}
