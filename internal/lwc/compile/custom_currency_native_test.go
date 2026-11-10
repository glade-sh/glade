package compile

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// The custom-object currency import follows the captured feature shape.
func TestCustomCurrencyImportNativeDeploy(t *testing.T) {
	const fixture = "testdata/pr163_custom_currency"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 9 {
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
			if row[2] != "deploy" || (!strings.HasPrefix(row[0], "single_") && !strings.HasPrefix(row[0], "multi_")) {
				t.Fatalf("invalid native row: %v", row)
			}
			root := t.TempDir()
			bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "probe")
			writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
			features := `[]`
			if strings.HasPrefix(row[0], "multi_") {
				features = `["MultiCurrency"]`
			}
			writeCompileFixtureFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer","features":`+features+`}`)
			object, err := os.ReadFile(filepath.Join(fixture, "Shape__c.object-meta.xml"))
			if err != nil {
				t.Fatal(err)
			}
			writeCompileFixtureFile(t, filepath.Join(root, "force-app", "main", "default", "objects", "Shape__c", "Shape__c.object-meta.xml"), string(object))
			source := string(js)
			if strings.HasSuffix(row[0], "_invalid") {
				source = strings.ReplaceAll(source, "CurrencyIsoCode", "NotARealField__c")
			}
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js"), source)
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.html"), string(html))
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
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
				if err == nil || !strings.Contains(err.Error(), row[4]) {
					t.Fatalf("native rejected with %s, Glade error: %v", row[4], err)
				}
			default:
				t.Fatalf("invalid native result: %v", row)
			}
		})
	}
}
