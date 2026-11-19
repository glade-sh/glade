package project

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Source deployment ignores JavaScript files directly beneath lwc while
// retaining JavaScript contained in a component bundle.
func TestDiscoverLooseLWCNativeDeploy(t *testing.T) {
	const fixture = "testdata/pr163_loose_lwc"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows: got %d, err %v", len(rows), err)
	}
	invalid, err := os.ReadFile(filepath.Join(fixture, "invalid.js"))
	if err != nil {
		t.Fatal(err)
	}
	valid, err := os.ReadFile(filepath.Join(fixture, "probe.js"))
	if err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(filepath.Join(fixture, "probe.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"_"+row[1], func(t *testing.T) {
			if row[2] != "deploy" || (row[0] != "loose" && row[0] != "bundle_invalid") {
				t.Fatalf("invalid row: %v", row)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
			lwc := filepath.Join(root, "force-app", "main", "default", "lwc")
			writeFile(t, filepath.Join(lwc, "invalid.js"), string(invalid))
			writeFile(t, filepath.Join(lwc, "helper.js"), string(valid))
			js := valid
			if row[0] == "bundle_invalid" {
				js = invalid
			}
			bundle := filepath.Join(lwc, "probe")
			writeFile(t, filepath.Join(bundle, "probe.js"), string(js))
			writeFile(t, filepath.Join(bundle, "probe.html"), string(html))
			writeFile(t, filepath.Join(bundle, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
			p, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.LWCFiles) != 1 || !strings.HasSuffix(filepath.ToSlash(p.LWCFiles[0]), "/lwc/probe/probe.js") {
				t.Fatalf("LWCFiles = %#v; want only bundle JavaScript", p.LWCFiles)
			}
			if len(p.LWCHTMLFiles) != 1 || len(p.LWCMetaFiles) != 1 {
				t.Fatalf("bundle companions lost: html=%#v meta=%#v", p.LWCHTMLFiles, p.LWCMetaFiles)
			}
		})
	}
}
