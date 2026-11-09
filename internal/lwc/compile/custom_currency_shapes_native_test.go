package compile

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestCustomCurrencyShapeNativeDeploy(t *testing.T) {
	const fixture = "testdata/pr163_custom_currency_shapes"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 17 {
		t.Fatalf("native rows %d: %v", len(rows), err)
	}
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"_"+row[1], func(t *testing.T) {
			object, field, ok := strings.Cut(row[0], ".")
			if !ok {
				t.Fatalf("invalid row: %v", row)
			}
			root := t.TempDir()
			writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[1]+`"}`)
			writeCompileFixtureFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer","features":["MultiCurrency"]}`)
			body, err := os.ReadFile(filepath.Join(fixture, object+".object-meta.xml"))
			if err != nil {
				t.Fatal(err)
			}
			writeCompileFixtureFile(t, filepath.Join(root, "force-app", "main", "default", "objects", object, object+".object-meta.xml"), string(body))
			bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "probe")
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js"), `import { LightningElement } from 'lwc'; import FIELD from '@salesforce/schema/`+object+`.`+field+`'; export default class Probe extends LightningElement { field = FIELD; }`)
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.html"), `<template><p>Test</p></template>`)
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Compile(p, Options{OutDir: filepath.Join(root, "dist"), Namespace: "c", RepoRoot: repo})
			if row[3] == "accepted" && err != nil {
				t.Fatalf("native accepted; error %v", err)
			}
			if row[3] == "rejected" && (err == nil || !strings.Contains(err.Error(), row[4])) {
				t.Fatalf("native rejected with %s, error %v", row[4], err)
			}
		})
	}
}
