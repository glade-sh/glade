package compile

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestDynamicImportModuleNativeDeploy(t *testing.T) {
	const fixture = "testdata/pr163_dynamic_modules"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 9 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	module, probe, html := read("module.js"), read("probe.js"), read("probe.html")
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"_"+row[1], func(t *testing.T) {
			root := t.TempDir()
			bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "probe")
			writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
			caps := ""
			if strings.HasSuffix(row[0], "_cap") {
				caps = `<capabilities><capability>lightning__dynamicComponent</capability></capabilities>`
			}
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><isExposed>false</isExposed>`+caps+`</LightningComponentBundle>`)
			if strings.HasPrefix(row[0], "utility_") {
				writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js"), module)
			} else if strings.HasPrefix(row[0], "sibling_") {
				writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js"), probe)
				writeCompileFixtureFile(t, filepath.Join(bundle, "probe.html"), html)
				writeCompileFixtureFile(t, filepath.Join(bundle, "helper.js"), module)
			} else {
				t.Fatalf("invalid case: %v", row)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Compile(p, Options{OutDir: filepath.Join(root, "dist"), Namespace: "c", RepoRoot: repo})
			if row[3] == "accepted" && err != nil {
				t.Fatalf("native accepted; error %v", err)
			}
			if row[3] == "rejected" && (err == nil || !strings.Contains(err.Error(), "LWC1503")) {
				t.Fatalf("native rejected with %s, error %v", row[4], err)
			}
		})
	}
}
